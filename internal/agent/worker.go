package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/artifactsx"
)

// Worker обрабатывает задания из очереди
type Worker struct {
	id     int
	db     *sql.DB
	docker *DockerClient
	config Config
	stopCh chan struct{}
	wg     *sync.WaitGroup
}

// NewWorker создаёт нового воркера
func NewWorker(id int, db *sql.DB, docker *DockerClient, config Config, stopCh chan struct{}, wg *sync.WaitGroup) *Worker {
	return &Worker{
		id:     id,
		db:     db,
		docker: docker,
		config: config,
		stopCh: stopCh,
		wg:     wg,
	}
}

// Start запускает воркера
func (w *Worker) Start() {
	w.wg.Add(1)
	go w.run()
}

func (w *Worker) run() {
	defer w.wg.Done()

	log.Printf("[Worker %d] Запущен", w.id)

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			log.Printf("[Worker %d] Остановка", w.id)
			return
		case <-ticker.C:
			if err := w.processNextJob(); err != nil {
				log.Printf("[Worker %d] Ошибка обработки задания: %v", w.id, err)
			}
		}
	}
}

// Job представляет задание из очереди
type Job struct {
	ID          string
	RunID       string
	Status      string
	Attempts    int
	MaxAttempts int
}

// Run представляет прогон
type Run struct {
	ID         string
	ClientID   string
	UserID     string
	ArtifactID string
	Status     string
	OS         string
	Display    string
	QtVersion  string
}

// processNextJob забирает и обрабатывает следующее задание
func (w *Worker) processNextJob() error {
	ctx := context.Background()

	// Начинаем транзакцию
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Забираем задание с lease (FOR UPDATE SKIP LOCKED)
	var job Job
	var run Run
	var artifactPath string
	var manifestJSON []byte
	var networkAllowed bool

	err = tx.QueryRowContext(ctx, `
		SELECT 
			j.id, j.run_id, j.status, j.attempts, j.max_attempts,
			r.id, r.client_id, r.user_id, r.artifact_id, r.status, r.os, r.display, r.qt_version,
			a.stored_path, a.manifest,
			c.network_allowed
		FROM jobs j
		JOIN runs r ON r.id = j.run_id
		JOIN artifacts a ON a.id = r.artifact_id
		JOIN clients c ON c.id = r.client_id
		WHERE j.status = 'queued'
		  AND j.available_at <= now()
		  AND (j.lease_until IS NULL OR j.lease_until < now())
		ORDER BY j.available_at ASC
		FOR UPDATE OF j SKIP LOCKED
		LIMIT 1
	`).Scan(
		&job.ID, &job.RunID, &job.Status, &job.Attempts, &job.MaxAttempts,
		&run.ID, &run.ClientID, &run.UserID, &run.ArtifactID, &run.Status, &run.OS, &run.Display, &run.QtVersion,
		&artifactPath, &manifestJSON,
		&networkAllowed,
	)

	if err == sql.ErrNoRows {
		// Нет заданий в очереди
		return nil
	}
	if err != nil {
		return fmt.Errorf("не удалось получить задание: %w", err)
	}

	// Устанавливаем lease
	leaseDuration := w.config.MaxTimeout + 30*time.Second
	leaseUntil := time.Now().Add(leaseDuration)

	_, err = tx.ExecContext(ctx, `
		UPDATE jobs 
		SET status = 'running',
			lease_until = $1,
			leased_by = $2,
			attempts = attempts + 1,
			updated_at = now()
		WHERE id = $3
	`, leaseUntil, fmt.Sprintf("worker-%d", w.id), job.ID)
	if err != nil {
		return fmt.Errorf("не удалось обновить lease: %w", err)
	}

	// Обновляем статус прогона
	_, err = tx.ExecContext(ctx, `
		UPDATE runs 
		SET status = 'running',
			started_at = now()
		WHERE id = $1
	`, run.ID)
	if err != nil {
		return fmt.Errorf("не удалось обновить статус прогона: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}

	log.Printf("[Worker %d] Взял задание %s (run: %s)", w.id, job.ID, run.ID)

	// Выполняем задание
	w.executeJob(ctx, job, run, artifactPath, manifestJSON, networkAllowed)

	return nil
}

// executeJob выполняет задание
func (w *Worker) executeJob(ctx context.Context, job Job, run Run, artifactPath string, manifestJSON []byte, networkAllowed bool) {
	// Парсим манифест
	var manifest artifactsx.Manifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		w.failJob(ctx, job, run, "MANIFEST_PARSE_ERROR", fmt.Sprintf("не удалось распарсить манифест: %v", err))
		return
	}

	// Получаем имя образа
	image, err := w.config.GetImageName(run.OS, run.QtVersion, run.Display)
	if err != nil {
		w.failJob(ctx, job, run, "IMAGE_NOT_FOUND", err.Error())
		return
	}

	// Создаём директорию для артефактов прогона
	artifactsDir := filepath.Join(w.config.DataDir, "runs", run.ID, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0750); err != nil {
		w.failJob(ctx, job, run, "ARTIFACTS_DIR_ERROR", fmt.Sprintf("не удалось создать директорию: %v", err))
		return
	}

	// Запускаем контейнер
	params := RunParams{
		RunID:          run.ID,
		ArtifactPath:   artifactPath,
		ArtifactsDir:   artifactsDir,
		Image:          image,
		TimeoutSec:     manifest.TimeoutSec,
		NetworkAllowed: networkAllowed,
		Env:            manifest.Env,
	}

	log.Printf("[Worker %d] Запускаю контейнер для run %s (image: %s)", w.id, run.ID, image)

	result := w.docker.RunContainer(ctx, params)

	// Обрабатываем результат
	logPath := filepath.Join(artifactsDir, "run.log")

	if result.Error != nil {
		w.failJob(ctx, job, run, "RUNNER_ERROR", result.Error.Error())
		return
	}

	if result.Timeout {
		w.finishRun(ctx, job, run, "timeout", -1, logPath)
		return
	}

	// Определяем статус по exit code
	status := "passed"
	if result.ExitCode != 0 {
		status = "failed"
	}

	w.finishRun(ctx, job, run, status, int(result.ExitCode), logPath)
}

// failJob помечает задание как проваленное
func (w *Worker) failJob(ctx context.Context, job Job, run Run, errorCode, errorMsg string) {
	log.Printf("[Worker %d] Задание %s провалено: %s - %s", w.id, job.ID, errorCode, errorMsg)

	// Проверяем, можно ли повторить
	if job.Attempts < job.MaxAttempts {
		// Возвращаем в очередь с задержкой
		retryDelay := time.Duration(job.Attempts*30) * time.Second
		_, _ = w.db.ExecContext(ctx, `
			UPDATE jobs 
			SET status = 'queued',
				lease_until = NULL,
				leased_by = NULL,
				available_at = now() + $1::interval,
				last_error = $2,
				updated_at = now()
			WHERE id = $3
		`, retryDelay.String(), fmt.Sprintf("%s: %s", errorCode, errorMsg), job.ID)

		_, _ = w.db.ExecContext(ctx, `
			UPDATE runs 
			SET status = 'pending'
			WHERE id = $1
		`, run.ID)
	} else {
		// Финальный провал
		_, _ = w.db.ExecContext(ctx, `
			UPDATE jobs 
			SET status = 'failed',
				last_error = $1,
				updated_at = now()
			WHERE id = $2
		`, fmt.Sprintf("%s: %s", errorCode, errorMsg), job.ID)

		_, _ = w.db.ExecContext(ctx, `
			UPDATE runs 
			SET status = 'error',
				finished_at = now()
			WHERE id = $1
		`, run.ID)
	}
}

// finishRun завершает прогон успешно или с провалом
func (w *Worker) finishRun(ctx context.Context, job Job, run Run, status string, exitCode int, logPath string) {
	log.Printf("[Worker %d] Прогон %s завершён: status=%s, exit_code=%d", w.id, run.ID, status, exitCode)

	// Проверяем, существует ли лог-файл
	var logPathValue sql.NullString
	if _, err := os.Stat(logPath); err == nil {
		logPathValue = sql.NullString{String: logPath, Valid: true}
	}

	// Обновляем job
	_, _ = w.db.ExecContext(ctx, `
		UPDATE jobs 
		SET status = 'done',
			updated_at = now()
		WHERE id = $1
	`, job.ID)

	// Обновляем run
	var exitCodeValue sql.NullInt32
	if exitCode >= 0 {
		exitCodeValue = sql.NullInt32{Int32: int32(exitCode), Valid: true}
	}

	_, _ = w.db.ExecContext(ctx, `
		UPDATE runs 
		SET status = $1,
			exit_code = $2,
			finished_at = now(),
			log_path = $3
		WHERE id = $4
	`, status, exitCodeValue, logPathValue, run.ID)
}

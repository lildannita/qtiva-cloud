package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/dbx"
)

// Управляет пулом воркеров
type Agent struct {
	config Config
	db     *sql.DB
	docker *DockerClient
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Создаёт нового агента
func NewAgent(config Config, db *sql.DB) (*Agent, error) {
	docker, err := NewDockerClient(config)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать Docker клиент: %w", err)
	}

	return &Agent{
		config: config,
		db:     db,
		docker: docker,
		stopCh: make(chan struct{}),
	}, nil
}

// Запускает агента
func (a *Agent) Start(ctx context.Context) error {
	log.Printf("Запуск агента с %d воркерами", a.config.Concurrency)

	// Проверяем подключение к Docker
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := a.docker.Ping(pingCtx); err != nil {
		return fmt.Errorf("не удалось подключиться к Docker: %w", err)
	}
	log.Println("Подключение к Docker установлено")

	// Запускаем воркеров
	for i := 0; i < a.config.Concurrency; i++ {
		worker := NewWorker(i, a.db, a.docker, a.config, a.stopCh, &a.wg)
		worker.Start()
	}

	// Запускаем фоновую очистку просроченных lease
	go a.cleanupExpiredLeases(ctx)

	return nil
}

// Останавливает агента
func (a *Agent) Stop() {
	log.Println("Остановка агента...")
	close(a.stopCh)
	a.wg.Wait()

	if a.docker != nil {
		_ = a.docker.Close()
	}

	log.Println("Агент остановлен")
}

// Периодически освобождает просроченные lease
func (a *Agent) cleanupExpiredLeases(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.releaseExpiredLeases(ctx); err != nil {
				log.Printf("Ошибка освобождения просроченных lease: %v", err)
			}
		}
	}
}

// Возвращает в очередь задания с истёкшим lease
func (a *Agent) releaseExpiredLeases(ctx context.Context) error {
	result, err := a.db.ExecContext(ctx, `
		UPDATE jobs 
		SET status = 'queued',
			lease_until = NULL,
			leased_by = NULL,
			updated_at = now()
		WHERE status = 'running'
		  AND lease_until < now()
		  AND attempts < max_attempts
	`)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Printf("Освобождено %d просроченных lease", affected)

		// Также сбрасываем статус runs
		_, _ = a.db.ExecContext(ctx, `
			UPDATE runs r
			SET status = 'pending',
				started_at = NULL
			FROM jobs j
			WHERE j.run_id = r.id
			  AND j.status = 'queued'
			  AND r.status = 'running'
		`)
	}

	return nil
}

// Запускает агента как демон
func RunAgentDaemon(ctx context.Context) error {
	config, err := LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("не удалось загрузить конфигурацию: %w", err)
	}

	db, pingTimeout, err := dbx.OpenFromEnv()
	if err != nil {
		return fmt.Errorf("не удалось подключиться к БД: %w", err)
	}
	defer db.Close()

	if err := dbx.Ping(ctx, db, pingTimeout); err != nil {
		return fmt.Errorf("не удалось проверить подключение к БД: %w", err)
	}

	agent, err := NewAgent(config, db)
	if err != nil {
		return err
	}

	if err := agent.Start(ctx); err != nil {
		return err
	}

	// Ждём сигнала остановки
	<-ctx.Done()

	agent.Stop()
	return nil
}

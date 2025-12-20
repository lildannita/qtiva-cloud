package manager

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/lildannita/qtiva-cloud/internal/idgen"
)

// Содержит зависимости для работы с прогонами
type RunsAPI struct {
	DB *sql.DB
}

// Регистрирует маршруты для работы с прогонами
func RegisterRunRoutes(mux *http.ServeMux, api RunsAPI, authMiddleware func(http.Handler) http.Handler) {
	// POST /runs — создание прогона
	mux.Handle("/runs", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleCreateRun(w, r, api.DB)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// GET /runs/{id} — получение статуса прогона
	// GET /runs/{id}/log — скачивание лога
	mux.Handle("/runs/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем run_id из пути: /runs/run_xxx или /runs/run_xxx/log
		path := strings.TrimPrefix(r.URL.Path, "/runs/")
		parts := strings.Split(path, "/")
		runID := parts[0]

		if runID == "" {
			httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "run_id не указан")
			return
		}

		// Проверяем, запрашивается ли лог
		if len(parts) > 1 && parts[1] == "log" {
			// GET /runs/{id}/log
			if r.Method == http.MethodGet {
				handleGetRunLog(w, r, api.DB, runID)
				return
			}
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// GET /runs/{id} — статус прогона
		switch r.Method {
		case http.MethodGet:
			handleGetRun(w, r, api.DB, runID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
}

// === POST /runs ===

// Описывает входные данные для создания прогона
type runRequest struct {
	ArtifactID string `json:"artifact_id"` // ID загруженного артефакта
	OS         string `json:"os"`          // Целевая ОС (manjaro, ubuntu)
	Display    string `json:"display"`     // Тип дисплея (wayland, x11)
	QtVersion  string `json:"qt_version"`  // Версия Qt (5.15)
}

// Описывает ответ при создании прогона
type runCreateResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

// Создаёт новый прогон и ставит задачу в очередь.
// Сразу устанавливает delete_after для автоочистки через дефолтный TTL.
func handleCreateRun(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var req runRequest
	if err := httpx.DecodeJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Получаем данные пользователя из контекста
	u, ok := httpx.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
		return
	}

	// Валидация обязательных полей
	if req.ArtifactID == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "artifact_id обязателен")
		return
	}

	// Валидация допустимых значений
	if !isValidOS(req.OS) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "os должен быть manjaro или ubuntu")
		return
	}
	if !isValidDisplay(req.Display) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "display должен быть wayland или x11")
		return
	}
	if !isValidQtVersion(req.QtVersion) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "qt_version должен быть 5.15")
		return
	}

	// Проверяем существование артефакта и принадлежность клиенту
	var artifactExists bool
	err := db.QueryRowContext(r.Context(),
		`SELECT EXISTS (
			SELECT 1 FROM artifacts 
			WHERE id = $1 
			  AND client_id = $2 
			  AND deleted_at IS NULL
			  AND consumed_at IS NULL
		)`,
		req.ArtifactID, u.ClientID,
	).Scan(&artifactExists)

	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка проверки артефакта")
		return
	}
	if !artifactExists {
		httpx.WriteError(w, r, http.StatusNotFound, "ARTIFACT_NOT_FOUND", "Артефакт не найден или уже использован")
		return
	}

	// Генерируем ID для прогона
	runID, err := idgen.New("run")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать run_id")
		return
	}

	// Генерируем ID для задачи
	jobID, err := idgen.New("job")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать job_id")
		return
	}

	// Читаем дефолтный TTL для прогонов
	defaultTTL, err := commonx.RequireDuration("QTIVA_RUN_DEFAULT_TTL")
	if err != nil {
		defaultTTL = 3 * time.Hour
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось начать транзакцию")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Создаём прогон
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO runs (id, client_id, user_id, artifact_id, status, ack_mode, os, display, qt_version, delete_after)
		 VALUES ($1, $2, $3, $4, 'pending', 'auto', $5, $6, $7, now() + $8::interval)`,
		runID, u.ClientID, u.UserID, req.ArtifactID, req.OS, req.Display, req.QtVersion, defaultTTL.String(),
	)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать прогон")
		return
	}

	// Создаём задачу в очереди
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO jobs (id, run_id, status, attempts, max_attempts)
		 VALUES ($1, $2, 'queued', 0, 3)`,
		jobID, runID,
	)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать задачу")
		return
	}

	// Помечаем артефакт как использованный
	_, err = tx.ExecContext(r.Context(),
		`UPDATE artifacts SET consumed_at = now() WHERE id = $1`,
		req.ArtifactID,
	)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обновить артефакт")
		return
	}

	// Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось завершить транзакцию")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, runCreateResponse{
		RunID:  runID,
		Status: "pending",
	})
}

// === GET /runs/{id} ===

// Описывает ответ со статусом прогона
type runStatusResponse struct {
	RunID      string     `json:"run_id"`
	Status     string     `json:"status"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Log        *logInfo   `json:"log,omitempty"`
}

// Содержит информацию о доступности лога
type logInfo struct {
	Available   bool       `json:"available"`
	DownloadURL string     `json:"download_url,omitempty"`
	ReceivedAt  *time.Time `json:"received_at,omitempty"`
}

// Возвращает статус прогона
func handleGetRun(w http.ResponseWriter, r *http.Request, db *sql.DB, runID string) {
	// Получаем данные пользователя
	u, ok := httpx.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
		return
	}

	// Запрашиваем данные прогона
	var (
		status     string
		exitCode   sql.NullInt32
		createdAt  time.Time
		startedAt  sql.NullTime
		finishedAt sql.NullTime
		logPath    sql.NullString
		receivedAt sql.NullTime
		clientID   string
	)

	err := db.QueryRowContext(r.Context(),
		`SELECT status, exit_code, created_at, started_at, finished_at, 
		        log_path, received_at, client_id
		 FROM runs 
		 WHERE id = $1 AND deleted_at IS NULL`,
		runID,
	).Scan(&status, &exitCode, &createdAt, &startedAt, &finishedAt,
		&logPath, &receivedAt, &clientID)

	if err == sql.ErrNoRows {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Прогон не найден")
		return
	}
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка получения прогона")
		return
	}

	// Проверяем принадлежность прогона клиенту пользователя
	if clientID != u.ClientID {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Нет доступа к этому прогону")
		return
	}

	// Формируем ответ
	resp := runStatusResponse{
		RunID:     runID,
		Status:    status,
		CreatedAt: createdAt,
	}

	if exitCode.Valid {
		code := int(exitCode.Int32)
		resp.ExitCode = &code
	}

	if startedAt.Valid {
		resp.StartedAt = &startedAt.Time
	}

	if finishedAt.Valid {
		resp.FinishedAt = &finishedAt.Time
	}

	// Информация о логе (доступен только после завершения)
	if status == "passed" || status == "failed" || status == "timeout" || status == "error" {
		logAvailable := logPath.Valid && logPath.String != ""
		resp.Log = &logInfo{
			Available: logAvailable,
		}
		if logAvailable {
			resp.Log.DownloadURL = "/runs/" + runID + "/log"
		}
		if receivedAt.Valid {
			resp.Log.ReceivedAt = &receivedAt.Time
		}
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

// === GET /runs/{id}/log ===

// Выдаёт лог прогона и сокращает TTL при полной передаче.
// Range-запросы игнорируются — засчитывается только полное скачивание.
func handleGetRunLog(w http.ResponseWriter, r *http.Request, db *sql.DB, runID string) {
	u, ok := httpx.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
		return
	}

	var (
		status   string
		logPath  sql.NullString
		clientID string
	)

	err := db.QueryRowContext(r.Context(),
		`SELECT status, log_path, client_id
		 FROM runs
		 WHERE id = $1 AND deleted_at IS NULL`,
		runID,
	).Scan(&status, &logPath, &clientID)

	if err == sql.ErrNoRows {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Прогон не найден")
		return
	}
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка получения прогона")
		return
	}

	if clientID != u.ClientID {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Нет доступа к этому прогону")
		return
	}

	if status != "passed" && status != "failed" && status != "timeout" && status != "error" {
		httpx.WriteError(w, r, http.StatusConflict, "RUN_NOT_FINISHED", "Прогон ещё не завершён")
		return
	}

	if !logPath.Valid || logPath.String == "" {
		httpx.WriteError(w, r, http.StatusNotFound, "LOG_NOT_FOUND", "Лог не найден")
		return
	}

	file, err := os.Open(logPath.String)
	if err != nil {
		if os.IsNotExist(err) {
			httpx.WriteError(w, r, http.StatusNotFound, "LOG_NOT_FOUND", "Файл лога не существует")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось открыть файл лога")
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось получить информацию о файле")
		return
	}
	fileSize := stat.Size()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Del("Accept-Ranges")

	w.WriteHeader(http.StatusOK)

	sentBytes, err := io.Copy(w, file)

	// При полной передаче — фиксируем получение и сокращаем TTL
	if err == nil && sentBytes == fileSize {
		markLogAsReceived(r.Context(), db, runID)
	}
}

// Помечает лог как полученный и сокращает время до удаления
func markLogAsReceived(ctx context.Context, db *sql.DB, runID string) {
	receivedTTL, err := commonx.RequireDuration("QTIVA_LOG_RECEIVED_TTL")
	if err != nil {
		receivedTTL = 10 * time.Minute
	}

	// received_at ставится один раз (COALESCE), но delete_after перезаписывается
	// при каждом успешном скачивании (продлевает жизнь на 10 минут от последнего скачивания)
	_, _ = db.ExecContext(ctx,
		`UPDATE runs
		 SET received_at = COALESCE(received_at, now()),
		     delete_after = now() + $1::interval
		 WHERE id = $2`,
		receivedTTL.String(), runID,
	)
}

// === Валидация ===

func isValidOS(v string) bool {
	return v == "manjaro" || v == "ubuntu"
}

func isValidDisplay(v string) bool {
	return v == "wayland" || v == "x11"
}

func isValidQtVersion(v string) bool {
	return v == "5.15"
}

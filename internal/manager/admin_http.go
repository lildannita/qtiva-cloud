package manager

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/lildannita/qtiva-cloud/internal/idgen"
)

// Cодержит зависимости для admin endpoints
type AdminAPI struct {
	DB      *sql.DB
	MaxJSON int64 // Максимальный размер JSON в байтах
}

// Регистрирует admin endpoints
// Все маршруты защищены middleware авторизации + проверкой роли admin
func RegisterAdminRoutes(mux *http.ServeMux, api AdminAPI, authMiddleware func(http.Handler) http.Handler) {
	// POST /admin/clients — создание клиента
	mux.Handle("/admin/clients", authMiddleware(httpx.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleCreateClient(w, r, api)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// PATCH /admin/clients/{id} — обновление клиента
	// Используем префиксный маршрут для захвата ID
	mux.Handle("/admin/clients/", authMiddleware(httpx.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем client_id из пути: /admin/clients/cl_xxx
		path := strings.TrimPrefix(r.URL.Path, "/admin/clients/")
		clientID := strings.TrimSuffix(path, "/")

		if clientID == "" {
			httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "client_id не указан")
			return
		}

		switch r.Method {
		case http.MethodPatch:
			handleUpdateClient(w, r, api, clientID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// POST /admin/invites — создание invite-кода
	mux.Handle("/admin/invites", authMiddleware(httpx.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleCreateInvite(w, r, api)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
}

// === Clients ===

// Описывает запрос на создание клиента
type createClientRequest struct {
	Name             string `json:"name"`              // Название клиента/компании
	NetworkAllowed   bool   `json:"network_allowed"`   // Разрешён ли доступ к сети в контейнерах
	ConcurrencyLimit int    `json:"concurrency_limit"` // Лимит параллельных прогонов
}

// Описывает ответ при создании клиента
type createClientResponse struct {
	ClientID string `json:"client_id"`
}

// Создаёт нового клиента (тенанта)
func handleCreateClient(w http.ResponseWriter, r *http.Request, api AdminAPI) {
	var req createClientRequest
	if err := httpx.DecodeJSON(r, &req, api.MaxJSON); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Валидация
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name обязателен")
		return
	}
	if len(name) > 255 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name слишком длинный (максимум 255 символов)")
		return
	}

	// Если лимит не указан или <= 0, ставим дефолт
	concurrencyLimit := req.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = 1
	}
	if concurrencyLimit > 10 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "concurrency_limit не может быть больше 10")
		return
	}

	// Генерируем ID клиента
	clientID, err := idgen.New("cl")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать client_id")
		return
	}

	// Создаём запись в БД
	_, err = api.DB.ExecContext(r.Context(),
		`INSERT INTO clients (id, name, network_allowed, concurrency_limit)
		 VALUES ($1, $2, $3, $4)`,
		clientID, name, req.NetworkAllowed, concurrencyLimit,
	)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать клиента")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, createClientResponse{
		ClientID: clientID,
	})
}

// Описывает запрос на обновление клиента
type updateClientRequest struct {
	NetworkAllowed   *bool `json:"network_allowed,omitempty"`   // Разрешён ли доступ к сети
	ConcurrencyLimit *int  `json:"concurrency_limit,omitempty"` // Лимит параллельных прогонов
}

// Обновляет настройки клиента
func handleUpdateClient(w http.ResponseWriter, r *http.Request, api AdminAPI, clientID string) {
	var req updateClientRequest
	if err := httpx.DecodeJSON(r, &req, api.MaxJSON); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Проверяем существование клиента
	var exists bool
	err := api.DB.QueryRowContext(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1)`,
		clientID,
	).Scan(&exists)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка проверки клиента")
		return
	}
	if !exists {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Клиент не найден")
		return
	}

	// Собираем поля для обновления
	// Используем динамическое построение запроса для частичного обновления
	updates := []string{}
	args := []any{}
	argIdx := 1

	if req.NetworkAllowed != nil {
		updates = append(updates, "network_allowed = $"+string(rune('0'+argIdx)))
		args = append(args, *req.NetworkAllowed)
		argIdx++
	}

	if req.ConcurrencyLimit != nil {
		if *req.ConcurrencyLimit <= 0 {
			httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "concurrency_limit должен быть > 0")
			return
		}
		if *req.ConcurrencyLimit > 10 {
			httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "concurrency_limit не может быть больше 10")
			return
		}
		updates = append(updates, "concurrency_limit = $"+string(rune('0'+argIdx)))
		args = append(args, *req.ConcurrencyLimit)
		argIdx++
	}

	if len(updates) == 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Нет полей для обновления")
		return
	}

	// Добавляем updated_at
	updates = append(updates, "updated_at = now()")

	// Добавляем client_id в конец аргументов
	args = append(args, clientID)

	query := "UPDATE clients SET " + strings.Join(updates, ", ") + " WHERE id = $" + string(rune('0'+argIdx))

	_, err = api.DB.ExecContext(r.Context(), query, args...)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обновить клиента")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "updated",
	})
}

// === Invites ===

// Описывает запрос на создание invite-кода
type createInviteRequest struct {
	ClientID  string     `json:"client_id"`            // ID клиента, для которого создаётся invite
	MaxUses   int        `json:"max_uses"`             // Максимальное количество использований
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // Срок действия (опционально)
}

// Описывает ответ при создании invite-кода
type createInviteResponse struct {
	InviteCode string `json:"invite_code"`
}

// Создаёт новый invite-код для клиента
func handleCreateInvite(w http.ResponseWriter, r *http.Request, api AdminAPI) {
	var req createInviteRequest
	if err := httpx.DecodeJSON(r, &req, api.MaxJSON); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Валидация
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "client_id обязателен")
		return
	}

	if req.MaxUses <= 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "max_uses должен быть > 0")
		return
	}
	if req.MaxUses > 1000 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "max_uses не может быть больше 1000")
		return
	}

	// Проверяем, что expires_at в будущем (если указан)
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "expires_at должен быть в будущем")
		return
	}

	// Проверяем существование клиента
	var clientExists bool
	err := api.DB.QueryRowContext(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1)`,
		clientID,
	).Scan(&clientExists)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка проверки клиента")
		return
	}
	if !clientExists {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Клиент не найден")
		return
	}

	// Генерируем invite-код
	inviteCode, err := idgen.GenerateInviteCode(clientID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось сгенерировать invite-код")
		return
	}

	// Проверяем уникальность кода (маловероятно, но возможно)
	var codeExists bool
	err = api.DB.QueryRowContext(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM invites WHERE code = $1)`,
		inviteCode,
	).Scan(&codeExists)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ошибка проверки кода")
		return
	}
	if codeExists {
		// Пробуем сгенерировать ещё раз (очень редкий случай)
		inviteCode, err = idgen.GenerateInviteCode(clientID)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось сгенерировать invite-код")
			return
		}
	}

	// Создаём запись в БД
	_, err = api.DB.ExecContext(r.Context(),
		`INSERT INTO invites (code, client_id, max_uses, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		inviteCode, clientID, req.MaxUses, req.ExpiresAt,
	)
	if err != nil {
		// Проверяем на дубликат (constraint violation)
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			httpx.WriteError(w, r, http.StatusConflict, "INVITE_CODE_EXISTS", "Код уже существует, попробуйте ещё раз")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать invite-код")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, createInviteResponse{
		InviteCode: inviteCode,
	})
}

// === Вспомогательные функции ===

// Строит SQL запрос для частичного обновления
// Возвращает строку запроса и слайс аргументов
func buildUpdateQuery(table string, idColumn string, id any, fields map[string]any) (string, []any, error) {
	if len(fields) == 0 {
		return "", nil, errors.New("нет полей для обновления")
	}

	var setClauses []string
	var args []any
	argIdx := 1

	for column, value := range fields {
		setClauses = append(setClauses, column+" = $"+string(rune('0'+argIdx)))
		args = append(args, value)
		argIdx++
	}

	// Добавляем updated_at
	setClauses = append(setClauses, "updated_at = now()")

	// Добавляем ID в конец
	args = append(args, id)

	query := "UPDATE " + table + " SET " + strings.Join(setClauses, ", ") +
		" WHERE " + idColumn + " = $" + string(rune('0'+argIdx))

	return query, args, nil
}

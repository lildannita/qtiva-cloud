package manager

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lildannita/qtiva-cloud/internal/authx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/lildannita/qtiva-cloud/internal/idgen"
	"github.com/lildannita/qtiva-cloud/internal/jwtx"
)

type AuthAPI struct {
	DB      *sql.DB
	JWT     jwtx.Config
	MaxJSON int64
}

func RegisterAuthRoutes(mux *http.ServeMux, api AuthAPI) {
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleRegister(w, r, api)
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleLogin(w, r, api)
	})

	mux.Handle("/me", httpx.RequireAuth(api.DB, api.JWT, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := httpx.UserFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"user_id":   u.UserID,
			"email":     u.Email,
			"client_id": u.ClientID,
			"role":      u.Role,
		})
	})))
}

type registerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

func handleRegister(w http.ResponseWriter, r *http.Request, api AuthAPI) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req, api.MaxJSON); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	email := strings.TrimSpace(req.Email)
	pass := strings.TrimSpace(req.Password)
	code := strings.TrimSpace(req.InviteCode)

	if email == "" || pass == "" || code == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "email/password/invite_code обязательны")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Некорректный email")
		return
	}
	if len(pass) < 12 || len(pass) > 256 {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Пароль должен быть 12..256 символов")
		return
	}

	hash, err := authx.HashPassword(pass)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось обработать пароль")
		return
	}

	userID, err := idgen.New("usr")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать user_id")
		return
	}

	var clientID string

	tx, err := api.DB.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось начать транзакцию")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Атомарно "потребляем" invite: увеличиваем used_count, проверяя ограничения
	err = tx.QueryRowContext(r.Context(),
		`UPDATE invites
		 SET used_count = used_count + 1
		 WHERE code=$1
		   AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > now())
		   AND used_count < max_uses
		 RETURNING client_id`,
		code,
	).Scan(&clientID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INVITE_CODE", "Invite-код недействителен")
		return
	}

	// Роль по умолчанию user (admin остаётся один и создаётся/меняется через seed-admin)
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO users (id, client_id, email, password_hash, role)
		 VALUES ($1,$2,$3,$4,'user')`,
		userID, clientID, email, hash,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 == unique_violation - нарушение уникального ограничения
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.WriteError(w, r, http.StatusConflict, "USER_ALREADY_EXISTS", "Пользователь с таким email уже существует")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать пользователя")
		return
	}

	if err := tx.Commit(); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось завершить транзакцию")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"user_id":   userID,
		"client_id": clientID,
		"email":     email,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request, api AuthAPI) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req, api.MaxJSON); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	email := strings.TrimSpace(req.Email)
	pass := strings.TrimSpace(req.Password)
	if email == "" || pass == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "email/password обязательны")
		return
	}

	var (
		userID   string
		clientID string
		role     string
		hash     string
	)

	err := api.DB.QueryRowContext(r.Context(),
		`SELECT id, client_id, role, password_hash
		 FROM users
		 WHERE lower(email)=lower($1) AND deleted_at IS NULL
		 LIMIT 1`,
		email,
	).Scan(&userID, &clientID, &role, &hash)
	if err != nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Неверный email или пароль")
		return
	}

	if !authx.VerifyPassword(hash, pass) {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Неверный email или пароль")
		return
	}

	now := time.Now().UTC()
	token, exp, err := jwtx.Issue(api.JWT, userID, clientID, role, now)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось выдать токен")
		return
	}

	ttlSec := int64(exp.Sub(now).Seconds())

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":   token,
		"token_type":     "Bearer",
		"expires_in_sec": ttlSec,
	})
}

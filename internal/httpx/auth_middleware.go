package httpx

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/jwtx"
)

// Ключ для хранения данных пользователя в контексте запроса
type ctxKeyUser struct{}

// Данные авторизованного пользователя
type AuthedUser struct {
	UserID   string
	ClientID string
	Email    string
	Role     string
}

// Проверяет JWT токен и загружает данные пользователя
// Если токен невалидный или пользователь не найден — возвращает 401
func RequireAuth(db *sql.DB, jwtCfg jwtx.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimSpace(r.Header.Get("Authorization"))
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется Bearer токен")
			return
		}
		tokenStr := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))

		claims, err := jwtx.Parse(jwtCfg, tokenStr, time.Now().UTC())
		if err != nil {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Невалидный токен")
			return
		}

		userID := claims.Subject
		if userID == "" {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Невалидный токен")
			return
		}

		var u AuthedUser
		err = db.QueryRowContext(r.Context(),
			`SELECT id, client_id, email, role
			 FROM users
			 WHERE id=$1 AND deleted_at IS NULL`,
			userID,
		).Scan(&u.UserID, &u.ClientID, &u.Email, &u.Role)
		if err != nil {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Пользователь не найден или удалён")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyUser{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Проверяет, что пользователь имеет роль admin
// Должен использоваться ПОСЛЕ RequireAuth
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
			return
		}

		if u.Role != "admin" {
			WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Требуются права администратора")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Извлекает данные пользователя из контекста запроса
func UserFromContext(ctx context.Context) (AuthedUser, bool) {
	v, ok := ctx.Value(ctxKeyUser{}).(AuthedUser)
	return v, ok
}

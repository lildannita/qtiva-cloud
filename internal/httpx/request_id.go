package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/lildannita/qtiva-cloud/internal/idgen"
)

type ctxKeyRequestID struct{}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if rid == "" || len(rid) > 64 {
			newID, err := idgen.New("req")
			if err == nil {
				rid = newID
			} else {
				rid = "req_unknown"
			}
		}
		w.Header().Set("X-Request-Id", rid)

		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID{}).(string)
	if v == "" {
		return "req_unknown"
	}
	return v
}

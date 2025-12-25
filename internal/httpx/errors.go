package httpx

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:     code,
		Message:   msg,
		RequestID: RequestIDFromContext(r.Context()),
	})
}

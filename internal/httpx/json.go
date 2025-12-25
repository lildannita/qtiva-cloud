package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			return fmt.Errorf("ошибка JSON: некорректный синтаксис")
		}
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("ошибка JSON: пустое тело")
		}
		return fmt.Errorf("ошибка JSON: %v", err)
	}

	if dec.More() {
		return fmt.Errorf("ошибка JSON: лишние данные после объекта")
	}
	return nil
}

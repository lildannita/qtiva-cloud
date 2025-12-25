package idgen

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// Генерирует идентификатор вида "<prefix>_<ulid>"
// Пример: usr_01JFD3K5Y2Z2JH3Q0W2B6J3J9T
func New(prefix string) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("idgen: prefix пустой")
	}

	// Monotonic-entropy требует синхронизации при параллельной генерации
	mu.Lock()
	defer mu.Unlock()

	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return "", fmt.Errorf("idgen: ulid.New: %w", err)
	}
	return prefix + "_" + id.String(), nil
}

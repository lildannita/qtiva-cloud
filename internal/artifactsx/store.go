package artifactsx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

type SaveResult struct {
	SHA256    string
	SizeBytes int64
}

func SaveMultipartFile(dstPath string, part *multipart.Part, maxBytes int64) (SaveResult, error) {
	if maxBytes <= 0 {
		return SaveResult{}, fmt.Errorf("maxBytes должен быть > 0")
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
		return SaveResult{}, fmt.Errorf("не удалось создать директорию: %w", err)
	}

	tmpPath := dstPath + ".tmp"

	// Создаёт (или перезаписывает) файл и открывает его только для записи с правами 0640 (rw-r-----)
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return SaveResult{}, fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer out.Close()

	h := sha256.New()

	lr := &io.LimitedReader{R: part, N: maxBytes + 1}
	n, err := io.Copy(io.MultiWriter(out, h), lr)
	if err != nil {
		_ = os.Remove(tmpPath)
		return SaveResult{}, fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	if n > maxBytes {
		_ = os.Remove(tmpPath)
		return SaveResult{}, ErrTooLarge
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return SaveResult{}, fmt.Errorf("не удалось закрыть файл: %w", err)
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return SaveResult{}, fmt.Errorf("не удалось переименовать файл: %w", err)
	}

	return SaveResult{
		SHA256:    hex.EncodeToString(h.Sum(nil)),
		SizeBytes: n,
	}, nil
}

var ErrTooLarge = fmt.Errorf("artifact too large")

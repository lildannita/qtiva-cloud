package artifactsx

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest описывает содержимое manifest.json в артефакте
// Обновлённая структура для работы с QtAda
type Manifest struct {
	// ConfigPath — путь к JSON-файлу конфигурации QtAda (обязательно)
	ConfigPath string `json:"config_path"`

	// Scripts — массив путей к тестовым сценариям QtAda (обязательно, минимум 1)
	Scripts []string `json:"scripts"`

	// Application — команда запуска тестируемого приложения с аргументами (обязательно)
	// Пример: "./myapp --some-arg"
	Application string `json:"application"`

	// TimeoutSec — таймаут для каждого теста в секундах (опционально, по умолчанию 60)
	TimeoutSec int `json:"timeout_sec"`

	// Env — дополнительные переменные окружения (опционально)
	Env map[string]string `json:"env"`
}

type ValidateConfig struct {
	MaxTimeoutSec int
}

func ParseManifestJSON(b []byte) (Manifest, error) {
	var m Manifest

	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("некорректный manifest.json: %w", err)
	}

	// Устанавливаем значение по умолчанию для timeout
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 60
	}

	return m, nil
}

func ValidateManifest(m Manifest, cfg ValidateConfig) error {
	// Проверка config_path
	if strings.TrimSpace(m.ConfigPath) == "" {
		return fmt.Errorf("config_path обязателен")
	}

	// Проверка scripts
	if len(m.Scripts) == 0 {
		return fmt.Errorf("scripts должен содержать минимум один путь к тестовому сценарию")
	}
	for i, script := range m.Scripts {
		if strings.TrimSpace(script) == "" {
			return fmt.Errorf("scripts[%d] не может быть пустым", i)
		}
	}

	// Проверка application
	if strings.TrimSpace(m.Application) == "" {
		return fmt.Errorf("application обязателен")
	}

	// Проверка timeout
	if m.TimeoutSec <= 0 {
		return fmt.Errorf("timeout_sec должен быть > 0")
	}
	if cfg.MaxTimeoutSec <= 0 {
		return fmt.Errorf("max_timeout_sec в конфиге должен быть > 0")
	}
	if m.TimeoutSec > cfg.MaxTimeoutSec {
		return fmt.Errorf("timeout_sec превышает лимит %d", cfg.MaxTimeoutSec)
	}

	return nil
}

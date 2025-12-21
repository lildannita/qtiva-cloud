package artifactsx

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Описывает содержимое manifest.json в артефакте
type Manifest struct {
	ConfigPath  string            `json:"config_path"`
	Scripts     []string          `json:"scripts"`
	Application string            `json:"application"`
	TimeoutSec  int               `json:"timeout_sec"`
	Env         map[string]string `json:"env"`
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

package artifactsx

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Manifest struct {
	RunCmd       string            `json:"run_cmd"`
	TimeoutSec   int               `json:"timeout_sec"`
	NeedsDisplay bool              `json:"needs_display"`
	QtVersion    string            `json:"qt_version"`
	OS           string            `json:"os"`
	Display      string            `json:"display"`
	Env          map[string]string `json:"env"`
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
	return m, nil
}

func ValidateManifest(m Manifest, cfg ValidateConfig) error {
	if strings.TrimSpace(m.RunCmd) == "" {
		return fmt.Errorf("run_cmd обязателен")
	}
	if m.TimeoutSec <= 0 {
		return fmt.Errorf("timeout_sec должен быть > 0")
	}
	if cfg.MaxTimeoutSec <= 0 {
		return fmt.Errorf("max_timeout_sec в конфиге должен быть > 0")
	}
	if m.TimeoutSec > cfg.MaxTimeoutSec {
		return fmt.Errorf("timeout_sec превышает лимит %d", cfg.MaxTimeoutSec)
	}

	if !isAllowedOS(m.OS) {
		return fmt.Errorf("os не поддерживается")
	}
	if !isAllowedDisplay(m.Display) {
		return fmt.Errorf("display не поддерживается")
	}
	if !isAllowedQt(m.QtVersion) {
		return fmt.Errorf("qt_version не поддерживается")
	}

	return nil
}

func isAllowedOS(v string) bool {
	switch v {
	case "manjaro", "ubuntu":
		return true
	default:
		return false
	}
}

func isAllowedDisplay(v string) bool {
	switch v {
	case "wayland", "x11":
		return true
	default:
		return false
	}
}

func isAllowedQt(v string) bool {
	switch v {
	case "5.15":
		return true
	default:
		return false
	}
}

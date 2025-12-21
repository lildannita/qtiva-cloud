package agent

import (
	"fmt"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
)

// Config содержит настройки агента
type Config struct {
	// Количество параллельных воркеров
	Concurrency int

	// Интервал опроса очереди
	PollInterval time.Duration

	// Директория для данных
	DataDir string

	// Docker настройки
	DockerHost    string
	DockerRuntime string

	// Лимиты по умолчанию
	DefaultCPU      float64
	DefaultMemoryMB int64
	DefaultPids     int64
	// DefaultTimeout  time.Duration
	MaxTimeout time.Duration

	// Сеть
	DefaultNetworkMode string
	AllowedNetworkMode string
	AllowedNetworkName string

	// Образы runner'ов
	Images map[string]map[string]map[string]string
}

// LoadConfigFromEnv загружает конфигурацию из переменных окружения
func LoadConfigFromEnv() (Config, error) {
	concurrency, err := commonx.RequireInt("QTIVA_AGENT_CONCURRENCY")
	if err != nil {
		concurrency = 2 // default
	}

	pollInterval, err := commonx.RequireDuration("QTIVA_AGENT_POLL_INTERVAL")
	if err != nil {
		pollInterval = 500 * time.Millisecond
	}

	dataDir, err := commonx.RequireString("QTIVA_DATA_DIR")
	if err != nil {
		return Config{}, err
	}

	dockerHost, _ := commonx.RequireString("QTIVA_DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	dockerRuntime, err := commonx.RequireString("QTIVA_DOCKER_RUNTIME")
	if err != nil {
		dockerRuntime = "runc"
	}

	defaultCPU := 2.0
	defaultMemoryMB := int64(2048)
	defaultPids := int64(256)

	// defaultTimeout, err := commonx.RequireDuration("QTIVA_DEFAULT_TIMEOUT")
	// if err != nil {
	// 	defaultTimeout = 90 * time.Second
	// }

	maxTimeout, err := commonx.RequireDuration("QTIVA_MAX_TIMEOUT")
	if err != nil {
		maxTimeout = 180 * time.Second
	}

	// Образы runner'ов
	images := map[string]map[string]map[string]string{
		"manjaro": {
			"5.15": {
				"wayland": "qtiva-runner:manjaro-qt5.15-wayland",
				"x11":     "qtiva-runner:manjaro-qt5.15-x11",
			},
		},
		"ubuntu": {
			"5.15": {
				"wayland": "qtiva-runner:ubuntu-qt5.15-wayland",
				"x11":     "qtiva-runner:ubuntu-qt5.15-x11",
			},
		},
	}

	return Config{
		Concurrency:     concurrency,
		PollInterval:    pollInterval,
		DataDir:         dataDir,
		DockerHost:      dockerHost,
		DockerRuntime:   dockerRuntime,
		DefaultCPU:      defaultCPU,
		DefaultMemoryMB: defaultMemoryMB,
		DefaultPids:     defaultPids,
		// DefaultTimeout:     defaultTimeout,
		MaxTimeout:         maxTimeout,
		DefaultNetworkMode: "none",
		AllowedNetworkMode: "bridge",
		AllowedNetworkName: "qtiva-net",
		Images:             images,
	}, nil
}

// Возвращает имя образа для заданных параметров
func (c Config) GetImageName(os, qtVersion, display string) (string, error) {
	osImages, ok := c.Images[os]
	if !ok {
		return "", fmt.Errorf("ОС %s не поддерживается", os)
	}

	qtImages, ok := osImages[qtVersion]
	if !ok {
		return "", fmt.Errorf("Qt версия %s не поддерживается для %s", qtVersion, os)
	}

	image, ok := qtImages[display]
	if !ok {
		return "", fmt.Errorf("Display %s не поддерживается для %s/Qt%s", display, os, qtVersion)
	}

	return image, nil
}

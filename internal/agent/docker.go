package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Обёртка над Docker SDK
type DockerClient struct {
	cli    *client.Client
	config Config
}

// NewDockerClient создаёт клиент Docker
func NewDockerClient(cfg Config) (*DockerClient, error) {
	cli, err := client.New(
		client.WithHost(cfg.DockerHost),
	)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать Docker клиент: %w", err)
	}

	return &DockerClient{
		cli:    cli,
		config: cfg,
	}, nil
}

// Закрывает клиент
func (d *DockerClient) Close() error {
	return d.cli.Close()
}

// Проверяет доступность Docker daemon
func (d *DockerClient) Ping(ctx context.Context) error {
	_, err := d.cli.Ping(ctx, client.PingOptions{})
	return err
}

// Параметры для запуска контейнера
type RunParams struct {
	RunID          string
	ArtifactPath   string
	ArtifactsDir   string
	Image          string
	TimeoutSec     int // Таймаут для контейнера (из QTIVA_MAX_TIMEOUT)
	NetworkAllowed bool
	DisplayType    string            // Тип дисплея: "wayland" или "x11"
	Env            map[string]string // Дополнительные переменные окружения из manifest
}

// Результат выполнения контейнера
type RunResult struct {
	ExitCode int64
	Error    error
	Timeout  bool
}

// Запускает runner контейнер и ждёт завершения
func (d *DockerClient) RunContainer(ctx context.Context, params RunParams) RunResult {
	containerName := fmt.Sprintf("qtiva-run-%s", params.RunID)

	// Собираем переменные окружения
	envList := []string{
		fmt.Sprintf("QTIVA_RUN_ID=%s", params.RunID),
		fmt.Sprintf("QTIVA_DISPLAY_TYPE=%s", params.DisplayType),
	}
	for k, v := range params.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	// Определяем сеть
	networkMode := container.NetworkMode(d.config.DefaultNetworkMode)
	if params.NetworkAllowed {
		networkMode = container.NetworkMode(d.config.AllowedNetworkMode)
	}

	// Конфигурация контейнера
	containerConfig := &container.Config{
		Image: params.Image,
		Env:   envList,
		Cmd:   []string{"/qtiva/runner"},
	}

	// Конфигурация хоста
	hostConfig := &container.HostConfig{
		AutoRemove: true,
		Runtime:    d.config.DockerRuntime,
		Resources: container.Resources{
			NanoCPUs:  int64(d.config.DefaultCPU * 1e9),
			Memory:    d.config.DefaultMemoryMB * 1024 * 1024,
			PidsLimit: &d.config.DefaultPids,
		},
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges"},
		NetworkMode:    networkMode,
		// Прописываем work отдельно, чтобы оставить exec право
		Tmpfs: map[string]string{
			"/work": "rw,exec,nosuid,nodev,mode=1777,size=1024m",
		},
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   params.ArtifactPath,
				Target:   "/input/app.tar.gz",
				ReadOnly: true,
			},
			{
				Type:     mount.TypeBind,
				Source:   params.ArtifactsDir,
				Target:   "/artifacts",
				ReadOnly: false,
			},
			// "Чистый" Tmpfs
			{
				Type:   mount.TypeTmpfs,
				Target: "/tmp",
				TmpfsOptions: &mount.TmpfsOptions{
					SizeBytes: 512 * 1024 * 1024, // 512MB
					Mode:      0o1777,
				},
			},
			// {
			// 	Type:   mount.TypeTmpfs,
			// 	Target: "/work",
			// 	TmpfsOptions: &mount.TmpfsOptions{
			// 		SizeBytes: 1024 * 1024 * 1024, // 1GB
			// 		Mode:      0o1777,
			// 	},
			// },
		},
	}

	// Конфигурация сети (для allowed network)
	var networkConfig *network.NetworkingConfig
	if params.NetworkAllowed && d.config.AllowedNetworkName != "" {
		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				d.config.AllowedNetworkName: {},
			},
		}
	}

	// Создаём контейнер
	resp, err := d.cli.ContainerCreate(
		ctx,
		client.ContainerCreateOptions{
			Config:           containerConfig,
			HostConfig:       hostConfig,
			NetworkingConfig: networkConfig,
			Platform:         &ocispec.Platform{},
			Name:             containerName,
		},
	)

	if err != nil {
		return RunResult{ExitCode: -1, Error: fmt.Errorf("не удалось создать контейнер: %w", err)}
	}

	containerID := resp.ID

	// Запускаем контейнер
	if _, err := d.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		// Пытаемся удалить созданный контейнер
		_, removeErr := d.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true})
		if removeErr != nil {
			return RunResult{ExitCode: -1, Error: fmt.Errorf("не удалось запустить контейнер: %w\nне удалось удалить проблемный контейнер: %w", err, removeErr)}
		}
		return RunResult{ExitCode: -1, Error: fmt.Errorf("не удалось запустить контейнер: %w", err)}
	}

	// Ждём завершения с таймаутом
	timeout := time.Duration(params.TimeoutSec) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout+10*time.Second) // +10s buffer
	defer cancel()

	// Канал для результата Wait
	wr := d.cli.ContainerWait(timeoutCtx, containerID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})

	select {
	case waitResult := <-wr.Result:
		return RunResult{ExitCode: waitResult.StatusCode, Error: nil}

	case err := <-wr.Error:
		// Проверяем, был ли это таймаут
		if timeoutCtx.Err() == context.DeadlineExceeded {
			// Останавливаем контейнер
			stopTimeout := 5
			_, stopErr := d.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &stopTimeout})
			if stopErr != nil {
				return RunResult{ExitCode: -1, Error: fmt.Errorf("не удалось остановить контейнер: %w", stopErr), Timeout: true}
			}
			return RunResult{ExitCode: -1, Error: nil, Timeout: true}
		}
		return RunResult{ExitCode: -1, Error: fmt.Errorf("ошибка ожидания контейнера: %w", err)}

	case <-ctx.Done():
		// Внешняя отмена
		stopTimeout := 5
		_, stopErr := d.cli.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{Timeout: &stopTimeout})
		if stopErr != nil {
			return RunResult{ExitCode: -1, Error: fmt.Errorf("не удалось остановить контейнер: %w", stopErr), Timeout: true}
		}
		return RunResult{ExitCode: -1, Error: ctx.Err()}
	}
}

package agent

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/dbx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/spf13/cobra"
)

func RegisterServeCommand(root *cobra.Command) {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Запустить агент (воркеры + HTTP сервер)",
		RunE: func(cmd *cobra.Command, args []string) error {
			commonx.LoadDotenvIfDev()

			httpAddr, err := commonx.RequireString("QTIVA_AGENT_HTTP_ADDR")
			if err != nil {
				return err
			}

			// Загружаем конфигурацию агента
			config, err := LoadConfigFromEnv()
			if err != nil {
				return err
			}

			// Подключаемся к БД
			db, pingTimeout, err := dbx.OpenFromEnv()
			if err != nil {
				return err
			}
			defer db.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := dbx.Ping(ctx, db, pingTimeout); err != nil {
				return err
			}

			// Создаём агента
			agent, err := NewAgent(config, db)
			if err != nil {
				return err
			}

			// Запускаем агента (воркеры)
			if err := agent.Start(ctx); err != nil {
				return err
			}

			// Настраиваем HTTP сервер для health check
			mux := http.NewServeMux()
			httpx.RegisterHealthCheck(mux, "qtiva-agent")

			// Readiness probe
			mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
				// Проверяем подключение к Docker
				pingCtx, pingCancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer pingCancel()

				if err := agent.docker.Ping(pingCtx); err != nil {
					http.Error(w, "docker not ready", http.StatusServiceUnavailable)
					return
				}

				// Проверяем БД
				if err := dbx.Ping(r.Context(), db, pingTimeout); err != nil {
					http.Error(w, "db not ready", http.StatusServiceUnavailable)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			// Канал для graceful shutdown
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

			// Запускаем HTTP сервер в горутине
			srv := &http.Server{
				Addr:              httpAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			}

			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					cmd.PrintErrf("HTTP сервер: %v\n", err)
				}
			}()

			cmd.Printf("Агент запущен: HTTP %s, воркеров: %d\n", httpAddr, config.Concurrency)

			// Ждём сигнала остановки
			<-stop

			cmd.Println("Получен сигнал остановки...")

			// Останавливаем агента
			cancel()
			agent.Stop()

			// Останавливаем HTTP сервер
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			_ = srv.Shutdown(shutdownCtx)

			cmd.Println("Агент остановлен")
			return nil
		},
	}
	root.AddCommand(serveCmd)
}

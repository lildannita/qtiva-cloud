package manager

import (
	"context"
	"net/http"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/dbx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/lildannita/qtiva-cloud/internal/jwtx"
	"github.com/spf13/cobra"
)

// Регистрирует команду serve для запуска HTTP сервера
func RegisterServeCommand(root *cobra.Command) {
	var httpAddr string
	var uploadHTTPAddr string

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Запустить HTTP сервер",
		RunE: func(cmd *cobra.Command, args []string) error {
			commonx.LoadDotenvIfDev()

			if httpAddr == "" {
				v, err := commonx.RequireString("QTIVA_MANAGER_HTTP_ADDR")
				if err != nil {
					return err
				}
				httpAddr = v
			}

			if uploadHTTPAddr == "" {
				v, err := commonx.RequireString("QTIVA_MANAGER_UPLOAD_HTTP_ADDR")
				if err != nil {
					return err
				}
				uploadHTTPAddr = v
			}

			jwtCfg, err := jwtx.LoadConfigFromEnv()
			if err != nil {
				return err
			}

			db, pingTimeout, err := dbx.OpenFromEnv()
			if err != nil {
				return err
			}
			defer db.Close()

			if err := dbx.Ping(context.Background(), db, pingTimeout); err != nil {
				return err
			}

			// Функция-обёртка для middleware авторизации
			authMiddleware := func(next http.Handler) http.Handler {
				return httpx.RequireAuth(db, jwtCfg, next)
			}

			// === Основной API сервер ===
			mainMux := http.NewServeMux()

			// Health check
			httpx.RegisterHealthCheck(mainMux, "qtiva-manager")

			// Readiness probe (проверка БД)
			mainMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
				if err := dbx.Ping(r.Context(), db, pingTimeout); err != nil {
					http.Error(w, "db not ready", http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			// Auth routes (публичные: register, login; защищённый: /me)
			RegisterAuthRoutes(mainMux, AuthAPI{
				DB:      db,
				JWT:     jwtCfg,
				MaxJSON: 1 << 20, // 1 MB
			})

			// Admin routes (защищённые, только для role=admin)
			RegisterAdminRoutes(mainMux, AdminAPI{
				DB:      db,
				MaxJSON: 1 << 20,
			}, authMiddleware)

			// Runs routes (защищённые)
			RegisterRunRoutes(mainMux, RunsAPI{DB: db}, authMiddleware)

			// Сообщаем, что загрузка артефактов на другом порту
			mainMux.HandleFunc("/artifacts", func(w http.ResponseWriter, r *http.Request) {
				httpx.WriteError(w, r, http.StatusBadRequest, "UPLOAD_PORT_REQUIRED",
					"Загрузка доступна на отдельном upload-порту")
			})

			mainHandler := httpx.WithRequestID(mainMux)
			mainSrv := httpx.NewServer(httpAddr, mainHandler, httpx.ServerTimeouts{
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       15 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
			})

			// === Upload сервер ===
			uploadReadHeaderTimeout, err := commonx.RequireDuration("QTIVA_UPLOAD_READ_HEADER_TIMEOUT")
			if err != nil {
				return err
			}
			uploadReadTimeout, err := commonx.RequireDuration("QTIVA_UPLOAD_READ_TIMEOUT")
			if err != nil {
				return err
			}
			uploadWriteTimeout, err := commonx.RequireDuration("QTIVA_UPLOAD_WRITE_TIMEOUT")
			if err != nil {
				return err
			}
			uploadIdleTimeout, err := commonx.RequireDuration("QTIVA_UPLOAD_IDLE_TIMEOUT")
			if err != nil {
				return err
			}

			uploadMux := http.NewServeMux()
			httpx.RegisterHealthCheck(uploadMux, "qtiva-manager-upload")
			uploadMux.Handle("/artifacts",
				httpx.RequireAuth(db, jwtCfg, ArtifactUploadHandler(ArtifactsAPI{DB: db})),
			)
			uploadHandler := httpx.WithRequestID(uploadMux)
			uploadSrv := httpx.NewServer(uploadHTTPAddr, uploadHandler, httpx.ServerTimeouts{
				ReadHeaderTimeout: uploadReadHeaderTimeout,
				ReadTimeout:       uploadReadTimeout,
				WriteTimeout:      uploadWriteTimeout,
				IdleTimeout:       uploadIdleTimeout,
			})

			return httpx.RunHTTPServers(10*time.Second, mainSrv, uploadSrv)
		},
	}

	serveCmd.Flags().StringVar(&httpAddr, "http-addr", "", "Адрес для HTTP (например, :8080)")
	serveCmd.Flags().StringVar(&uploadHTTPAddr, "upload-http-addr", "", "Адрес для upload HTTP (например, :8081)")
	root.AddCommand(serveCmd)
}

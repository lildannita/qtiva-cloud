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

func RegisterServeCommand(root *cobra.Command) {
	var httpAddr string
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

			mux := http.NewServeMux()
			httpx.RegisterHealthCheck(mux, "qtiva-manager")
			mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
				if err := dbx.Ping(r.Context(), db, pingTimeout); err != nil {
					http.Error(w, "db not ready", http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
			RegisterAuthRoutes(mux, AuthAPI{
				DB:      db,
				JWT:     jwtCfg,
				MaxJSON: 1 << 20,
			})
			RegisterArtifactRoutes(mux, ArtifactsAPI{DB: db}, db, jwtCfg)
			handler := httpx.WithRequestID(mux)

			return httpx.RunHTTPServer(httpAddr, handler, 10*time.Second)
		},
	}
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", "", "Адрес для HTTP (например, :8080)")
	root.AddCommand(serveCmd)
}

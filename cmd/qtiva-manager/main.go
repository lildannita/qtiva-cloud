package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/lildannita/qtiva-cloud/internal/buildinfo"
	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	root := &cobra.Command{
		Use:   "qtiva-manager",
		Short: "qtiva-manager — HTTP API сервис управления прогонами GUI-тестирования",
		// SilenceUsage отключает повторный вывод usage при runtime-ошибках команды
		SilenceUsage: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Показать версию и commit сборки",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("version=%s commit=%s\n", buildinfo.Version, buildinfo.Commit)
		},
	})

	var httpAddr string
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Запустить HTTP сервер",
		RunE: func(cmd *cobra.Command, args []string) error {
			commonx.LoadDotenvIfDev()

			if v := os.Getenv("QTIVA_MANAGER_HTTP_ADDR"); v != "" && httpAddr == "" {
				httpAddr = v
			}
			if httpAddr == "" {
				httpAddr = ":8080"
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(httpx.HealthResponse{
					Status:  "ok",
					Service: "qtiva-manager",
					Version: buildinfo.Version,
					Commit:  buildinfo.Commit,
				})
			})

			return httpx.RunHTTPServer(httpAddr, mux, 10*time.Second)
		},
	}
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", "", "Адрес для HTTP (например, :8080)")
	root.AddCommand(serveCmd)

	// (пока только каркас, реализация будет когда подключим БД)
	seedCmd := &cobra.Command{
		Use:   "seed-admin --email <email> --password <password>",
		Short: "Создать первого администратора (bootstrap) через CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Здесь будет логика:
			// 1) подключиться к БД
			// 2) проверить, есть ли уже admin
			// 3) если нет — создать admin + (при необходимости) базового клиента
			return cmd.Help()
		},
	}
	seedCmd.Flags().String("email", "", "Email администратора")
	seedCmd.Flags().String("password", "", "Пароль администратора")
	root.AddCommand(seedCmd)

	if err := root.Execute(); err != nil {
		// Cobra уже печатает понятную ошибку, нам достаточно code=1
		os.Exit(1)
	}
}

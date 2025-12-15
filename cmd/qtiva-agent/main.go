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

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	root := &cobra.Command{
		Use:   "qtiva-agent",
		Short: "qtiva-agent — служба, выполняющая задания на запуск тестов",
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

			if v := os.Getenv("QTIVA_AGENT_HTTP_ADDR"); v != "" && httpAddr == "" {
				httpAddr = v
			}
			if httpAddr == "" {
				httpAddr = ":8090"
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(healthResponse{
					Status:  "ok",
					Service: "qtiva-agent",
					Version: buildinfo.Version,
					Commit:  buildinfo.Commit,
				})
			})

			return httpx.RunHTTPServer(httpAddr, mux, 10*time.Second)
		},
	}
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", "", "Адрес для HTTP (например, :8090)")
	root.AddCommand(serveCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

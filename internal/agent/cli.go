package agent

import (
	"net/http"
	"time"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
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
				v, err := commonx.RequireString("QTIVA_AGENT_HTTP_ADDR")
				if err != nil {
					return err
				}
				httpAddr = v
			}
			mux := http.NewServeMux()
			httpx.RegisterHealthCheck(mux, "qtiva-agent")
			return httpx.RunHTTPServer(httpAddr, mux, 10*time.Second)
		},
	}
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", "", "Адрес для HTTP (например, :8090)")
	root.AddCommand(serveCmd)
}

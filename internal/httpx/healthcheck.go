package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/lildannita/qtiva-cloud/internal/buildinfo"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

func RegisterHealthCheck(mux *http.ServeMux, service string) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:  "ok",
			Service: service,
			Version: buildinfo.Version,
			Commit:  buildinfo.Commit,
		})
	})
}

package httpx

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

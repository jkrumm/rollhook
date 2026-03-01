package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/jkrumm/rollhook/internal/state"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	resp := healthResponse{Version: version}
	if state.IsShuttingDown() {
		resp.Status = "shutting_down"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		resp.Status = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

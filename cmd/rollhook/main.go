package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jkrumm/rollhook/internal/api"
)

func main() {
	secret := os.Getenv("ROLLHOOK_SECRET")
	if len(secret) < 7 {
		log.Fatal("ROLLHOOK_SECRET must be set and at least 7 characters")
	}

	r := chi.NewRouter()
	r.Get("/health", api.HealthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7700"
	}

	slog.Info("RollHook starting", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

// gendocs generates the OpenAPI spec from the registered huma operations and
// writes it to stdout. Run once to produce apps/dashboard/openapi.json:
//
//	go run ./cmd/gendocs > apps/dashboard/openapi.json
//
// Re-run whenever API operations change to keep the spec in sync.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jkrumm/rollhook/internal/api"
)

func main() {
	r := chi.NewRouter()

	config := huma.DefaultConfig("RollHook API", "0.1.0")
	config.Info.Description = "Webhook-driven rolling deployment orchestrator for Docker Compose stacks"
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:   "http",
			Scheme: "bearer",
		},
	}
	config.DocsPath = ""

	humaAPI := humachi.New(r, config)

	// Register all operations — nil deps are safe here since no requests are made.
	api.RegisterHealth(humaAPI)
	api.RegisterDeploy(humaAPI, nil, nil)
	api.RegisterJobsAPI(humaAPI, nil)

	// Fetch the spec via the huma-registered /openapi.json route.
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		log.Fatalf("unexpected status %d from /openapi.json", rr.Code)
	}

	// Pretty-print the JSON before writing to stdout.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, rr.Body.Bytes(), "", "  "); err != nil {
		log.Fatalf("indent JSON: %v", err)
	}
	pretty.WriteByte('\n')

	if _, err := os.Stdout.Write(pretty.Bytes()); err != nil {
		log.Fatal(err)
	}
}

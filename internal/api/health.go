package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/docker/docker/client"
	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
	"github.com/jkrumm/rollhook/internal/middleware"
	"github.com/jkrumm/rollhook/internal/state"
)

type healthOutput struct {
	Status int
	Body   struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
}

func RegisterHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Description: "Returns server status and deployed version. Returns 503 during graceful shutdown so load balancers deregister this backend before the process exits.",
		Tags:        []string{"Health"},
	}, func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		version := os.Getenv("VERSION")
		if version == "" {
			version = "dev"
		}
		out := &healthOutput{}
		out.Body.Version = version
		if state.IsShuttingDown() {
			out.Status = http.StatusServiceUnavailable
			out.Body.Status = "shutting_down"
		} else {
			out.Status = http.StatusOK
			out.Body.Status = "ok"
		}
		return out, nil
	})
}

type readyInput struct {
	Authorization string `header:"Authorization" doc:"Optional. A valid bearer token widens the response to include docker_host and detail."`
}

type readyOutput struct {
	Status int
	Body   struct {
		Status     string `json:"status" doc:"ready, docker_unreachable, or shutting_down"`
		Docker     string `json:"docker" doc:"ok, unreachable, or unknown (not probed during shutdown)"`
		DockerHost string `json:"docker_host,omitempty" doc:"Docker API endpoint being dialled. Authenticated callers only."`
		Detail     string `json:"detail,omitempty" doc:"Underlying connection error. Authenticated callers only."`
	}
}

// dockerUnreachable tracks the last observed daemon state so the readiness
// probe logs one line per outage and one per recovery, rather than one per poll.
var dockerUnreachable atomic.Bool

// RegisterReady registers GET /ready — the readiness probe, distinct from the
// liveness probe at /health. /health only reflects whether the HTTP process is
// up and accepting connections (used by Traefik's loadbalancer healthcheck so
// it keeps routing traffic here even during a Docker-side fault, which is what
// lets this diagnostic error reach the caller instead of an opaque proxy
// error). /ready additionally verifies the Docker daemon is actually reachable
// — this is the endpoint container healthchecks (Dockerfile HEALTHCHECK) and
// uptime monitoring should target, so an unreachable daemon is visible as
// "unhealthy" instead of hiding behind a 200 from /health.
//
// The endpoint is public — a probe that needs a credential is a probe that
// silently stops probing — so the status fields are always returned but
// docker_host and the raw connection error are withheld from anonymous
// callers. Traefik routes every path here, which would otherwise publish the
// internal Docker endpoint to the internet. Operators get the full detail from
// stderr and from an authenticated request.
func RegisterReady(api huma.API, cli *client.Client, secret string) {
	huma.Register(api, huma.Operation{
		OperationID: "get-ready",
		Method:      http.MethodGet,
		Path:        "/ready",
		Summary:     "Readiness check",
		Description: "Reports whether the Docker daemon is reachable, in addition to the liveness signal /health provides. Unlike /health, this returns 503 whenever the Docker API cannot be reached (wrong DOCKER_HOST, socket proxy gone, socket not mounted) — the fault that caused every deploy to fail while /health kept reporting 200. Target this endpoint from container healthchecks and uptime monitoring; keep load balancer healthchecks on /health.",
		Tags:        []string{"Health"},
	}, func(ctx context.Context, input *readyInput) (*readyOutput, error) {
		out := &readyOutput{}
		out.Status = http.StatusOK
		trusted := middleware.MatchesSecret(input.Authorization, secret)

		if state.IsShuttingDown() {
			out.Status = http.StatusServiceUnavailable
			out.Body.Status = "shutting_down"
			out.Body.Docker = "unknown"
			return out, nil
		}

		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if err := dockerpkg.Ping(pingCtx, cli); err != nil {
			// Logged on the transition only — the probe polls every 10s, and an
			// operator reading `docker logs` needs one line per outage, not one
			// per poll. Without this, a daemon outage leaves no trace on stderr
			// at all, which is what makes it invisible for as long as nobody
			// happens to attempt a deploy.
			if !dockerUnreachable.Swap(true) {
				slog.Error("docker daemon unreachable — every deploy will fail until this is fixed",
					"docker_host", cli.DaemonHost(), "error", err)
			}
			out.Status = http.StatusServiceUnavailable
			out.Body.Status = "docker_unreachable"
			out.Body.Docker = "unreachable"
			if trusted {
				out.Body.DockerHost = cli.DaemonHost()
				out.Body.Detail = err.Error()
			}
			return out, nil
		}

		if dockerUnreachable.Swap(false) {
			slog.Info("docker daemon reachable again", "docker_host", cli.DaemonHost())
		}
		out.Status = http.StatusOK
		out.Body.Status = "ready"
		out.Body.Docker = "ok"
		if trusted {
			out.Body.DockerHost = cli.DaemonHost()
		}
		return out, nil
	})
}

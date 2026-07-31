package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/jkrumm/rollhook/internal/api"
)

// newReadyTestServer registers only RegisterReady against cli.
func newReadyTestServer(cli *client.Client) http.Handler {
	r := chi.NewRouter()
	config := huma.DefaultConfig("RollHook", "test")
	config.DocsPath = ""
	humaAPI := humachi.New(r, config)
	api.RegisterReady(humaAPI, cli, testSecret)
	return r
}

// getReady issues a GET /ready, optionally authenticated, and returns the
// recorder. Assertions run against the HTTP response only — RegisterReady keeps
// package-level state for transition logging, so tests must not depend on the
// order they run in.
func getReady(h http.Handler, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+testSecret)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// unreachableDockerClient points at a port nothing listens on — hermetic, no
// real daemon required.
func unreachableDockerClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.WithHost("tcp://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("NewClientWithOpts error: %v", err)
	}
	t.Cleanup(func() { cli.Close() })
	return cli
}

// fakeDockerClient points at a stub daemon that answers any ping.
func fakeDockerClient(t *testing.T) *client.Client {
	t.Helper()
	pingSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("API-Version", "1.51")
		w.Header().Set("OSType", "linux")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(pingSrv.Close)

	cli, err := client.NewClientWithOpts(client.WithHost(pingSrv.URL))
	if err != nil {
		t.Fatalf("NewClientWithOpts error: %v", err)
	}
	t.Cleanup(func() { cli.Close() })

	if _, err := cli.Ping(context.Background()); err != nil {
		t.Skipf("fake docker daemon ping failed, skipping: %v", err)
	}
	return cli
}

func TestReady_DaemonUnreachable(t *testing.T) {
	w := getReady(newReadyTestServer(unreachableDockerClient(t)), false)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"status":"docker_unreachable"`) {
		t.Errorf("expected status docker_unreachable, got: %s", body)
	}
	if !strings.Contains(body, `"docker":"unreachable"`) {
		t.Errorf("expected docker unreachable, got: %s", body)
	}
}

// The endpoint is public because a probe that needs a credential stops probing,
// but Traefik routes every path here — so the internal Docker endpoint and the
// raw connection error must never reach an anonymous caller.
func TestReady_AnonymousCallerGetsNoHostDetail(t *testing.T) {
	w := getReady(newReadyTestServer(unreachableDockerClient(t)), false)

	body := w.Body.String()
	if strings.Contains(body, "docker_host") || strings.Contains(body, "127.0.0.1:1") {
		t.Errorf("anonymous response leaked the docker host: %s", body)
	}
	if strings.Contains(body, "detail") {
		t.Errorf("anonymous response leaked the connection error: %s", body)
	}
}

func TestReady_AuthenticatedCallerGetsHostDetail(t *testing.T) {
	w := getReady(newReadyTestServer(unreachableDockerClient(t)), true)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"docker_host":"tcp://127.0.0.1:1"`) {
		t.Errorf("expected docker_host for an authenticated caller, got: %s", body)
	}
	if !strings.Contains(body, `"detail"`) {
		t.Errorf("expected detail for an authenticated caller, got: %s", body)
	}
}

func TestReady_WrongSecretIsTreatedAsAnonymous(t *testing.T) {
	h := newReadyTestServer(unreachableDockerClient(t))
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	req.Header.Set("Authorization", "Bearer not-the-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Still answers — the probe must never be gated — but reveals nothing.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "docker_host") {
		t.Errorf("a bad token must not unlock the docker host: %s", w.Body.String())
	}
}

func TestReady_DaemonReachable(t *testing.T) {
	w := getReady(newReadyTestServer(fakeDockerClient(t)), false)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"status":"ready"`) {
		t.Errorf("expected status ready, got: %s", body)
	}
	if !strings.Contains(body, `"docker":"ok"`) {
		t.Errorf("expected docker ok, got: %s", body)
	}
}

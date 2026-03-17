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
	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
	"github.com/jkrumm/rollhook/internal/middleware"
	oidcpkg "github.com/jkrumm/rollhook/internal/oidc"
)

// requireDockerAvailable skips the test if Docker daemon is unreachable.
// NewClient succeeds even without a socket; Ping is the first real connection attempt.
func requireDockerAvailable(t *testing.T, cli *client.Client) {
	t.Helper()
	if _, err := cli.Ping(context.Background()); err != nil {
		t.Skipf("skipping test: docker daemon unreachable: %v", err)
	}
}

// mockVerifier satisfies oidcpkg.Verifiable and returns preset claims for any token.
type mockVerifier struct {
	claims oidcpkg.Claims
}

func (m *mockVerifier) Verify(_ context.Context, _ string) (oidcpkg.Claims, error) {
	return m.claims, nil
}

// newMiddlewareGateTestServer creates a server with the real HumaAuth middleware
// and a mock verifier that accepts any JWT-looking token, allowing tests to verify
// that the post-auth-token operationID is in the allowlist.
func newMiddlewareGateTestServer(t *testing.T, claims oidcpkg.Claims) http.Handler {
	t.Helper()
	cli, err := dockerpkg.NewClient()
	if err != nil {
		t.Skipf("skipping test: docker client init failed: %v", err)
	}
	requireDockerAvailable(t, cli)
	t.Cleanup(func() { cli.Close() })

	r := chi.NewRouter()
	config := huma.DefaultConfig("RollHook", "test")
	config.DocsPath = ""
	humaAPI := humachi.New(r, config)

	humaAPI.UseMiddleware(middleware.HumaAuth(humaAPI, testSecret, &mockVerifier{claims: claims}))
	api.RegisterAuthToken(humaAPI, testSecret, cli)
	return r
}

// newAuthTestServer creates a minimal server for /auth/token tests.
// If claims is non-nil, any valid Bearer token passes auth and gets those claims
// injected, bypassing real OIDC JWT verification.
// If claims is nil, the standard static-secret middleware is used.
// cli may be nil for tests that return before steps.Discover (auth-only paths).
func newAuthTestServer(t *testing.T, claims *oidcpkg.Claims, cli *client.Client) http.Handler {
	t.Helper()

	r := chi.NewRouter()
	config := huma.DefaultConfig("RollHook", "test")
	config.DocsPath = ""
	humaAPI := humachi.New(r, config)

	if claims != nil {
		// Custom middleware: any Bearer token passes auth; inject the provided claims.
		humaAPI.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
			if len(ctx.Operation().Security) == 0 {
				next(ctx)
				return
			}
			_, ok := strings.CutPrefix(ctx.Header("Authorization"), "Bearer ")
			if !ok {
				_ = huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, "unauthorized")
				return
			}
			next(huma.WithValue(ctx, oidcpkg.ClaimsContextKey, *claims))
		})
	} else {
		humaAPI.UseMiddleware(middleware.HumaAuth(humaAPI, testSecret, nil))
	}

	api.RegisterAuthToken(humaAPI, testSecret, cli)
	return r
}

func TestAuthToken_NoAuth(t *testing.T) {
	srv := newAuthTestServer(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"image_name":"myapp"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthToken_StaticSecretRejected(t *testing.T) {
	srv := newAuthTestServer(t, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"image_name":"myapp"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader()) // static secret, no OIDC claims injected
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "OIDC token required") {
		t.Errorf("expected OIDC-required denial, got: %s", w.Body.String())
	}
}

func TestAuthToken_PRRefDenied(t *testing.T) {
	claims := &oidcpkg.Claims{
		Repository: "myorg/myapp",
		Ref:        "refs/pull/1/merge",
		Actor:      "testuser",
	}
	// nil cli — handler returns 403 on PR ref check before reaching steps.Discover.
	srv := newAuthTestServer(t, claims, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"image_name":"myapp"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "PR ref is not allowed") {
		t.Errorf("expected PR-ref denial, got: %s", w.Body.String())
	}
}

func TestAuthToken_DiscoverFailsDenied(t *testing.T) {
	cli, err := dockerpkg.NewClient()
	if err != nil {
		t.Skipf("skipping test: docker client init failed: %v", err)
	}
	requireDockerAvailable(t, cli)
	t.Cleanup(func() { cli.Close() })

	claims := &oidcpkg.Claims{
		Repository: "myorg/myapp",
		Ref:        "refs/heads/main",
		Actor:      "testuser",
	}
	srv := newAuthTestServer(t, claims, cli)
	req := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"image_name":"nonexistent-test-app-for-unit-tests"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "service not found") {
		t.Errorf("expected service-not-found denial, got: %s", w.Body.String())
	}
}

// TestAuthToken_OIDCMiddlewareGate exercises the real HumaAuth middleware to verify
// that "post-auth-token" is in the operationID allowlist. A JWT-looking token is
// sent through the real middleware (mock verifier accepts it, injects claims).
// The handler then proceeds past auth and rejects at Discover (no matching container) → 403.
// If the gate were broken, the middleware would return "OIDC tokens are only accepted for…" instead.
func TestAuthToken_OIDCMiddlewareGate(t *testing.T) {
	srv := newMiddlewareGateTestServer(t, oidcpkg.Claims{
		Repository: "myorg/myapp",
		Ref:        "refs/heads/main",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/token",
		strings.NewReader(`{"image_name":"nonexistent-test-app-for-unit-tests"}`))
	req.Header.Set("Content-Type", "application/json")
	// eyJ prefix makes IsJWT return true, triggering the real OIDC path in HumaAuth.
	req.Header.Set("Authorization", "Bearer eyJmake-believe-jwt")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// 403 means: middleware passed through (gate allowed post-auth-token),
	// handler rejected at Discover (no running container).
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (handler reject at Discover), got %d: %s", w.Code, w.Body.String())
	}
	// Confirm the rejection is NOT from the operationID gate.
	if strings.Contains(w.Body.String(), "OIDC tokens are only accepted") {
		t.Errorf("middleware incorrectly blocked OIDC on post-auth-token: %s", w.Body.String())
	}
}

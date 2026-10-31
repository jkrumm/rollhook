package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jkrumm/rollhook/internal/deploytoken"
	"github.com/jkrumm/rollhook/internal/middleware"
)

var testScope = deploytoken.Scope{App: "myapp", Project: "proj", Service: "web"}

func TestRequireAuth(t *testing.T) {
	const secret = "test-secret-ok"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequireAuth(secret)(next)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong-token", http.StatusForbidden},
		{"correct token", "Bearer " + secret, http.StatusOK},
		{"missing Bearer prefix", secret, http.StatusUnauthorized},
		// Empty bearer string is parsed as a valid Bearer format with empty token —
		// token mismatch → 403, not 401.
		{"empty bearer", "Bearer ", http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// Deploy tokens are service-scoped: a plain RequireAuth route must never accept
// them, only routes that opt in via RequireAuthAllowScoped (and then enforce the
// scope in the handler).
func TestRequireAuth_DeployTokens(t *testing.T) {
	const secret = "test-secret-ok"
	valid := "Bearer " + deploytoken.Mint(secret, testScope)
	forged := "Bearer " + deploytoken.Mint("other-secret", testScope)

	var gotScope bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotScope = deploytoken.ScopeFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		mw         func(string) func(http.Handler) http.Handler
		authHeader string
		wantStatus int
		wantScope  bool
	}{
		{"default rejects valid deploy token", middleware.RequireAuth, valid, http.StatusForbidden, false},
		{"default rejects forged deploy token", middleware.RequireAuth, forged, http.StatusForbidden, false},
		{"default accepts secret", middleware.RequireAuth, "Bearer " + secret, http.StatusOK, false},
		{"scoped accepts valid deploy token", middleware.RequireAuthAllowScoped, valid, http.StatusOK, true},
		{"scoped rejects forged deploy token", middleware.RequireAuthAllowScoped, forged, http.StatusForbidden, false},
		{"scoped accepts secret without scope", middleware.RequireAuthAllowScoped, "Bearer " + secret, http.StatusOK, false},
		{"scoped rejects wrong token", middleware.RequireAuthAllowScoped, "Bearer wrong", http.StatusForbidden, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotScope = false
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tc.authHeader)
			rec := httptest.NewRecorder()
			tc.mw(secret)(next).ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if gotScope != tc.wantScope {
				t.Errorf("scope in context = %v, want %v", gotScope, tc.wantScope)
			}
		})
	}
}

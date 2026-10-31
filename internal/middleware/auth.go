package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jkrumm/rollhook/internal/deploytoken"
)

// RequireAuth returns a standard http.Handler middleware that enforces bearer token auth
// with the static secret only. Behavior mirrors HumaAuth:
//   - No Authorization header or non-Bearer format → 401 Unauthorized
//   - Bearer with wrong token → 403 Forbidden
//   - Deploy token (even a valid one) → 403 Forbidden; use RequireAuthAllowScoped
//     on the few routes that enforce a deploy-token scope
//
// Use as a standalone chi middleware for non-huma routes (e.g. SSE endpoint).
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return requireAuth(secret, false)
}

// RequireAuthAllowScoped is RequireAuth that additionally accepts a valid deploy
// token (minted by POST /auth/token) and stores its scope in the request
// context. The wrapped handler must enforce that scope (see
// deploytoken.ScopeFromContext); only opt in for routes that do.
func RequireAuthAllowScoped(secret string) func(http.Handler) http.Handler {
	return requireAuth(secret, true)
}

func requireAuth(secret string, allowScoped bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				writeJSON(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if deploytoken.IsDeployToken(token) {
				if !allowScoped {
					writeJSON(w, http.StatusForbidden, "deploy tokens are not accepted on this route")
					return
				}
				scope, valid := deploytoken.Parse(secret, token)
				if !valid {
					writeJSON(w, http.StatusForbidden, "forbidden")
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deploytoken.ContextKey, scope)))
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
				writeJSON(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MatchesSecret reports whether a raw Authorization header value carries the
// static secret. Unlike RequireAuth it never rejects — it answers "is this
// caller trusted?", which is what public endpoints need when they widen their
// response for operators without gating access to it.
func MatchesSecret(authHeader, secret string) bool {
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}

func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

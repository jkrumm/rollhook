package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// RequireAuth returns a standard http.Handler middleware that enforces bearer token auth.
// Behavior mirrors HumaAuth:
//   - No Authorization header or non-Bearer format → 401 Unauthorized
//   - Bearer with wrong token → 403 Forbidden
//
// Use as a standalone chi middleware for non-huma routes (e.g. SSE endpoint).
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				writeJSON(w, http.StatusUnauthorized, "unauthorized")
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

func writeJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

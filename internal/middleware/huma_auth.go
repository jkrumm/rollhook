package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// HumaAuth returns a huma middleware that enforces bearer token auth for operations
// with a Security requirement. Behavior:
//   - No Authorization header or non-Bearer format → 401 Unauthorized
//   - Bearer with wrong token → 403 Forbidden
//
// Use this in main.go and tests to ensure both paths exercise the same auth logic.
func HumaAuth(api huma.API, secret string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if len(ctx.Operation().Security) == 0 {
			next(ctx)
			return
		}
		auth := ctx.Header("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}

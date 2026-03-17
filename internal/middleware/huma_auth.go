package middleware

import (
	"crypto/subtle"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	oidcpkg "github.com/jkrumm/rollhook/internal/oidc"
)

// HumaAuth returns a huma middleware that enforces bearer token auth for operations
// with a Security requirement. Accepts both GitHub Actions OIDC JWTs and static secrets.
//
//   - OIDC JWT (starts with "eyJ"): verified via verifier → claims stored in request context
//   - Static secret: constant-time comparison against secret
//   - Missing/malformed Bearer → 401
//   - Invalid JWT or wrong static secret → 403
//
// hasVerifier reports whether v is a non-nil, usable verifier.
// A plain nil interface and a typed-nil pointer wrapped in an interface both return false.
func hasVerifier(v oidcpkg.Verifiable) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !rv.IsNil()
	default:
		return true
	}
}

// Pass nil as verifier to disable OIDC support (static secret only).
func HumaAuth(api huma.API, secret string, verifier oidcpkg.Verifiable) func(huma.Context, func(huma.Context)) {
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

		if hasVerifier(verifier) && oidcpkg.IsJWT(token) {
			// OIDC JWTs are only accepted on POST /auth/token.
			// All other routes require the static ROLLHOOK_SECRET.
			if ctx.Operation().OperationID != "post-auth-token" {
				_ = huma.WriteErr(api, ctx, http.StatusForbidden, "OIDC tokens are only accepted for POST /auth/token")
				return
			}
			claims, err := verifier.Verify(ctx.Context(), token)
			if err != nil {
				_ = huma.WriteErr(api, ctx, http.StatusForbidden, "invalid OIDC token")
				return
			}
			// Inject claims into the context so the deploy handler can authorize via labels.
			next(huma.WithValue(ctx, oidcpkg.ClaimsContextKey, claims))
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}

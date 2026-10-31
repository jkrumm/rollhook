// Package deploytoken mints and validates short-lived, service-scoped API
// tokens handed out by POST /auth/token to OIDC callers.
//
// A deploy token authorizes POST /deploy for exactly one Compose service and
// read access to that app's jobs (GET /jobs/{id}, GET /jobs/{id}/logs). It is
// never a substitute for the static ROLLHOOK_SECRET.
package deploytoken

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// Prefix marks a bearer token as a deploy token. It cannot collide with the
// OIDC JWT prefix ("eyJ") and keeps the token distinguishable in logs.
const Prefix = "rhd_"

// TTL bounds how long a minted deploy token is accepted. Tokens are checked at
// request start only, so a long synchronous /deploy is not cut off mid-flight.
const TTL = 30 * time.Minute

// hmacDomain separates deploy-token signatures from registry-token signatures,
// which are keyed with the same secret.
const hmacDomain = "rollhook-deploy-token:"

// Scope is the service a deploy token is bound to.
type Scope struct {
	App     string `json:"app"`     // last path segment of the image name (== db.Job.App)
	Project string `json:"project"` // com.docker.compose.project of the authorized service
	Service string `json:"service"` // com.docker.compose.service of the authorized service
}

type payload struct {
	Scope
	Exp int64 `json:"exp"`
}

type contextKey struct{}

// ContextKey is the request-context key under which auth middleware stores the
// validated Scope of a deploy-token caller.
var ContextKey = contextKey{}

// ScopeFromContext returns the deploy-token scope stored by the auth middleware.
// ok is false for callers authenticated with the static secret.
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(ContextKey).(Scope)
	return s, ok
}

// IsDeployToken reports whether token has the deploy-token shape (prefix only;
// use Parse to validate).
func IsDeployToken(token string) bool {
	return strings.HasPrefix(token, Prefix)
}

// Mint returns a deploy token for scope, valid for TTL.
func Mint(secret string, scope Scope) string {
	return mintAt(secret, scope, time.Now().Add(TTL))
}

func mintAt(secret string, scope Scope, exp time.Time) string {
	raw, _ := json.Marshal(payload{Scope: scope, Exp: exp.Unix()}) // cannot fail: plain strings + int
	body := base64.RawURLEncoding.EncodeToString(raw)
	return Prefix + body + "." + sign(secret, body)
}

// Parse validates the token's signature and expiry and returns its scope.
func Parse(secret, token string) (Scope, bool) {
	rest, ok := strings.CutPrefix(token, Prefix)
	if !ok {
		return Scope{}, false
	}
	body, mac, ok := strings.Cut(rest, ".")
	if !ok || body == "" || mac == "" {
		return Scope{}, false
	}
	if subtle.ConstantTimeCompare([]byte(mac), []byte(sign(secret, body))) != 1 {
		return Scope{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Scope{}, false
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Scope{}, false
	}
	if p.App == "" || p.Project == "" || p.Service == "" || time.Now().Unix() > p.Exp {
		return Scope{}, false
	}
	return p.Scope, true
}

func sign(secret, body string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(hmacDomain + body))
	return hex.EncodeToString(h.Sum(nil))
}

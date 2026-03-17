package registry

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
)

// hopByHopHeaders lists connection-specific headers that must not be forwarded upstream.
// httputil.ReverseProxy strips most of these automatically, but we strip them explicitly
// in Director to match the TypeScript proxy behaviour (notably Authorization).
var hopByHopHeaders = []string{
	"Authorization",
	"Host",
	"Transfer-Encoding",
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Upgrade",
}

// zotAbsoluteURLPattern matches absolute URLs pointing at the internal Zot address.
// Location headers from Zot contain these and must be rewritten to relative paths
// so Docker follows redirects through our proxy, not directly to the loopback address.
var zotAbsoluteURLPattern = regexp.MustCompile(`(?i)^https?://127\.0\.0\.1:\d+`)

// extractRepoFromPath parses the repository name from an OCI distribution API path.
// Returns "" for the /v2/ ping and any path where the repo cannot be determined.
//
// Scans path segments in reverse so that repo names containing OCI operation
// keywords (e.g. "org/manifests/app") are handled correctly.
//
//	/v2/myapp/manifests/latest        → "myapp"
//	/v2/myorg/myapp/blobs/sha256:abc  → "myorg/myapp"
//	/v2/                              → ""
func extractRepoFromPath(urlPath string) string {
	rest, ok := strings.CutPrefix(urlPath, "/v2/")
	if !ok || rest == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	for i := len(parts) - 1; i >= 0; i-- {
		switch parts[i] {
		case "manifests", "tags", "referrers":
			if i == 0 {
				return ""
			}
			return strings.Join(parts[:i], "/")
		case "blobs":
			if i == 0 {
				return ""
			}
			return strings.Join(parts[:i], "/")
		case "uploads":
			// /v2/<repo>/blobs/uploads[/<uuid>]
			if i > 0 && parts[i-1] == "blobs" {
				if i-1 == 0 {
					return ""
				}
				return strings.Join(parts[:i-1], "/")
			}
		}
	}
	return ""
}

// validateProxyAuth validates the Authorization header from r against the shared secret.
// Accepts the static secret (for admin use) or a short-lived HMAC registry token
// minted by MintRegistryToken (for CI push credentials).
//
// For repo-scoped endpoints (/v2/<repo>/...) an HMAC token is only accepted when
// its bound imageName matches the target repository (scope enforcement).
// For the /v2/ ping endpoint (no repo in path) any valid HMAC token is accepted.
//
// Bearer: token must equal secret or be a valid registry token.
// Basic: base64-decoded password (after the first colon) must equal secret or valid token — any username accepted.
func validateProxyAuth(r *http.Request, secret string) bool {
	header := r.Header.Get("Authorization")
	if header == "" {
		return false
	}
	repo := extractRepoFromPath(r.URL.Path)
	isPing := r.URL.Path == "/v2" || r.URL.Path == "/v2/"
	tokenValid := func(token string) bool {
		if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1 {
			return true
		}
		if repo != "" {
			return ValidateRegistryTokenForRepo(secret, token, repo)
		}
		// Accept any valid (signed, unexpired) HMAC token on the /v2 ping only.
		// Unknown or catalog paths require the static secret.
		if isPing {
			return ValidateRegistryToken(secret, token)
		}
		return false
	}
	if bearer, ok := strings.CutPrefix(header, "Bearer "); ok {
		return tokenValid(bearer)
	}
	if basic, ok := strings.CutPrefix(header, "Basic "); ok {
		decoded, err := base64.StdEncoding.DecodeString(basic)
		if err != nil {
			return false
		}
		_, password, found := strings.Cut(string(decoded), ":")
		if !found {
			return false
		}
		return tokenValid(password)
	}
	return false
}

// NewProxy returns an http.Handler that:
//  1. Validates the client's Authorization header (Bearer or Basic, password = secret).
//  2. Strips hop-by-hop headers and injects Zot Basic auth credentials.
//  3. Proxies the request to zotAddr via httputil.ReverseProxy (streaming, no buffering).
//  4. Rewrites absolute Location headers from Zot to relative paths.
func NewProxy(zotAddr, secret string) http.Handler {
	target, err := url.Parse(zotAddr)
	if err != nil {
		panic("invalid zot address: " + err.Error())
	}

	zotAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(ZotUser+":"+secret))

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Rewrite absolute Zot Location URLs to relative paths so Docker follows
	// redirects through our proxy endpoint, not directly to 127.0.0.1:5000.
	proxy.ModifyResponse = func(resp *http.Response) error {
		if loc := resp.Header.Get("Location"); loc != "" {
			rewritten := zotAbsoluteURLPattern.ReplaceAllString(loc, "")
			if rewritten == "" {
				rewritten = "/"
			}
			resp.Header.Set("Location", rewritten)
		}
		return nil
	}

	// Override Director: strip hop-by-hop headers and inject Zot credentials.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		for _, h := range hopByHopHeaders {
			req.Header.Del(h)
		}
		req.Header.Set("Authorization", zotAuth)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateProxyAuth(r, secret) {
			writeUnauthorized(w)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	body, _ := json.Marshal(map[string]any{
		"errors": []map[string]any{
			{"code": "UNAUTHORIZED", "message": "authentication required", "detail": nil},
		},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Basic realm="RollHook Registry"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write(body)
}

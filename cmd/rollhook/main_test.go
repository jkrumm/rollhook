package main

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The docs page shares the dashboard origin, so every external script must be
// pinned and integrity-checked, and the page must carry a CSP.
func TestServeScalar_PinnedScriptAndCSP(t *testing.T) {
	w := httptest.NewRecorder()
	serveScalar(w, httptest.NewRequest(http.MethodGet, "/openapi", nil))
	body := w.Body.String()

	scripts := regexp.MustCompile(`<script[^>]*\ssrc="([^"]+)"[^>]*>`).FindAllStringSubmatch(body, -1)
	if len(scripts) == 0 {
		t.Fatal("expected an external script tag")
	}
	for _, m := range scripts {
		tag, src := m[0], m[1]
		if !regexp.MustCompile(`@\d+\.\d+\.\d+/`).MatchString(src) {
			t.Errorf("script %q is not pinned to an exact version", src)
		}
		if !strings.Contains(tag, `integrity="sha384-`) || !strings.Contains(tag, `crossorigin="anonymous"`) {
			t.Errorf("script %q lacks SRI", src)
		}
	}

	csp := w.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src https://cdn.jsdelivr.net;", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
}

// An inline <script> only runs if the CSP allows it. The config element must
// either stay empty (it is read via data-* attributes, never executed) or its
// exact content hash must be listed in script-src.
func TestServeScalar_InlineScriptsAllowedByCSP(t *testing.T) {
	w := httptest.NewRecorder()
	serveScalar(w, httptest.NewRequest(http.MethodGet, "/openapi", nil))
	csp := w.Header().Get("Content-Security-Policy")
	scriptSrc := cspDirective(csp, "script-src")
	if scriptSrc == "" {
		t.Fatalf("CSP %q has no script-src", csp)
	}

	inline := regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`).FindAllStringSubmatch(w.Body.String(), -1)
	found := false
	for _, m := range inline {
		attrs, body := m[1], m[2]
		if strings.Contains(attrs, " src=") {
			continue
		}
		found = true
		if strings.TrimSpace(body) == "" {
			continue // nothing to execute; data-* config is read by the bundle
		}
		sum := sha256.Sum256([]byte(body))
		hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		if !strings.Contains(scriptSrc, hash) && !strings.Contains(scriptSrc, "'unsafe-inline'") {
			t.Errorf("inline script %q is blocked by script-src %q (add %s)", body, scriptSrc, hash)
		}
	}
	if !found {
		t.Fatal("expected the inline api-reference config script")
	}
}

// Resources the pinned bundle loads at runtime (see scalarCSP).
func TestServeScalar_CSPAllowsBundleResources(t *testing.T) {
	w := httptest.NewRecorder()
	serveScalar(w, httptest.NewRequest(http.MethodGet, "/openapi", nil))
	csp := w.Header().Get("Content-Security-Policy")
	for directive, want := range map[string]string{
		"font-src":  "https://fonts.scalar.com",
		"style-src": "'unsafe-inline'",
		"img-src":   "blob:",
		"media-src": "blob:",
	} {
		if !strings.Contains(" "+cspDirective(csp, directive)+" ", " "+want+" ") {
			t.Errorf("CSP %s %q does not allow %s", directive, cspDirective(csp, directive), want)
		}
	}
}

// cspDirective returns the source list of directive in csp ("" if absent).
func cspDirective(csp, directive string) string {
	for _, d := range strings.Split(csp, ";") {
		name, sources, _ := strings.Cut(strings.TrimSpace(d), " ")
		if name == directive {
			return sources
		}
	}
	return ""
}

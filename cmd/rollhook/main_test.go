package main

import (
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

package registry

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeReq creates a GET request to path with the given Authorization header.
func makeReq(path, header string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	return req
}

func TestValidateProxyAuth_Bearer(t *testing.T) {
	secret := "test-secret-ok"
	if !validateProxyAuth(makeReq("/v2/", "Bearer "+secret), secret) {
		t.Error("expected true for valid Bearer token")
	}
}

func TestValidateProxyAuth_Basic_AnyUsername(t *testing.T) {
	secret := "test-secret-ok"

	for _, username := range []string{"rollhook", "anyuser", "docker", ""} {
		creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
		if !validateProxyAuth(makeReq("/v2/", "Basic "+creds), secret) {
			t.Errorf("expected true for username=%q with correct password", username)
		}
	}
}

func TestValidateProxyAuth_InvalidToken(t *testing.T) {
	secret := "test-secret-ok"

	cases := []struct {
		name   string
		header string
	}{
		{"wrong Bearer", "Bearer wrong-token"},
		{"wrong Basic password", "Basic " + base64.StdEncoding.EncodeToString([]byte("user:wrongpass"))},
		{"malformed Basic", "Basic not-base64!!!"},
		{"Basic no colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolon"))},
	}

	for _, tc := range cases {
		if validateProxyAuth(makeReq("/v2/myapp/manifests/latest", tc.header), secret) {
			t.Errorf("%s: expected false, got true", tc.name)
		}
	}
}

func TestValidateProxyAuth_Missing(t *testing.T) {
	secret := "test-secret-ok"
	if validateProxyAuth(makeReq("/v2/", ""), secret) {
		t.Error("expected false for empty header")
	}
	if validateProxyAuth(makeReq("/v2/", "Token something"), secret) {
		t.Error("expected false for unknown auth scheme")
	}
}

func TestExtractRepoFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/v2/", ""},
		{"/v2", ""},
		{"/v2/myapp/manifests/latest", "myapp"},
		{"/v2/myapp/manifests/sha256:abc", "myapp"},
		{"/v2/myorg/myapp/manifests/latest", "myorg/myapp"},
		{"/v2/myapp/blobs/sha256:abc", "myapp"},
		{"/v2/myapp/blobs/uploads/", "myapp"},
		{"/v2/myapp/blobs/uploads/someuuid", "myapp"},
		{"/v2/myapp/tags/list", "myapp"},
		{"/v2/myorg/myapp/tags/list", "myorg/myapp"},
		{"/v2/myapp/referrers/sha256:abc", "myapp"},
		// repo names that contain OCI operation keywords — must use last occurrence
		{"/v2/org/manifests/app/manifests/latest", "org/manifests/app"},
		{"/v2/org/blobs/app/blobs/sha256:abc", "org/blobs/app"},
	}
	for _, tc := range cases {
		got := extractRepoFromPath(tc.path)
		if got != tc.want {
			t.Errorf("extractRepoFromPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestNewProxy_ForwardedRequest pins what Zot sees on the wire: the client's Host,
// the injected Zot credentials, the upstream X-Forwarded-* chain plus the client IP,
// and relative Location headers on the way back.
func TestNewProxy_ForwardedRequest(t *testing.T) {
	secret := "test-secret-ok"
	var got *http.Request
	zot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Header().Set("Location", "http://127.0.0.1:5000/v2/app/blobs/uploads/abc")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer zot.Close()

	req := httptest.NewRequest(http.MethodPost, "http://rollhook.example.com/v2/app/blobs/uploads/?mount=x", nil)
	req.RemoteAddr = "10.0.0.7:41234"
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Forwarded-Host", "rollhook.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	NewProxy(zot.URL, secret).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if loc := rec.Header().Get("Location"); loc != "/v2/app/blobs/uploads/abc" {
		t.Errorf("Location = %q, want relative path", loc)
	}
	if got == nil {
		t.Fatal("request did not reach upstream")
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(ZotUser+":"+secret))
	checks := []struct{ name, got, want string }{
		{"Host", got.Host, "rollhook.example.com"},
		{"path", got.URL.Path, "/v2/app/blobs/uploads/"},
		{"query", got.URL.RawQuery, "mount=x"},
		{"Authorization", got.Header.Get("Authorization"), wantAuth},
		{"X-Forwarded-For", got.Header.Get("X-Forwarded-For"), "203.0.113.9, 10.0.0.7"},
		{"X-Forwarded-Host", got.Header.Get("X-Forwarded-Host"), "rollhook.example.com"},
		{"X-Forwarded-Proto", got.Header.Get("X-Forwarded-Proto"), "https"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

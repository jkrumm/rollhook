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

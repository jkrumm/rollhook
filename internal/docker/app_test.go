package docker

import (
	"testing"
)

func TestAppFromRef(t *testing.T) {
	tests := []struct {
		imageTag string
		wantApp  string
	}{
		// Simple image:tag
		{"myapp:v1", "myapp"},
		{"myapp:latest", "myapp"},
		{"myapp", "myapp"},

		// Registry host:port/image:tag
		{"localhost:7700/myapp:v1", "myapp"},
		{"registry.example.com/myapp:sha-abc123", "myapp"},

		// registry/org/image:tag
		{"ghcr.io/org/myapp:sha-abc123", "myapp"},
		{"ghcr.io/user/myapp:v2", "myapp"},
		{"registry.io/namespace/subgroup/myapp:v1", "myapp"},

		// registry host:port/org/image:tag
		{"localhost:7700/rollhook-e2e-hello:v1", "rollhook-e2e-hello"},
		{"127.0.0.1:5000/org/api:sha256", "api"},

		// No tag (no colon after last slash)
		{"ghcr.io/org/notagapp", "notagapp"},

		// Dash in app name
		{"my-app:v1", "my-app"},
		{"registry.io/org/my-app:v1", "my-app"},

		// Digest references
		{"localhost:7700/myapp@sha256:deadbeef", "myapp"},
		{"ghcr.io/org/myapp:v1@sha256:deadbeef", "myapp"},
	}

	for _, tt := range tests {
		t.Run(tt.imageTag, func(t *testing.T) {
			got := AppFromRef(tt.imageTag)
			if got != tt.wantApp {
				t.Errorf("AppFromRef(%q) = %q, want %q", tt.imageTag, got, tt.wantApp)
			}
		})
	}
}

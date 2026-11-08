package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
	"github.com/jkrumm/rollhook/internal/notifier"
)

func TestExtractApp(t *testing.T) {
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

		// Digests
		{"reg/app@sha256:deadbeef", "app"},
		{"ghcr.io/org/app:v1@sha256:deadbeef", "app"},
		{"localhost:5000/app@sha256:deadbeef", "app"},

		// Registry with port, no tag
		{"registry.example.com:5000/team/app", "app"},

		// Dash in app name
		{"my-app:v1", "my-app"},
		{"registry.io/org/my-app:v1", "my-app"},
	}

	for _, tt := range tests {
		t.Run(tt.imageTag, func(t *testing.T) {
			got := extractApp(tt.imageTag)
			if got != tt.wantApp {
				t.Errorf("extractApp(%q) = %q, want %q", tt.imageTag, got, tt.wantApp)
			}
		})
	}
}

func TestNotifyAsync_DoesNotBlockAndDrainWaits(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	e := &Executor{
		queue: NewQueue(context.Background()),
		notify: func(context.Context, notifier.Config, db.Job) {
			close(started)
			<-release
		},
	}

	returned := make(chan struct{})
	go func() {
		e.notifyAsync(notifier.Config{}, db.Job{ID: "j"})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("notifyAsync blocked the caller")
	}
	<-started

	if e.Drain(100 * time.Millisecond) {
		t.Fatal("Drain returned true while notify still in flight")
	}

	e2 := &Executor{
		queue: NewQueue(context.Background()),
		notify: func(context.Context, notifier.Config, db.Job) {
			<-release
		},
	}
	e2.notifyAsync(notifier.Config{}, db.Job{ID: "j2"})
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(release)
	}()
	if !e2.Drain(5 * time.Second) {
		t.Fatal("Drain should return true once in-flight notify completes")
	}
}

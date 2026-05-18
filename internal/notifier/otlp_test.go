package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jkrumm/rollhook/internal/db"
)

func findAttr(attrs []otlpKeyValue, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.StringValue, true
		}
	}
	return "", false
}

func TestNotifier_OTLP_SuccessEnvelope(t *testing.T) {
	var received otlpPayload
	var path string
	var contentType string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{OTLPEndpoint: ts.URL}, job)

	if path != "/v1/logs" {
		t.Errorf("path = %q, want /v1/logs", path)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	if len(received.ResourceLogs) != 1 {
		t.Fatalf("resourceLogs len = %d, want 1", len(received.ResourceLogs))
	}
	rl := received.ResourceLogs[0]

	svc, ok := findAttr(rl.Resource.Attributes, "service.name")
	if !ok || svc != "rollhook" {
		t.Errorf("service.name = %q (found=%v), want rollhook", svc, ok)
	}
	if _, ok := findAttr(rl.Resource.Attributes, "deployment.environment"); ok {
		t.Error("deployment.environment should be omitted when env unset")
	}

	if len(rl.ScopeLogs) != 1 || len(rl.ScopeLogs[0].LogRecords) != 1 {
		t.Fatalf("expected 1 scope/log record, got scopes=%d", len(rl.ScopeLogs))
	}
	rec := rl.ScopeLogs[0].LogRecords[0]

	if rec.SeverityNumber != otlpSeverityInfo {
		t.Errorf("severityNumber = %d, want %d", rec.SeverityNumber, otlpSeverityInfo)
	}
	if rec.SeverityText != "INFO" {
		t.Errorf("severityText = %q, want INFO", rec.SeverityText)
	}
	if rec.TimeUnixNano == "" {
		t.Error("timeUnixNano is empty")
	}
	if !strings.Contains(rec.Body.StringValue, "Deploy: myapp localhost:7700/myapp:v2") {
		t.Errorf("body = %q, want Deploy: myapp <image>", rec.Body.StringValue)
	}

	checks := map[string]string{
		"deploy.service":   "myapp",
		"deploy.image_tag": "localhost:7700/myapp:v2",
		"deploy.status":    "success",
		"deploy.job_id":    "test-job-id",
	}
	for k, want := range checks {
		got, ok := findAttr(rec.Attributes, k)
		if !ok || got != want {
			t.Errorf("attr %s = %q (found=%v), want %q", k, got, ok, want)
		}
	}
}

func TestNotifier_OTLP_FailureSeverity(t *testing.T) {
	var received otlpPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	errMsg := "boom"
	job := makeJob(db.StatusFailed, &errMsg)
	Notify(context.Background(), Config{OTLPEndpoint: ts.URL}, job)

	rec := received.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
	if rec.SeverityNumber != otlpSeverityError {
		t.Errorf("severityNumber = %d, want %d", rec.SeverityNumber, otlpSeverityError)
	}
	if rec.SeverityText != "ERROR" {
		t.Errorf("severityText = %q, want ERROR", rec.SeverityText)
	}
	status, _ := findAttr(rec.Attributes, "deploy.status")
	if status != "failed" {
		t.Errorf("deploy.status = %q, want failed", status)
	}
}

func TestNotifier_OTLP_DeployEnvironment(t *testing.T) {
	var received otlpPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{OTLPEndpoint: ts.URL, DeployEnvironment: "production"}, job)

	env, ok := findAttr(received.ResourceLogs[0].Resource.Attributes, "deployment.environment")
	if !ok || env != "production" {
		t.Errorf("deployment.environment = %q (found=%v), want production", env, ok)
	}
}

func TestNotifier_OTLP_Headers(t *testing.T) {
	var auth, tenant string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		tenant = r.Header.Get("X-Tenant")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{
		OTLPEndpoint: ts.URL,
		OTLPHeaders:  "Authorization=Bearer abc, X-Tenant=acme ",
	}, job)

	if auth != "Bearer abc" {
		t.Errorf("Authorization = %q, want Bearer abc", auth)
	}
	if tenant != "acme" {
		t.Errorf("X-Tenant = %q, want acme", tenant)
	}
}

func TestParseOTLPHeaders(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]string
	}{
		{"", map[string]string{}},
		{"k=v", map[string]string{"k": "v"}},
		{"k1=v1, k2=v2 ", map[string]string{"k1": "v1", "k2": "v2"}},
		{"  Authorization = Bearer abc ,X-Tenant=acme", map[string]string{"Authorization": "Bearer abc", "X-Tenant": "acme"}},
		{"malformed,key=val", map[string]string{"key": "val"}},
		{"k=v=with=equals", map[string]string{"k": "v=with=equals"}},
	}
	for _, c := range cases {
		got := parseOTLPHeaders(c.in)
		if len(got) != len(c.want) {
			t.Errorf("parseOTLPHeaders(%q) len = %d, want %d (%v)", c.in, len(got), len(c.want), got)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("parseOTLPHeaders(%q)[%q] = %q, want %q", c.in, k, got[k], v)
			}
		}
	}
}

func TestNotifier_OTLPError_DoesNotPanic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	Notify(context.Background(), Config{OTLPEndpoint: ts.URL}, makeJob(db.StatusSuccess, nil))
}

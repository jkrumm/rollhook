package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
)

func makeJob(status db.JobStatus, errMsg *string) db.Job {
	return db.Job{
		ID:        "test-job-id",
		App:       "myapp",
		Status:    status,
		ImageTag:  "localhost:7700/myapp:v2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Error:     errMsg,
	}
}

func TestNotifier_Webhook(t *testing.T) {
	var received []byte
	var contentType string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{WebhookURL: ts.URL}, job)

	if len(received) == 0 {
		t.Fatal("webhook received no body")
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	var got db.Job
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("unmarshal webhook body: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("job ID = %q, want %q", got.ID, job.ID)
	}
	if got.App != job.App {
		t.Errorf("job App = %q, want %q", got.App, job.App)
	}
	if got.Status != db.StatusSuccess {
		t.Errorf("job Status = %q, want success", got.Status)
	}
}

func TestNotifier_Pushover(t *testing.T) {
	var received map[string]string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Override the Pushover endpoint to point at our test server.
	orig := pushoverEndpoint
	pushoverEndpoint = ts.URL
	defer func() { pushoverEndpoint = orig }()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{PushoverUserKey: "userkey", PushoverAppToken: "apptoken"}, job)

	if received["token"] != "apptoken" {
		t.Errorf("token = %q, want apptoken", received["token"])
	}
	if received["user"] != "userkey" {
		t.Errorf("user = %q, want userkey", received["user"])
	}
	if received["title"] == "" {
		t.Error("title is empty")
	}
	if received["message"] == "" {
		t.Error("message is empty")
	}
}

func TestNotifier_Pushover_FailureTitle(t *testing.T) {
	var received map[string]string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	orig := pushoverEndpoint
	pushoverEndpoint = ts.URL
	defer func() { pushoverEndpoint = orig }()

	errMsg := "compose file not found"
	job := makeJob(db.StatusFailed, &errMsg)
	Notify(context.Background(), Config{PushoverUserKey: "u", PushoverAppToken: "t"}, job)

	if received["title"] == "" {
		t.Error("title is empty for failed job")
	}
	// Title should contain the failure indicator, not success.
	if received["title"] == "✅ Deployed myapp" {
		t.Error("failed job should not produce success title")
	}
}

func TestNotifier_NotCalledWhenUnconfigured(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Empty config — no notifications should be sent.
	Notify(context.Background(), Config{}, makeJob(db.StatusSuccess, nil))

	if callCount != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", callCount)
	}
}

func TestNotifier_WebhookError_DoesNotPanic(t *testing.T) {
	// Webhook that returns 500 should not propagate the error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	// Should complete without panic or fatal.
	Notify(context.Background(), Config{WebhookURL: ts.URL}, makeJob(db.StatusSuccess, nil))
}

func TestNotifier_Slack_Success(t *testing.T) {
	var received map[string]string
	var contentType string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	job := makeJob(db.StatusSuccess, nil)
	Notify(context.Background(), Config{SlackWebhookURL: ts.URL}, job)

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	text, ok := received["text"]
	if !ok || text == "" {
		t.Fatal("slack payload missing text field")
	}
	if !strings.Contains(text, "✅ Deployed myapp") {
		t.Errorf("text = %q, want success indicator", text)
	}
	if !strings.Contains(text, "localhost:7700/myapp:v2") {
		t.Errorf("text = %q, want image tag", text)
	}
}

func TestNotifier_Slack_Failure(t *testing.T) {
	var received map[string]string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	errMsg := "compose file not found"
	job := makeJob(db.StatusFailed, &errMsg)
	Notify(context.Background(), Config{SlackWebhookURL: ts.URL}, job)

	text := received["text"]
	if !strings.Contains(text, "❌ Deployment failed: myapp") {
		t.Errorf("text = %q, want failure indicator", text)
	}
	if !strings.Contains(text, "compose file not found") {
		t.Errorf("text = %q, want error message", text)
	}
	if !strings.Contains(text, "Status: failed") {
		t.Errorf("text = %q, want status line", text)
	}
}

func TestNotifier_SlackError_DoesNotPanic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	Notify(context.Background(), Config{SlackWebhookURL: ts.URL}, makeJob(db.StatusSuccess, nil))
}

type ntfyCapture struct {
	header http.Header
	body   string
	path   string
	user   string
	pass   string
	hasAut bool
}

func newNtfyServer(t *testing.T, status int) (*httptest.Server, *ntfyCapture) {
	t.Helper()
	c := &ntfyCapture{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.header = r.Header.Clone()
		c.body = string(b)
		c.path = r.URL.Path
		c.user, c.pass, c.hasAut = r.BasicAuth()
		w.WriteHeader(status)
	}))
	t.Cleanup(ts.Close)
	return ts, c
}

func TestNotifier_Ntfy_Success(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates"}, makeJob(db.StatusSuccess, nil))

	if c.path != "/updates" {
		t.Errorf("path = %q, want /updates", c.path)
	}
	if got := c.header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := c.header.Get("Title"); got != "Deployed myapp" {
		t.Errorf("Title = %q", got)
	}
	if got := c.header.Get("Tags"); got != "white_check_mark" {
		t.Errorf("Tags = %q", got)
	}
	if _, ok := c.header["Priority"]; ok {
		t.Error("Priority header must not be set on success")
	}
	if want := "myapp:v2"; c.body != want {
		t.Errorf("body = %q, want %q", c.body, want)
	}
	if _, ok := c.header["Click"]; ok {
		t.Error("Click header must not be set without a dashboard URL")
	}
}

func TestNotifier_Ntfy_Failure(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	errMsg := "container exited"
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates"}, makeJob(db.StatusFailed, &errMsg))

	if got := c.header.Get("Title"); got != "Deployment failed: myapp" {
		t.Errorf("Title = %q", got)
	}
	if got := c.header.Get("Tags"); got != "x" {
		t.Errorf("Tags = %q", got)
	}
	if got := c.header.Get("Priority"); got != "high" {
		t.Errorf("Priority = %q", got)
	}
	if want := "container exited\n\nmyapp:v2"; c.body != want {
		t.Errorf("body = %q, want %q", c.body, want)
	}
}

func TestNotifier_Ntfy_ClickURL(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	cfg := Config{NtfyURL: ts.URL + "/updates", DashboardURL: "https://rollhook.example.com/"}
	Notify(context.Background(), cfg, makeJob(db.StatusSuccess, nil))

	if got := c.header.Get("Click"); got != "https://rollhook.example.com/?job=test-job-id" {
		t.Errorf("Click = %q", got)
	}
}

func TestNotifier_Ntfy_TruncatesLongError(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	errMsg := strings.Repeat("é", ntfyMaxErrorRunes+50)
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates"}, makeJob(db.StatusFailed, &errMsg))

	want := strings.Repeat("é", ntfyMaxErrorRunes) + "…\n\nmyapp:v2"
	if c.body != want {
		t.Errorf("body has %d bytes, want %d", len(c.body), len(want))
	}
}

func TestShortImage(t *testing.T) {
	sha := "b6ec0b4008cb73da336d73139925e2cc8fbee198"
	cases := map[string]string{
		"rollhook.example.com/app:" + sha: "app:b6ec0b40",
		"localhost:7700/app:v2":           "app:v2",
		"ghcr.io/owner/app:1.2.3":         "owner/app:1.2.3",
		"owner/app:latest":                "owner/app:latest",
		"app":                             "app",
		"registry.example.com/app":        "app",
	}
	for in, want := range cases {
		if got := shortImage(in); got != want {
			t.Errorf("shortImage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNotifier_Ntfy_BearerToken(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates", NtfyToken: "tk_abc"}, makeJob(db.StatusSuccess, nil))

	if got := c.header.Get("Authorization"); got != "Bearer tk_abc" {
		t.Errorf("Authorization = %q", got)
	}
}

func TestNotifier_Ntfy_BasicAuthFromURL(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	u := strings.Replace(ts.URL, "http://", "http://:tk_abc@", 1) + "/updates"
	Notify(context.Background(), Config{NtfyURL: u}, makeJob(db.StatusSuccess, nil))

	if !c.hasAut || c.user != "" || c.pass != "tk_abc" {
		t.Errorf("BasicAuth = (%q, %q, %v), want (\"\", tk_abc, true)", c.user, c.pass, c.hasAut)
	}
	if c.path != "/updates" {
		t.Errorf("path = %q, want /updates", c.path)
	}
}

func TestNotifier_Ntfy_TokenWinsOverUserinfo(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	u := strings.Replace(ts.URL, "http://", "http://:url_tok@", 1) + "/updates"
	Notify(context.Background(), Config{NtfyURL: u, NtfyToken: "env_tok"}, makeJob(db.StatusSuccess, nil))

	if got := c.header.Get("Authorization"); got != "Bearer env_tok" {
		t.Errorf("Authorization = %q, want Bearer env_tok", got)
	}
}

func TestNotifier_Ntfy_NoAuth(t *testing.T) {
	ts, c := newNtfyServer(t, http.StatusOK)
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates"}, makeJob(db.StatusSuccess, nil))

	if got := c.header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty", got)
	}
}

func TestNotifier_Ntfy_ServerError(t *testing.T) {
	ts, _ := newNtfyServer(t, http.StatusInternalServerError)
	// Must not panic.
	Notify(context.Background(), Config{NtfyURL: ts.URL + "/updates"}, makeJob(db.StatusSuccess, nil))

	err := sendNtfy(context.Background(), ts.URL+"/updates", "", "", makeJob(db.StatusSuccess, nil))
	if err == nil || err.Error() != "ntfy returned 500" {
		t.Errorf("err = %v, want ntfy returned 500", err)
	}
}

func TestNotifier_Ntfy_InvalidURL(t *testing.T) {
	err := sendNtfy(context.Background(), "://:secret@bad", "", "", makeJob(db.StatusSuccess, nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error leaks credentials: %v", err)
	}
}

func TestNotify_HangingSinkIsBounded(t *testing.T) {
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer ts.Close()
	defer close(release)

	orig := sendTimeout
	sendTimeout = 150 * time.Millisecond
	defer func() { sendTimeout = orig }()

	start := time.Now()
	Notify(context.Background(), Config{WebhookURL: ts.URL, SlackWebhookURL: ts.URL}, makeJob(db.StatusSuccess, nil))
	elapsed := time.Since(start)

	// Two hanging sinks, each bounded by sendTimeout.
	if elapsed > 2*time.Second {
		t.Errorf("Notify took %s with hanging sinks, want bounded by per-send timeout", elapsed)
	}
}

func TestHTTPClient_HasTimeout(t *testing.T) {
	if httpClient.Timeout <= 0 {
		t.Error("httpClient must have a non-zero timeout")
	}
}

package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
	"github.com/jkrumm/rollhook/internal/deploytoken"
)

// Deploy tokens minted by POST /auth/token must only reach the service they were
// issued for — never the admin surface that ROLLHOOK_SECRET unlocks.

func deployTokenHeader(app string) string {
	return "Bearer " + deploytoken.Mint(testSecret, deploytoken.Scope{App: app, Project: "proj", Service: "web"})
}

func doRequest(t *testing.T, srv http.Handler, method, path, body, auth string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func insertJob(t *testing.T, store *db.Store, id, app string) {
	t.Helper()
	j := db.Job{ID: id, App: app, Status: db.StatusSuccess, ImageTag: "reg/" + app + ":v1",
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.Insert(j); err != nil {
		t.Fatalf("insert job: %v", err)
	}
}

func TestDeployToken_RejectedOnListJobs(t *testing.T) {
	srv, _, _ := newTestServer(t)
	w := doRequest(t, srv, http.MethodGet, "/jobs", "", deployTokenHeader("myapp"))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeployToken_InvalidRejected(t *testing.T) {
	srv, store, _ := newTestServer(t)
	insertJob(t, store, "job-1", "myapp")
	forged := "Bearer " + deploytoken.Mint("not-the-secret", deploytoken.Scope{App: "myapp", Project: "proj", Service: "web"})
	w := doRequest(t, srv, http.MethodGet, "/jobs/job-1", "", forged)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	w = doRequest(t, srv, http.MethodGet, "/jobs/job-1/logs", "", forged)
	if w.Code != http.StatusForbidden {
		t.Errorf("logs: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeployToken_DeployOtherAppForbidden(t *testing.T) {
	srv, store, _ := newTestServer(t)
	w := doRequest(t, srv, http.MethodPost, "/deploy?async=true",
		`{"image_tag":"registry.example.com/otherapp:v1"}`, deployTokenHeader("myapp"))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if jobs, _ := store.List("", "", 10); len(jobs) != 0 {
		t.Errorf("no job may be queued for an out-of-scope deploy, got %d", len(jobs))
	}
}

func TestDeployToken_GetJobScoped(t *testing.T) {
	srv, store, _ := newTestServer(t)
	insertJob(t, store, "job-mine", "myapp")
	insertJob(t, store, "job-other", "otherapp")

	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-mine", "", deployTokenHeader("myapp")); w.Code != http.StatusOK {
		t.Errorf("own job: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other", "", deployTokenHeader("myapp")); w.Code != http.StatusNotFound {
		t.Errorf("other app's job: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	// Static secret keeps full access.
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other", "", authHeader()); w.Code != http.StatusOK {
		t.Errorf("static secret: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeployToken_StreamLogsScoped(t *testing.T) {
	srv, store, _ := newTestServer(t)
	// Terminal jobs without a log file return immediately — no streaming wait.
	insertJob(t, store, "job-mine", "myapp")
	insertJob(t, store, "job-other", "otherapp")

	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-mine/logs", "", deployTokenHeader("myapp")); w.Code != http.StatusOK {
		t.Errorf("own job logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other/logs", "", deployTokenHeader("myapp")); w.Code != http.StatusNotFound {
		t.Errorf("other app's logs: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jkrumm/rollhook/internal/db"
	"github.com/jkrumm/rollhook/internal/deploytoken"
	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

const (
	myRepo      = "ghcr.io/org/myapp"
	foreignRepo = "attacker.io/evil/myapp" // same base name, other registry
)

// Deploy tokens minted by POST /auth/token must only reach the service they were
// issued for — never the admin surface that ROLLHOOK_SECRET unlocks.

func deployTokenHeader(repo string) string {
	return "Bearer " + deploytoken.Mint(testSecret, deploytoken.Scope{Repo: repo, Project: "proj", Service: "web"})
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

func insertJob(t *testing.T, store *db.Store, id, imageTag string) {
	t.Helper()
	j := db.Job{ID: id, App: dockerpkg.AppFromRef(imageTag), Status: db.StatusSuccess, ImageTag: imageTag,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.Insert(j); err != nil {
		t.Fatalf("insert job: %v", err)
	}
}

func TestDeployToken_RejectedOnListJobs(t *testing.T) {
	srv, _, _ := newTestServer(t)
	w := doRequest(t, srv, http.MethodGet, "/jobs", "", deployTokenHeader(myRepo))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeployToken_InvalidRejected(t *testing.T) {
	srv, store, _ := newTestServer(t)
	insertJob(t, store, "job-1", myRepo+":v1")
	forged := "Bearer " + deploytoken.Mint("not-the-secret", deploytoken.Scope{Repo: myRepo, Project: "proj", Service: "web"})
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
		`{"image_tag":"registry.example.com/otherapp:v1"}`, deployTokenHeader(myRepo))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if jobs, _ := store.List("", "", 10); len(jobs) != 0 {
		t.Errorf("no job may be queued for an out-of-scope deploy, got %d", len(jobs))
	}
}

func TestDeployToken_GetJobScoped(t *testing.T) {
	srv, store, _ := newTestServer(t)
	insertJob(t, store, "job-mine", myRepo+":v1")
	insertJob(t, store, "job-other", "ghcr.io/org/otherapp:v1")
	insertJob(t, store, "job-foreign", foreignRepo+":v1")

	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-mine", "", deployTokenHeader(myRepo)); w.Code != http.StatusOK {
		t.Errorf("own job: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other", "", deployTokenHeader(myRepo)); w.Code != http.StatusNotFound {
		t.Errorf("other app's job: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	// Same app name (last path segment), different repository → not visible.
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-foreign", "", deployTokenHeader(myRepo)); w.Code != http.StatusNotFound {
		t.Errorf("foreign repo's job: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	// Static secret keeps full access.
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other", "", authHeader()); w.Code != http.StatusOK {
		t.Errorf("static secret: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeployToken_StreamLogsScoped(t *testing.T) {
	srv, store, _ := newTestServer(t)
	// Terminal jobs without a log file return immediately — no streaming wait.
	insertJob(t, store, "job-mine", myRepo+":v1")
	insertJob(t, store, "job-other", "ghcr.io/org/otherapp:v1")
	insertJob(t, store, "job-foreign", foreignRepo+":v1")

	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-mine/logs", "", deployTokenHeader(myRepo)); w.Code != http.StatusOK {
		t.Errorf("own job logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-other/logs", "", deployTokenHeader(myRepo)); w.Code != http.StatusNotFound {
		t.Errorf("other app's logs: expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, srv, http.MethodGet, "/jobs/job-foreign/logs", "", deployTokenHeader(myRepo)); w.Code != http.StatusNotFound {
		t.Errorf("foreign repo's logs: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// The token binds the full repository: an image_tag that only shares the app
// name (last path segment) — on another registry or under another path — must
// be refused before discovery, and nothing may be queued.
func TestDeployToken_DeployForeignRepoSameBaseNameForbidden(t *testing.T) {
	for _, imageTag := range []string{
		foreignRepo + ":v1",
		"attacker.io/org/myapp:v1", // same path, other registry
		"ghcr.io/evil/myapp:v1",    // same registry, other path
		"myapp:v1",                 // bare name
		foreignRepo + "@sha256:0000000000000000000000000000000000000000000000000000000000000000",
	} {
		srv, store, _ := newTestServer(t)
		w := doRequest(t, srv, http.MethodPost, "/deploy?async=true",
			fmt.Sprintf(`{"image_tag":%q}`, imageTag), deployTokenHeader(myRepo))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d: %s", imageTag, w.Code, w.Body.String())
		}
		if jobs, _ := store.List("", "", 10); len(jobs) != 0 {
			t.Errorf("%s: no job may be queued for an out-of-scope deploy, got %d", imageTag, len(jobs))
		}
	}
}

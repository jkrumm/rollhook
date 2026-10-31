package steps_test

import (
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"

	"github.com/jkrumm/rollhook/internal/jobs/steps"
)

func TestExtractImageName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"nginx", "nginx"},
		{"nginx:latest", "nginx"},
		{"localhost:5000/app:v1", "localhost:5000/app"},
		{"localhost:7700/rollhook-e2e-hello:v1", "localhost:7700/rollhook-e2e-hello"},
		{"registry.example.com/org/app:sha256abc", "registry.example.com/org/app"},
		{"myapp", "myapp"},
		{"myapp:1.2.3", "myapp"},
	}
	for _, tc := range cases {
		got := steps.ExtractImageName(tc.input)
		if got != tc.want {
			t.Errorf("ExtractImageName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// svc returns a running container started by Compose as project/service.
func svc(id, image, project, service string) container.Summary {
	return container.Summary{ID: id, Image: image, Names: []string{"/" + id}, Labels: map[string]string{
		"com.docker.compose.project": project,
		"com.docker.compose.service": service,
	}}
}

func TestFindMatchingContainer(t *testing.T) {
	containers := []container.Summary{
		svc("aaa", "nginx:latest", "proxy", "nginx"),
		svc("bbb", "localhost:5000/app", "app", "web"),
		svc("ccc", "localhost:5000/myapp:v2", "myapp", "web"),
		{ID: "ddd", Image: "localhost:5000/manual:v1"}, // not started by Compose
	}

	cases := []struct {
		imageName string
		wantID    string
	}{
		{"nginx", "aaa"},                    // bare name matches "nginx:latest"
		{"localhost:5000/app", "bbb"},       // exact match without tag
		{"localhost:5000/myapp", "ccc"},     // exact match, container has a tag
		{"app", "bbb"},                      // bare name matches on a path-segment boundary
		{"localhost:5000/other", ""},        // no match
		{"localhost:5000/app:v3", ""},       // full tag doesn't match bare image
		{"pp", ""},                          // not a path-segment boundary
		{"localhost:5000/manual", ""},       // non-Compose containers are never discovered
		{"other.io/localhost:5000/app", ""}, // longer than the repository
	}

	for _, tc := range cases {
		got, err := steps.FindMatchingContainer(containers, tc.imageName, steps.Target{})
		if err != nil {
			t.Errorf("FindMatchingContainer(%q) unexpected error: %v", tc.imageName, err)
			continue
		}
		if tc.wantID == "" {
			if got != nil {
				t.Errorf("FindMatchingContainer(%q) = %q, want nil", tc.imageName, got.ID)
			}
		} else {
			if got == nil {
				t.Errorf("FindMatchingContainer(%q) = nil, want %q", tc.imageName, tc.wantID)
			} else if got.ID != tc.wantID {
				t.Errorf("FindMatchingContainer(%q) = %q, want %q", tc.imageName, got.ID, tc.wantID)
			}
		}
	}
}

func TestFindMatchingContainer_ExactMatchWins(t *testing.T) {
	containers := []container.Summary{
		svc("aaa", "registry.example.com/myapp:v1", "a", "web"),
		svc("bbb", "registry.example.com/team/myapp:v1", "b", "web"),
	}
	got, err := steps.FindMatchingContainer(containers, "registry.example.com/team/myapp", steps.Target{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ID != "bbb" {
		t.Fatalf("expected exact match bbb, got %+v", got)
	}
}

func TestFindMatchingContainer_AmbiguousBaseName(t *testing.T) {
	containers := []container.Summary{
		svc("aaa", "registry.example.com/myapp:v1", "a", "web"),
		svc("bbb", "registry.example.com/team/myapp:v1", "b", "web"),
	}
	got, err := steps.FindMatchingContainer(containers, "myapp", steps.Target{})
	if !errors.Is(err, steps.ErrAmbiguousImage) {
		t.Errorf("FindMatchingContainer(myapp) = %+v, %v; want ErrAmbiguousImage", got, err)
	}
}

// A qualified name is matched on path segments, so a repository that only
// shares the base name is neither a match nor a reason to report ambiguity.
func TestFindMatchingContainer_QualifiedNameNarrowsBySuffix(t *testing.T) {
	containers := []container.Summary{
		svc("aaa", "ghcr.io/other/myapp:v1", "other", "web"),
		svc("bbb", "ghcr.io/org/myapp:v1", "org", "web"),
	}
	cases := []struct{ imageName, wantID string }{
		{"org/myapp", "bbb"},
		{"other/myapp", "aaa"},
		{"ghcr.io/org/myapp", "bbb"},
		{"nope/myapp", ""},
	}
	for _, tc := range cases {
		got, err := steps.FindMatchingContainer(containers, tc.imageName, steps.Target{})
		if err != nil {
			t.Errorf("FindMatchingContainer(%q) unexpected error: %v", tc.imageName, err)
			continue
		}
		gotID := ""
		if got != nil {
			gotID = got.ID
		}
		if gotID != tc.wantID {
			t.Errorf("FindMatchingContainer(%q) = %q, want %q", tc.imageName, gotID, tc.wantID)
		}
	}
}

// A registry-qualified name never matches a container from another registry,
// even when the rest of the path is identical.
func TestFindMatchingContainer_ForeignRegistryDoesNotMatch(t *testing.T) {
	containers := []container.Summary{svc("aaa", "ghcr.io/org/myapp:v1", "org", "web")}
	for _, name := range []string{"attacker.io/evil/myapp", "attacker.io/org/myapp", "evil/myapp"} {
		got, err := steps.FindMatchingContainer(containers, name, steps.Target{})
		if err != nil || got != nil {
			t.Errorf("FindMatchingContainer(%q) = %+v, %v; want nil, nil", name, got, err)
		}
	}
}

func TestFindMatchingContainer_ReplicasAreNotAmbiguous(t *testing.T) {
	// Old and new replicas of one service during a rollout run the same repo;
	// the newest replica wins regardless of listing order.
	older := svc("bbb", "registry.example.com/myapp:v1", "proj", "web")
	older.Created = 100
	newer := svc("aaa", "registry.example.com/myapp:v2", "proj", "web")
	newer.Created = 200
	for _, containers := range [][]container.Summary{{older, newer}, {newer, older}} {
		got, err := steps.FindMatchingContainer(containers, "myapp", steps.Target{})
		if err != nil || got == nil || got.ID != "aaa" {
			t.Fatalf("got %+v, %v; want aaa, nil", got, err)
		}
	}
}

// Two Compose services running the same repository (e.g. web + worker, or the
// same app in two projects) must not be resolved by listing order.
func TestFindMatchingContainer_SameRepoDifferentServices(t *testing.T) {
	web := svc("aaa", "ghcr.io/org/myapp:v1", "shop", "web")
	worker := svc("bbb", "ghcr.io/org/myapp:v1", "shop", "worker")
	staging := svc("ccc", "ghcr.io/org/myapp:v1", "shop-staging", "web")
	all := []container.Summary{web, worker, staging}

	for _, name := range []string{"myapp", "ghcr.io/org/myapp"} {
		for _, containers := range [][]container.Summary{all, {staging, worker, web}} {
			got, err := steps.FindMatchingContainer(containers, name, steps.Target{})
			if !errors.Is(err, steps.ErrAmbiguousImage) {
				t.Errorf("FindMatchingContainer(%q) = %+v, %v; want ErrAmbiguousImage", name, got, err)
			}
		}
	}

	cases := []struct {
		target steps.Target
		wantID string
		ambig  bool
	}{
		{steps.Target{Project: "shop", Service: "worker"}, "bbb", false},
		{steps.Target{Project: "shop-staging"}, "ccc", false},
		{steps.Target{Project: "shop", Service: "web"}, "aaa", false},
		{steps.Target{Service: "web"}, "", true}, // shop/web vs shop-staging/web
		{steps.Target{Project: "shop"}, "", true},
		{steps.Target{Project: "nope", Service: "web"}, "", false},
	}
	for _, tc := range cases {
		got, err := steps.FindMatchingContainer(all, "myapp", tc.target)
		if tc.ambig {
			if !errors.Is(err, steps.ErrAmbiguousImage) {
				t.Errorf("target %+v: got %+v, %v; want ErrAmbiguousImage", tc.target, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("target %+v: unexpected error %v", tc.target, err)
			continue
		}
		gotID := ""
		if got != nil {
			gotID = got.ID
		}
		if gotID != tc.wantID {
			t.Errorf("target %+v: got %q, want %q", tc.target, gotID, tc.wantID)
		}
	}
}

func TestExtractComposeInfo_Success(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project.config_files": "/app/compose.yml,/app/override.yml",
		"com.docker.compose.service":              "web",
		"com.docker.compose.project":              "myproject",
	}
	result, err := steps.ExtractComposeInfo(labels, "mycontainer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ComposePath != "/app/compose.yml" {
		t.Errorf("ComposePath = %q, want %q", result.ComposePath, "/app/compose.yml")
	}
	if result.Service != "web" {
		t.Errorf("Service = %q, want %q", result.Service, "web")
	}
	if result.Project != "myproject" {
		t.Errorf("Project = %q, want %q", result.Project, "myproject")
	}
}

func TestExtractComposeInfo_NilLabels(t *testing.T) {
	_, err := steps.ExtractComposeInfo(nil, "mycontainer")
	if err == nil {
		t.Error("expected error for nil labels")
	}
}

func TestExtractComposeInfo_MissingConfigFiles(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.service": "web",
		"com.docker.compose.project": "myproject",
	}
	_, err := steps.ExtractComposeInfo(labels, "mycontainer")
	if err == nil {
		t.Error("expected error for missing config_files label")
	}
}

func TestExtractComposeInfo_MissingService(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project.config_files": "/app/compose.yml",
		"com.docker.compose.project":              "myproject",
	}
	_, err := steps.ExtractComposeInfo(labels, "mycontainer")
	if err == nil {
		t.Error("expected error for missing service label")
	}
}

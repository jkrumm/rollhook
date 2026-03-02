package steps_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkrumm/rollhook/internal/jobs/steps"
)

func writeCompose(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	return path
}

func TestValidate_RelativePath(t *testing.T) {
	err := steps.Validate("compose.yml", "web", "myapp:v1", nil)
	if err == nil {
		t.Error("expected error for relative path")
	}
}

func TestValidate_MissingFile(t *testing.T) {
	err := steps.Validate("/tmp/rollhook-nonexistent-xyz/compose.yml", "web", "myapp:v1", nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestValidate_ServiceNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  api:
    image: myapp:v1
`)
	err := steps.Validate(path, "web", "myapp:v1", nil)
	if err == nil {
		t.Error("expected error for missing service")
	}
}

func TestValidate_Success(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: localhost:7700/myapp:v1
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 5s
      timeout: 3s
      retries: 3
`)
	err := steps.Validate(path, "web", "localhost:7700/myapp:v1", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_BuildOnlyService(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    build: .
`)
	// Service with no image: field — image check is skipped
	err := steps.Validate(path, "web", "myapp:v1", nil)
	if err != nil {
		t.Errorf("unexpected error for build-only service: %v", err)
	}
}

func TestValidate_NoHealthcheckWarning(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: myapp:v1
`)
	var warnings []string
	logFn := func(s string) { warnings = append(warnings, s) }
	err := steps.Validate(path, "web", "myapp:v1", logFn)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Error("expected a healthcheck warning, got none")
	}
	if !strings.Contains(warnings[0], "healthcheck") {
		t.Errorf("expected warning to mention healthcheck, got: %q", warnings[0])
	}
}

func TestValidate_NoHealthcheckNoLogFn(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: myapp:v1
`)
	// nil logFn must not panic even when healthcheck is missing
	if err := steps.Validate(path, "web", "myapp:v1", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Image mismatch is no longer validated — the fragile strings.Contains check
// was removed in favour of letting the discover step catch mismatches at runtime.
// (The imageTag is used to find a running container; wrong images simply won't
// have a matching container and discovery returns "no container found".)
func TestValidate_ImageMismatchNoLongerErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: other-image:v1
`)
	err := steps.Validate(path, "web", "myapp:v1", nil)
	if err != nil {
		t.Errorf("image mismatch should not be an error at validate time, got: %v", err)
	}
}

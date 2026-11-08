package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

const pollIntervalMS = 1_000

// Timing knobs; package vars so tests can shrink them.
var (
	newContainerPollInterval = 500 * time.Millisecond
	newContainerPollAttempts = 10
	healthPollInterval       = pollIntervalMS * time.Millisecond
	rollbackTimeout          = 30 * time.Second
)

// composeRunner runs `docker compose <args...>` in dir and returns the combined output.
type composeRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// runCompose is the production composeRunner. It passes only an allow-listed
// environment so RollHook's own VERSION/PORT/DATA_DIR never shadow the app's .env.
func runCompose(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = dir
	cmd.Env = composeEnv(os.Environ())
	return cmd.CombinedOutput()
}

// Rollout performs a zero-downtime rolling deploy:
// scale up new containers, wait for health, drain old containers.
// On any failure after scale-up it rolls back new containers before returning the error.
func Rollout(ctx context.Context, cli *client.Client, composePath, service, project, imageTag string, healthTimeoutMS int, logFn func(string)) error {
	return rollout(ctx, cli, runCompose, composePath, service, project, imageTag, healthTimeoutMS, logFn)
}

func rollout(ctx context.Context, api dockerpkg.ContainerAPI, compose composeRunner, composePath, service, project, imageTag string, healthTimeoutMS int, rawLog func(string)) (err error) {
	cwd := filepath.Dir(composePath)

	// Replicas are awaited in parallel, so the log callback must be goroutine-safe.
	var logMu sync.Mutex
	logFn := func(line string) {
		logMu.Lock()
		defer logMu.Unlock()
		rawLog(line)
	}

	// 1. Capture old container IDs before scale-up. Stale leftovers (exited/created/dead)
	// are removed first: they do not serve traffic, must not inflate the replica count,
	// and `compose up --no-recreate` would otherwise restart them as "new" replicas.
	existing, err := dockerpkg.ListServiceContainers(ctx, api, project, service)
	if err != nil {
		return err
	}
	oldIDs := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		if dockerpkg.IsStale(c) {
			removeStale(ctx, api, c.ID, logFn)
			continue
		}
		oldIDs[c.ID] = struct{}{}
	}

	scaleCount := scaleTarget(len(oldIDs))
	logFn(fmt.Sprintf("[rollout] Scaling %s: %d→%d (IMAGE_TAG=%s)", service, len(oldIDs), scaleCount, imageTag))

	// Write IMAGE_TAG into a job-scoped temp env file — never touch the user's .env.
	// Docker Compose v2 --env-file replaces the auto-loaded .env entirely, so we read
	// the user's .env (if present), merge IMAGE_TAG in, and write a temp file. This
	// preserves all the user's existing variables while overriding IMAGE_TAG.
	tmpEnv, err := writeTempEnv(cwd, imageTag)
	if err != nil {
		return fmt.Errorf("rollout: prepare env file: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(tmpEnv); removeErr != nil {
			logFn(fmt.Sprintf("[rollout] Warning: failed to remove temp env file: %s", removeErr))
		}
	}()

	// From here on any failure may have left new containers behind.
	defer func() {
		if err != nil {
			cleanupNewContainers(api, project, service, oldIDs, err.Error(), logFn)
		}
	}()

	// 2. Scale up via docker compose subprocess.
	out, err := compose(ctx, cwd,
		"-f", composePath,
		"--env-file", tmpEnv,
		"up", "-d", "--no-recreate",
		"--scale", fmt.Sprintf("%s=%d", service, scaleCount),
	)
	if err != nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				logFn(fmt.Sprintf("[rollout] %s", line))
			}
		}
		return fmt.Errorf("docker compose up --scale failed: %w", err)
	}

	// 3. Find new containers — poll until all expected replicas appear.
	expectedNew := scaleCount - len(oldIDs)
	if expectedNew < 1 {
		expectedNew = 1
	}
	newContainers, err := pollNewContainers(ctx, api, project, service, oldIDs, expectedNew)
	if err != nil {
		return err
	}

	// 4. Wait for all new containers in parallel against one shared deadline.
	if err := waitAllHealthy(ctx, api, newContainers, healthTimeoutMS, logFn); err != nil {
		return fmt.Errorf("rolling deploy failed: %w", err)
	}

	// 5. All new containers healthy — drain old containers.
	for id := range oldIDs {
		short := id[:min(12, len(id))]
		logFn(fmt.Sprintf("[rollout] Draining old container %s", short))
		if err := dockerpkg.StopContainer(ctx, api, id); err != nil {
			logFn(fmt.Sprintf("[rollout] Warning stopping %s: %s", short, err))
		}
		if err := dockerpkg.RemoveContainer(ctx, api, id); err != nil {
			logFn(fmt.Sprintf("[rollout] Warning removing %s: %s", short, err))
		}
	}

	logFn(fmt.Sprintf("[rollout] Service %s rolled out successfully", service))
	return nil
}

// removeStale best-effort removes a leftover non-running container.
func removeStale(ctx context.Context, api dockerpkg.ContainerAPI, id string, logFn func(string)) {
	short := id[:min(12, len(id))]
	logFn(fmt.Sprintf("[rollout] Removing stale container %s", short))
	if err := dockerpkg.RemoveContainer(ctx, api, id); err != nil {
		logFn(fmt.Sprintf("[rollout] Warning removing stale %s: %s", short, err))
	}
}

// waitAllHealthy waits for every container concurrently; the first failure
// cancels the rest. All replicas share one deadline (healthTimeoutMS from now).
func waitAllHealthy(ctx context.Context, api dockerpkg.ContainerAPI, containers []container.Summary, timeoutMS int, logFn func(string)) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		firstMu  sync.Mutex
		firstErr error
	)
	for i, c := range containers {
		id := c.ID
		logFn(fmt.Sprintf("[rollout] Waiting for %s (%d/%d)", id[:min(12, len(id))], i+1, len(containers)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := waitHealthy(wctx, api, id, timeoutMS, logFn); err != nil {
				firstMu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				firstMu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}

// cleanupNewContainers rolls back every container that is not in oldIDs. It uses
// a fresh context so a cancelled deploy context does not skip the cleanup.
func cleanupNewContainers(api dockerpkg.ContainerAPI, project, service string, oldIDs map[string]struct{}, reason string, logFn func(string)) {
	lctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()
	created, err := findNewContainers(lctx, api, project, service, oldIDs)
	if err != nil {
		logFn(fmt.Sprintf("[rollout] Rollback cleanup error: %s", err))
		return
	}
	if len(created) == 0 {
		return
	}
	ids := make([]string, len(created))
	for i, c := range created {
		ids[i] = c.ID
	}
	rollbackContainers(api, ids, reason, logFn)
}

// scaleTarget returns the number of replicas to scale up to.
// First deploy (no existing containers) starts 1 replica; subsequent deploys double.
func scaleTarget(currentCount int) int {
	if currentCount == 0 {
		return 1
	}
	return currentCount * 2
}

// waitHealthy polls container health until healthy, unhealthy, or timeout.
func waitHealthy(ctx context.Context, cli dockerpkg.ContainerAPI, id string, timeoutMS int, logFn func(string)) error {
	short := id[:min(12, len(id))]
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	start := time.Now()
	lastProgress := time.Now()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("container %s did not become healthy within %ds", short, timeoutMS/1000)
		}

		if time.Since(lastProgress) >= 10*time.Second {
			logFn(fmt.Sprintf("[rollout] Still waiting for %s... (%ds)", short, int(time.Since(start).Seconds())))
			lastProgress = time.Now()
		}

		detail, err := dockerpkg.InspectContainer(ctx, cli, id)
		if err != nil {
			return fmt.Errorf("container inspection failed: %w", err)
		}

		if detail.State.Health == nil {
			return fmt.Errorf("container %s has no healthcheck — add a HEALTHCHECK to your service for zero-downtime deploys", short)
		}

		switch detail.State.Health.Status {
		case "healthy":
			elapsed := time.Since(start).Seconds()
			logFn(fmt.Sprintf("[rollout] %s healthy after %.1fs", short, elapsed))
			return nil
		case "unhealthy":
			return fmt.Errorf("container %s became unhealthy", short)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthPollInterval):
		}
	}
}

// rollbackContainers stops and removes the given container IDs.
// Uses a background context with 30s timeout per container so cleanup completes even if
// the parent context is cancelled, but doesn't hang indefinitely.
func rollbackContainers(cli dockerpkg.ContainerAPI, ids []string, reason string, logFn func(string)) {
	logFn(fmt.Sprintf("[rollout] Rollback triggered: %s", reason))
	for _, id := range ids {
		short := id[:min(12, len(id))]
		logFn(fmt.Sprintf("[rollout] Rollback: stopping new container %s", short))

		// Give each container operation 30s to complete, independent of parent context.
		bctx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
		if err := dockerpkg.StopContainer(bctx, cli, id); err != nil {
			logFn(fmt.Sprintf("[rollout] Rollback cleanup error: %s", err))
		}
		if err := dockerpkg.RemoveContainer(bctx, cli, id); err != nil {
			logFn(fmt.Sprintf("[rollout] Rollback cleanup error: %s", err))
		}
		cancel()
	}
}

// pollNewContainers retries findNewContainers up to 10 times with 500ms delays
// to handle Docker API propagation delay after compose scale-up.
// Respects context cancellation to avoid blocking after SIGTERM.
func pollNewContainers(ctx context.Context, cli dockerpkg.ContainerAPI, project, service string, oldIDs map[string]struct{}, expectedNew int) ([]container.Summary, error) {
	found, err := findNewContainers(ctx, cli, project, service, oldIDs)
	if err != nil {
		return nil, err
	}
	if len(found) >= expectedNew {
		return found, nil
	}

	ticker := time.NewTicker(newContainerPollInterval)
	defer ticker.Stop()

	for attempt := 0; attempt < newContainerPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			found, err = findNewContainers(ctx, cli, project, service, oldIDs)
			if err != nil {
				return nil, err
			}
			if len(found) >= expectedNew {
				return found, nil
			}
		}
	}

	return nil, fmt.Errorf("scale-up produced only %d/%d new containers for service %s after %s", len(found), expectedNew, service, time.Duration(newContainerPollAttempts)*newContainerPollInterval)
}

func findNewContainers(ctx context.Context, cli dockerpkg.ContainerAPI, project, service string, oldIDs map[string]struct{}) ([]container.Summary, error) {
	all, err := dockerpkg.ListServiceContainers(ctx, cli, project, service)
	if err != nil {
		return nil, err
	}
	var result []container.Summary
	for _, c := range all {
		if _, isOld := oldIDs[c.ID]; !isOld {
			result = append(result, c)
		}
	}
	return result, nil
}

// writeTempEnv reads the user's .env (if present), merges IMAGE_TAG, and writes
// a temp file. Returns the temp file path; caller is responsible for removing it.
func writeTempEnv(cwd, imageTag string) (string, error) {
	content := ""
	dotEnvPath := filepath.Join(cwd, ".env")
	if data, err := os.ReadFile(dotEnvPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading .env: %w", err)
	}

	merged := setEnvLine(content, "IMAGE_TAG", imageTag)

	f, err := os.CreateTemp("", "rollhook-*.env")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(merged); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// setEnvLine merges KEY=VALUE into .env file content.
// Replaces the last occurrence of KEY= if present, appends otherwise.
// Preserves the file's trailing-newline convention.
func setEnvLine(content, key, value string) string {
	keyPrefix := key + "="
	newLine := key + "=" + value
	lines := strings.Split(content, "\n")

	lastIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, keyPrefix) {
			lastIdx = i
		}
	}
	if lastIdx >= 0 {
		lines[lastIdx] = newLine
		return strings.Join(lines, "\n")
	}

	// Append, preserving the file's trailing-newline convention.
	if content == "" {
		return newLine
	}
	if strings.HasSuffix(content, "\n") {
		return content + newLine + "\n"
	}
	return content + "\n" + newLine
}

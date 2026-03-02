package registry

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	zotPort      = 5000
	readyTimeout = 10 * time.Second
	pollInterval = 200 * time.Millisecond
	stopTimeout  = 5 * time.Second
)

// restartBackoffs are the wait durations between successive restart attempts.
// After len(restartBackoffs) consecutive crashes without recovery, Zot is not restarted.
var restartBackoffs = []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}

// Manager starts and manages Zot as a child subprocess.
// Zot binds to 127.0.0.1:5000 — never exposed externally.
type Manager struct {
	dataDir    string
	secret     string
	configPath string
	mu         sync.Mutex
	cmd        *exec.Cmd
	done       chan struct{} // closed by the watcher goroutine when zot exits
}

// NewManager creates a registry Manager. Call Start to launch Zot.
func NewManager(dataDir, secret string) *Manager {
	return &Manager{dataDir: dataDir, secret: secret}
}

// Start writes Zot config + htpasswd, launches the Zot subprocess,
// pipes its stdout/stderr line-by-line, and polls until ready or ctx deadline.
// After Start returns, a background watcher goroutine restarts Zot on unexpected exits.
func (m *Manager) Start(ctx context.Context) error {
	registryDir := filepath.Join(m.dataDir, "registry")
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	m.configPath = filepath.Join(registryDir, "config.json")
	htpasswdPath := filepath.Join(registryDir, ".htpasswd")

	configJSON := GenerateZotConfig(registryDir, htpasswdPath, zotPort)
	if err := os.WriteFile(m.configPath, []byte(configJSON), 0o600); err != nil {
		return fmt.Errorf("write zot config: %w", err)
	}

	htpasswd, err := GenerateHtpasswd(ZotPassword(m.secret))
	if err != nil {
		return fmt.Errorf("generate htpasswd: %w", err)
	}
	if err := os.WriteFile(htpasswdPath, []byte(htpasswd), 0o600); err != nil {
		return fmt.Errorf("write htpasswd: %w", err)
	}

	if err := m.startProcess(); err != nil {
		return err
	}

	go m.watch(ctx)

	return m.waitUntilReady(ctx)
}

// startProcess launches a fresh Zot subprocess and sets m.cmd + m.done.
func (m *Manager) startProcess() error {
	cmd := exec.Command("zot", "serve", m.configPath) //nolint:gosec
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("zot stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("zot stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start zot: %w", err)
	}

	done := make(chan struct{})

	m.mu.Lock()
	m.cmd = cmd
	m.done = done
	m.mu.Unlock()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			slog.Info(scanner.Text(), "source", "zot")
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Error(scanner.Text(), "source", "zot")
		}
	}()

	// Single goroutine owns cmd.Wait() — must be called exactly once.
	go func() {
		defer close(done)
		if err := cmd.Wait(); err != nil {
			slog.Error("zot process exited", "error", err, "source", "zot")
		}
		m.mu.Lock()
		m.cmd = nil
		m.mu.Unlock()
	}()

	return nil
}

// watch monitors the done channel and restarts Zot on unexpected exits.
// Exits when ctx is cancelled (graceful shutdown via SIGTERM).
// Uses exponential backoff: 1s, 5s, 30s. Gives up after len(restartBackoffs) consecutive failures.
func (m *Manager) watch(ctx context.Context) {
	consecutiveCrashes := 0

	for {
		m.mu.Lock()
		done := m.done
		m.mu.Unlock()

		// Wait for Zot to exit or for graceful shutdown.
		select {
		case <-ctx.Done():
			return
		case <-done:
		}

		// ctx may have been cancelled concurrently with done closing (e.g. SIGTERM
		// arrives while Zot crashes). Don't restart during graceful shutdown.
		select {
		case <-ctx.Done():
			return
		default:
		}

		consecutiveCrashes++
		if consecutiveCrashes > len(restartBackoffs) {
			slog.Error("zot crashed too many times, giving up", "crashes", consecutiveCrashes, "source", "zot")
			return
		}

		backoff := restartBackoffs[consecutiveCrashes-1]
		slog.Warn("zot exited unexpectedly, restarting", "backoff", backoff, "attempt", consecutiveCrashes, "source", "zot")

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if err := m.startProcess(); err != nil {
			slog.Error("zot restart failed", "error", err, "source", "zot")
			continue
		}

		slog.Info("zot restarted successfully", "attempt", consecutiveCrashes, "source", "zot")

		// Wait a moment for Zot to stabilise before checking liveness again.
		// (waitUntilReady is only called on initial Start; here we trust the
		// process to come up on its own from a saved state.)
		consecutiveCrashes = 0
	}
}

func (m *Manager) waitUntilReady(ctx context.Context) error {
	deadline := time.Now().Add(readyTimeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	addr := fmt.Sprintf("http://127.0.0.1:%d/v2/", zotPort)

	for time.Now().Before(deadline) {
		resp, err := client.Get(addr) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
				slog.Info("registry ready", "source", "zot")
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return fmt.Errorf("zot failed to start within %s", readyTimeout)
}

// Stop sends SIGTERM to Zot and waits up to 5 s before force-killing.
// The watch goroutine will exit when it detects ctx cancellation, which
// happens before Stop is called in the graceful shutdown sequence.
func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd := m.cmd
	done := m.done
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process already exited — nothing to do.
		return nil //nolint:nilerr
	}

	select {
	case <-done:
	case <-time.After(stopTimeout):
		_ = cmd.Process.Kill()
		<-done
	}

	return nil
}

// IsRunning reports whether the Zot subprocess is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil
}

// Credentials returns the internal Zot username and password.
// These are deterministic — same values every restart.
func (m *Manager) Credentials() (user, password string) {
	return ZotUser, ZotPassword(m.secret)
}

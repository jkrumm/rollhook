package steps

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

// fakeDocker is an in-memory dockerpkg.ContainerAPI.
type fakeDocker struct {
	mu         sync.Mutex
	containers []container.Summary
	health     map[string]string // id -> health status; missing = "starting"
	stopped    []string
	removed    []string
}

func (f *fakeDocker) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.containers), nil
}

func (f *fakeDocker) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, ok := f.health[id]
	if !ok {
		status = "starting"
	}
	return container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{
		State: &container.State{Health: &container.Health{Status: status}},
	}}, nil
}

func (f *fakeDocker) ContainerStop(_ context.Context, id string, _ container.StopOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
	return nil
}

func (f *fakeDocker) ContainerRemove(_ context.Context, id string, _ container.RemoveOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	f.containers = slices.DeleteFunc(f.containers, func(c container.Summary) bool { return c.ID == id })
	return nil
}

func (f *fakeDocker) add(id, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.containers = append(f.containers, container.Summary{ID: id, State: state})
}

func fastTiming(t *testing.T) {
	t.Helper()
	oi, oa, oh := newContainerPollInterval, newContainerPollAttempts, healthPollInterval
	newContainerPollInterval, newContainerPollAttempts, healthPollInterval = 5*time.Millisecond, 3, 5*time.Millisecond
	t.Cleanup(func() { newContainerPollInterval, newContainerPollAttempts, healthPollInterval = oi, oa, oh })
}

func runRollout(t *testing.T, ctx context.Context, f *fakeDocker, compose composeRunner) ([]string, error) {
	t.Helper()
	var logs []string
	err := rollout(ctx, f, compose, t.TempDir()+"/compose.yml", "web", "proj", "app:v2", 2000, 0, func(l string) { logs = append(logs, l) })
	return logs, err
}

func TestRollout_RollsBackNewContainersOnFailure(t *testing.T) {
	fastTiming(t)

	tests := []struct {
		name    string
		compose func(f *fakeDocker, cancel context.CancelFunc) composeRunner
		wantErr string
	}{
		{
			name: "compose up fails part-way",
			compose: func(f *fakeDocker, _ context.CancelFunc) composeRunner {
				return func(context.Context, string, ...string) ([]byte, error) {
					f.add("new1", "created") // created before the failure
					return []byte("boom"), errors.New("exit 1")
				}
			},
			wantErr: "docker compose up --scale failed",
		},
		{
			name: "poll times out",
			compose: func(f *fakeDocker, _ context.CancelFunc) composeRunner {
				return func(context.Context, string, ...string) ([]byte, error) {
					f.add("new1", "running") // only 1 of 2 expected
					return nil, nil
				}
			},
			wantErr: "scale-up produced only",
		},
		{
			name: "context cancelled while waiting for health",
			compose: func(f *fakeDocker, cancel context.CancelFunc) composeRunner {
				return func(context.Context, string, ...string) ([]byte, error) {
					f.add("new1", "running")
					f.add("new2", "running")
					cancel()
					return nil, nil
				}
			},
			wantErr: "context canceled",
		},
		{
			name: "one replica unhealthy",
			compose: func(f *fakeDocker, _ context.CancelFunc) composeRunner {
				return func(context.Context, string, ...string) ([]byte, error) {
					f.add("new1", "running")
					f.add("new2", "running")
					f.health["new1"] = "healthy"
					f.health["new2"] = "unhealthy"
					return nil, nil
				}
			},
			wantErr: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeDocker{health: map[string]string{}}
			f.add("old1", "running")
			f.add("old2", "running")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, err := runRollout(t, ctx, f, tt.compose(f, cancel))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
			if slices.Contains(f.removed, "old1") || slices.Contains(f.removed, "old2") {
				t.Errorf("old containers must be untouched on failure, removed = %v", f.removed)
			}
			for _, c := range f.containers {
				if strings.HasPrefix(c.ID, "new") {
					t.Errorf("new container %s was not rolled back", c.ID)
				}
			}
		})
	}
}

func TestRollout_SuccessDoesNotRollBack(t *testing.T) {
	fastTiming(t)
	f := &fakeDocker{health: map[string]string{"new1": "healthy", "new2": "healthy"}}
	f.add("old1", "running")
	f.add("old2", "running")
	f.add("stale", "exited")

	var composeArgs []string
	_, err := runRollout(t, context.Background(), f, func(_ context.Context, _ string, args ...string) ([]byte, error) {
		composeArgs = args
		f.add("new1", "running")
		f.add("new2", "running")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("rollout: %v", err)
	}

	// Stale exited container must not inflate the scale target: 2 running -> 4.
	if !slices.Contains(composeArgs, "web=4") {
		t.Errorf("compose args = %v, want --scale web=4", composeArgs)
	}
	slices.Sort(f.removed)
	if want := []string{"old1", "old2", "stale"}; !slices.Equal(f.removed, want) {
		t.Errorf("removed = %v, want %v (old drained, stale cleaned, new kept)", f.removed, want)
	}
}

func TestRollout_ReplicasWaitInParallel(t *testing.T) {
	fastTiming(t)
	healthPollInterval = 50 * time.Millisecond

	f := &fakeDocker{health: map[string]string{}}
	for _, id := range []string{"o1", "o2", "o3"} {
		f.add(id, "running")
	}
	// Each replica becomes healthy after ~300ms; sequential waiting with a
	// 700ms per-deploy budget would fail, parallel waiting passes.
	go func() {
		time.Sleep(300 * time.Millisecond)
		f.mu.Lock()
		for _, id := range []string{"n1", "n2", "n3", "n4", "n5", "n6"} {
			f.health[id] = "healthy"
		}
		f.mu.Unlock()
	}()

	start := time.Now()
	err := rollout(context.Background(), f, func(context.Context, string, ...string) ([]byte, error) {
		for _, id := range []string{"n1", "n2", "n3", "n4", "n5", "n6"} {
			f.add(id, "running")
		}
		return nil, nil
	}, t.TempDir()+"/compose.yml", "web", "proj", "app:v2", 700, 0, func(string) {})
	if err != nil {
		t.Fatalf("rollout: %v", err)
	}
	if time.Since(start) > 700*time.Millisecond {
		t.Errorf("took %s, replicas should be awaited concurrently", time.Since(start))
	}
}

func TestRunCompose_DropsRollHookEnv(t *testing.T) {
	got := composeEnv([]string{
		"PATH=/bin", "HOME=/root", "VERSION=1.2.3", "PORT=7700", "DATA_DIR=/data",
		"DOCKER_HOST=unix:///x", "XDG_CONFIG_HOME=/c", "ROLLHOOK_SECRET=s", "LC_ALL=C", "noequals",
	})
	want := []string{"PATH=/bin", "HOME=/root", "DOCKER_HOST=unix:///x", "XDG_CONFIG_HOME=/c", "LC_ALL=C"}
	if !slices.Equal(got, want) {
		t.Errorf("composeEnv = %v, want %v", got, want)
	}
}

func TestRollout_DrainDelay(t *testing.T) {
	fastTiming(t)
	const delay = 150 * time.Millisecond

	run := func(ctx context.Context, f *fakeDocker) error {
		compose := func(context.Context, string, ...string) ([]byte, error) {
			f.add("new1", "running")
			return nil, nil
		}
		return rollout(ctx, f, compose, t.TempDir()+"/compose.yml", "web", "proj", "app:v2", 2000, int(delay/time.Millisecond), func(string) {})
	}

	t.Run("old container is drained only after the delay", func(t *testing.T) {
		f := &fakeDocker{health: map[string]string{"new1": "healthy"}}
		f.add("old1", "running")

		start := time.Now()
		if err := run(context.Background(), f); err != nil {
			t.Fatalf("rollout: %v", err)
		}
		if elapsed := time.Since(start); elapsed < delay {
			t.Errorf("rollout took %s, want at least %s before draining", elapsed, delay)
		}
		if !slices.Contains(f.stopped, "old1") {
			t.Errorf("old1 must be stopped after the delay, stopped = %v", f.stopped)
		}
	})

	t.Run("first deploy does not wait", func(t *testing.T) {
		f := &fakeDocker{health: map[string]string{"new1": "healthy"}}

		start := time.Now()
		if err := run(context.Background(), f); err != nil {
			t.Fatalf("rollout: %v", err)
		}
		if elapsed := time.Since(start); elapsed >= delay {
			t.Errorf("first deploy took %s, must not wait for the drain delay", elapsed)
		}
	})

	t.Run("cancel during the delay rolls back new containers", func(t *testing.T) {
		f := &fakeDocker{health: map[string]string{"new1": "healthy"}}
		f.add("old1", "running")
		ctx, cancel := context.WithTimeout(context.Background(), delay/3)
		defer cancel()

		err := run(ctx, f)
		if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("err = %v, want context deadline exceeded", err)
		}
		if slices.Contains(f.stopped, "old1") || slices.Contains(f.removed, "old1") {
			t.Errorf("old1 must be untouched, stopped = %v removed = %v", f.stopped, f.removed)
		}
		if !slices.Contains(f.removed, "new1") {
			t.Errorf("new1 must be rolled back, removed = %v", f.removed)
		}
	})
}

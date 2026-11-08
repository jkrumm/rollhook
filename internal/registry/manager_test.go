package registry

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testManager(t *testing.T, cmd func() *exec.Cmd, stable time.Duration) *Manager {
	m := NewManager(t.TempDir(), "secret", DefaultKeepTags)
	m.zotCmd = cmd
	m.backoffs = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	m.stableAfter = stable
	return m
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatchGivesUpAfterThreeCrashes(t *testing.T) {
	var starts atomic.Int32
	m := testManager(t, func() *exec.Cmd {
		starts.Add(1)
		return exec.Command("sh", "-c", "exit 1")
	}, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.startProcess(); err != nil {
		t.Fatal(err)
	}
	go m.watch(ctx)

	waitFor(t, func() bool { return m.State() == StateUnavailable && starts.Load() >= 4 })
	time.Sleep(100 * time.Millisecond)
	if got := starts.Load(); got != 4 {
		t.Fatalf("starts = %d, want 4 (initial + 3 restarts)", got)
	}
}

func TestWatchStableRunResetsCounter(t *testing.T) {
	var starts atomic.Int32
	m := testManager(t, func() *exec.Cmd {
		starts.Add(1)
		return exec.Command("sh", "-c", "sleep 0.15; exit 1")
	}, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.startProcess(); err != nil {
		t.Fatal(err)
	}
	go m.watch(ctx)

	waitFor(t, func() bool { return starts.Load() >= 6 })
	if m.State() != StateOK {
		t.Fatalf("state = %s, want ok: stable runs must keep resetting the crash counter", m.State())
	}
}

func TestStateBeforeStart(t *testing.T) {
	m := testManager(t, nil, time.Hour)
	if m.State() != StateUnavailable {
		t.Fatalf("state = %s, want unavailable before start", m.State())
	}
}

func TestStopTerminatesProcess(t *testing.T) {
	m := testManager(t, func() *exec.Cmd { return exec.Command("sleep", "30") }, time.Hour)
	if err := m.startProcess(); err != nil {
		t.Fatal(err)
	}
	if !m.IsRunning() {
		t.Fatal("expected running")
	}
	start := time.Now()
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("Stop should terminate promptly via SIGTERM")
	}
	waitFor(t, func() bool { return !m.IsRunning() })
}

func TestDrainSurvivesOversizedLine(t *testing.T) {
	// The line exceeds the scanner limit; the child must still finish
	// writing (pipe drained) and exit rather than block.
	size := maxLogLine + 4096
	m := testManager(t, func() *exec.Cmd {
		return exec.Command("sh", "-c", "head -c "+strconv.Itoa(size)+" /dev/zero | tr '\\0' x; echo; echo after")
	}, time.Hour)
	if err := m.startProcess(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return !m.IsRunning() })
}

func TestDrainKeepsLongLineBelowLimit(t *testing.T) {
	var got []string
	log := func(msg string, _ ...any) { got = append(got, msg) }
	long := strings.Repeat("y", 100*1024)
	drain(strings.NewReader(long+"\nnext\n"), log)
	if len(got) != 2 || len(got[0]) != len(long) || got[1] != "next" {
		t.Fatalf("unexpected lines: %d", len(got))
	}
}

package docker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/client"
)

// unreachableClient returns a client pointed at a host nothing listens on —
// hermetic, no real Docker daemon required.
func unreachableClient(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.WithHost("tcp://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("NewClientWithOpts error: %v", err)
	}
	t.Cleanup(func() { cli.Close() })
	return cli
}

func TestPing_DaemonUnreachable(t *testing.T) {
	cli := unreachableClient(t)

	err := Ping(context.Background(), cli)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrDaemonUnreachable) {
		t.Errorf("expected errors.Is(err, ErrDaemonUnreachable) to be true, got: %v", err)
	}
	if !strings.Contains(err.Error(), cli.DaemonHost()) {
		t.Errorf("expected error to mention host %q, got: %v", cli.DaemonHost(), err)
	}
}

func TestWrapConnErr_NilError(t *testing.T) {
	cli := unreachableClient(t)
	if err := wrapConnErr(cli, nil); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestWrapConnErr_NonConnectionError(t *testing.T) {
	cli := unreachableClient(t)
	boom := errors.New("boom")

	got := wrapConnErr(cli, boom)
	if !errors.Is(got, boom) {
		t.Errorf("expected the original error to pass through, got: %v", got)
	}
	if errors.Is(got, ErrDaemonUnreachable) {
		t.Error("non-connection errors must not be classified as ErrDaemonUnreachable")
	}
}

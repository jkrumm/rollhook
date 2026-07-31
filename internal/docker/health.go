package docker

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/client"
)

// ErrDaemonUnreachable marks an error as "the Docker API endpoint could not be
// reached at all" — a host-side fault (wrong DOCKER_HOST, socket proxy gone,
// socket not mounted) — as distinct from a well-formed API response that simply
// did not contain what we looked for. The two need different operator
// responses, so callers pick status codes and messages via errors.Is.
var ErrDaemonUnreachable = errors.New("docker daemon unreachable")

// Ping reports whether the Docker daemon answers. It is the cheapest possible
// call against the API — no container/image state is touched — which makes it
// safe to poll frequently from a readiness probe.
func Ping(ctx context.Context, cli *client.Client) error {
	if _, err := cli.Ping(ctx); err != nil {
		return wrapConnErr(cli, err)
	}
	return nil
}

// wrapConnErr tags transport-level failures with ErrDaemonUnreachable and the
// endpoint being dialled. client.IsErrConnectionFailed classifies the error by
// type (errors.As), not by matching its string, so it survives fmt.Errorf's
// %w wrapping all the way up from the client's internal transport. Errors that
// are not connection failures — e.g. a well-formed 404 from the daemon — pass
// through untouched, since those are not host-side faults.
func wrapConnErr(cli *client.Client, err error) error {
	if err == nil || !client.IsErrConnectionFailed(err) {
		return err
	}
	return fmt.Errorf("%w at %s: %w", ErrDaemonUnreachable, cli.DaemonHost(), err)
}

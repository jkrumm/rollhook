package steps

import (
	"context"

	"github.com/docker/docker/client"

	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

// Prune delegates to dockerpkg.PruneImages. Best-effort: if ctx is already
// cancelled (e.g. SIGTERM arrived during rollout), it skips quietly rather
// than logging a scary error for work that was never attempted.
func Prune(ctx context.Context, cli *client.Client, imageTag string, keep int, logFn func(string)) error {
	if ctx.Err() != nil {
		return nil
	}
	return dockerpkg.PruneImages(ctx, cli, imageTag, keep, logFn)
}

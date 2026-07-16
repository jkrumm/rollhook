package steps

import (
	"context"

	"github.com/docker/docker/client"

	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

// ImageKeepCount returns the configured ROLLHOOK_IMAGE_KEEP_COUNT, defaulting
// to defaultImageKeepCount when unset or unparseable.
func ImageKeepCount() int {
	return envInt("ROLLHOOK_IMAGE_KEEP_COUNT", defaultImageKeepCount)
}

// Prune delegates to dockerpkg.PruneImages. Best-effort: if ctx is already
// cancelled (e.g. SIGTERM arrived during rollout), it skips quietly rather
// than logging a scary error for work that was never attempted.
func Prune(ctx context.Context, cli *client.Client, imageTag string, keep int, logFn func(string)) error {
	if ctx.Err() != nil {
		return nil
	}
	return dockerpkg.PruneImages(ctx, cli, imageTag, keep, logFn)
}

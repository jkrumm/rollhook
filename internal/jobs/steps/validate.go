package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
)

// Validate checks that composePath is absolute, the compose file is parseable,
// and the named service exists.
//
// If the service has no healthcheck configured (compose-level), a warning is
// emitted via logFn so the user sees it before scale-up. This is advisory —
// if the Docker image itself has a HEALTHCHECK instruction, the deploy will
// still succeed. logFn may be nil.
func Validate(composePath, service, imageTag string, logFn func(string)) error {
	if !filepath.IsAbs(composePath) {
		return fmt.Errorf("compose_path must be absolute, got: %s", composePath)
	}

	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("compose file not found: %s", composePath)
	}

	opts, err := cli.NewProjectOptions(
		[]string{composePath},
		cli.WithOsEnv,
		cli.WithLoadOptions(func(o *loader.Options) {
			o.SkipValidation = true // skip strict JSON schema checks
		}),
	)
	if err != nil {
		return fmt.Errorf("validate: compose options: %w", err)
	}

	project, err := opts.LoadProject(context.Background())
	if err != nil {
		return fmt.Errorf("compose file invalid: %w", err)
	}

	svc, err := project.GetService(service)
	if err != nil {
		return fmt.Errorf("service %q not found in %s", service, composePath)
	}

	// Warn if the service has no compose-level healthcheck.
	// The deploy will still fail at rollout if the Docker image also lacks a
	// HEALTHCHECK instruction — this is an early heads-up to the user.
	if svc.HealthCheck == nil && logFn != nil {
		logFn(fmt.Sprintf(
			"[validate] Warning: service %q has no healthcheck configured. "+
				"Add a HEALTHCHECK to your Dockerfile or a healthcheck: block to your compose service "+
				"for zero-downtime deploys to work.", service))
	}

	return nil
}

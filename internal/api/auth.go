package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/docker/docker/client"
	"github.com/jkrumm/rollhook/internal/deploytoken"
	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
	"github.com/jkrumm/rollhook/internal/jobs/steps"
	oidcpkg "github.com/jkrumm/rollhook/internal/oidc"
	"github.com/jkrumm/rollhook/internal/registry"
)

type TokenInput struct {
	Body struct {
		ImageName string `json:"image_name" required:"true" doc:"Image repository without tag: a bare name (myapp), a path suffix (org/myapp) or the full repository (ghcr.io/org/myapp). Matched on whole path segments against the image of the running service; a registry-qualified name only matches that registry."`
		Project   string `json:"project,omitempty" doc:"Compose project (com.docker.compose.project) of the target service. Only needed when the image runs in more than one Compose service."`
		Service   string `json:"service,omitempty" doc:"Compose service (com.docker.compose.service) of the target service. Only needed when the image runs in more than one Compose service."`
	}
}

type TokenOutput struct {
	Status int
	Body   struct {
		Token  string `json:"token" doc:"Registry password for docker login"`
		Secret string `json:"secret" doc:"Short-lived deploy token (not ROLLHOOK_SECRET), bound to the full image repository and Compose service that was discovered: valid as Bearer for POST /deploy of that repository to that service and GET /jobs/{id} and /jobs/{id}/logs of its jobs. Sensitive credential, valid for 30 minutes and only invalidated early by rotating ROLLHOOK_SECRET: never log it; in GitHub Actions mask it with ::add-mask:: before use."`
	}
}

func RegisterAuthToken(humaAPI huma.API, secret string, cli *client.Client) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-auth-token",
		Method:      http.MethodPost,
		Path:        "/auth/token",
		Summary:     "Exchange a GitHub Actions OIDC JWT for a registry credential and a scoped deploy token",
		Description: "Validates the OIDC token and checks allowed_repos/allowed_refs labels on the running service. Returns an image-scoped registry password for docker login and a short-lived deploy token scoped to that service (POST /deploy, GET /jobs/{id}, GET /jobs/{id}/logs). Never returns ROLLHOOK_SECRET. PR refs are always denied.",
		Tags:        []string{"Auth"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, func(ctx context.Context, input *TokenInput) (*TokenOutput, error) {
		// Require OIDC — static ROLLHOOK_SECRET callers have no claims injected.
		claims, ok := oidcpkg.ClaimsFromContext(ctx)
		if !ok {
			return nil, huma.NewError(http.StatusForbidden, "OIDC token required for registry credential exchange")
		}

		// Hard-deny PRs — same as /deploy.
		if strings.HasPrefix(claims.Ref, "refs/pull/") {
			return nil, huma.NewError(http.StatusForbidden, "PR ref is not allowed")
		}

		// Validate image_name: must be non-empty, trimmed, and tag-free (no colon).
		imageName := strings.TrimSpace(input.Body.ImageName)
		if imageName == "" || strings.Contains(imageName, ":") {
			return nil, huma.NewError(http.StatusBadRequest, "image_name must be a non-empty name without a tag, e.g. myapp")
		}

		// Discover the running container to get its labels, then check allowed_repos/refs.
		// Fail-secure: if no container is running (first deploy), deny.
		disc, err := steps.Discover(ctx, cli, imageName+":latest", steps.Target{
			Project: strings.TrimSpace(input.Body.Project),
			Service: strings.TrimSpace(input.Body.Service),
		})
		if err != nil {
			return nil, discoverError(cli, err)
		}
		if err := checkOIDCLabels(claims, disc.Labels); err != nil {
			return nil, huma.NewError(http.StatusForbidden, err.Error())
		}

		out := &TokenOutput{}
		out.Status = http.StatusOK
		// Both credentials are short-lived HMAC tokens — never the static secret.
		// The registry token is scoped to this image; the deploy token to the
		// full repository the discovered Compose service runs and that service
		// (and read access to that repository's jobs).
		out.Body.Token = registry.MintRegistryToken(secret, imageName)
		out.Body.Secret = deploytoken.Mint(secret, deploytoken.Scope{
			Repo:    disc.Image,
			Project: disc.Project,
			Service: disc.Service,
		})
		return out, nil
	})
}

// discoverError maps a steps.Discover failure on an authorization path to an
// HTTP error. A missing service is a fail-secure 403, not a lookup miss.
func discoverError(cli *client.Client, err error) error {
	switch {
	case errors.Is(err, dockerpkg.ErrDaemonUnreachable):
		slog.Error("docker daemon unreachable during service discovery",
			"docker_host", cli.DaemonHost(), "error", err)
		return huma.NewError(http.StatusServiceUnavailable, fmt.Sprintf(
			"RollHook cannot reach the Docker daemon at %s — host-side fault (check DOCKER_HOST and the Docker socket / socket proxy), not a problem with the calling workflow",
			cli.DaemonHost()))
	case errors.Is(err, steps.ErrAmbiguousImage):
		return huma.NewError(http.StatusConflict, err.Error())
	case errors.Is(err, steps.ErrServiceNotFound):
		// Fail-secure authorization denial: no running container means
		// allowed_repos cannot be verified, so this is deliberately a 403,
		// not a lookup-miss status.
		return huma.NewError(http.StatusForbidden, "service not found — ensure the app is running before requesting a registry credential or deploying with a deploy token")
	default:
		return huma.NewError(http.StatusInternalServerError, "service discovery failed: "+err.Error())
	}
}

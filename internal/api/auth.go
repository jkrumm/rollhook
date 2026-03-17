package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/docker/docker/client"
	"github.com/jkrumm/rollhook/internal/jobs/steps"
	oidcpkg "github.com/jkrumm/rollhook/internal/oidc"
	"github.com/jkrumm/rollhook/internal/registry"
)

type TokenInput struct {
	Body struct {
		ImageName string `json:"image_name" required:"true" doc:"Image name (without registry prefix or tag), e.g. myapp"`
	}
}

type TokenOutput struct {
	Status int
	Body   struct {
		Token string `json:"token" doc:"Registry password for docker login"`
	}
}

func RegisterAuthToken(humaAPI huma.API, secret string, cli *client.Client) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "post-auth-token",
		Method:      http.MethodPost,
		Path:        "/auth/token",
		Summary:     "Exchange a GitHub Actions OIDC JWT for a registry credential",
		Description: "Validates the OIDC token and checks allowed_repos/allowed_refs labels on the running service. Returns the registry password for docker login. PR refs are always denied.",
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
		disc, err := steps.Discover(ctx, cli, imageName+":latest")
		if err != nil {
			if errors.Is(err, steps.ErrServiceNotFound) {
				return nil, huma.NewError(http.StatusForbidden, "service not found — ensure the app is running before requesting a registry credential")
			}
			return nil, huma.NewError(http.StatusInternalServerError, "service discovery failed")
		}
		if err := checkOIDCLabels(claims, disc.Labels); err != nil {
			return nil, huma.NewError(http.StatusForbidden, err.Error())
		}

		out := &TokenOutput{}
		out.Status = http.StatusOK
		// Return a short-lived HMAC token scoped to this image — not the static secret.
		// The registry proxy accepts these tokens alongside the static secret.
		out.Body.Token = registry.MintRegistryToken(secret, imageName)
		return out, nil
	})
}

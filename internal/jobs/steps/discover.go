package steps

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

// ErrServiceNotFound is returned by Discover when no running container matches the image.
// Callers can use errors.Is to distinguish this from infrastructure errors.
var ErrServiceNotFound = errors.New("service not found")

// ErrAmbiguousImage is returned by Discover when the image name matches running
// Compose containers of more than one image repository, or of more than one
// Compose service, and no Target narrows it down to one.
var ErrAmbiguousImage = errors.New("ambiguous image name")

const (
	labelProject = "com.docker.compose.project"
	labelService = "com.docker.compose.service"
)

// Target optionally pins discovery to one Compose service. Empty fields match
// any project / service.
type Target struct {
	Project string
	Service string
}

// DiscoveryResult holds Compose metadata extracted from a running container.
type DiscoveryResult struct {
	ComposePath string
	Service     string
	Project     string
	Labels      map[string]string // all labels from the discovered container
	Image       string            // image repository (no tag) the discovered container runs
}

// Discover finds the running Compose container whose image matches imageTag
// (narrowed to target, if set) and extracts its Docker Compose metadata
// (compose file path, service, project). See FindMatchingContainer for the
// matching rules.
func Discover(ctx context.Context, cli *client.Client, imageTag string, target Target) (*DiscoveryResult, error) {
	imageName := ExtractImageName(imageTag)

	containers, err := dockerpkg.ListRunningContainers(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	match, err := FindMatchingContainer(containers, imageName, target)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: no running Compose container found matching image: %s", ErrServiceNotFound, imageName)
	}

	detail, err := dockerpkg.InspectContainer(ctx, cli, match.ID)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	result, err := ExtractComposeInfo(detail.Config.Labels, containerName(match))
	if err != nil {
		return nil, err
	}
	result.Image = ExtractImageName(match.Image)
	return result, nil
}

// ExtractImageName strips the tag (and any digest) from an image reference,
// preserving registry host:port prefixes. Delegates to the canonical parser
// in the docker package.
//
//	"localhost:5000/app:v1" → "localhost:5000/app"
//	"nginx:latest" → "nginx"
//	"nginx" → "nginx"
func ExtractImageName(imageTag string) string {
	return dockerpkg.RepoFromRef(imageTag)
}

// FindMatchingContainer returns the running Compose container (one carrying
// project and service labels) whose image repository matches imageName:
//
//  1. Exact repository match ("ghcr.io/org/myapp" == "ghcr.io/org/myapp").
//  2. Otherwise a path-segment suffix match: "myapp" and "org/myapp" match
//     "ghcr.io/org/myapp", "rg/myapp" and "evil.io/org/myapp" do not. A
//     registry-qualified name therefore only ever matches its own registry.
//
// Containers outside target (when set) and containers not started by Compose
// are ignored. The remaining candidates must all run the same repository and
// belong to the same Compose project/service — replicas of one service — or
// ErrAmbiguousImage is returned instead of guessing. Among replicas the newest
// container wins (ties broken by ID), so the result does not depend on the
// daemon's listing order. Returns (nil, nil) when nothing matches.
func FindMatchingContainer(containers []container.Summary, imageName string, target Target) (*container.Summary, error) {
	var exact, suffix []*container.Summary
	for i := range containers {
		c := &containers[i]
		project, service := c.Labels[labelProject], c.Labels[labelService]
		if project == "" || service == "" {
			continue // not a Compose service — RollHook cannot roll it out
		}
		if (target.Project != "" && project != target.Project) || (target.Service != "" && service != target.Service) {
			continue
		}
		repo := ExtractImageName(c.Image)
		switch {
		case repo == imageName:
			exact = append(exact, c)
		case strings.HasSuffix(repo, "/"+imageName):
			suffix = append(suffix, c)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = suffix
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	slices.SortFunc(candidates, func(a, b *container.Summary) int {
		return cmp.Or(cmp.Compare(b.Created, a.Created), strings.Compare(a.ID, b.ID)) // newest first
	})

	repos := map[string]bool{}
	services := map[string]bool{}
	for _, c := range candidates {
		repos[ExtractImageName(c.Image)] = true
		services[c.Labels[labelProject]+"/"+c.Labels[labelService]] = true
	}
	if len(repos) > 1 {
		return nil, fmt.Errorf("%w: %q matches containers running %s — use the full image name",
			ErrAmbiguousImage, imageName, strings.Join(sortedKeys(repos), ", "))
	}
	if len(services) > 1 {
		return nil, fmt.Errorf("%w: %q is run by Compose services %s — pass project and service to select one",
			ErrAmbiguousImage, imageName, strings.Join(sortedKeys(services), ", "))
	}
	return candidates[0], nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// ExtractComposeInfo reads Docker Compose labels from a container's label map
// and returns the compose file path, service name, and project name.
func ExtractComposeInfo(labels map[string]string, name string) (*DiscoveryResult, error) {
	if labels == nil {
		return nil, fmt.Errorf("container %s has no Docker labels — not started via docker compose", name)
	}

	configFiles := labels["com.docker.compose.project.config_files"]
	composePath := strings.TrimSpace(strings.SplitN(configFiles, ",", 2)[0])
	if composePath == "" {
		return nil, fmt.Errorf("container %s is missing 'config_files' label — not started via docker compose", name)
	}

	service := labels[labelService]
	if service == "" {
		return nil, fmt.Errorf("container %s is missing 'service' label — not started via docker compose", name)
	}

	project := labels[labelProject]
	if project == "" {
		return nil, fmt.Errorf("container %s is missing 'project' label — not started via docker compose", name)
	}

	return &DiscoveryResult{ComposePath: composePath, Service: service, Project: project, Labels: labels}, nil
}

func containerName(c *container.Summary) string {
	if len(c.Names) == 0 {
		return c.ID[:min(12, len(c.ID))]
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	dockerpkg "github.com/jkrumm/rollhook/internal/docker"
)

// ErrServiceNotFound is returned by Discover when no running container matches the image.
// Callers can use errors.Is to distinguish this from infrastructure errors.
var ErrServiceNotFound = errors.New("service not found")

// ErrAmbiguousImage is returned by Discover when a bare or partial image name
// only matches running containers by base name and those containers run
// different image repositories.
var ErrAmbiguousImage = errors.New("ambiguous image name")

// DiscoveryResult holds Compose metadata extracted from a running container.
type DiscoveryResult struct {
	ComposePath string
	Service     string
	Project     string
	Labels      map[string]string // all labels from the discovered container
	Image       string            // image repository (no tag) the discovered container runs
}

// Discover finds the running container whose image matches imageTag and
// extracts its Docker Compose metadata (compose file path, service, project).
func Discover(ctx context.Context, cli *client.Client, imageTag string) (*DiscoveryResult, error) {
	imageName := ExtractImageName(imageTag)

	containers, err := dockerpkg.ListRunningContainers(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	match, err := FindMatchingContainer(containers, imageName)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: no running container found matching image: %s", ErrServiceNotFound, imageName)
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

// FindMatchingContainer returns the container whose Image field matches
// imageName. An exact repository match always wins. Otherwise it falls back to
// a base-name match (last path segment) to handle registry-prefixed images when
// searching by bare name (e.g. "myapp" matches "registry.example.com/myapp").
// The fallback only succeeds when every base-name candidate runs the same image
// repository; otherwise ErrAmbiguousImage is returned instead of guessing.
// Returns (nil, nil) when nothing matches.
func FindMatchingContainer(containers []container.Summary, imageName string) (*container.Summary, error) {
	searchBase := baseName(imageName)
	var fallback *container.Summary
	var fallbackRepo string
	for i := range containers {
		c := &containers[i]
		cName := ExtractImageName(c.Image)
		if cName == imageName {
			return c, nil
		}
		if baseName(cName) != searchBase {
			continue
		}
		if fallback == nil {
			fallback, fallbackRepo = c, cName
		} else if cName != fallbackRepo {
			return nil, fmt.Errorf("%w: %q matches containers running %s and %s — use the full image name",
				ErrAmbiguousImage, imageName, fallbackRepo, cName)
		}
	}
	return fallback, nil
}

// baseName returns the last path segment of an image name (after the final slash).
func baseName(image string) string {
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		return image[idx+1:]
	}
	return image
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

	service := labels["com.docker.compose.service"]
	if service == "" {
		return nil, fmt.Errorf("container %s is missing 'service' label — not started via docker compose", name)
	}

	project := labels["com.docker.compose.project"]
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

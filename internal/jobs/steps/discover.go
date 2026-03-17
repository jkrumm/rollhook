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

// DiscoveryResult holds Compose metadata extracted from a running container.
type DiscoveryResult struct {
	ComposePath string
	Service     string
	Project     string
	Labels      map[string]string // all labels from the discovered container
}

// Discover finds the running container whose image matches imageTag and
// extracts its Docker Compose metadata (compose file path, service, project).
func Discover(ctx context.Context, cli *client.Client, imageTag string) (*DiscoveryResult, error) {
	imageName := ExtractImageName(imageTag)

	containers, err := dockerpkg.ListRunningContainers(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	match := FindMatchingContainer(containers, imageName)
	if match == nil {
		return nil, fmt.Errorf("%w: no running container found matching image: %s", ErrServiceNotFound, imageName)
	}

	detail, err := dockerpkg.InspectContainer(ctx, cli, match.ID)
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}

	return ExtractComposeInfo(detail.Config.Labels, containerName(match))
}

// ExtractImageName strips the tag from an image reference, preserving
// registry host:port prefixes.
//
//	"localhost:5000/app:v1" → "localhost:5000/app"
//	"nginx:latest" → "nginx"
//	"nginx" → "nginx"
func ExtractImageName(imageTag string) string {
	lastSlash := strings.LastIndex(imageTag, "/")
	afterLastSlash := imageTag[lastSlash+1:]
	tagStart := strings.Index(afterLastSlash, ":")
	if tagStart < 0 {
		return imageTag
	}
	return imageTag[:lastSlash+1+tagStart]
}

// FindMatchingContainer returns the first container whose Image field matches
// imageName. Matching is tried in order: exact name match, then base-name
// fallback (last path segment) to handle registry-prefixed images when
// searching by bare name (e.g. "myapp" matches "registry.example.com/myapp").
func FindMatchingContainer(containers []container.Summary, imageName string) *container.Summary {
	searchBase := baseName(imageName)
	for i := range containers {
		c := &containers[i]
		cName := ExtractImageName(c.Image)
		if cName == imageName || baseName(cName) == searchBase {
			return c
		}
	}
	return nil
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

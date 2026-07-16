package docker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// pullLogPrefixes are the high-signal pull events forwarded to logFn.
// A typical image with 20 layers emits 100+ lines; this filter keeps it to ~10.
var pullLogPrefixes = []string{
	"Status:",
}

// ListRunningContainers returns all running containers on the Docker host.
func ListRunningContainers(ctx context.Context, cli *client.Client) ([]container.Summary, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing running containers: %w", err)
	}
	return containers, nil
}

// ListServiceContainers returns all containers (any state) matching the given
// Compose project and service labels. Includes created/starting containers so
// rollout health polling can track newly scaled replicas.
func ListServiceContainers(ctx context.Context, cli *client.Client, project, service string) ([]container.Summary, error) {
	f := filters.NewArgs()
	f.Add("label", "com.docker.compose.project="+project)
	f.Add("label", "com.docker.compose.service="+service)
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("listing service containers for %s/%s: %w", project, service, err)
	}
	return containers, nil
}

// InspectContainer returns detailed info about a container.
func InspectContainer(ctx context.Context, cli *client.Client, id string) (container.InspectResponse, error) {
	resp, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return container.InspectResponse{}, fmt.Errorf("inspecting container %s: %w", shortID(id), err)
	}
	return resp, nil
}

// StopContainer stops a container. Already-stopped (304) is treated as success.
func StopContainer(ctx context.Context, cli *client.Client, id string) error {
	err := cli.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil && !cerrdefs.IsNotModified(err) {
		return fmt.Errorf("stopping container %s: %w", shortID(id), err)
	}
	return nil
}

// RemoveContainer removes a container. Already-removed (404) is treated as success.
func RemoveContainer(ctx context.Context, cli *client.Client, id string) error {
	err := cli.ContainerRemove(ctx, id, container.RemoveOptions{})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("removing container %s: %w", shortID(id), err)
	}
	return nil
}

// PullImage pulls a Docker image, streaming high-signal log lines to logFn.
// For RollHook's own built-in registry — localhost OR the host in ROLLHOOK_URL —
// registryPassword is used to inject X-Registry-Auth so the Docker daemon can
// authenticate without relying on its credential store (e.g. on macOS where
// keychain credentials are inaccessible via the Docker API, or production
// daemons that don't have docker login state for the self-hosted registry).
func PullImage(ctx context.Context, cli *client.Client, imageTag string, logFn func(string), registryPassword string) error {
	opts := image.PullOptions{}
	if isOwnRegistry(imageTag) {
		auth, err := buildRegistryAuth("rollhook", registryPassword, extractHost(imageTag))
		if err != nil {
			return fmt.Errorf("encoding registry auth: %w", err)
		}
		opts.RegistryAuth = auth
	}

	reader, err := cli.ImagePull(ctx, imageTag, opts)
	if err != nil {
		return fmt.Errorf("docker pull failed: %w", err)
	}
	defer reader.Close()

	return parsePullStream(reader, logFn)
}

// isLocalhost reports whether the image tag references a localhost registry.
func isLocalhost(imageTag string) bool {
	slashIdx := strings.Index(imageTag, "/")
	if slashIdx < 0 {
		return false
	}
	host := imageTag[:slashIdx]
	return strings.HasPrefix(host, "localhost:") || strings.HasPrefix(host, "127.0.0.1:")
}

// isOwnRegistry reports whether imageTag references RollHook's own built-in
// registry — either via a localhost reference or via the public hostname set
// in ROLLHOOK_URL. The public-hostname case matters for self-hosted setups
// where compose files reference the registry by its external FQDN (e.g.
// rollhook.example.com/myapp:latest); the Docker daemon then has no
// credentials for that host and a `docker pull` would fall back to anonymous
// and fail.
func isOwnRegistry(imageTag string) bool {
	if isLocalhost(imageTag) {
		return true
	}
	rollhookURL := os.Getenv("ROLLHOOK_URL")
	if rollhookURL == "" {
		return false
	}
	u, err := url.Parse(rollhookURL)
	if err != nil || u.Host == "" {
		return false
	}
	return extractHost(imageTag) == u.Host
}

// extractHost returns the registry host portion of an image tag (e.g. "localhost:7700").
func extractHost(imageTag string) string {
	slashIdx := strings.Index(imageTag, "/")
	if slashIdx < 0 {
		return ""
	}
	return imageTag[:slashIdx]
}

// buildRegistryAuth encodes registry credentials as a base64 JSON string
// suitable for Docker's X-Registry-Auth header or image.PullOptions.RegistryAuth.
func buildRegistryAuth(username, password, serverAddress string) (string, error) {
	authJSON, err := json.Marshal(map[string]string{
		"username":      username,
		"password":      password,
		"serveraddress": serverAddress,
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(authJSON), nil
}

type pullEvent struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// parsePullStream reads NDJSON pull output, forwarding high-signal events to logFn.
// Returns an error if the stream contains a Docker pull error event.
func parsePullStream(r io.Reader, logFn func(string)) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event pullEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // skip malformed NDJSON lines
		}
		if event.Error != "" {
			return fmt.Errorf("docker pull failed: %s", event.Error)
		}
		if event.Status != "" {
			for _, prefix := range pullLogPrefixes {
				if strings.HasPrefix(event.Status, prefix) {
					logFn(event.Status)
					break
				}
			}
		}
	}
	return scanner.Err()
}

// PruneImages removes stale images for the repository backing imageTag,
// keeping the newest `keep` images (by creation time) plus the just-deployed
// imageTag itself, regardless of its position. Never removes an image
// backing any container, running or stopped. keep <= 0 is a no-op — the
// escape hatch for "keep every pulled image forever".
func PruneImages(ctx context.Context, cli *client.Client, imageTag string, keep int, logFn func(string)) error {
	if keep <= 0 {
		return nil
	}

	repo := RepoFromRef(imageTag)
	if repo == "" {
		return nil
	}

	f := filters.NewArgs()
	f.Add("reference", repo)
	images, err := cli.ImageList(ctx, image.ListOptions{All: true, Filters: f})
	if err != nil {
		return fmt.Errorf("listing images for %s: %w", repo, err)
	}

	inUse, err := inUseImageIDs(ctx, cli)
	if err != nil {
		return fmt.Errorf("listing containers for prune: %w", err)
	}

	candidates := make([]image.Summary, 0, len(images))
	for _, img := range images {
		if _, used := inUse[img.ID]; !used {
			candidates = append(candidates, img)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Created > candidates[j].Created
	})

	for i, img := range candidates {
		if i < keep || hasTag(img, imageTag) {
			continue
		}
		removeImage(ctx, cli, img, logFn)
	}
	return nil
}

// inUseImageIDs returns the set of image IDs backing any container, running
// or stopped — these must never be pruned.
func inUseImageIDs(ctx context.Context, cli *client.Client) (map[string]struct{}, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(containers))
	for _, c := range containers {
		ids[c.ImageID] = struct{}{}
	}
	return ids, nil
}

// removeImage removes a single image, logging the outcome. Force is
// intentionally false — an in-use conflict should be skipped, not forced.
func removeImage(ctx context.Context, cli *client.Client, img image.Summary, logFn func(string)) {
	label := imageLabel(img)
	_, err := cli.ImageRemove(ctx, img.ID, image.RemoveOptions{PruneChildren: true})
	if err == nil {
		logFn(fmt.Sprintf("[prune] Removed image %s", label))
		return
	}
	if cerrdefs.IsNotFound(err) || cerrdefs.IsConflict(err) {
		logFn(fmt.Sprintf("[prune] Skipped image %s: %s", label, err))
		return
	}
	logFn(fmt.Sprintf("[prune] Warning: failed to remove image %s: %s", label, err))
}

// hasTag reports whether img is tagged as tag.
func hasTag(img image.Summary, tag string) bool {
	for _, t := range img.RepoTags {
		if t == tag {
			return true
		}
	}
	return false
}

// imageLabel returns a human-readable identifier for logging: the first repo
// tag if present, otherwise the short image ID.
func imageLabel(img image.Summary) string {
	if len(img.RepoTags) > 0 {
		return img.RepoTags[0]
	}
	return shortID(img.ID)
}

// RepoFromRef returns the repository portion of an image reference, stripping
// any digest and tag while preserving registry host:port prefixes. This is
// the canonical image-reference parser — both docker.PruneImages and
// steps.ExtractImageName use it, so registry-port and digest handling never
// drifts between the two call sites.
//
//	"registry.example.com:5000/app:v1" → "registry.example.com:5000/app"
//	"registry.example.com:5000/app@sha256:deadbeef" → "registry.example.com:5000/app"
//	"app@sha256:deadbeef" → "app"
//	"nginx:latest" → "nginx"
//	"nginx" → "nginx"
func RepoFromRef(ref string) string {
	if atIdx := strings.Index(ref, "@"); atIdx >= 0 {
		ref = ref[:atIdx]
	}
	lastSlash := strings.LastIndex(ref, "/")
	afterLastSlash := ref[lastSlash+1:]
	tagStart := strings.Index(afterLastSlash, ":")
	if tagStart < 0 {
		return ref
	}
	return ref[:lastSlash+1+tagStart]
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

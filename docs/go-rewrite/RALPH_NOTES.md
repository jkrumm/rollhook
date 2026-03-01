# RollHook Go Rewrite — RALPH Notes

Implementation notes, gotchas, security observations, and future improvements captured after each group.

---

<!-- Claude appends a ## Group N section after completing each group -->

## Group 1: Go Module + Project Skeleton

### What was implemented
Initialized `github.com/jkrumm/rollhook` Go module at repo root with all required dependency declarations, created the full `internal/` directory skeleton with placeholder package files, implemented `cmd/rollhook/main.go` with `ROLLHOOK_SECRET` validation + chi router + `/health` endpoint, and confirmed the Dockerfile Go build stage (binary at `/usr/local/bin/rollhook-go`, CMD stays Bun). Fixed a Content-Type header ordering bug in health.go.

### Deviations from prompt
- **Go bumped to 1.24.1** (not 1.23): huma/v2 v2.36, sqlite v1.46, compose-go/v2 v2.10, x/crypto v0.48 all require go ≥ 1.24. Staying at 1.23 would require very old package versions with known bugs. 1.24.1 is the minimum that satisfies all deps cleanly.
- **Dockerfile FROM updated to `golang:1.24-alpine`** to match go.mod directive.
- **Library versions pinned** due to go version constraints: `huma/v2@v2.36.0` (v2.37+ requires 1.25), `go-containerregistry@v0.20.6` (v0.20.8+ requires 1.25.6).
- **`github.com/docker/docker@v28.5.2+incompatible`** used for Docker client. `github.com/docker/docker/client` is a sub-package of the main docker module, not its own module — attempting `go get github.com/docker/docker/client` resolves to the renamed `github.com/moby/moby/client` (wrong).
- **Fixed Content-Type header ordering bug**: original code called `w.WriteHeader(503)` before `w.Header().Set("Content-Type", ...)` on the shutting-down path. In Go's net/http, headers must be set before `WriteHeader` — otherwise they are silently dropped.

### Gotchas & surprises
- `google/go-containerregistry` jumped its minimum Go from 1.24.0 (v0.20.6) to 1.25.6 in v0.20.8 — a surprising jump within the same minor series. Pin carefully in future.
- `go get github.com/docker/docker/client` (treating it as a module path) produces a confusing error: "module declares its path as: github.com/moby/moby/client". The correct approach: `go get github.com/docker/docker` and use `github.com/docker/docker/client` as an import path.
- Go is not natively installed on the dev machine — all `go` commands run via `docker run --rm -v $PWD:/workspace -w /workspace golang:1.24-alpine`.
- GOTOOLCHAIN=local with mismatched versions produces hard errors. Use GOTOOLCHAIN=auto or ensure go.mod matches deps' minimum.

### Security notes
- `ROLLHOOK_SECRET` check happens before any network listener opens — process exits before binding port 7700 if misconfigured.
- Binary compiled with `-ldflags="-w -s"` (strip debug/symbol info) and `CGO_ENABLED=0` (fully static, no shared lib deps).

### Tests added
None — no business logic yet. `go test ./...` reports `[no test files]` across all packages as expected at skeleton stage.

### Future improvements
- The `// indirect` markers on most deps in go.mod will resolve naturally when packages are imported in later groups and `go mod tidy` is run.
- `golangci-lint` not verified locally (not installed); will be enforced in CI once GitHub Actions are configured.
- Consider pinning `golang:1.24-alpine` to a specific digest in the Dockerfile for reproducible builds.

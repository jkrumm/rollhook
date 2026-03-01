# RollHook Go Rewrite — RALPH Notes

Implementation notes, gotchas, security observations, and future improvements captured after each group.

---

<!-- Claude appends a ## Group N section after completing each group -->

## Group 1: Go Module + Project Skeleton

### What was implemented
Initialized `github.com/jkrumm/rollhook` Go module at repo root, created the full `internal/` directory skeleton with placeholder package files, implemented `cmd/rollhook/main.go` with `ROLLHOOK_SECRET` validation + chi router + `/health` endpoint, and added the Go build stage to the Dockerfile (binary lands at `/usr/local/bin/rollhook-go`, CMD stays Bun).

### Deviations from prompt
- Used `sync/atomic.Bool` in `internal/state/state.go` instead of a plain bool — atomic reads are correct for concurrent access, zero overhead cost.
- `health.go` sets `Content-Type: application/json` header before writing the response. `json.NewEncoder` error is suppressed with `//nolint:errcheck` since writes to `http.ResponseWriter` are unactionable.
- Other deps from the prompt (huma, sqlite, docker, compose-go, etc.) were NOT added to `go.mod` at this stage — `go mod tidy` correctly pruned them since no code imports them yet. They will be added group by group as code actually uses them.

### Gotchas & surprises
- Go is not installed locally on the dev machine — used `docker run --rm -v "$(pwd):/app" golang:1.23-alpine` for all `go mod tidy`, `go build`, `go vet`, and `go test` invocations.
- `go mod tidy` only retains deps that are actually imported. Adding all deps upfront without real imports causes immediate removal. Correct pattern: add deps group by group as code references them.
- The runner image (`oven/bun:1.3.9-slim`) has neither `curl` nor `wget`. Use `bun -e "fetch(...)"` for smoke tests inside the image.

### Security notes
- `ROLLHOOK_SECRET` check happens before any network listener opens — process exits before binding port 7700 if misconfigured.
- Binary compiled with `-ldflags="-w -s"` (strip debug/symbol info) and `CGO_ENABLED=0` (fully static, no shared lib deps).

### Tests added
None — no business logic yet. `go test ./...` reports `[no test files]` across all packages as expected at skeleton stage.

### Future improvements
- Add a `go test` / `go build` convenience target via Docker for local development without native Go.
- `golangci-lint` not verified locally; will be enforced in CI once GitHub Actions are configured.

# RollHook — Go Implementation Gotchas

Reference for LLMs working on this codebase. Organized by component. Each entry is a concrete trap with the fix.

---

## Go stdlib

### `strings.TrimPrefix` silently passes missing prefix
`strings.TrimPrefix(s, "Bearer ")` returns `s` unchanged if the prefix is absent — the raw secret becomes the token, bypassing auth.
**Fix:** use `strings.CutPrefix` (Go 1.20+): returns `(after string, found bool)`. If `!found`, reject immediately.
Applied in: `internal/middleware/auth.go`, `internal/registry/proxy.go`.

### `WriteHeader` must come after all `Header().Set()` calls
In Go's `net/http`, calling `w.WriteHeader(code)` before `w.Header().Set(...)` silently drops the headers. No error, no panic.
**Fix:** set all headers first, then call `WriteHeader`.

### `bufio.Reader` tail-follow at EOF preserves position
`ReadString('\n')` returns `("partial", io.EOF)` when the file hasn't been fully written yet. The reader's internal position does NOT reset — on the next call after a sleep it continues from where it left off, appending to the partial buffer correctly.
No seek required. Used in `GET /jobs/{id}/logs` SSE handler.

### `time.RFC3339` vs SQLite `CURRENT_TIMESTAMP`
SQLite's `CURRENT_TIMESTAMP` default writes `"YYYY-MM-DD HH:MM:SS"` (no T, no Z). `time.RFC3339` won't parse it.
**Fix:** `parseTime` tries multiple layouts: `time.RFC3339`, `"2006-01-02 15:04:05"`. Needed for rows inserted by SQLite defaults or legacy data.

---

## modernc.org/sqlite

### Driver name is `"sqlite"`, not `"sqlite3"`
`sql.Open("sqlite3", ...)` panics: `sql: unknown driver "sqlite3"`. That name belongs to `mattn/go-sqlite3` (CGO).
`modernc.org/sqlite` registers as `"sqlite"`.

### WAL mode cannot be set on `:memory:` databases
`PRAGMA journal_mode=WAL` is a no-op (and may error) on in-memory databases — the journal is already in-memory by definition.
**Fix:** in tests, call `migrate()` directly after `sql.Open("sqlite", ":memory:")` and skip the WAL pragma.

### `PRAGMA table_info` returns exactly 6 columns — scan all 6
`PRAGMA table_info(tablename)` returns `cid, name, type, notnull, dflt_value, pk`. Scanning fewer columns yields: `sql: expected N destination arguments in Scan, not M`.

### `SetMaxOpenConns(1)` — not `busy_timeout` — fixes SQLITE_BUSY
`PRAGMA busy_timeout=5000` is applied per-connection. New connections from `database/sql`'s pool don't inherit it.
When two goroutines write concurrently, the second connection has no timeout → immediate `SQLITE_BUSY` → 500.
**Fix:** `db.SetMaxOpenConns(1)`. Serializes all access through one connection. WAL mode ensures readers (SSE log streaming) don't block the single writer.

### `sql.NullString` required for nullable TEXT columns
Scanning a nullable TEXT column into a plain `string` panics on NULL. Use `sql.NullString`, then convert: `if ns.Valid { s := ns.String; return &s }`.

---

## Docker SDK v28

### Type names changed — `[]container.Summary`, not `[]types.Container`
- `ContainerList` → `[]container.Summary` (not `[]types.Container`)
- `ContainerInspect` → `container.InspectResponse` (not `types.ContainerJSON`)
- Options types moved to `github.com/docker/docker/api/types/container` and `.../image` sub-packages

### `go get github.com/docker/docker/client` is wrong
Treats `github.com/docker/docker/client` as a module path → error: "module declares its path as: github.com/moby/moby/client".
**Fix:** `go get github.com/docker/docker` — then import `github.com/docker/docker/client` as a sub-package path.

### `container.Summary.ID` is uppercase `ID`
Not `Id`. Consistent with Docker SDK v28 field naming throughout.

### `errdefs` package for graceful 304/404 handling
`github.com/docker/docker/errdefs` provides `IsNotModified(err)` and `IsNotFound(err)` predicates — use instead of string-matching error messages for already-stopped/already-removed containers.

### X-Registry-Auth for localhost pulls
macOS/OrbStack: Docker daemon keychain credentials not accessible when pulls are initiated via Docker API (vs Docker CLI).
**Fix:** for `isLocalhost(imageTag)`, build base64-encoded JSON credentials: `{"username": "rollhook", "password": ROLLHOOK_SECRET, "serveraddress": "localhost:PORT"}` and pass as `X-Registry-Auth` header to `ImagePull`.
`base64.StdEncoding` — not URL-safe — matches Docker daemon expectation.

---

## Zot Registry

### Alpine runner fails silently — use `debian:12-slim`
Zot pre-built binaries are dynamically linked against glibc. Alpine uses musl libc.
Symptom: `zot: not found` even though the file exists — the ELF interpreter `/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2` is absent on musl.
**Fix:** runner stage must be `debian:12-slim`. The `tool-downloader` stage can remain Alpine (downloads static binaries only).

### `docker2s2` compat mode required — `distSpecVersion` alone is not sufficient
Without `http.compat: ["docker2s2"]` in Zot config, Zot rejects Docker v2 manifests (`application/vnd.docker.distribution.manifest.v2+json`) with 415.
`distSpecVersion: "1.1.1"` controls the OCI spec version advertised — it does NOT control media type acceptance.
Both are needed. See `internal/registry/config.go`.

### `cmd.Wait()` may only be called once
If both the watcher goroutine and `Stop()` call `Wait()`, the second call errors: "waitid: no child processes".
**Fix:** single watcher goroutine owns `cmd.Wait()` and closes a `done` channel. `Stop()` sends SIGTERM and blocks on `<-done`.

### `exec.CommandContext` vs `exec.Command` for Zot lifecycle
`exec.CommandContext` kills the process immediately on context cancellation — no SIGTERM, no graceful shutdown.
Zot needs: SIGTERM → wait for clean exit → SIGKILL if stuck.
**Fix:** use `exec.Command` and manage lifecycle manually in `Stop()`.

### `httputil.ReverseProxy.Director` override — capture original first
`httputil.NewSingleHostReverseProxy(target)` sets a `Director` that rewrites the request URL. Overriding `proxy.Director` without capturing and calling the original means the URL is never rewritten → all proxied requests hit the wrong host.
**Fix:**
```go
original := proxy.Director
proxy.Director = func(r *http.Request) {
    original(r)
    // then add auth headers etc.
}
```

---

## huma/v2

### `out.Status = 0` panics
huma passes `out.Status` directly to `http.ResponseWriter.WriteHeader()`. Go zero value (0) → `WriteHeader(0)` → panic: "invalid WriteHeader code 0".
**Fix:** always set `out.Status = http.StatusOK` immediately after `out := &FooOutput{}`, before any early returns.

### `DefaultConfig` enables SwaggerUI at `/docs`
`huma.DefaultConfig` sets `DocsPath = "/docs"` (SwaggerUI) and `SpecPath = "/openapi"` (serves `.json`/`.yaml`). The base `/openapi` path is NOT registered by huma — no conflict when registering a custom Scalar handler there.
Set `DocsPath = ""` to disable default SwaggerUI.

### SSE endpoints cannot go through huma
huma has no SSE support. Register SSE handlers directly on the chi router after calling huma.NewAPI.
huma routes and chi direct routes coexist on the same underlying chi router — chi's radix tree treats them as distinct routes.
**Auth:** SSE handler must use `middleware.RequireAuth(secret)` explicitly (chi middleware), not huma's security middleware.

### `huma.Register` is safe with nil handler deps for spec generation
`api.RegisterDeploy(humaAPI, nil, nil)` works in `cmd/gendocs` because huma only invokes the handler closure on HTTP requests, not during registration. Spec is generated without any running deps.

---

## compose-go v2

### `cli.WithSkipValidation` does not exist
`cli.WithSkipValidation` is not an exported option. The field lives on `loader.Options`:
```go
cli.WithLoadOptions(func(o *loader.Options) { o.SkipValidation = true })
```

### `cli.WithOsEnv` exposes all process env vars to compose interpolation
This is correct behavior (matches `docker compose` CLI), but means any secret env var with a name that collides with a compose variable will be interpolated. Intentional — document if this becomes a concern.

---

## orval / gendocs

### orval v8 fetch client signature is `customInstance(url, RequestInit)` — not `{method, params, body}`
Context7 docs show an older object-based signature. orval v8 generates calls to `customInstance(url, init)` where `url` already contains query params.
**Fix:** inspect the generated output first, then write `client.ts` to match. See `apps/dashboard/src/api/client.ts`.

### `bun run X --cwd Y` in package.json recurses infinitely
`"generate:api": "bun run generate:api --cwd apps/dashboard"` appends `--cwd` on every invocation → infinite loop.
**Fix:** `bun run --filter @rollhook/dashboard generate:api`.

### Redirect stderr before stdout when capturing JSON from `go run`
`docker run ... go run ./cmd/gendocs > openapi.json 2>&1` merges go module download logs into the JSON file → invalid JSON.
**Fix:** `docker run ... go run ./cmd/gendocs 2>/dev/null > openapi.json`.

### `mode: 'tags-split'` generates `'.././models'` relative imports
orval splits by OpenAPI tag (`deploy.ts`, `jobs.ts`, `health.ts`) + `models/` directory. The relative import path `'.././models'` is intentional and valid TypeScript.

---

## Go Module / Dependency Gotchas

### `GOTOOLCHAIN=local` with version mismatch fails hard
If `go.mod` specifies `go 1.24` but the local toolchain is older, `GOTOOLCHAIN=local` hard-errors.
**Fix:** use `GOTOOLCHAIN=auto` or ensure go.mod matches the installed version.

### Library minimum Go versions can jump within a minor series
`google/go-containerregistry` went from requiring Go 1.24.0 (v0.20.6) to Go 1.25.6 (v0.20.8) — within the same minor.
Check release notes before minor bumps. Pinned: `go-containerregistry@v0.20.6` for Go 1.24 compatibility.
Similarly: `huma/v2@v2.36.0` is the last version compatible with Go 1.24 (v2.37+ requires 1.25).

### `go mod tidy` required after adding compose-go
`go get github.com/compose-spec/compose-go/v2` adds direct dep but doesn't update transitive deps in `go.sum`. `go build` fails until `go mod tidy` is run.

---

## Dockerfile

### Multi-stage: tool-downloader can stay Alpine, runner must be Debian
`tool-downloader` stage only downloads static binaries (docker CLI, docker-compose) — Alpine is fine.
`runner` stage executes Zot (glibc-linked) — must be `debian:12-slim`.

### `.env` file conflict with docker compose scale-up
`docker compose` auto-reads `.env` from the project directory, potentially overriding `IMAGE_TAG` set by the rollout step.
**Fix:** write a temp env file, pass via `--env-file <tmpfile>`. The temp file contains the existing `.env` contents with `IMAGE_TAG` merged/replaced. `setEnvLine` handles duplicate key edge cases.
Temp file removed via `defer os.Remove(tmpFile)`.

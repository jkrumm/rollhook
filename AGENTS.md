# RollHook — Project Configuration

## Validate

`make check` runs exactly the local gates CI runs (`.github/workflows/ci.yml`): `bun install --frozen-lockfile`, `bun run lint`, `bun run typecheck`, `bun run check:basalt`, then Go `build` / `vet` / `test` and `golangci-lint` in Docker, then the OpenAPI drift check.

**Go is not installed locally.** All Go commands run via Docker:

```bash
# Build
docker run --rm -v "$(pwd)":/workspace -w /workspace golang:1.25-alpine go build ./...

# Test
docker run --rm -v "$(pwd)":/workspace -w /workspace golang:1.25-alpine go test ./...

# Regenerate openapi.json (redirect stderr first or go module logs corrupt the JSON):
docker run --rm -v "$(pwd)":/workspace -w /workspace golang:1.25-alpine \
  go run ./cmd/gendocs 2>/dev/null > apps/dashboard/openapi.json

# Then regenerate TypeScript types:
bun run --filter @rollhook/dashboard generate:api
```

CI runs Go natively (`go build ./...`, `go vet ./...`, `go test ./...`, `golangci-lint`).

**Not covered by `make check`:** the E2E suite (`bun run test:e2e`, CI job `e2e-test`) — it builds and starts Docker containers and runs ~12 min; run it separately.

**OpenAPI generation chain:** huma operations → `cmd/gendocs` → `openapi.json` → orval → `src/api/generated/`. Commit `openapi.json` + `src/api/generated/` together.

---

## Deploy

**The server image ships through CI, not from a checkout.** `.github/workflows/release.yml` (the **Make Release** workflow, manual `workflow_dispatch`) runs semantic-release on `master`, then builds and pushes `ghcr.io/jkrumm/rollhook:<version>` and `:latest` to GHCR. The VPS runs `ghcr.io/jkrumm/rollhook:latest`; Watchtower pulls it at the 04:00 sweep, and `make rollhook-update` in `~/SourceRoot/vps` takes a fresh release immediately. RollHook cannot deploy itself.

`make deploy` ships nothing: it prints `deployed by CI on push` (plus a note that the server image goes through the manual release workflow) and exits 0.

The marketing site (`rollhook.com`) is separate: `.github/workflows/deploy-marketing.yml` deploys it on every push to `master` that touches it, via `jkrumm/rollhook-action@v1`.

**Rollback:** pushed image tags are immutable. Roll the server back by pointing the VPS at the previous tag (or reverting the commit and cutting a new release). Revert a marketing change and push — the workflow redeploys it.

---

## Verify & Monitor

- **Liveness — `https://<rollhook-domain>/health`:** 200 while the HTTP process is up, 503 during graceful shutdown. Container and Traefik healthchecks target this.
- **Readiness — `https://<rollhook-domain>/ready`:** also pings the Docker daemon; 503 with `"docker":"unreachable"` when it is not. **Point monitoring at `/ready`, not `/health`.**
- **Uptime Kuma** monitors (exact names, `~/SourceRoot/homelab/uptime-kuma/monitors.yaml`): `RollHook - HTTP` (type http, `/health`) and `RollHook - Ready - HTTP` (type keyword, `/ready`, keyword `"docker":"ok"`).
- **OTel `service.name`:** `rollhook` (`internal/notifier/otlp.go`), exported over OTLP to ClickStack with `DEPLOY_ENVIRONMENT=prod`.
- `make verify` probes `/ready` at `$ROLLHOOK_URL` (base URL, e.g. `https://<rollhook-domain>`); `make logs` tails the last 200 lines of the production container over ssh to `$ROLLHOOK_SSH_HOST`. Both exit 2 with a message when the variable is unset — this repo is public, so no production host is tracked.

---

## Gotchas

**huma response status:** always set `out.Status = http.StatusOK` immediately after `out := &FooOutput{}`. Zero value → `WriteHeader(0)` → panic.

**RollHook compose `stop_grace_period: 3m`** — Docker's default 10 s SIGKILLs the process mid-deploy. Required in production:

```yaml
services:
  rollhook:
    stop_grace_period: 3m
```

**SQLite:** `SetMaxOpenConns(1)` is the fix for `SQLITE_BUSY`, not `busy_timeout`. `busy_timeout` is per-connection and new pool connections don't inherit it.

**`bun run X --cwd Y` recurses infinitely** in package.json scripts. Use `bun run --filter @pkg X` instead.

**`/ready` must stay off the container healthcheck.** Traefik's Docker provider drops containers whose Docker health status is not `healthy`, so routing a daemon blip through it would 404 every route on this host — including the bundled registry at `/v2/*`. Keep container and load-balancer healthchecks on `/health`; point uptime monitoring at `/ready`. Details: `docs/GO_GOTCHAS.md`.

---

## Project-Specific Conventions

**Companion repo:** `~/SourceRoot/rollhook-action` (`jkrumm/rollhook-action`) — versioned independently (`v1.x`). Users reference as `uses: jkrumm/rollhook-action@v1`.

**No `!` or `BREAKING CHANGE` in commits** — greenfield, no external consumers. All changes are `feat:` or `fix:`.

**Types shared between packages** (`JobResult`, `JobStatus`) live in `packages/ui/src/types.ts`, exported from `@rollhook/ui`.

**Styling: Tailwind v4 + basalt-ui tokens only.** basalt-ui 1.x is a Mantine/React framework; both apps consume only its `--vx-*` token layer and stay on Tailwind. Never import `basalt-ui/css` (dropped in 1.0) or `basalt-ui/styles.css` (needs Mantine). Both apps declare `"basalt": { "profile": "tokens-only" }` — without it `check-theme` reports 18 Mantine-only kinds (e.g. "use `TextInput` from `@mantine/core`") against apps that have no React chrome.

| App              | Route                                                                                  | Wiring                         |
| ---------------- | -------------------------------------------------------------------------------------- | ------------------------------ |
| `apps/dashboard` | `@import 'basalt-ui/tokens.css'` (runtime dep, Vite resolves it)                       | `src/styles/global.css`        |
| `apps/marketing` | committed emit — `bun run --filter @rollhook/marketing tokens` (devDep, nothing ships) | `src/styles/basalt-tokens.css` |

Each `global.css` maps `--vx-*` onto Tailwind's `@theme inline` namespaces (`--color-*`, `--radius-*`, `--font-*`); use those utilities, not raw hex. Both apps are `<html class="dark">` and dark-only. The marketing emit uses `--selector-class dark` (`:root, :root.dark`); the dashboard's prebuilt subpath can't take that flag and relies on the emitter's default of parking dark on bare `:root`.

**CI gate: `bun run check:basalt`** — `tokens:css --check` (the committed emit must match what the pinned CLI emits) + `check-theme` and `check-theme --audit-allows` in both apps. Bump `basalt-ui` and re-run `bun run --filter @rollhook/marketing tokens` in the same commit.

**Fonts stay ours.** RollHook is Instrument Sans + JetBrains Mono (`@fontsource-variable` in the dashboard, `astro:fonts` in `astro.config.mjs`). basalt-ui 1.x ships Nunito Sans + Hubot Sans, so `basalt-ui fonts:css` would rebrand both apps — do not adopt it.

`apps/marketing/src/styles/basalt-tokens.css` is no longer eslint-ignored: 1.21.0's emitter writes `0.1` rather than `0.10`, so the sheet lints clean. `public/site.webmanifest`'s `theme_color`/`background_color` must be kept equal to `--vx-surface-bg` dark (`#27272a`) by hand; the file declares its own exception with a `"basalt:theme-allow-file"` member on line 2 — no `basalt.exemptRules` anywhere. `check-theme --audit-allows` proves every waiver still suppresses something and exits 1 when one does not.

---

## References

- `docs/GO_GOTCHAS.md` — battle-tested fixes for Go stdlib, SQLite, Docker SDK, Zot, huma, orval, compose-go
- `compose.yml` — canonical production stack (Traefik + RollHook + example app service)
- `e2e/hello-world/` — reference app with healthcheck + graceful shutdown

---

## When Something Seems Wrong

If you encounter confusing code, contradictory patterns, or something that doesn't match expectations — flag it explicitly rather than silently working around it. Suggest a codebase fix over a docs fix. Check `docs/GO_GOTCHAS.md` before researching library quirks externally.

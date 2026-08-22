# RollHook — Project Configuration

## Critical Commands

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

CI runs Go natively (`go build ./...`, `go vet ./...`, `go test ./...`).

**OpenAPI generation chain:** huma operations → `cmd/gendocs` → `openapi.json` → orval → `src/api/generated/`. Commit `openapi.json` + `src/api/generated/` together.

---

## Project-Specific Conventions

**Companion repo:** `~/SourceRoot/rollhook-action` (`jkrumm/rollhook-action`) — versioned independently (`v1.x`). Users reference as `uses: jkrumm/rollhook-action@v1`.

**No `!` or `BREAKING CHANGE` in commits** — greenfield, no external consumers. All changes are `feat:` or `fix:`.

**Types shared between packages** (`JobResult`, `JobStatus`) live in `packages/ui/src/types.ts`, exported from `@rollhook/ui`.

**Styling: Tailwind v4 + basalt-ui tokens only.** basalt-ui 1.x is a Mantine/React framework; both apps consume only its `--vx-*` token layer and stay on Tailwind. Never import `basalt-ui/css` (dropped in 1.0) or `basalt-ui/styles.css` (needs Mantine). Both apps declare `"basalt": { "profile": "tokens-only" }` — without it `check-theme` reports 17 Mantine-only kinds (e.g. "use `TextInput` from `@mantine/core`") against apps that have no React chrome.

| App              | Route                                                                                  | Wiring                         |
| ---------------- | -------------------------------------------------------------------------------------- | ------------------------------ |
| `apps/dashboard` | `@import 'basalt-ui/tokens.css'` (runtime dep, Vite resolves it)                       | `src/styles/global.css`        |
| `apps/marketing` | committed emit — `bun run --filter @rollhook/marketing tokens` (devDep, nothing ships) | `src/styles/basalt-tokens.css` |

Each `global.css` maps `--vx-*` onto Tailwind's `@theme inline` namespaces (`--color-*`, `--radius-*`, `--font-*`); use those utilities, not raw hex. Both apps are `<html class="dark">` and dark-only. The marketing emit uses `--selector-class dark` (`:root, :root.dark`); the dashboard's prebuilt subpath can't take that flag and relies on the emitter's default of parking dark on bare `:root`.

**CI gate: `bun run check:basalt`** — `tokens:css --check` (the committed emit must match what the pinned CLI emits) + `check-theme` in both apps. Bump `basalt-ui` and re-run `bun run --filter @rollhook/marketing tokens` in the same commit.

**Fonts stay ours.** RollHook is Instrument Sans + JetBrains Mono (`@fontsource-variable` in the dashboard, `astro:fonts` in `astro.config.mjs`). basalt-ui 1.x ships Nunito Sans + Hubot Sans, so `basalt-ui fonts:css` would rebrand both apps — do not adopt it.

`apps/marketing/src/styles/basalt-tokens.css` stays eslint-ignored: two `rgba(…, 0.10)` alphas fail `format/prettier`, and `--fix` would put the file into `tokens:check` drift. `public/site.webmanifest`'s `theme_color` must be kept equal to `--vx-surface-bg` dark (`#27272a`) by hand — JSON carries no `theme-allow` comment, so it is silenced via `basalt.exemptRules`.

---

## Known Pitfalls

**huma response status:** always set `out.Status = http.StatusOK` immediately after `out := &FooOutput{}`. Zero value → `WriteHeader(0)` → panic.

**RollHook compose `stop_grace_period: 3m`** — Docker's default 10 s SIGKILLs the process mid-deploy. Required in production:

```yaml
services:
  rollhook:
    stop_grace_period: 3m
```

**SQLite:** `SetMaxOpenConns(1)` is the fix for `SQLITE_BUSY`, not `busy_timeout`. `busy_timeout` is per-connection and new pool connections don't inherit it.

**`bun run X --cwd Y` recurses infinitely** in package.json scripts. Use `bun run --filter @pkg X` instead.

---

## References

- `docs/GO_GOTCHAS.md` — battle-tested fixes for Go stdlib, SQLite, Docker SDK, Zot, huma, orval, compose-go
- `compose.yml` — canonical production stack (Traefik + RollHook + example app service)
- `e2e/hello-world/` — reference app with healthcheck + graceful shutdown

---

## When Something Seems Wrong

If you encounter confusing code, contradictory patterns, or something that doesn't match expectations — flag it explicitly rather than silently working around it. Suggest a codebase fix over a docs fix. Check `docs/GO_GOTCHAS.md` before researching library quirks externally.

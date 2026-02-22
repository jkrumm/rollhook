# StackCommander — Project Configuration

## Project Overview

Self-hosted unified admin suite for solo devs. Replaces Coolify, Doppler, Umami, Sentry, and LaunchDarkly with a single infra container on a VPS.

See: `~/Obsidian/Vault/03_Projects/stackcommander.md`
North Star Stack: `~/Obsidian/Vault/04_Areas/Engineering/north-star-stack.md`

---

## Monorepo Structure

```
stackcommander/
  apps/
    server/          # @stackcommander/server — Elysia unified server (port 7700)
    web/             # @stackcommander/web — TanStack Start frontend (build only)
  packages/
    stackcommander/  # stackcommander — Core shared package
  data/              # File-based store (gitignored) — temp DB until Drizzle added
  package.json       # Bun workspace root
  tsconfig.json      # Root TypeScript 6.0 config (inherited by all packages)
  eslint.config.mjs  # @antfu/eslint-config (flat config, handles formatting too)
```

---

## Tech Stack

| Layer | Choice |
|-|-|
| Runtime | Bun 1.3.9 |
| Monorepo | Bun workspaces (native) |
| Language | TypeScript 6.0.0-beta |
| Backend | Elysia (Bun-native, auto-typed routes, hosts frontend) |
| API Client | Eden Treaty (`@elysiajs/eden`) — isomorphic, zero-HTTP on server |
| API Docs | Scalar via `@elysiajs/openapi` at `/openapi` |
| Frontend | TanStack Start v1 (SSR React, file-based routing, served by Elysia) |
| Styling | Tailwind CSS v4 |
| Linting/Formatting | @antfu/eslint-config (ESLint flat config, no Prettier) |

---

## Package Manager

**Bun** — always use `bun` commands.

```bash
# Install all workspace deps from root
bun install

# Add dep to a specific workspace
bun add <pkg> --cwd apps/server
bun add <pkg> --cwd apps/web
bun add <pkg> --cwd packages/stackcommander
```

---

## Scripts

### Root

| Command | Action |
|-|-|
| `bun run dev` | Start single dev server (Elysia + Vite middleware on port 7700) |
| `bun run build` | Build TanStack Start frontend to `apps/web/dist/` |
| `bun run typecheck` | Type-check all workspaces |
| `bun run lint` | Lint entire monorepo |
| `bun run lint:fix` | Auto-fix lint + formatting |

### Dev (single process)

```bash
bun run dev
# → http://localhost:7700           TanStack Start SSR (via Vite middleware)
# → http://localhost:7700/api/*     Elysia API routes
# → http://localhost:7700/openapi   Scalar API docs
```

### Prod build + start

```bash
bun run build                                    # builds apps/web/dist/
NODE_ENV=production bun --cwd apps/server run start
# → http://localhost:7700  (static + TanStack Start handler)
```

---

## TypeScript

- **Version:** 6.0.0-beta (managed at root, all workspaces inherit)
- `ignoreDeprecations: "6.0"` is set where `baseUrl` is needed (TS6 deprecation)
- `routeTree.gen.ts` in `apps/web/src/` is auto-generated — do not edit manually, regenerated on `bun run dev`

---

## ESLint / Formatting

Uses `@antfu/eslint-config` — handles both linting AND formatting (no Prettier).

```bash
bun run lint        # Check
bun run lint:fix    # Fix + format
```

---

## Workspace Package Names

| Directory | Package Name |
|-|-|
| `apps/server` | `@stackcommander/server` |
| `apps/web` | `@stackcommander/web` |
| `packages/stackcommander` | `stackcommander` |

---

## Elysia Server

| File | Purpose |
|-|-|
| `apps/server/server.ts` | Entry point — adds Vite middleware (dev) or static serving (prod), calls `.listen(7700)` |
| `apps/server/src/app.ts` | Bare Elysia instance — OpenAPI + API routes, no `.listen()`. Exported for Eden Treaty direct calls. |
| `apps/server/src/api.ts` | API plugin (prefix `/api`) — TypeBox-typed routes |
| `apps/server/src/store.ts` | File-based store — reads/writes `data/store.json` (shared across module systems) |

`apps/server/package.json` exports:
- `.` → `server.ts` (entry)
- `./app` → `src/app.ts` (bare app for `treaty(app)` direct calls)

OpenAPI (Scalar UI): `@elysiajs/openapi` — served at `/openapi`, JSON spec at `/openapi/json`.

---

## TanStack Start Frontend

| File | Purpose |
|-|-|
| `apps/web/src/router.tsx` | Router entry |
| `apps/web/src/server.ts` | SSR handler entry (loaded by Elysia via `ssrLoadModule`) |
| `apps/web/src/routes/` | File-based routes |
| `apps/web/src/lib/api.ts` | Browser-side Eden Treaty client |
| `apps/web/vite.config.ts` | Vite config — middleware mode only, no standalone server |
| `apps/web/src/styles.css` | Tailwind v4 styles |

---

## Eden Treaty — Isomorphic API Client

### Architecture

Bun's native module system and Vite's SSR module system are isolated. `treaty(app)` works zero-HTTP because state lives in `data/store.json` (external to both module systems), not in module-level variables.

### Browser client (`apps/web/src/lib/api.ts`)

```ts
import { treaty } from '@elysiajs/eden'
import type { App } from '@stackcommander/server/app'

export const api = treaty<App>('localhost:7700')
```

### SSR data loading — `createIsomorphicFn` (not `createServerFn`)

**Critical:** Use `createIsomorphicFn` for route loaders in this architecture. `createServerFn` creates an RPC endpoint and makes HTTP calls to `/_server/*` during SSR — Elysia has no such route, causing AbortErrors.

```ts
import { createIsomorphicFn } from '@tanstack/react-start'

const getData = createIsomorphicFn()
  .server(async () => {
    // Zero-HTTP: calls Elysia handler directly, reads from file store
    const [{ treaty }, { app }] = await Promise.all([
      import('@elysiajs/eden'),
      import('@stackcommander/server/app'),
    ])
    const { data } = await treaty(app).api.someRoute.get()
    return data
  })
  .client(async () => {
    // HTTP via Eden Treaty
    const { data } = await api.api.someRoute.get()
    return data
  })

export const Route = createFileRoute('/example')({
  loader: () => getData(),
  component: MyComponent,
})
```

### When to use each

| Tool | Use when |
|-|-|
| `createIsomorphicFn` | Route loaders — different server/client behaviour, no RPC endpoint |
| `createServerFn` | Server-only mutations (DB writes, auth) called from client components |
| `api.*` (plain Eden Treaty) | Client-side mutations from event handlers |

---

## Data Store

`apps/server/src/store.ts` — synchronous file I/O on `data/store.json`.

- Shared across Bun native and Vite SSR module systems (file is external to both)
- Intentionally simple until Drizzle + Postgres is added
- `data/` is gitignored

Replace with Drizzle when proper persistence is needed. Module-level `let` variables must NOT be used for shared state — each module system gets its own copy.

---

## Git Workflow

Follow SourceRoot conventions (see `~/SourceRoot/CLAUDE.md`):
- `/commit` for conventional commits
- `/pr` for GitHub PR workflow
- No ticket numbers (personal project)
- No AI attribution

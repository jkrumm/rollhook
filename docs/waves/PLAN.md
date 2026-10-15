# rollhook on basalt-ui 2.7.3

**Goal:** `apps/dashboard` and `apps/marketing` run on `basalt-ui@2.7.3` (exact pin) in production, with the
`--vx-*` token layer wired through the 2.x route (`basalt-ui tokens:css`, committed emit), no workaround
for basalt anywhere, and no visible regression against the recorded 1.36.0 baseline.

**Gate (every wave):** `make check` green (`AGENTS.md` § Validate: lint, typecheck, `check:basalt`, Go
build/vet/test/golangci-lint, OpenAPI drift); `/review` clean on the diff; the wave's evidence in
`docs/waves/evidence/w<n>.md`. **rollhook is PR-required: every wave below lands through a PR**
(open, `/review`, fix, merge) — never direct-to-master. The E2E suite (`bun run test:e2e`, ~12 min, not
in `make check`) runs once, in the bump wave.

**Target:** `basalt-ui@2.8.0` (released 2026-10-15; carries the fleet-probe fixes, ledger rows 61–67). Where this plan pins 2.7.3, pin 2.8.0; claims verified on 2.7.3 stay valid.

**Fleet note:** feeds basalt-ui `docs/waves/argo-friction.md`: a basalt-rooted problem becomes a row
there, prefixed `rollhook W<n>`; never worked around locally; stay exact-pinned on the 1.x pin until
Wave 2 runs.

**Shape:** rollhook is a *tokens-only* consumer (`"basalt": { "profile": "tokens-only" }` in both apps).
It has **zero** `basalt-ui` imports in TypeScript/Astro — the whole surface is two CSS routes, the CLI
(`tokens:css`, `check-theme`, `sync --check`) and the pin. So the 2.0 chart/shell/table/forms deletions
do not apply, and the feature waves argo needs collapse into one token-route wave. The plan is
short on purpose; padding it with waves for surfaces rollhook does not use would be fiction.

**Charts:** the dashboard's one chart (`packages/ui/src/components/DeployChart.tsx`) is **recharts 3.10.1,
not basalt**. It stays out of scope — no `CartesianChart` migration, no recharts removal. Only its
`.recharts-text…` CSS override in `apps/dashboard/src/styles/global.css` must keep resolving against the
re-emitted tokens (checked in Wave 3 by the visual diff).

**Decisions**
- **D1 — Dashboard token route = committed emit.** 2.0 deleted the `basalt-ui/tokens.css` subpath
  (§ 2.0.0 "Subpaths, peers and gates": → `basalt-ui tokens:css`; "weatherorb 1.30 and rollhook 1.36 only").
  The dashboard adopts marketing's pattern: `src/styles/basalt-tokens.css`, emitted with
  `--selector-class dark --only core --no-legacy-aliases`, `@import './basalt-tokens.css'` in `global.css`,
  a `tokens`/`tokens:check` script pair, `check:basalt` extended to cover it. This also retires the
  dashboard's "`--selector-class` is not reachable on this route" caveat. Probe-verified: the build
  passes on it.
- **D2 — Stay `tokens-only`, stay on Tailwind, keep our fonts.** Nothing in 2.x § 2.0.0–2.7.3 changes that;
  `doctor` under 2.7.3 reports "All checks passed" for both apps in the tokens-only profile.
- **D3 — Marketing deploys on merge.** `deploy-marketing.yml` fires on any master push touching
  `apps/marketing`. Wave 2 re-emits `basalt-tokens.css` (header + one light-scheme value), so it
  deploys. Accepted: the site is dark-only and the diff is invisible; Wave 4 still proves it live.
- **D4 — The server image ships by manual release.** The dashboard reaches users only through the
  **Make Release** workflow + `make rollhook-update` in `~/SourceRoot/vps` (`AGENTS.md` § Deploy). Merging
  Wave 2 deploys nothing for the dashboard; Wave 4 cuts the release.

---

## Wave 1 — Baseline evidence on 1.36.0 <!-- status: done -->

- [x] Read `node_modules/basalt-ui/MIGRATING.md` for **every version 1.36 → 2.7.3** (§ 1.36.1 … § 1.41.0,
      § 2.0.0 – § 2.7.3 and the "What changed, by surface" index), not just § 2.0.0 — rollhook once sat on a
      dead 0.4.2 for 19 minors after a dropped export. Record in `evidence/w1.md` § MIGRATING read, per
      version: rows touching tokens / CLI / `tokens-only` / `check-theme` / `--vx-*` (the Appendix A map is
      the starting point; confirm or amend it).
- [x] Record the 1.36.0 numbers (`make check` green; gzip JS/CSS from `bun run build:dashboard` and
      `bun run --filter @rollhook/marketing build`; dashboard `index-*.css` / `index-*.js`).
- [x] Screenshot every basalt-token-rendering screen, light n/a (dark-only), 1280×800 and 390×844:
      dashboard `TokenGate`, job list (`vite --mode demo` via `lib/demo.ts` data — a build with
      `MODE=demo`, served for the shot only, not left running), job detail + log view, empty and
      filtered states; marketing `/` (hero, code block, how-it-works, CTA). Raw PNGs gitignored; the file
      lists names + hashes.
- [x] Capture computed values for the probe-flagged token: `--vx-faint`, plus the ~21 `--vx-*` names the apps
      reference, via `get_css_styles` on one element per usage, to diff in Wave 3.
- [x] Commit `docs/waves/evidence/baseline-1.36.0.md` + `w1.md`. PR → `/review` → merge. No source change.

**Left behind:** full detail in `evidence/baseline-1.36.0.md` and `evidence/w1.md` (§ Findings for Wave 2).
- `make check` green on 1.36.0; numbers, screenshot hashes and computed-value JSONs (`evidence/vx-*-1.36.0.json`, shape `{vars, elements?}`) are committed; raw PNGs live in gitignored `evidence/shots/` on the mini.
- Wave 2 pins **2.8.0** (Target line). MIGRATING was read from the 2.8.0 tarball; Appendix A confirmed unchanged.
- **basalt friction** (orchestrator files as `rollhook W1` in `argo-friction.md`): `--vx-faint` light `#737376 → #69696c` has no MIGRATING row 1.36.1 → 2.8.0 (Appendix C #2, confirmed). Appendix C #1 is fixed in 2.8.0.

## Wave 2 — Atomic bump to 2.8.0 (plan text says 2.7.3; the Target line above overrides) <!-- status: active -->

Sized from the probe (Appendix B): 0 tsc errors in either app, `check:theme`/`sync --check` green, lint
0 errors — the only failures are the **dashboard build** (missing `./tokens.css` specifier) and
`tokens:check` drift. Two files of real change.

- [ ] Pin `basalt-ui` to exactly `2.7.3` in `apps/dashboard/package.json` and `apps/marketing/package.json`;
      `bun install`; `basalt-ui sync` in each app (expected: no-op, tokens-only has no scaffold).
- [ ] Dashboard (D1): add `tokens` / `tokens:check` scripts, emit `src/styles/basalt-tokens.css`, swap the
      `@import`, rewrite the stale caveat comment in `global.css`. Marketing: `bun run
      --filter @rollhook/marketing tokens`. Extend `check:basalt` to run the dashboard `tokens:check`.
- [ ] Update `AGENTS.md` in the same commit: "basalt-ui 1.x" → 2.x wording, the Styling table's dashboard
      row (`basalt-ui/tokens.css` → committed emit), "`--selector-class` not reachable" sentence, the
      "Never import `basalt-ui/css`/`styles.css`" line (add `basalt-ui/tokens.css`), the `18`→`16`
      Mantine-only kinds figure, "basalt-ui 1.x ships Nunito Sans + Hubot Sans" → still true on 2.x,
      re-verify.
- [ ] `make check` + `bun run test:e2e` green. PR → `/review` → merge.
- [ ] Friction rows filed for any basalt-rooted surprise (candidates in Appendix C), prefixed `rollhook W2`.

## Wave 3 — Visual check against the baseline (before any release) <!-- status: pending -->

- [ ] Rebuild both apps on the merged bump; capture the Wave 1 screenshot matrix and the computed-value
      list; diff pixel-wise / by value.
- [ ] Expected deltas: marketing `--vx-faint` light-scheme `#737376 → #69696c` (not rendered in a
      dark-only site) — everything else must be identical. Any other delta is either explained against
      MIGRATING.md or becomes a fix PR / friction row.
- [ ] Check the `.recharts-text…` override and `DeployChart` still read correctly (recharts is out of
      scope, its token inputs are not).
- [ ] Bundle bytes vs baseline (dashboard CSS/JS gzip, marketing). Write `evidence/w3.md`; PR → `/review` → merge.

## Wave 4 — Live verify <!-- status: pending -->

- [ ] Marketing: confirm the `deploy-marketing.yml` run from Wave 2 succeeded; load `rollhook.com`, compare
      against the baseline screenshots.
- [ ] Dashboard: run **Make Release** (`release.yml`, `workflow_dispatch`), then `make rollhook-update` in
      `~/SourceRoot/vps`. Rollback = previous immutable tag (`AGENTS.md` § Deploy).
- [ ] `make verify` (`/ready` → `"docker":"ok"`); open the live dashboard with the real token, screenshot
      job list + log view; `make logs` shows no startup errors; OTel `service.name = rollhook` still
      reporting.
- [ ] `evidence/w4.md`; if verification fails, the next PR fixes forward. Merge the evidence PR.

## Wave 5 — Fresh-agent DX trial <!-- status: pending -->

- [ ] A fresh agent (no migration context) adds a new dashboard view (e.g. a deploy-history panel) styled
      only with basalt tokens + Tailwind, and a new marketing section, following `AGENTS.md` alone.
- [ ] It runs `make check`; record every basalt-rooted wrong turn (stale AGENTS.md text, token name
      guesses, `check-theme` messages) as a `rollhook W5` row in basalt-ui `argo-friction.md`.
- [ ] Fix rollhook-side doc drift in `AGENTS.md`; do not merge the trial feature. PR for doc fixes →
      `/review` → merge; `evidence/w5.md`.

---

## Appendix A — Inventory (excludes `node_modules`, `.claude/worktrees`, `dist`)

`basalt-ui` / `basalt-ui/*` imports in `.ts` / `.tsx` / `.astro` / `.mjs`: **0**. Surface is CSS + CLI + pins:

| Where | Subpath / command | Symbols | 2.x fate |
|-|-|-|-|
| `apps/dashboard/src/styles/global.css:4` | `@import 'basalt-ui/tokens.css'` | `--vx-*` (21 names referenced in apps) | **Deleted** in 2.0.0 (§ Subpaths, peers and gates) → `basalt-ui tokens:css` committed emit (D1). **Build breaks**; tsc does not see it |
| `apps/marketing/src/styles/global.css:2` | `./basalt-tokens.css` (committed emit) | `--vx-*` | Kept; re-emit. Diff vs 1.36.0: header version line, light-scheme `--vx-faint #737376 → #69696c` |
| `apps/dashboard/package.json`, `apps/marketing/package.json` | dep pin `basalt-ui 1.36.0` (runtime dep in dashboard, devDep in marketing) | — | → `2.7.3`. Dashboard drops to build-time-only use after D1 → move to devDependency (decide in Wave 2) |
| both `package.json` | `"basalt": { "profile": "tokens-only" }` | — | Kept; `doctor` honours it on 2.7.3 |
| both `package.json` | `check-theme`, `check-theme --audit-allows` | 16 Mantine-only kinds off (was 18) | Kept; green on 2.7.3 |
| both `package.json` | `sync --check` | — | Kept; n/a message for tokens-only, exit 0 |
| `apps/marketing/package.json` | `tokens:css --selector-class dark --only core --no-legacy-aliases --out … [--check]` | — | Kept; `--check` fails once until re-emit (expected) |
| `apps/marketing/public/site.webmanifest:2` | `basalt:theme-allow-file` | `raw-hex@19,20` | Kept; `--audit-allows` 1 live / 0 dead |
| `.github/workflows/ci.yml:29` | `bun run check:basalt` | — | Kept |
| vite plugin / oxlint / tsconfig presets / lefthook preset | — | — | **None used** (vite config has no basalt plugin; lint is `@antfu/eslint-config`; no `.basalt/`, no managed AGENTS.md block, no `.claude/rules|skills`) |
| `fonts:css`, `BasaltProvider`, charts, shell, table, forms, controls, agent, commands, state, format | — | — | **Not used.** All 2.0 deletions are N/A |

**Unmapped symbols: none.** The only 1.x → 2.x break is the `./tokens.css` subpath; the silent part is
that nothing but `vite build` reports it. `v2-chart-inventory.md` already lists rollhook as "no basalt chart".

## Appendix B — Probe numbers (throwaway branch `tmp/basalt-2.7.3-probe`, deleted, never pushed)

Both pins → `2.7.3`, `bun install`.

| Check | Result |
|-|-|
| tsc `@rollhook/dashboard` (`tsc --noEmit`) | **0 errors** |
| tsc `@rollhook/marketing` (`astro check && tsc --noEmit`) | **0 errors**, 0 warnings, 0 hints (5 files) |
| Total tsc errors / per file / top codes | 0 / none / none |
| `bun run lint` | 0 errors, 2 warnings (`LogContainer.tsx:22,23` `no-direct-set-state-in-use-effect`, pre-existing, untouched) |
| `bunx basalt-ui doctor` (dashboard, marketing) | "All checks passed"; all tokens-only checks `n/a`; resolves 2.7.3 |
| `check:theme` + `--audit-allows` (both) | green; dashboard 0 waivers, marketing 1 live |
| `sync --check` (both) | exit 0 (n/a, tokens-only) |
| `tokens:check` (marketing) | **fails**: "differs from what `basalt-ui tokens:css` emits today (on disk 169 lines, emitted 169)" — header + `--vx-faint` |
| `vite build` dashboard, unchanged `global.css` | **fails**: `[@tailwindcss/vite:generate:build] Missing "./tokens.css" specifier in "basalt-ui" package` |
| `vite build` dashboard with committed emit (D1) | passes — CSS 30.28 kB (8.78 kB gz), JS 669.53 kB (204.84 kB gz) |
| `astro build` marketing | passes |
| Emit coverage | all 21 `--vx-*` names referenced by the apps exist in the 2.7.3 emit |

Representative messages: the vite error above; the `tokens:css --check` drift message above. Everything else clean.

## Appendix C — Friction candidates (file as `rollhook W2` rows if confirmed)

1. `tokens.css` removal is invisible to tsc, `doctor` ("All checks passed"), `check-theme` and `sync --check`
   — only `vite build` fails, with a Tailwind resolver error that names no migration. A tokens-only
   `doctor` check for `@import 'basalt-ui/tokens.css'` would catch it. (MIGRATING does name rollhook, in one table row.)
2. `--vx-faint` light value changed (`#737376 → #69696c`) with no MIGRATING row in 1.41→2.7.3 for tokens-only
   consumers (the surface index shows `tokens` rows ending at 1.38.1). Verify in Wave 1's read.

## Blast radius

0 `basalt-ui` imports · 5 files touched (2 `package.json`, `bun.lock`, dashboard `global.css` + new `basalt-tokens.css`, marketing `basalt-tokens.css`, `AGENTS.md`) · 0 tsc errors (1 dashboard build break) · 0 charts to rewrite (recharts out of scope) · deploy: marketing site on merge, dashboard via manual release into the self-hosted RollHook image · **small**

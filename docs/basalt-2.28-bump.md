# basalt-ui 2.11.1 → 2.28.0 bump notes

Both apps (`apps/dashboard`, `apps/marketing`) are tokens-only consumers: they ship the committed `--vx-*` emit and no basalt JS. The bump therefore changes exactly one artifact, `src/styles/basalt-tokens.css`, plus the pin.

## What changed

- `basalt-ui` pinned to exactly `2.28.0` in both apps; `bun run --filter '*' tokens` re-emitted both sheets (169 → 183 lines).
- `sync` and `doctor` are no-ops/clean for the `tokens-only` profile. `check:basalt` passes after the re-emit; `bun run lint` has no guard findings (two pre-existing `react-hooks-extra` warnings in `LogContainer.tsx`, untouched).
- Token deltas that reach the UI: `--vx-radius-card` 7→10px, `--vx-radius-floating` 8→11px, `--vx-text-xs` 12.5→12px, `--vx-text-sm` 13.5→13px, `--vx-text-kpi` 24→22px, `--vx-status-bad` / `--vx-bad-solid` `#e06761`→`#fe8178`, `--vx-axis` rgba→`#a7a7aa`; new `--vx-space-control-height-dense` and a `[data-basalt-dense]` block (Mantine variables, inert here).
- No consumer code needed changes. The `site.webmanifest` waiver is still live; `--vx-surface-bg` dark is still `#27272a`.

## Shots (`bunx basalt-ui shot`, 2.28.0 CLI for both sides)

BEFORE = the pre-bump tokens sheet, AFTER = the bumped one, both built from the same source with temporary `data-shot` anchors (not committed). Dashboard is a SPA behind a token gate: the list page is the `--mode demo` build, the gate is the plain build. Images live in each app's gitignored `.design-parity/w47/{before,after}/`.

| Page | Before → after | Detect before | Detect after |
|-|-|-|-|
| Dashboard `/` (demo data) | Same layout; "failed" red and chart bars turn lighter/pinker; radii slightly rounder | 75 (P0 contrast 3.68:1 on "failed" 12px) | 100 |
| Dashboard `/` (token gate) | Visually identical | 100 | 100 |
| Marketing `/` | Same layout; rounder cards, 0.5px smaller xs/sm text | 0 (1 P0, 9 P1) | 0 (same 10 findings) |

The marketing findings are identical before and after and are not raised by a guard: section paddings of 56/64/80/96/104px (Tailwind `py-14…py-26`) are off the spacing scale, `# IMAGE_TAG…` code comment text is 3.05:1, the footer is 40% empty, the CTA uses 4 font sizes. Left alone: out of scope for the bump.

## FRICTION

1. **`shot --out` must be inside the project.** `--out ../../.design-parity/…` fails with "must be a directory inside the project", so a monorepo cannot collect before/after shots from several apps in one root dir. Expected: allow any path under the repo root, or document it.
2. **Plain Tailwind apps have zero anchors.** `shot /` → "no data-shot anchors", and `shot detect` returns a P0 `scan/no-anchors` (score 75) for every page. `--auto-anchors` only finds Mantine `Card`/`Paper`/tables, so it also finds nothing in a tokens-only app. Expected: the tokens-only profile documents the `data-shot` route (or `--auto-anchors` falls back to `main`, `section`, `header`, `form`). A score of 75 for "nothing measured" reads as a result when it is not one.
3. **Score semantics.** A P0 caps the score at 75 and one P1 costs 10, so a page with a single contrast failure and a page with nothing measured both read 75. Marketing scores 0 from ten findings; the number says little about size of the problem.
4. **Shot needs Playwright in the app, and the install dirties the manifest.** `bun add -d playwright` edits `package.json` and `bun.lock`; we reverted it. Expected: `shot` can use a global/browser cache, or the docs say to revert it.
5. **`tokens:css --check` fails on every bump.** The error says so ("expect a no-op commit on every upgrade"), but the failure still blocks `check:basalt` until the emit is re-run. The bump procedure in AGENTS.md ("bump and re-run `tokens` in the same commit") covers it.
6. **Contrast regression was fixed silently by the token change.** `--vx-status-bad` moved to `#fe8178`, which cured a 3.68:1 failure under 12px text (the dashboard's `failed` badge), but MIGRATING.md did not call out the color shift as consumer-visible for tokens-only apps. Worse-reading side effect: the chart's failed bar and the row accent are now pink-leaning rather than red.
7. **Empty/loading/error states are never exercised.** "no data-shot-states declared" prints for every route; 42 shots are "not declared" on the dashboard. The CLI cannot tell a page that has no such states from one that forgot to declare them.
8. **Marketing spacing findings are Tailwind-native.** `py-20`, `py-24` are Tailwind's own scale, but detect judges them against basalt's 4/6/8/10/12/16/20/24/28/32/42 steps. A tokens-only consumer on Tailwind will always fail `craft-1/off-scale-inset` on section padding unless it rewrites to `--vx-space-*`. Expected: a Tailwind note in the profile docs, or a configurable scale.

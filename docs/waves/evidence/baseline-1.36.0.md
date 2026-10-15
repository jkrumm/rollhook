# Baseline — rollhook on basalt-ui 1.36.0

Recorded 2026-10-15 on `master` @ `0f65457`, before any bump. Both apps pinned `basalt-ui 1.36.0`.

## Gate

`make check`: **green** (exit 0, 2m35s). Lint, typecheck, `check:basalt`, Go build/vet/test, golangci-lint (0 issues), OpenAPI drift (no diff).

## Bundle bytes (`gzip -9` on the built file)

| Artifact | Raw | Gzip |
|-|-|-|
| dashboard `index-B8nJJhL-.css` | 36,539 | 9,916 |
| dashboard `index-B5EgFsc6.js` | 669,527 | 203,729 |
| marketing `_astro/index@_@astro.DgPuPxnf.css` | 22,269 | 5,129 |
| marketing `_astro/client.MHp4qZIg.js` | 222,900 | 69,246 |
| marketing `index.html` | 26,444 | 6,349 |

Vite's own report uses a different gzip level, so its figures differ slightly from the table; Wave 3 must use the table's `gzip -9` method. Vite's report for the dashboard: CSS 36.54 kB (9.95 kB gz), JS 669.53 kB (204.84 kB gz). The Appendix B probe on 2.7.3 + committed emit measured CSS 30.28 kB (8.78 kB gz), JS unchanged (669.53 kB): expect a ~6 kB CSS drop in Wave 3 (the 2.x emit omits what the 1.x subpath shipped), not a regression.

## Screenshots

Raw PNGs are gitignored (`docs/waves/evidence/shots/`). Dark-only, so no light variants. Dashboard shots come from `vite build --mode demo` (`lib/demo.ts` data) served statically for the capture only; `TokenGate` comes from the normal production build. Marketing is full-page.

Demo URLs: list `/`, detail `/?job=642dde5a-9b22-4660-a8b4-a81804d8550f` (success job, log view open), filtered `/?status=failed&app=rollhook-e2e-hello`, empty `/?status=running`.

| File | sha256[:12] |
|-|-|
| dash-tokengate-1280.png | 5764c57f53e1 |
| dash-tokengate-390.png | 86fd005773fb |
| dash-list-1280.png | c1d47c6a5b1c |
| dash-list-390.png | 31489b9cb0ac |
| dash-detail-1280.png | 49a7f2eaba68 |
| dash-detail-390.png | 6958ef460bfc |
| dash-filtered-1280.png | e682d4004312 |
| dash-filtered-390.png | 6eaf0e209587 |
| dash-empty-1280.png | 61b3bcf863e0 |
| dash-empty-390.png | 87472faad460 |
| mkt-home-1280.png | d26ec65a1b83 |
| mkt-home-390.png | e5be6366d38c |

Hashes identify the baseline files only; Wave 3 re-captures with the same URLs and viewports and diffs by pixels, not by hash (PNG encoding is not stable across Chrome versions).

## Computed values

Captured via `evaluate_script` (`getComputedStyle`) rather than `get_css_styles` per element: one call yields every declared `--vx-*`, and every rendered element.

Both files: `{ vars, declaredOnSelectors, elements }`.

- `vars` — every `--vx-*` custom property resolved on `:root` (dashboard 249, marketing 109).
- `declaredOnSelectors` — every selector that declares a `--vx-*`: only `:root`, the dark scope and the light scope (dashboard `html[data-mantine-color-scheme=…]`, marketing `:root.dark`/`:root.light`). No element-level override exists, so `:root` values plus the resolved element styles below cover every usage.
- `elements` — every `body *` element keyed by its DOM path (`body > div:nth-child(1) > …`, reproducible on the same demo build/URL), with computed `color`, `background-color`, `border-top-color`, `box-shadow`, `border-top-left-radius`, `font-family`, `font-size` (dashboard 381 from the job-detail view, marketing 249 from `/`).
- `--vx-faint`: dashboard computed `#a1a1a4` (dark). Marketing emit holds `#a1a1a4` (dark block, line 101) and `#737376` (light block, line 148); the latter is the value the 2.7.3 probe saw change to `#69696c`.

Wave 3 diffs both files key by key after the bump. The dashboard `vars` set will shrink because D1 switches to `--only core --no-legacy-aliases`; compare the names the apps reference, not the whole set.

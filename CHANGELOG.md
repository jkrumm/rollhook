# [1.9.0](https://github.com/jkrumm/rollhook/compare/v1.8.1...v1.9.0) (2026-03-17)


### Features

* **api:** add POST /auth/token for OIDC registry credential exchange ([d8c2a2d](https://github.com/jkrumm/rollhook/commit/d8c2a2d8fcf3c0be42ef567f907c56a2e73644da))

## [1.8.1](https://github.com/jkrumm/rollhook/compare/v1.8.0...v1.8.1) (2026-03-16)


### Bug Fixes

* **release:** correct Dockerfile path in docker build step ([fdeb50f](https://github.com/jkrumm/rollhook/commit/fdeb50fb5add2a46e20dd5beb8f70b4ab9a6999a))

# [1.8.0](https://github.com/jkrumm/rollhook/compare/v1.7.0...v1.8.0) (2026-03-16)


### Bug Fixes

* address CodeRabbit review findings ([cd58df4](https://github.com/jkrumm/rollhook/commit/cd58df4bf6a58453c2f7b84b5d74b35dba04a6f2))
* **coderabbit:** address all critical review findings ([1a84af3](https://github.com/jkrumm/rollhook/commit/1a84af392dade9796f2edbf2fcbfa1fe4048f3c9))
* **dashboard:** handle responsive layout on narrow viewports ([d47bf2c](https://github.com/jkrumm/rollhook/commit/d47bf2cc0c53f884513214f32246a976b73cefd8))
* **dev:** align demo:export token with dev server and auto-run on startup ([519f0d5](https://github.com/jkrumm/rollhook/commit/519f0d5a1fc973e302d31ea6e50c7f29f91b370c))
* **dev:** restore demo data and drop harmful dev:demo script ([e129eed](https://github.com/jkrumm/rollhook/commit/e129eedadbc28e681d3aa50b29c4af32fb7e1b6c))
* **docs:** update README, compose.yml, and mock-oidc for OIDC rollout ([443456a](https://github.com/jkrumm/rollhook/commit/443456a960f5617423349881bbfc4f2ec3b332a1))
* **examples:** replace stale bun healthcheck with curl ([a3b5224](https://github.com/jkrumm/rollhook/commit/a3b5224182bae7c3f3c3ab1c95ad9486e0727ade))
* **oidc:** address CodeRabbit follow-up findings ([c64c7c6](https://github.com/jkrumm/rollhook/commit/c64c7c6c5919798deb5f868e03d0cb9a5a05c2fb))
* **registry:** drop stream-json output format (requires --verbose) ([60b732a](https://github.com/jkrumm/rollhook/commit/60b732a279a350ef2f8169488e70b8b03b160e76))
* **registry:** fix RALPH runner hanging with empty log ([a1bbdc9](https://github.com/jkrumm/rollhook/commit/a1bbdc9f42b07dd0a254de8e82061012c02728d6))
* **registry:** resolve E2E test failures with Zot docker2s2 compat and auth fixes ([1b0c41d](https://github.com/jkrumm/rollhook/commit/1b0c41d50e14afbd5cc812b5b0015847aaa45f37))
* **registry:** use stream-json+verbose for realtime stdout flushing ([e043186](https://github.com/jkrumm/rollhook/commit/e0431861ced86888ab33cd8cf7301cc66a990400))
* remove WriteTimeout and upgrade Go to 1.24.13 ([6a7b079](https://github.com/jkrumm/rollhook/commit/6a7b079cb2527b0407e37d617d12b96cb323c335))
* resolve CodeRabbit findings for error handling and context cancellation ([d392040](https://github.com/jkrumm/rollhook/commit/d3920403396d40cff0d083dd6daf1391b4c6c9eb))
* **rollout:** log each docker compose output line separately ([2a415d6](https://github.com/jkrumm/rollhook/commit/2a415d662d3e581472ab5c46beed0743b7f9643e))


### Features

* **auth:** implement GitHub Actions OIDC for RollHook deploys ([c8c8cd1](https://github.com/jkrumm/rollhook/commit/c8c8cd191ce5209f26e9ea5bebd33ca95dd92428))
* **auth:** replace dual-token model with single ROLLHOOK_SECRET + startup validation ([5acda13](https://github.com/jkrumm/rollhook/commit/5acda13dc582cb87d75ade4aaf81a518125ac8e6))
* **dashboard,ui:** add RollHook dashboard with shared UI component library ([3a4d027](https://github.com/jkrumm/rollhook/commit/3a4d027ae9c3fb297d03788cb7c640adcf43101c))
* **dashboard:** add orval API client generation from Go OpenAPI spec ([7d9a7f2](https://github.com/jkrumm/rollhook/commit/7d9a7f29f95dc4d1b9f05103f5456a72df9d545b))
* **dev:** add dev:server script for local Go development ([b5f008a](https://github.com/jkrumm/rollhook/commit/b5f008a35a25df6ad5de57f4d12a1e08c304d3bb))
* **go-rewrite:** add 10-group RALPH loop for Go server migration ([6e847cd](https://github.com/jkrumm/rollhook/commit/6e847cd275119ba0b6ec8cd3c7b8bce415b29300))
* **logging:** reduce noise and add stale job cleanup ([1f0d911](https://github.com/jkrumm/rollhook/commit/1f0d9117bbdb742d1c18b1d160da3705556c9c93))
* **marketing:** upgrade to Astro 6 with native Fonts API ([2ee0422](https://github.com/jkrumm/rollhook/commit/2ee0422de6f06c1a1cd92fbc35e051bd86a8a6f8))
* **registry:** add OCI reverse proxy and migrate E2E off registry:2 ([863cc98](https://github.com/jkrumm/rollhook/commit/863cc98e05f5c34515a09e6dcc574e3d13679d6f))
* **registry:** add Zot binary to Docker image and process manager ([0d2f5ce](https://github.com/jkrumm/rollhook/commit/0d2f5cef11665576fc8a33c199be7d1b003fe4d4))
* **release:** publish server image to ghcr.io ([e7273a1](https://github.com/jkrumm/rollhook/commit/e7273a117674c0fd9580cff4ba8e85204833166c))
* **server:** add auth middleware, huma OpenAPI 3.1, and Scalar docs ([14e5515](https://github.com/jkrumm/rollhook/commit/14e5515b2146fd36e5917532b5b77135709dcbd8))
* **server:** add job queue, service discovery, and compose validation ([15b084a](https://github.com/jkrumm/rollhook/commit/15b084a6a5e24052325e02ef66173ed22db43757))
* **server:** add SQLite persistence layer and job CRUD ([78ee266](https://github.com/jkrumm/rollhook/commit/78ee26668cd86cd40dc08f7c99491bc2a72118ac))
* **server:** add Zot process manager and streaming OCI proxy ([0fba52f](https://github.com/jkrumm/rollhook/commit/0fba52f1be7ce91d2e3d10de9011a6a6f08dc4e5))
* **server:** implement full Go HTTP API surface and wire all components ([8bb0ba7](https://github.com/jkrumm/rollhook/commit/8bb0ba70a7a3e7ec64d450a0bb9d3c81b98f3aaa))
* **server:** implement pull, rolling deploy, and notification pipeline ([df69e36](https://github.com/jkrumm/rollhook/commit/df69e36fe67e176d94da40f8807519110d4bbc7d))
* **server:** initialize Go module and project skeleton ([43e6f65](https://github.com/jkrumm/rollhook/commit/43e6f651d0ce5ad72bcbb451118652fcbbd29c88))
* **server:** initialize Go module and project skeleton ([9222e71](https://github.com/jkrumm/rollhook/commit/9222e71e52905481733f0a55466630f399662fde))
* **server:** replace hand-rolled Docker HTTP layer with official Go SDK ([43a49f0](https://github.com/jkrumm/rollhook/commit/43a49f0e6dddc134a9ce0f209c0bf06053a3f8d2)), closes [hi#signal](https://github.com/hi/issues/signal)
* **validate:** enforce deployment validation rules + expand E2E coverage ([7a1786e](https://github.com/jkrumm/rollhook/commit/7a1786e1341147d41e68ccd683eb6094b4b42a40))

# [1.7.0](https://github.com/jkrumm/rollhook/compare/v1.6.0...v1.7.0) (2026-03-01)


### Bug Fixes

* **deploy:** fix critical bugs in rolling deploy, harden Docker adapter ([c5d6d0a](https://github.com/jkrumm/rollhook/commit/c5d6d0a019f8af96e19a0debd50f6bd94225ad53)), closes [hi#level](https://github.com/hi/issues/level)
* **e2e:** update tests for stateless app derivation from image tag ([30e979d](https://github.com/jkrumm/rollhook/commit/30e979d469db7382bf24a29168e852486ba3dd20))
* **review:** address all CodeRabbit review findings ([8913aa2](https://github.com/jkrumm/rollhook/commit/8913aa28d2df53019158916a2f81e6f48f00789e))


### Features

* **deploy:** replace docker-rollout with TypeScript rolling deploy via Docker API ([4aed7be](https://github.com/jkrumm/rollhook/commit/4aed7bed21f44710b0dcffffd99219a7034c7e61))
* **e2e,examples:** containerize RollHook in E2E, add production reference stacks ([d498ce4](https://github.com/jkrumm/rollhook/commit/d498ce4a315c46d4d1872765a36033abb48eec1b))

# [1.6.0](https://github.com/jkrumm/rollhook/compare/v1.5.1...v1.6.0) (2026-02-28)


### Features

* derive app from image_tag, remove :app from deploy route ([3621aad](https://github.com/jkrumm/rollhook/commit/3621aad73481a79e515dacc591e5a7e61ea13523))

## [1.5.1](https://github.com/jkrumm/rollhook/compare/v1.5.0...v1.5.1) (2026-02-28)


### Bug Fixes

* **docs:** add app input to rollhook-action example, clarify default ([8dca5f9](https://github.com/jkrumm/rollhook/commit/8dca5f98a15f8a4bbc1c1ae104896c21a43bca45))

# [1.5.0](https://github.com/jkrumm/rollhook/compare/v1.4.2...v1.5.0) (2026-02-28)


### Features

* stateless discovery, improved E2E and auth ([b12777f](https://github.com/jkrumm/rollhook/commit/b12777fd717ebf08ed8df2e5adb98674cf7c783d))

## [1.4.2](https://github.com/jkrumm/rollhook/compare/v1.4.1...v1.4.2) (2026-02-28)


### Bug Fixes

* **ci:** pass admin_token to rollhook-action for job polling ([9676173](https://github.com/jkrumm/rollhook/commit/96761734e2464d8505e8ec5fb2e80e943d2da63b))

## [1.4.1](https://github.com/jkrumm/rollhook/compare/v1.4.0...v1.4.1) (2026-02-28)


### Bug Fixes

* **ci:** show response body on failed deploy webhook curl ([569f3cc](https://github.com/jkrumm/rollhook/commit/569f3cc2b09dea7e9827fd5f87180d4e21c58bdf))
* **infra:** add NETWORKS=1 and registry credential mount to socket proxy reference ([386cddd](https://github.com/jkrumm/rollhook/commit/386cddd5340a470813df7912ce28fce5a9697507))

# [1.4.0](https://github.com/jkrumm/rollhook/compare/v1.3.0...v1.4.0) (2026-02-27)


### Features

* **ci:** deploy marketing site to VPS via RollHook after release ([900c0f2](https://github.com/jkrumm/rollhook/commit/900c0f2d529d70f28ae03a2f6ca42d133dd4a5cc))
* **e2e:** comprehensive test suite with server stability fixes ([5c2e6e1](https://github.com/jkrumm/rollhook/commit/5c2e6e1ee494b31b4387ca69a488f4918000c140))
* **marketing:** implement comprehensive SEO improvements ([505dd7d](https://github.com/jkrumm/rollhook/commit/505dd7d816a1c1e83bdd69785488fda1c6f34f05)), closes [#1e1f25](https://github.com/jkrumm/rollhook/issues/1e1f25)


### Performance Improvements

* **marketing:** compress og.png 1.5MB → 504KB via pngquant ([5eedec9](https://github.com/jkrumm/rollhook/commit/5eedec9a87a7b55e0311a52b183eeebb4ab1905e))

# [1.3.0](https://github.com/jkrumm/rollhook/compare/v1.2.0...v1.3.0) (2026-02-27)


### Bug Fixes

* **e2e:** use async deploy in zero-downtime test ([67dba14](https://github.com/jkrumm/rollhook/commit/67dba142bc6e11be1b446eb338970dcc5882f19d))
* **examples:** replace direct docker.sock mount with read-only socket proxy for Traefik ([ff90437](https://github.com/jkrumm/rollhook/commit/ff904374cfdc7b786735ea498a5385ea738b65b6))
* **executor:** avoid Bun spawn ENOENT by mutating process.env for IMAGE_TAG ([1e39597](https://github.com/jkrumm/rollhook/commit/1e39597731b86d91d8da63e2dffd3abe597b31e9))
* **executor:** warm up posix_spawn to prevent ENOENT on first docker rollout ([70415d3](https://github.com/jkrumm/rollhook/commit/70415d3ddeb0e79b19b3995e56b7b87ab41f197f))
* import order and HTML tag in test/marketing files ([9a0878d](https://github.com/jkrumm/rollhook/commit/9a0878d86041bec81323aa815b96f2b44d55f5a4))
* **lint:** pin @antfu/eslint-config and treat bun:* as builtins ([efdd18c](https://github.com/jkrumm/rollhook/commit/efdd18c0b3ce199979ce439f993be088c5f4c222))


### Features

* bundle docker-compose, expose version in /health endpoint ([d50ed7d](https://github.com/jkrumm/rollhook/commit/d50ed7d1e00893f67a6da535ba20c02fcb963aca))

# [1.2.0](https://github.com/jkrumm/rollhook/compare/v1.1.1...v1.2.0) (2026-02-26)


### Bug Fixes

* **ci:** add explicit OCI url label to Docker images ([dfe0ef1](https://github.com/jkrumm/rollhook/commit/dfe0ef13a47085cf75fe638e026c206a8c605854))


### Features

* **deploy:** make deployment endpoint synchronous by default ([d65ee09](https://github.com/jkrumm/rollhook/commit/d65ee090520faa1ae9076921b11f47233062d501))
* **jobs:** add timestamps to logs and queued deployment entry ([7b0fefa](https://github.com/jkrumm/rollhook/commit/7b0fefae3ad1e2a5b9c390211a2ac7581cfbd9ab))
* **marketing:** sharpen brand voice and redesign homepage ([80ef7f3](https://github.com/jkrumm/rollhook/commit/80ef7f3a1e157ed234d3f0d58bb97bba156e1a47))

## [1.1.1](https://github.com/jkrumm/rollhook/compare/v1.1.0...v1.1.1) (2026-02-26)


### Bug Fixes

* **docker:** run bun install in runner stage to fix broken symlinks ([37efe65](https://github.com/jkrumm/rollhook/commit/37efe65e7ec1f09d510fcc7c56e304b23911a76f))

# [1.1.0](https://github.com/jkrumm/rollhook/compare/v1.0.8...v1.1.0) (2026-02-26)


### Features

* **registry:** persist config changes to rollhook.config.yaml ([d76ce71](https://github.com/jkrumm/rollhook/commit/d76ce71fedf21a603bed6fbbc1cbf0be0a365d65))

## [1.0.8](https://github.com/jkrumm/rollhook/compare/v1.0.7...v1.0.8) (2026-02-25)


### Bug Fixes

* **ci:** drop --new-bundle-format flag, keep --tlog-upload=false for cosign v3 ([6173089](https://github.com/jkrumm/rollhook/commit/617308957b807a8547472acaa688103e4e10b21b))

## [1.0.7](https://github.com/jkrumm/rollhook/compare/v1.0.6...v1.0.7) (2026-02-25)


### Bug Fixes

* **ci:** add --tlog-upload=false for cosign v3 Zot-compatible signing ([f33fd0a](https://github.com/jkrumm/rollhook/commit/f33fd0a180b11900da4f247e24d2f21b7a9bdd99))

## [1.0.6](https://github.com/jkrumm/rollhook/compare/v1.0.5...v1.0.6) (2026-02-25)


### Bug Fixes

* **ci:** use old cosign bundle format for Zot compatibility ([ab80efb](https://github.com/jkrumm/rollhook/commit/ab80efbf27e5cb8d4af3573c7c4a08a49ef4d332))

## [1.0.5](https://github.com/jkrumm/rollhook/compare/v1.0.4...v1.0.5) (2026-02-25)


### Bug Fixes

* **ci:** pin cosign-installer to v4.0.0 SHA ([0c42614](https://github.com/jkrumm/rollhook/commit/0c4261470a612f362dd3b22a550370e43ff80a7b))

## [1.0.4](https://github.com/jkrumm/rollhook/compare/v1.0.3...v1.0.4) (2026-02-25)


### Bug Fixes

* **docker:** skip lifecycle scripts during bun install ([1b81b35](https://github.com/jkrumm/rollhook/commit/1b81b351fc975a903f263d40de05a3bd71bf9802))

## [1.0.3](https://github.com/jkrumm/rollhook/compare/v1.0.2...v1.0.3) (2026-02-25)


### Bug Fixes

* **rollout:** resolve docker binary path at module load via Bun.which ([0a79143](https://github.com/jkrumm/rollhook/commit/0a79143a480b62d64d669c7097defaf2199c83d9))

## [1.0.2](https://github.com/jkrumm/rollhook/compare/v1.0.1...v1.0.2) (2026-02-25)


### Bug Fixes

* **docker:** exclude e2e dir but keep e2e/package.json for bun workspace ([baa4b4a](https://github.com/jkrumm/rollhook/commit/baa4b4a249a2dc53fc077312933c2da750f701ee))

## [1.0.1](https://github.com/jkrumm/rollhook/compare/v1.0.0...v1.0.1) (2026-02-25)


### Bug Fixes

* **docker:** copy all workspace package.json files to satisfy bun workspace resolution ([7435509](https://github.com/jkrumm/rollhook/commit/7435509a14d55d456632cf31554f3e25f415d988))

# 1.0.0 (2026-02-25)


### Bug Fixes

* **ci:** disable commitlint body-max-line-length for semantic-release compatibility ([46130a6](https://github.com/jkrumm/rollhook/commit/46130a6be15c40196d950262883f6719eac53a62))
* **ci:** pin vite@6 in web app to fix typecheck, add E2E log artifact on failure ([2096722](https://github.com/jkrumm/rollhook/commit/20967224996999187874fb07511c6b207a80b97b))
* **ci:** track bun.lock + fix docker-rollout download URL ([595d208](https://github.com/jkrumm/rollhook/commit/595d20851a41c08f237d2cee90c9562f82337664))
* **e2e:** fix three root causes blocking test suite ([06014f6](https://github.com/jkrumm/rollhook/commit/06014f661b55fb40a3b0e52eb3410ff19b01b813))
* **executor:** ensure loadConfig errors mark jobs as failed ([f5db01a](https://github.com/jkrumm/rollhook/commit/f5db01a99012e597c47efd478509f094c5c2516b))
* **release:** use Node LTS + npm for semantic-release (requires Node 22.14+) ([6a9d5f3](https://github.com/jkrumm/rollhook/commit/6a9d5f3ca0feb3a9c1e06994be5e665812d31a1f))
* resolve Vite HMR port conflict in web app ([9c0998a](https://github.com/jkrumm/rollhook/commit/9c0998a89d90be98f98446b7fc113129d256d1ab))
* **test:** reduce container proliferation and fix E2E test failures ([c96278b](https://github.com/jkrumm/rollhook/commit/c96278b6f159354fa6be239be1ee25ab9fc7fe4b))


### Features

* **api:** set up Eden Treaty isomorphic client with file-based store ([ee661e6](https://github.com/jkrumm/rollhook/commit/ee661e6b0b0e7974b8fd8ebd7d1d7321b3ba2e66))
* **apps/web:** scaffold Astro marketing site with basalt-ui ([1daac56](https://github.com/jkrumm/rollhook/commit/1daac56f88be228891192155367e9c50ed077aaa))
* **config:** move Pushover credentials to environment variables ([5517a1f](https://github.com/jkrumm/rollhook/commit/5517a1fbd8646ad84658eb627322e59dfcd8a6cd))
* implement single-server architecture on port 7700 ([d54292c](https://github.com/jkrumm/rollhook/commit/d54292cbcabe2bc4b8445900fd77751686b1d855))
* pivot to webhook-driven deployment orchestrator ([25f5bb2](https://github.com/jkrumm/rollhook/commit/25f5bb27697102339ce32beaa5cadf30d1701c46))
* **rollhook:** implement core deployment orchestrator with path aliases ([71ff781](https://github.com/jkrumm/rollhook/commit/71ff7810ced846f0d3febb7ed319f02e475597f9))
* **web:** add Astro marketing site with basalt-ui ([8224303](https://github.com/jkrumm/rollhook/commit/8224303abb390e980ac1c3042f1a92881f4fd8dd))

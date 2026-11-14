# RollHook Makefile — thin wrappers over the repo's own scripts and the exact CI gates.
# Self-documenting: `make` (or `make help`) lists every target below.
# Go runs with the native toolchain: go.mod's `toolchain` line makes `go` fetch the exact version itself,
# and golangci-lint is version-checked. `make check` needs no Docker; GO_VIA=docker restores the pinned
# container path for a machine without Go (see AGENTS.md §Validate).

.DEFAULT_GOAL := help

GO_VIA       ?= native
GO_IMAGE     := golang:1.27.3-alpine
LINT_VERSION := v2.15.0
LINT_IMAGE   := golangci/golangci-lint:$(LINT_VERSION)

ifeq ($(GO_VIA),docker)
GO_RUN   := docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(GO_IMAGE)
LINT_RUN := docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(LINT_IMAGE) golangci-lint run
else ifeq ($(GO_VIA),native)
GO_RUN   :=
LINT_RUN := @golangci-lint --version | grep -q "version $(LINT_VERSION:v%=%) " || { echo "golangci-lint $(LINT_VERSION) required (found: $$(golangci-lint --version)); install it or use GO_VIA=docker"; exit 1; }; golangci-lint run
else
$(error GO_VIA must be native or docker)
endif

# Production coordinates: the public prod domain and an ssh alias; the environment overrides both.
ROLLHOOK_URL      ?= https://rollhook.jkrumm.com
ROLLHOOK_SSH_HOST ?= vps

.PHONY: help check check-js check-go image image-smoke deploy verify logs

help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

check: check-js check-go ## Run exactly the local validation CI runs: lint, format, typecheck, basalt, dashboard tests, then Go (no Docker with GO_VIA=native, the default)

check-js:
	bun install --frozen-lockfile
	bun run lint
	bun run format:check
	bun run typecheck
	bun run check:basalt
	bun run test:unit

check-go: ## Go build/vet/test, golangci-lint and OpenAPI drift (GO_VIA=docker runs them in the pinned images)
	$(GO_RUN) go build ./...
	$(GO_RUN) go vet ./...
	$(GO_RUN) go test ./...
	$(LINT_RUN)
	$(GO_RUN) go run ./cmd/gendocs 2>/dev/null > /tmp/rollhook-openapi.json
	diff -u apps/dashboard/openapi.json /tmp/rollhook-openapi.json

image: ## Build the server image (rollhook:local) from the Dockerfile
	docker build -t rollhook:local .

image-smoke: image ## Smoke-test the built image: non-root user, DATA_DIR=/data owned by rollhook, named-volume data survives recreate
	docker run --rm --entrypoint sh rollhook:local -c 'test "$$(id -un)" = rollhook && test "$$DATA_DIR" = /data && test "$$(stat -c %U /data)" = rollhook'
	docker volume rm -f rollhook-smoke >/dev/null
	docker run --rm -v rollhook-smoke:/data --entrypoint sh rollhook:local -c 'echo kept > /data/probe'
	docker run --rm -v rollhook-smoke:/data --entrypoint sh rollhook:local -c 'test "$$(cat /data/probe)" = kept'
	docker volume rm rollhook-smoke >/dev/null
	@echo "image-smoke: ok"

deploy: ## CI-deployed: the server image ships via the manual Make Release workflow, the marketing site on push
	@echo "deployed by CI on push (marketing site); the server image ships via the manual 'Make Release' workflow and the VPS pulls :latest - see AGENTS.md section Deploy. Nothing to run here."

verify: ## Probe production readiness at ROLLHOOK_URL (default: the public prod domain); exit 0 = live and healthy
	@curl -fsS --max-time 20 "$(ROLLHOOK_URL)/ready" | grep -q '"docker":"ok"' \
		&& echo "rollhook: healthy" \
		|| { echo "rollhook: UNHEALTHY ($(ROLLHOOK_URL)/ready)"; exit 1; }

logs: ## Bounded 200-line tail of production logs over ssh to ROLLHOOK_SSH_HOST (default: the vps alias), then exits (no follow)
	@ssh "$(ROLLHOOK_SSH_HOST)" 'docker logs --tail 200 $$(docker ps -q --filter "label=com.docker.compose.service=rollhook" | head -n1)'

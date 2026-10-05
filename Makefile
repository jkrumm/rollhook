# RollHook Makefile — thin wrappers over the repo's own scripts and the exact CI gates.
# Self-documenting: `make` (or `make help`) lists every target below.
# Go is not installed locally (see AGENTS.md §Validate), so Go runs in the pinned image.

.DEFAULT_GOAL := help

GO_IMAGE   := golang:1.25-alpine
GO_RUN     := docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(GO_IMAGE)
LINT_IMAGE := golangci/golangci-lint:v2.10.1

# Production coordinates: the public prod domain and an ssh alias; the environment overrides both.
ROLLHOOK_URL      ?= https://rollhook.jkrumm.com
ROLLHOOK_SSH_HOST ?= vps

.PHONY: help check deploy verify logs

help: ## List available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

check: ## Run exactly the local validation CI runs (lint, typecheck, basalt, Go build/vet/test/lint, OpenAPI drift)
	bun install --frozen-lockfile
	bun run lint
	bun run typecheck
	bun run check:basalt
	$(GO_RUN) go build ./...
	$(GO_RUN) go vet ./...
	$(GO_RUN) go test ./...
	docker run --rm -v "$(CURDIR)":/workspace -w /workspace $(LINT_IMAGE) golangci-lint run
	$(GO_RUN) go run ./cmd/gendocs 2>/dev/null > /tmp/rollhook-openapi.json
	diff -u apps/dashboard/openapi.json /tmp/rollhook-openapi.json

deploy: ## CI-deployed: the server image ships via the manual Make Release workflow, the marketing site on push
	@echo "deployed by CI on push (marketing site); the server image ships via the manual 'Make Release' workflow and the VPS pulls :latest - see AGENTS.md section Deploy. Nothing to run here."

verify: ## Probe production readiness at ROLLHOOK_URL (default: the public prod domain); exit 0 = live and healthy
	@curl -fsS --max-time 20 "$(ROLLHOOK_URL)/ready" | grep -q '"docker":"ok"' \
		&& echo "rollhook: healthy" \
		|| { echo "rollhook: UNHEALTHY ($(ROLLHOOK_URL)/ready)"; exit 1; }

logs: ## Bounded 200-line tail of production logs over ssh to ROLLHOOK_SSH_HOST (default: the vps alias), then exits (no follow)
	@ssh "$(ROLLHOOK_SSH_HOST)" 'docker logs --tail 200 $$(docker ps -q --filter "label=com.docker.compose.service=rollhook" | head -n1)'

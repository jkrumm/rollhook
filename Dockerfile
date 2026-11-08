# syntax=docker/dockerfile:1
FROM oven/bun:1.3.10-alpine AS dashboard-builder
WORKDIR /app
COPY package.json bun.lock tsconfig.json ./
COPY apps/dashboard/package.json apps/dashboard/
COPY apps/marketing/package.json apps/marketing/
COPY packages/ui/package.json packages/ui/
COPY e2e/package.json e2e/
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile --ignore-scripts
COPY apps/dashboard/ apps/dashboard/
COPY packages/ui/ packages/ui/
RUN bun run --filter @rollhook/dashboard build

FROM golang:1.27.3-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /rollhook ./cmd/rollhook

# Download Docker CLI static binary and Zot in a throw-away stage.
# Static binaries work on any Linux regardless of libc version.
FROM alpine:3.21 AS tool-downloader
ARG TARGETARCH
ARG DOCKER_VERSION=29.9.3
ARG DOCKER_AMD64_SHA256=3fb5d1f13014c18cc54945bc177c58a9056a482dada031e17d12cf2e6922c973
ARG DOCKER_ARM64_SHA256=1b27fd739378ef938401e81ce5afb73b7ad7432d4f598814a9ff5f826bb464d2
ARG COMPOSE_VERSION=2.40.3
ARG COMPOSE_AMD64_SHA256=dba9d98e1ba5bfe11d88c99b9bd32fc4a0624a30fafe68eea34d61a3e42fd372
ARG COMPOSE_ARM64_SHA256=d26373b19e89160546d15407516cc59f453030d9bc5b43ba7faf16f7b4980137
RUN apk add --no-cache curl \
  && if [ "$TARGETARCH" = "arm64" ]; then \
       DOCKER_ARCH=aarch64; DOCKER_SHA="${DOCKER_ARM64_SHA256}"; COMPOSE_SHA="${COMPOSE_ARM64_SHA256}"; \
     else \
       DOCKER_ARCH=x86_64; DOCKER_SHA="${DOCKER_AMD64_SHA256}"; COMPOSE_SHA="${COMPOSE_AMD64_SHA256}"; \
     fi \
  && curl -fsSL "https://download.docker.com/linux/static/stable/${DOCKER_ARCH}/docker-${DOCKER_VERSION}.tgz" -o /tmp/docker.tgz \
  && echo "${DOCKER_SHA}  /tmp/docker.tgz" | sha256sum -c - \
  && tar xz --strip-components=1 -C /usr/local/bin -f /tmp/docker.tgz docker/docker \
  && curl -fsSL "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-linux-${DOCKER_ARCH}" \
     -o /usr/local/bin/docker-compose-plugin \
  && echo "${COMPOSE_SHA}  /usr/local/bin/docker-compose-plugin" | sha256sum -c - \
  && chmod +x /usr/local/bin/docker-compose-plugin

ARG ZOT_VERSION=2.1.14
ARG ZOT_AMD64_SHA256=c96e2394e1d94ddd3439f3b19d1d2b707e5bcbf34fec49532805bf3cd734bfc7
ARG ZOT_ARM64_SHA256=e447ebe874d9b906feb2c769d535f1c8f4682f479a1320572c5201684b81c17c
RUN ZOT_ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "amd64") \
  && ZOT_SHA=$([ "$TARGETARCH" = "arm64" ] && echo "${ZOT_ARM64_SHA256}" || echo "${ZOT_AMD64_SHA256}") \
  && curl -fsSL \
     "https://github.com/project-zot/zot/releases/download/v${ZOT_VERSION}/zot-linux-${ZOT_ARCH}" \
     -o /usr/local/bin/zot \
  && echo "${ZOT_SHA}  /usr/local/bin/zot" | sha256sum -c - \
  && chmod +x /usr/local/bin/zot

# debian:12-slim provides glibc required by Zot's pre-built binaries.
FROM debian:12-slim AS runner
WORKDIR /app

# Install curl (healthcheck) and ca-certificates (OIDC TLS verification).
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=tool-downloader /usr/local/bin/docker /usr/local/bin/docker
RUN mkdir -p /usr/local/lib/docker/cli-plugins
COPY --from=tool-downloader /usr/local/bin/docker-compose-plugin /usr/local/lib/docker/cli-plugins/docker-compose
COPY --from=tool-downloader /usr/local/bin/zot /usr/local/bin/zot
COPY --from=go-builder /rollhook /usr/local/bin/rollhook
COPY --from=dashboard-builder /app/apps/dashboard/dist/ /app/public/

ARG VERSION=dev
ENV VERSION=$VERSION

# /data holds the SQLite DB, job logs and the embedded registry. It is created
# owned by rollhook so a named volume mounted here inherits that ownership on
# first use; the compose stack mounts its volume at /data.
ENV DATA_DIR=/data
RUN useradd --system --home-dir /app --shell /usr/sbin/nologin rollhook \
  && mkdir -p /data \
  && chown rollhook:rollhook /data \
  && chown -R rollhook:rollhook /app

# Liveness only — deliberately /health, NOT /ready. Do not "fix" this.
# Traefik's Docker provider drops unhealthy containers from its config entirely,
# so probing a dependency here 404s every route on the host, registry included.
# See docs/GO_GOTCHAS.md → "Traefik (reference stack)".
# Point monitoring (Uptime Kuma, Prometheus, …) at /ready instead.
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=5 \
  CMD curl -sf http://localhost:7700/health || exit 1

EXPOSE 7700
USER rollhook
CMD ["/usr/local/bin/rollhook"]

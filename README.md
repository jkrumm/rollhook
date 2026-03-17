# RollHook

**Webhook-triggered zero-downtime rolling deployments for Docker Compose.**

[![Docker](https://img.shields.io/badge/ghcr.io-jkrumm%2Frollhook-blue)](https://github.com/jkrumm/rollhook/pkgs/container/rollhook)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Receives a deploy webhook from GitHub Actions, pulls the new image, rolls it out one container at a time — each gated on a healthcheck passing — streams logs live to CI. No config files. Stateless auto-discovery from running container labels.

**Built-in OCI registry** — push your images directly to RollHook, no GHCR or Docker Hub needed. One service, one secret, zero external dependencies.

---

## Quick Start

### 1. Run RollHook on your VPS

Copy [`compose.yml`](compose.yml) from this repo to your server and create a `.env` file next to it:

```env
ACME_EMAIL=you@example.com
ROLLHOOK_SECRET=changeme          # openssl rand -hex 32
COMPOSE_DIR=/home/user/myapp      # directory where your compose.yml lives
```

Secrets managers like [Doppler](https://doppler.com) and [Infisical](https://infisical.com) both support Docker Compose natively as `.env` alternatives.

Then start the stack:

```bash
docker compose up -d
```

The included `compose.yml` contains Traefik (with automatic TLS via Let's Encrypt), RollHook, and a placeholder `app` service — replace it with your own.

### 2. Configure your app's compose.yml

Four requirements for zero-downtime:

```yaml
services:
  api:
    image: ${IMAGE_TAG:-rollhook.example.com/my-api:latest} # 1. IMAGE_TAG var — use RollHook as registry
    healthcheck: # 2. healthcheck required
      test: [CMD, curl, -f, http://localhost:3000/health]
      interval: 5s
      timeout: 5s
      start_period: 10s
      retries: 5
    # 3. No ports: — proxy routes via Docker DNS, ports: blocks scaling
    # 4. No container_name: — fixed names prevent a second instance from starting
    labels:
      - rollhook.allowed_repos=myorg/my-api # authorize your GitHub repo
    networks:
      - proxy

networks:
  proxy:
    external: true
```

Start the app manually once so RollHook can discover it from the running container's labels:

```bash
docker compose -f /srv/stacks/my-api/compose.yml up -d
```

### 3. Trigger deploys from GitHub Actions

RollHook ships a built-in OCI registry — push your image directly to RollHook, no separate registry needed. Deploys use GitHub Actions OIDC, so no deploy token sits in your CI secrets.

```yaml
name: Deploy
on:
  push:
    branches: [main]

permissions:
  id-token: write # required for OIDC token exchange
  contents: read # required for actions/checkout

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: jkrumm/rollhook-action@v1
        with:
          url: ${{ vars.ROLLHOOK_URL }}
          image_name: my-api
```

**What you need in GitHub:**

| Where                | What                                            |
| -------------------- | ----------------------------------------------- |
| Settings → Variables | `ROLLHOOK_URL` = `https://rollhook.example.com` |

No secrets needed. The action exchanges the GitHub Actions OIDC token for a short-lived registry credential automatically.

The action streams SSE logs live to CI and fails the step if the deployment fails. See [jkrumm/rollhook-action](https://github.com/jkrumm/rollhook-action) for full docs.

---

## Graceful Shutdown

Your app needs to handle `SIGTERM` cleanly — otherwise the proxy may route requests to a container that has already stopped accepting connections.

Pattern: on `SIGTERM`, return `503` from `/health` (proxy stops routing), wait 2–3 s for deregister, drain in-flight requests, exit.

```ts
// Bun
let isShuttingDown = false

process.on('SIGTERM', async () => {
  isShuttingDown = true
  await new Promise(resolve => setTimeout(resolve, 3000))
  await server.stop(true)
  process.exit(0)
})

// health handler:
if (pathname === '/health')
  return new Response('ok', { status: isShuttingDown ? 503 : 200 })
```

See [`e2e/hello-world/`](e2e/hello-world/) for a complete reference app.

---

## Environment Variables

| Var                        | Required | Description                                        |
| -------------------------- | -------- | -------------------------------------------------- |
| `ROLLHOOK_SECRET`          | yes      | Admin/static bearer token (min 7 chars)            |
| `ROLLHOOK_URL`             | no       | Public server URL — enables OIDC `aud` claim check |
| `DOCKER_HOST`              | no       | Docker daemon endpoint (default: local socket)     |
| `PORT`                     | no       | Listen port (default: `7700`)                      |
| `PUSHOVER_USER_KEY`        | no       | Pushover mobile notifications                      |
| `PUSHOVER_APP_TOKEN`       | no       | Pushover mobile notifications                      |
| `NOTIFICATION_WEBHOOK_URL` | no       | POST full job result JSON on completion            |

---

## API

Interactive docs at `/openapi` on your running instance. Key routes:

| Method | Route             | Auth         | Description                                          |
| ------ | ----------------- | ------------ | ---------------------------------------------------- |
| `POST` | `/auth/token`     | OIDC JWT     | Exchange OIDC token for registry credential + secret |
| `POST` | `/deploy`         | bearer       | Trigger deploy (`?async=true` to not block)          |
| `GET`  | `/jobs/{id}`      | bearer       | Job status + metadata                                |
| `GET`  | `/jobs/{id}/logs` | bearer       | SSE log stream                                       |
| `GET`  | `/jobs`           | bearer       | History (`?app=&status=&limit=`)                     |
| `GET`  | `/health`         | none         | `{ status, version }`                                |
| `*`    | `/v2/*`           | bearer/basic | Built-in OCI registry (push & pull)                  |

**Auth:** `POST /auth/token` accepts GitHub Actions OIDC JWTs and returns `ROLLHOOK_SECRET` for all subsequent API calls. All other routes require `Bearer <ROLLHOOK_SECRET>`.

---

## Notifications

Set `PUSHOVER_USER_KEY` + `PUSHOVER_APP_TOKEN` for mobile push on deploy completion.
Set `NOTIFICATION_WEBHOOK_URL` to POST the full `JobResult` JSON anywhere.
Notification failures are written to the job log — they never affect job status.

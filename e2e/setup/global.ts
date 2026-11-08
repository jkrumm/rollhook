import { execSync } from 'node:child_process'
import { join } from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'
import { REGISTRY_HOST, ROLLHOOK_SECRET } from './fixtures.ts'

const DIR = fileURLToPath(new URL('.', import.meta.url))
const ROOT = join(DIR, '../..')
const E2E_DIR = join(ROOT, 'e2e')
const HELLO_WORLD_DIR = join(E2E_DIR, 'hello-world')

const SERVER_IMAGE = 'rollhook-e2e-server:latest'

// Set in setup() once the server image exists; passed to every compose call.
let socketEnv: Record<string, string> = {}

function composeE2E(args: string, extraEnv?: Record<string, string>): void {
  execSync(`docker compose -f ${E2E_DIR}/compose.e2e.yml --project-name rollhook-e2e ${args}`, {
    stdio: 'inherit',
    env: { ...process.env, PROJECT_ROOT: ROOT, ROLLHOOK_SECRET, ...socketEnv, ...extraEnv },
  })
}

function imageExists(image: string): boolean {
  try {
    execSync(`docker image inspect ${image}`, { stdio: 'ignore' })
    return true
  } catch {
    return false
  }
}

// Group that owns the Docker socket as seen from inside a container. Asking a container
// (instead of stat-ing the host path) gives the right answer on Linux and on Docker Desktop.
// Returns an env that runs RollHook as its non-root user with that group, or root when the
// gid cannot be resolved.
function resolveSocketEnv(): Record<string, string> {
  try {
    const gid = execSync(
      `docker run --rm --entrypoint stat -v /var/run/docker.sock:/var/run/docker.sock ${SERVER_IMAGE} -c %g /var/run/docker.sock`,
      { encoding: 'utf8' },
    ).trim()
    if (/^\d+$/.test(gid)) return { DOCKER_GID: gid }
  } catch {}
  console.warn('Could not resolve the Docker socket gid; running RollHook as root in E2E')
  return { E2E_USER: 'root' }
}

export async function setup(): Promise<void> {
  // Tear down any stale state from a previous crashed run before starting fresh
  execSync(`docker compose --project-directory ${HELLO_WORLD_DIR} down -v 2>/dev/null || true`)
  composeE2E('down -v 2>/dev/null || true')
  // A foreign listener on :7700 would answer the health probe and hide a broken stack.
  const listener = execSync(`lsof -ti :7700 || true`, { encoding: 'utf8' }).trim()
  if (listener) {
    throw new Error(
      `Port 7700 is already in use (pid ${listener.split('\n').join(', ')}). Stop that process and re-run the E2E suite.`,
    )
  }

  // CI builds the server image once (docker/build-push-action, load: true) and the image
  // already exists here. Locally, build it from the monorepo root so E2E runs the real container.
  if (!imageExists(SERVER_IMAGE)) {
    execSync(`docker build -t ${SERVER_IMAGE} -f ${ROOT}/Dockerfile ${ROOT}`, {
      stdio: 'inherit',
    })
  }
  socketEnv = resolveSocketEnv()

  // Start infrastructure (Traefik) and RollHook container (with embedded Zot registry)
  composeE2E('up -d')

  // Wait for RollHook to be ready (Zot starts inside before app.listen(), so registry is also ready)
  await waitForUrl('http://localhost:7700/health', 30_000)

  // Authenticate with the embedded registry via the OCI proxy
  execSync(`docker login ${REGISTRY_HOST} --username rollhook --password ${ROLLHOOK_SECRET}`, {
    stdio: 'inherit',
  })

  // Build images
  execSync(
    `docker build -t rollhook-e2e-hello:v1 --build-arg BUILD_VERSION=v1 ${HELLO_WORLD_DIR}`,
    { stdio: 'inherit' },
  )
  execSync(
    `docker build -t rollhook-e2e-hello:v2 --build-arg BUILD_VERSION=v2 ${HELLO_WORLD_DIR}`,
    { stdio: 'inherit' },
  )

  // Build unhealthy variant: starts on port 3000 but /health always returns 503.
  // Used by rollout-failure.test.ts to exercise health-check failure + rollback.
  execSync(
    `docker build -t rollhook-e2e-hello:v-unhealthy -f ${HELLO_WORLD_DIR}/Dockerfile.unhealthy ${HELLO_WORLD_DIR}`,
    { stdio: 'inherit' },
  )

  // Remove stale registry-tagged images so Docker daemon re-checks layer existence
  // instead of skipping uploads based on a previous (now-gone) registry session.
  execSync(`docker rmi ${REGISTRY_HOST}/rollhook-e2e-hello:v1 2>/dev/null || true`)
  execSync(`docker rmi ${REGISTRY_HOST}/rollhook-e2e-hello:v2 2>/dev/null || true`)
  execSync(`docker rmi ${REGISTRY_HOST}/rollhook-e2e-hello:v-unhealthy 2>/dev/null || true`)

  // Push images to RollHook's embedded registry so executor's docker pull step succeeds
  execSync(`docker tag rollhook-e2e-hello:v1 ${REGISTRY_HOST}/rollhook-e2e-hello:v1`)
  execSync(`docker push ${REGISTRY_HOST}/rollhook-e2e-hello:v1`, { stdio: 'inherit' })
  execSync(`docker tag rollhook-e2e-hello:v2 ${REGISTRY_HOST}/rollhook-e2e-hello:v2`)
  execSync(`docker push ${REGISTRY_HOST}/rollhook-e2e-hello:v2`, { stdio: 'inherit' })
  execSync(
    `docker tag rollhook-e2e-hello:v-unhealthy ${REGISTRY_HOST}/rollhook-e2e-hello:v-unhealthy`,
  )
  execSync(`docker push ${REGISTRY_HOST}/rollhook-e2e-hello:v-unhealthy`, { stdio: 'inherit' })

  // Start hello-world app at v1
  // Pass IMAGE_TAG explicitly — a stale .env file in the project dir could override the compose default
  execSync(`docker compose --project-directory ${HELLO_WORLD_DIR} up -d`, {
    stdio: 'inherit',
    env: { ...process.env, IMAGE_TAG: `${REGISTRY_HOST}/rollhook-e2e-hello:v1` },
  })

  // Wait for hello-world to be routable through Traefik before starting any tests.
  // Traefik discovers container labels within ~100ms but needs its healthcheck to pass
  // before routing. Without this wait, zero-downtime "v1 is running" assertion can fail.
  await waitForUrl('http://localhost:9080/version', 30_000)
}

export async function teardown(): Promise<void> {
  // Rewrites the committed apps/dashboard/src/demo/data.json, so it is opt-in.
  if (process.env.EXPORT_DEMO === '1') {
    try {
      execSync(`bun ${E2E_DIR}/export-demo.ts`, {
        stdio: 'inherit',
        cwd: ROOT,
        env: { ...process.env, ROLLHOOK_SECRET },
      })
    } catch (err) {
      console.warn('demo export failed (non-fatal):', err)
    }
  }
  execSync(`docker compose --project-directory ${HELLO_WORLD_DIR} down -v`, {
    stdio: 'inherit',
  })
  composeE2E('down -v')
}

async function waitForUrl(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url)
      if (res.ok) return
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 500))
  }
  throw new Error(`Service did not become ready within ${timeoutMs}ms: ${url}`)
}

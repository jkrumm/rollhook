import type { ChildProcess } from 'node:child_process'
import { execSync, spawn } from 'node:child_process'
import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'
import { ADMIN_TOKEN, REGISTRY_HOST, WEBHOOK_TOKEN } from './fixtures.ts'

const DIR = fileURLToPath(new URL('.', import.meta.url))
const ROOT = join(DIR, '../..')
const E2E_DIR = join(ROOT, 'e2e')
const HELLO_WORLD_DIR = join(ROOT, 'examples/bun-hello-world')

let serverProcess: ChildProcess | null = null

export async function setup(): Promise<void> {
  // Start infrastructure (Traefik + local registry)
  execSync(`docker compose -f ${E2E_DIR}/compose.e2e.yml --project-name rollhook-e2e up -d`, {
    stdio: 'inherit',
  })

  // Wait for registry to be ready
  await waitForUrl('http://localhost:5001/v2/', 30_000)

  // Build images
  execSync(
    `docker build -t rollhook-e2e-hello:v1 --build-arg BUILD_VERSION=v1 ${HELLO_WORLD_DIR}`,
    { stdio: 'inherit' },
  )
  execSync(
    `docker build -t rollhook-e2e-hello:v2 --build-arg BUILD_VERSION=v2 ${HELLO_WORLD_DIR}`,
    { stdio: 'inherit' },
  )

  // Push images to local registry so executor's docker pull step succeeds
  execSync(`docker tag rollhook-e2e-hello:v1 ${REGISTRY_HOST}/rollhook-e2e-hello:v1`)
  execSync(`docker push ${REGISTRY_HOST}/rollhook-e2e-hello:v1`, { stdio: 'inherit' })
  execSync(`docker tag rollhook-e2e-hello:v2 ${REGISTRY_HOST}/rollhook-e2e-hello:v2`)
  execSync(`docker push ${REGISTRY_HOST}/rollhook-e2e-hello:v2`, { stdio: 'inherit' })

  // Write .env for docker-compose image tag resolution
  writeFileSync(
    join(HELLO_WORLD_DIR, '.env'),
    `IMAGE_TAG=v1\nREGISTRY=${REGISTRY_HOST}\n`,
  )

  // Start hello-world app at v1
  execSync(`docker compose --project-directory ${HELLO_WORLD_DIR} up -d`, {
    stdio: 'inherit',
  })

  // Generate rollhook.config.yaml with machine-absolute clone_path
  const configPath = join(E2E_DIR, 'rollhook.config.yaml')
  writeFileSync(configPath, `apps:\n  - name: hello-world\n    clone_path: ${HELLO_WORLD_DIR}\n`)

  // Spawn rollhook server natively
  serverProcess = spawn('bun', ['run', 'apps/server/server.ts'], {
    cwd: ROOT,
    env: {
      ...process.env,
      ADMIN_TOKEN,
      WEBHOOK_TOKEN,
      ROLLHOOK_CONFIG_PATH: configPath,
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  })

  serverProcess.stderr?.on('data', (chunk: Uint8Array) => {
    process.stderr.write(chunk)
  })

  // Wait for server to be ready
  await waitForUrl('http://localhost:7700/health', 30_000)
}

export async function teardown(): Promise<void> {
  if (serverProcess) {
    serverProcess.kill()
    serverProcess = null
  }

  execSync(`docker compose --project-directory ${HELLO_WORLD_DIR} down -v`, {
    stdio: 'inherit',
  })
  execSync(
    `docker compose -f ${E2E_DIR}/compose.e2e.yml --project-name rollhook-e2e down -v`,
    { stdio: 'inherit' },
  )
}

async function waitForUrl(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url)
      if (res.ok)
        return
    }
    catch {}
    await new Promise(resolve => setTimeout(resolve, 500))
  }
  throw new Error(`Service did not become ready within ${timeoutMs}ms: ${url}`)
}

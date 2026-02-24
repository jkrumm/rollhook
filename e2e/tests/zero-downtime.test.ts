import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL, pollJobUntilDone, REGISTRY_HOST, TRAEFIK_URL, webhookHeaders } from '../setup/fixtures.ts'

const DIR = fileURLToPath(new URL('.', import.meta.url))
const HELLO_WORLD_DIR = join(DIR, '../../examples/bun-hello-world')

const IMAGE_V1 = `${REGISTRY_HOST}/rollhook-e2e-hello:v1`
const IMAGE_V2 = `${REGISTRY_HOST}/rollhook-e2e-hello:v2`

beforeAll(async () => {
  // Ensure we're starting from a clean v1 state
  writeEnv('v1')
  const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ image_tag: IMAGE_V1 }),
  })
  const { job_id } = await res.json() as { job_id: string }
  const job = await pollJobUntilDone(job_id)
  expect(job.status).toBe('success')
})

afterAll(() => {
  // Reset .env back to v1 after test
  writeEnv('v1')
})

describe('zero-downtime rolling deployment', () => {
  it('v1 is running before deployment', async () => {
    const res = await fetch(`${TRAEFIK_URL}/version`)
    expect(res.status).toBe(200)
    const body = await res.json() as { version: string }
    expect(body.version).toBe('v1')
  })

  it('deploys v2 without dropping requests', async () => {
    // Update .env so docker-rollout picks up v2 from compose.yml
    writeEnv('v2')

    // Trigger v2 deployment
    const deployRes = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: webhookHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(deployRes.status).toBe(200)
    const { job_id } = await deployRes.json() as { job_id: string }

    // Hammer the version endpoint every 200ms while deployment runs
    const errors: string[] = []
    const versions: string[] = []

    const hammer = setInterval(async () => {
      try {
        const res = await fetch(`${TRAEFIK_URL}/version`)
        if (!res.ok) {
          errors.push(`HTTP ${res.status}`)
          return
        }
        const body = await res.json() as { version: string }
        versions.push(body.version)
      }
      catch (err) {
        errors.push(err instanceof Error ? err.message : String(err))
      }
    }, 200)

    const job = await pollJobUntilDone(job_id, 90_000)
    clearInterval(hammer)

    // Wait a tick to collect any in-flight responses
    await new Promise(resolve => setTimeout(resolve, 300))

    expect(job.status).toBe('success')
    expect(errors).toHaveLength(0)
    expect(versions.includes('v1')).toBe(true)
    expect(versions.includes('v2')).toBe(true)
  })

  it('v2 is serving after deployment', async () => {
    const res = await fetch(`${TRAEFIK_URL}/version`)
    expect(res.status).toBe(200)
    const body = await res.json() as { version: string }
    expect(body.version).toBe('v2')
  })
})

function writeEnv(imageTag: string): void {
  writeFileSync(
    join(HELLO_WORLD_DIR, '.env'),
    `IMAGE_TAG=${imageTag}\nREGISTRY=${REGISTRY_HOST}\n`,
  )
}

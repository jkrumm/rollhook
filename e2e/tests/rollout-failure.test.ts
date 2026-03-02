/**
 * E2E: Rollout health-check failure + rollback
 *
 * Deploys an image that starts successfully (Bun server listening on port 3000)
 * but whose /health endpoint always returns 503, causing the Docker HEALTHCHECK
 * to report "unhealthy" after a few retries. RollHook should:
 *   1. Scale up the new (unhealthy) container
 *   2. Detect the "unhealthy" status in waitHealthy()
 *   3. Call rollbackContainers() to stop and remove the new container
 *   4. Mark the job as failed
 *   5. Leave the original container untouched (still running)
 */
import type { JobResult } from '../setup/fixtures.ts'
import { beforeAll, describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL, getContainerCount, REGISTRY_HOST } from '../setup/fixtures.ts'

// Unhealthy image built in global.ts setup: starts on port 3000 but /health → 503
const UNHEALTHY_IMAGE = `${REGISTRY_HOST}/rollhook-e2e-hello:v-unhealthy`

let failedJob: JobResult

beforeAll(async () => {
  // Deploy synchronously — endpoint blocks until the job reaches a terminal state.
  // The unhealthy container takes ~8s to be marked "unhealthy" (3 checks × 2s each),
  // so the total beforeAll time is roughly: pull + scale-up + 8s healthcheck + rollback.
  const res = await fetch(`${BASE_URL}/deploy`, {
    method: 'POST',
    headers: adminHeaders(),
    body: JSON.stringify({ image_tag: UNHEALTHY_IMAGE }),
  })
  expect(res.status).toBe(500)
  const { job_id } = await res.json() as { job_id: string }

  const jobRes = await fetch(`${BASE_URL}/jobs/${job_id}`, { headers: adminHeaders() })
  failedJob = await jobRes.json() as JobResult
}, 120_000)

describe('rollout health-check failure + rollback', () => {
  it('job reaches failed status', () => {
    expect(failedJob.status).toBe('failed')
  })

  it('error field mentions the unhealthy container', () => {
    expect(failedJob.error).toBeTruthy()
    expect(failedJob.error).toContain('unhealthy')
  })

  it('job logs contain rollout step (scale-up happened)', async () => {
    const res = await fetch(`${BASE_URL}/jobs/${failedJob.id}/logs`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
    const text = await res.text()
    expect(text).toContain('[rollout]')
  })

  it('job logs confirm rollback was triggered', async () => {
    const res = await fetch(`${BASE_URL}/jobs/${failedJob.id}/logs`, { headers: adminHeaders() })
    const text = await res.text()
    expect(text).toContain('Rollback triggered')
    expect(text).toContain('[executor] ERROR:')
  })

  it('original container is still running after rollback — exactly 1 container', () => {
    // The unhealthy container must be stopped and removed; the old container survives.
    expect(getContainerCount()).toBe(1)
  })

  it('failed job appears in ?status=failed list', async () => {
    const res = await fetch(`${BASE_URL}/jobs?status=failed`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
    const jobs = await res.json() as Array<{ id: string, status: string }>
    expect(jobs.some(j => j.id === failedJob.id)).toBe(true)
  })
})

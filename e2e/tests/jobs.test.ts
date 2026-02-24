import { describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL, pollJobUntilDone, webhookHeaders } from '../setup/fixtures.ts'

const IMAGE_V1 = 'localhost:5001/rollhook-e2e-hello:v1'

describe('jobs API', () => {
  it('unknown job id → 404', async () => {
    const res = await fetch(`${BASE_URL}/jobs/00000000-0000-0000-0000-000000000000`, {
      headers: adminHeaders(),
    })
    expect(res.status).toBe(404)
  })

  it('/jobs/:id/logs returns SSE stream with log prefixes', async () => {
    // Trigger a deploy and wait for completion so logs exist
    const deployRes = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: webhookHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    const { job_id } = await deployRes.json() as { job_id: string }
    await pollJobUntilDone(job_id)

    const logRes = await fetch(`${BASE_URL}/jobs/${job_id}/logs`, { headers: adminHeaders() })
    expect(logRes.status).toBe(200)
    expect(logRes.headers.get('content-type')).toContain('text/event-stream')

    const text = await logRes.text()
    expect(text).toContain('[executor]')
    expect(text).toContain('[validate]')
    expect(text).toContain('[pull]')
    expect(text).toContain('[rollout]')
  })

  it('limit query param is respected', async () => {
    const res = await fetch(`${BASE_URL}/jobs?limit=1`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
    const jobs = await res.json() as unknown[]
    expect(jobs.length).toBeLessThanOrEqual(1)
  })
})

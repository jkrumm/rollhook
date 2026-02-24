import { describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL, pollJobUntilDone, webhookHeaders } from '../setup/fixtures.ts'

const IMAGE_V1 = 'localhost:5001/rollhook-e2e-hello:v1'

describe('deploy API', () => {
  it('deploy endpoint returns queued job', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: webhookHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    expect(res.status).toBe(200)
    const body = await res.json() as { job_id: string, app: string, status: string }
    expect(body.job_id).toBeTruthy()
    expect(body.app).toBe('hello-world')
    expect(body.status).toBe('queued')
  })

  it('deploy completes with success status', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    const { job_id } = await res.json() as { job_id: string }

    const job = await pollJobUntilDone(job_id)
    expect(job.status).toBe('success')
    expect(job.app).toBe('hello-world')
    expect(job.image_tag).toBe(IMAGE_V1)
    expect(job.created_at).toBeTruthy()
    expect(job.updated_at).toBeTruthy()
  })

  it('completed job has correct fields', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    const { job_id } = await res.json() as { job_id: string }
    await pollJobUntilDone(job_id)

    const detailRes = await fetch(`${BASE_URL}/jobs/${job_id}`, { headers: adminHeaders() })
    expect(detailRes.status).toBe(200)
    const detail = await detailRes.json() as Record<string, unknown>
    expect(detail.id).toBe(job_id)
    expect(detail.status).toBe('success')
    expect(detail.app).toBe('hello-world')
  })

  it('job is visible in app-filtered job list', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    const { job_id } = await res.json() as { job_id: string }
    await pollJobUntilDone(job_id)

    const listRes = await fetch(`${BASE_URL}/jobs?app=hello-world`, { headers: adminHeaders() })
    const jobs = await listRes.json() as Array<{ id: string }>
    expect(jobs.some(j => j.id === job_id)).toBe(true)
  })

  it('job is visible in status-filtered job list', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    const { job_id } = await res.json() as { job_id: string }
    await pollJobUntilDone(job_id)

    const listRes = await fetch(`${BASE_URL}/jobs?status=success`, { headers: adminHeaders() })
    const jobs = await listRes.json() as Array<{ id: string }>
    expect(jobs.some(j => j.id === job_id)).toBe(true)
  })

  it('deploying unknown app returns 404', async () => {
    const res = await fetch(`${BASE_URL}/deploy/nonexistent-app`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: IMAGE_V1 }),
    })
    expect(res.status).toBe(404)
  })
})

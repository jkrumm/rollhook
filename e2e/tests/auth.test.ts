import { describe, expect, it } from 'vitest'
import { ADMIN_TOKEN, adminHeaders, BASE_URL, REGISTRY_HOST, WEBHOOK_TOKEN, webhookHeaders } from '../setup/fixtures.ts'

// Use a non-existent tag so the deploy job fails at the pull step — no rollout runs,
// no new container is created, and the queue clears in seconds instead of ~16s per rollout.
const NONEXISTENT_IMAGE = `${REGISTRY_HOST}/rollhook-e2e-hello:nonexistent`

describe('authentication', () => {
  it('no Authorization header → 401', async () => {
    const res = await fetch(`${BASE_URL}/jobs`)
    expect(res.status).toBe(401)
  })

  it('webhook token on GET /jobs → 403', async () => {
    const res = await fetch(`${BASE_URL}/jobs`, {
      headers: { Authorization: `Bearer ${WEBHOOK_TOKEN}` },
    })
    expect(res.status).toBe(403)
  })

  it('admin token on GET /jobs → 200', async () => {
    const res = await fetch(`${BASE_URL}/jobs`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
  })

  it('webhook token on POST /deploy/hello-world → 200', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: webhookHeaders(),
      body: JSON.stringify({ image_tag: NONEXISTENT_IMAGE }),
    })
    expect(res.status).toBe(200)
  })

  it('admin token on POST /deploy/hello-world → 200', async () => {
    const res = await fetch(`${BASE_URL}/deploy/hello-world`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: NONEXISTENT_IMAGE }),
    })
    expect(res.status).toBe(200)
  })

  it('wrong token → 403', async () => {
    const res = await fetch(`${BASE_URL}/jobs`, {
      headers: { Authorization: 'Bearer wrong-token' },
    })
    expect(res.status).toBe(403)
  })

  it('invalid Authorization format → 401', async () => {
    const res = await fetch(`${BASE_URL}/jobs`, {
      headers: { Authorization: `Basic ${ADMIN_TOKEN}` },
    })
    expect(res.status).toBe(401)
  })
})

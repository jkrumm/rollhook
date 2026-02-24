import { describe, expect, it } from 'bun:test'
import { app } from '../app'

// Relies on preload.ts setting ADMIN_TOKEN=test-admin and WEBHOOK_TOKEN=test-webhook

describe('Auth middleware (app.handle)', () => {
  it('returns 401 when no Authorization header', async () => {
    const res = await app.handle(new Request('http://localhost/jobs'))
    expect(res.status).toBe(401)
  })

  it('returns 401 when Authorization is not Bearer', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs', {
        headers: { Authorization: 'Basic dXNlcjpwYXNz' },
      }),
    )
    expect(res.status).toBe(401)
  })

  it('returns 403 when webhook token is used on admin-only endpoint', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs', {
        headers: { Authorization: 'Bearer test-webhook' },
      }),
    )
    expect(res.status).toBe(403)
  })

  it('returns 403 when unknown token is used', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs', {
        headers: { Authorization: 'Bearer unknown-token' },
      }),
    )
    expect(res.status).toBe(403)
  })

  it('returns 200 when admin token is used on admin endpoint', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs', {
        headers: { Authorization: 'Bearer test-admin' },
      }),
    )
    expect(res.status).toBe(200)
  })

  it('returns 200 for GET /health with no token', async () => {
    const res = await app.handle(new Request('http://localhost/health'))
    expect(res.status).toBe(200)
  })

  it('webhook token is accepted on POST /deploy/:app', async () => {
    const res = await app.handle(
      new Request('http://localhost/deploy/nonexistent', {
        method: 'POST',
        headers: {
          'Authorization': 'Bearer test-webhook',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ image_tag: 'test:latest' }),
      }),
    )
    // Auth passed — response is not 401 or 403 (may be 404 or 500 depending on config availability)
    expect(res.status).not.toBe(401)
    expect(res.status).not.toBe(403)
  })
})

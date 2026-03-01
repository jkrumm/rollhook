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
    // Auth passed — response is not 401 or 403 (may be 200 or 500 depending on whether discover can find a running container)
    expect(res.status).not.toBe(401)
    expect(res.status).not.toBe(403)
  })

  it('webhook token is accepted on GET /jobs/:id', async () => {
    // A job that doesn't exist returns 404, but auth passes first
    const res = await app.handle(
      new Request('http://localhost/jobs/00000000-0000-0000-0000-000000000000', {
        headers: { Authorization: 'Bearer test-webhook' },
      }),
    )
    expect(res.status).not.toBe(401)
    expect(res.status).not.toBe(403)
    // 404 is the expected result — job doesn't exist, but auth passed
    expect(res.status).toBe(404)
  })

  it('webhook token is accepted on GET /jobs/:id/logs', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs/00000000-0000-0000-0000-000000000000/logs', {
        headers: { Authorization: 'Bearer test-webhook' },
      }),
    )
    expect(res.status).not.toBe(401)
    expect(res.status).not.toBe(403)
  })

  it('webhook token is accepted on GET /jobs (list)', async () => {
    const res = await app.handle(
      new Request('http://localhost/jobs', {
        headers: { Authorization: 'Bearer test-webhook' },
      }),
    )
    expect(res.status).not.toBe(401)
    expect(res.status).not.toBe(403)
  })
})

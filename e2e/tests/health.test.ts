import { describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL } from '../setup/fixtures.ts'

describe('/health endpoint', () => {
  it('returns 200 with no auth', async () => {
    const res = await fetch(`${BASE_URL}/health`)
    expect(res.status).toBe(200)
    const body = await res.json()
    expect(body).toMatchObject({ status: 'ok' })
  })

  it('response includes version field', async () => {
    const res = await fetch(`${BASE_URL}/health`)
    const body = await res.json() as { status: string, version: string }
    expect(typeof body.version).toBe('string')
    expect(body.version.length).toBeGreaterThan(0)
  })

  it('/openapi is accessible without authentication', async () => {
    const res = await fetch(`${BASE_URL}/openapi`)
    expect(res.status).toBe(200)
  })

  it('/openapi.json returns JSON spec with correct title', async () => {
    const res = await fetch(`${BASE_URL}/openapi.json`)
    expect(res.status).toBe(200)
    const spec = await res.json() as { info: { title: string } }
    expect(spec.info.title).toBe('RollHook API')
  })

  it('authenticated request to /health still returns 200', async () => {
    const res = await fetch(`${BASE_URL}/health`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
  })
})

describe('/ready endpoint', () => {
  it('returns 200 with no auth when Docker is reachable', async () => {
    const res = await fetch(`${BASE_URL}/ready`)
    expect(res.status).toBe(200)
    const body = await res.json()
    expect(body).toMatchObject({ status: 'ready', docker: 'ok' })
  })

  // The probe is public so it never stops probing, but the proxy routes every
  // path here — the internal Docker endpoint must not reach an anonymous
  // caller. Asserted explicitly: toMatchObject alone would pass if it leaked.
  it('withholds docker_host and detail from anonymous callers', async () => {
    const res = await fetch(`${BASE_URL}/ready`)
    const body = await res.json() as { docker_host?: string, detail?: string }
    expect(body.docker_host).toBeUndefined()
    expect(body.detail).toBeUndefined()
  })

  it('returns docker_host to authenticated callers', async () => {
    const res = await fetch(`${BASE_URL}/ready`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
    const body = await res.json() as { docker_host?: string }
    expect(typeof body.docker_host).toBe('string')
    expect(body.docker_host).toBeTruthy()
  })
})

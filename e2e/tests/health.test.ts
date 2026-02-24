import { describe, expect, it } from 'vitest'
import { BASE_URL } from '../setup/fixtures.ts'

describe('/health endpoint', () => {
  it('returns 200 with no auth', async () => {
    const res = await fetch(`${BASE_URL}/health`)
    expect(res.status).toBe(200)
    const body = await res.json()
    expect(body).toMatchObject({ status: 'ok' })
  })
})

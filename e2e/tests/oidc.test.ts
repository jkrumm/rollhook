import { describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL, REGISTRY_HOST } from '../setup/fixtures.ts'

const MOCK_OIDC_URL = 'http://localhost:8080'
const IMAGE_V2 = `${REGISTRY_HOST}/rollhook-e2e-hello:v2`

/**
 * Request a signed JWT from the mock OIDC server.
 */
async function getOIDCToken(opts: {
  repository: string
  ref: string
  aud?: string
  exp_offset?: number
}): Promise<string> {
  const res = await fetch(`${MOCK_OIDC_URL}/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  })
  if (!res.ok)
    throw new Error(`Mock OIDC server returned ${res.status}`)
  const { token } = await res.json() as { token: string }
  return token
}

function oidcHeaders(token: string): HeadersInit {
  return { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' }
}

describe('oidc authentication', () => {
  it('valid OIDC token → 403 (OIDC not accepted on /deploy)', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/main',
      aud: BASE_URL,
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('valid OIDC token but repo not in allowed_repos → 403', async () => {
    const token = await getOIDCToken({
      repository: 'attacker/evil-repo',
      ref: 'refs/heads/main',
      aud: BASE_URL,
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('oidc token with PR ref → 403 (hard deny)', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/pull/42/merge',
      aud: BASE_URL,
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('oidc token with feature branch ref → 403 (default fail-secure)', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/feature/my-feature',
      aud: BASE_URL,
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('oidc token with refs/heads/master → 403 (OIDC not accepted on /deploy)', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/master',
      aud: BASE_URL,
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('expired OIDC token → 403', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/main',
      aud: BASE_URL,
      exp_offset: -1, // already expired
    })
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: oidcHeaders(token),
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('static ROLLHOOK_SECRET still works on /deploy → deploy accepted', async () => {
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: adminHeaders(),
      body: JSON.stringify({ image_tag: `${REGISTRY_HOST}/rollhook-e2e-hello:nonexistent` }),
    })
    // 200 = deploy accepted (queued). 503 = queue full (also acceptable under load).
    expect([200, 503]).toContain(res.status)
  })

  it('oidc → /auth/token → use secret for /deploy', async () => {
    const oidcToken = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/main',
      aud: BASE_URL,
    })
    // Step 1: Exchange OIDC token for registry token + secret
    const authRes = await fetch(`${BASE_URL}/auth/token`, {
      method: 'POST',
      headers: oidcHeaders(oidcToken),
      body: JSON.stringify({ image_name: 'rollhook-e2e-hello' }),
    })
    expect(authRes.status).toBe(200)
    const { token, secret } = await authRes.json() as { token: string, secret: string }
    expect(token).toBeTruthy()
    expect(secret).toBeTruthy()

    // Step 2: Use secret to deploy (not the OIDC token)
    const deployRes = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${secret}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    // 200 = deploy accepted (queued). 503 = queue full (also acceptable under load).
    expect([200, 503]).toContain(deployRes.status)
  })

  it('invalid bearer token on /deploy → 403', async () => {
    const res = await fetch(`${BASE_URL}/deploy?async=true`, {
      method: 'POST',
      headers: { 'Authorization': 'Bearer invalid-secret', 'Content-Type': 'application/json' },
      body: JSON.stringify({ image_tag: IMAGE_V2 }),
    })
    expect(res.status).toBe(403)
  })

  it('oidc token cannot access admin /jobs endpoint → 403', async () => {
    const token = await getOIDCToken({
      repository: 'rollhook-e2e/hello',
      ref: 'refs/heads/main',
    })
    const res = await fetch(`${BASE_URL}/jobs`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    // OIDC JWTs are rejected at operationID check before reaching static-secret comparison.
    expect(res.status).toBe(403)
  })
})

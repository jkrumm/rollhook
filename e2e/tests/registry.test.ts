import { describe, expect, it } from 'vitest'
import { adminHeaders, BASE_URL } from '../setup/fixtures.ts'

describe('registry API', () => {
  it('lists hello-world with clone_path', async () => {
    const res = await fetch(`${BASE_URL}/registry`, { headers: adminHeaders() })
    expect(res.status).toBe(200)
    const apps = await res.json() as Array<{ name: string, clone_path: string }>
    const helloWorld = apps.find(a => a.name === 'hello-world')
    expect(helloWorld).toBeDefined()
    expect(helloWorld!.clone_path).toContain('bun-hello-world')
  })

  it('includes last_deploy field', async () => {
    const res = await fetch(`${BASE_URL}/registry`, { headers: adminHeaders() })
    const apps = await res.json() as Array<{ name: string, last_deploy: unknown }>
    const helloWorld = apps.find(a => a.name === 'hello-world')
    // May be null (no deploys yet) or a job object if auth tests already deployed
    expect(helloWorld).toBeDefined()
    expect(helloWorld).toHaveProperty('last_deploy')
  })

  it('patching clone_path returns updated value', async () => {
    // Save original path so we can restore after the test
    const listRes = await fetch(`${BASE_URL}/registry`, { headers: adminHeaders() })
    const apps = await listRes.json() as Array<{ name: string, clone_path: string }>
    const originalPath = apps.find(a => a.name === 'hello-world')!.clone_path

    const res = await fetch(`${BASE_URL}/registry/hello-world`, {
      method: 'PATCH',
      headers: adminHeaders(),
      body: JSON.stringify({ clone_path: '/tmp/test-path' }),
    })
    expect(res.status).toBe(200)
    const body = await res.json() as { name: string, clone_path: string }
    expect(body.clone_path).toBe('/tmp/test-path')

    // Restore original path to avoid breaking subsequent deploy tests
    await fetch(`${BASE_URL}/registry/hello-world`, {
      method: 'PATCH',
      headers: adminHeaders(),
      body: JSON.stringify({ clone_path: originalPath }),
    })
  })

  it('patching nonexistent app returns 404', async () => {
    const res = await fetch(`${BASE_URL}/registry/nonexistent-app`, {
      method: 'PATCH',
      headers: adminHeaders(),
      body: JSON.stringify({ clone_path: '/tmp/test' }),
    })
    expect(res.status).toBe(404)
  })
})

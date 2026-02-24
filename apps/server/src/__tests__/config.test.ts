import { Value } from '@sinclair/typebox/value'
import { describe, expect, it } from 'bun:test'
import { ServerConfigSchema } from 'rollhook'

// Tests Value.Check() directly to avoid the module-level cache in loadConfig()

describe('ServerConfigSchema', () => {
  it('accepts a valid config', () => {
    const valid = {
      apps: [{ name: 'my-api', clone_path: '/srv/apps/my-api' }],
    }
    expect(Value.Check(ServerConfigSchema, valid)).toBe(true)
  })

  it('rejects missing apps array', () => {
    expect(Value.Check(ServerConfigSchema, {})).toBe(false)
  })

  it('rejects invalid clone_path type', () => {
    const invalid = { apps: [{ name: 'my-api', clone_path: 123 }] }
    expect(Value.Check(ServerConfigSchema, invalid)).toBe(false)
  })

  it('rejects missing name in app entry', () => {
    const invalid = { apps: [{ clone_path: '/srv/apps/my-api' }] }
    expect(Value.Check(ServerConfigSchema, invalid)).toBe(false)
  })

  it('accepts optional notifications field', () => {
    const valid = {
      apps: [{ name: 'my-api', clone_path: '/srv' }],
      notifications: { webhook: 'https://hooks.example.com/notify' },
    }
    expect(Value.Check(ServerConfigSchema, valid)).toBe(true)
  })

  it('reports validation errors for invalid config', () => {
    const invalid = { apps: [{ name: 42, clone_path: '/srv' }] }
    const errors = [...Value.Errors(ServerConfigSchema, invalid)]
    expect(errors.length).toBeGreaterThan(0)
  })
})

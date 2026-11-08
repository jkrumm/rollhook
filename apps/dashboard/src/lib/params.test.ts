import { describe, expect, test } from 'bun:test'
import { buildSearch, parseParams } from './params'

describe('params', () => {
  test('parses valid params and drops invalid status', () => {
    expect(parseParams('?app=web&status=failed&job=j1&day=2026-03-05')).toEqual({
      app: 'web',
      status: 'failed',
      job: 'j1',
      day: '2026-03-05',
    })
    expect(parseParams('?status=bogus&app=').status).toBeUndefined()
    expect(parseParams('?app=').app).toBeUndefined()
  })

  test('buildSearch round-trips and omits empty values', () => {
    const p = { app: 'web', status: 'success' as const }
    expect(buildSearch(p)).toBe('app=web&status=success')
    expect(parseParams(buildSearch(p))).toEqual({
      app: 'web',
      status: 'success',
      job: undefined,
      day: undefined,
    })
    expect(buildSearch({})).toBe('')
  })
})

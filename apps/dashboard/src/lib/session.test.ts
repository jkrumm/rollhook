import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import { customInstance } from '../api/client'
import { endSession, restoreSession, startSession } from './session'

const store = new Map<string, string>()
const originalFetch = globalThis.fetch
let authHeader = ''

beforeEach(() => {
  store.clear()
  authHeader = ''
  Object.assign(globalThis, {
    sessionStorage: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    },
    fetch: async (_url: string, init: RequestInit) => {
      authHeader = (init.headers as Record<string, string>).Authorization
      return Response.json([])
    },
  })
  endSession()
})

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('session', () => {
  test('restoreSession configures the client before the first fetch', async () => {
    store.set('rh_token', 'stored')
    expect(restoreSession()).toBe('stored')
    await customInstance('/jobs')
    expect(authHeader).toBe('Bearer stored')
  })

  test('startSession configures the client synchronously', async () => {
    startSession('fresh')
    await customInstance('/jobs')
    expect(authHeader).toBe('Bearer fresh')
    expect(store.get('rh_token')).toBe('fresh')
  })

  test('endSession clears client and storage', async () => {
    startSession('fresh')
    endSession()
    await customInstance('/jobs')
    expect(authHeader).toBe('Bearer ')
    expect(store.has('rh_token')).toBe(false)
  })
})

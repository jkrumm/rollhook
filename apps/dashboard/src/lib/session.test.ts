import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import { customInstance } from '../api/client'
import { endSession, restoreSession, startDemoSession, startSession } from './session'

const store = new Map<string, string>()
const originalFetch = globalThis.fetch
const originalStorage = globalThis.sessionStorage
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
      authHeader = new Headers(init.headers).get('Authorization') ?? ''
      return Response.json([])
    },
  })
  endSession()
})

afterEach(() => {
  globalThis.fetch = originalFetch
  Object.assign(globalThis, { sessionStorage: originalStorage })
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

  test('startDemoSession configures the client without touching storage', async () => {
    expect(startDemoSession()).toBe('demo')
    await customInstance('/jobs')
    expect(authHeader).toBe('Bearer demo')
    expect(store.size).toBe(0)
  })

  test('endSession clears client and storage', async () => {
    startSession('fresh')
    endSession()
    await customInstance('/jobs')
    expect(authHeader).toBe('Bearer ')
    expect(store.has('rh_token')).toBe(false)
  })
})

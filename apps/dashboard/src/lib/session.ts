import { setApiToken } from '../api/client'

const STORAGE_KEY = 'rh_token'

// The API client must hold the token before any child component can fetch, so every
// token change configures it synchronously here, never from a parent effect.
export function restoreSession(): string {
  const token = sessionStorage.getItem(STORAGE_KEY) ?? ''
  setApiToken(token)
  return token
}

export function startSession(token: string) {
  sessionStorage.setItem(STORAGE_KEY, token)
  setApiToken(token)
}

export function endSession() {
  sessionStorage.removeItem(STORAGE_KEY)
  setApiToken('')
}

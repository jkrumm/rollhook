import { useState } from 'react'
import { Dashboard } from './components/Dashboard'
import { TokenGate } from './components/TokenGate'
import { endSession, restoreSession, startDemoSession, startSession } from './lib/session'

const IS_DEMO = import.meta.env.DEV || import.meta.env.MODE === 'demo'

export function App() {
  const [token, setToken] = useState(() => {
    if (IS_DEMO)
      return startDemoSession()
    return restoreSession()
  })

  function handleToken(t: string) {
    startSession(t)
    setToken(t)
  }

  function handleLogout() {
    endSession()
    setToken('')
  }

  if (!token) {
    return <TokenGate onToken={handleToken} />
  }

  return <Dashboard onLogout={handleLogout} />
}

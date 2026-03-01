import { useEffect, useState } from 'react'
import { Dashboard } from './components/Dashboard'
import { TokenGate } from './components/TokenGate'
import { configureApi } from './lib/api'

const IS_DEMO = import.meta.env.DEV || import.meta.env.MODE === 'demo'

export function App() {
  const [token, setToken] = useState(() => {
    if (IS_DEMO)
      return 'demo'
    return sessionStorage.getItem('rh_token') ?? ''
  })

  useEffect(() => {
    configureApi(token)
  }, [token])

  function handleToken(t: string) {
    sessionStorage.setItem('rh_token', t)
    setToken(t)
  }

  function handleLogout() {
    sessionStorage.removeItem('rh_token')
    setToken('')
  }

  if (!token) {
    return <TokenGate onToken={handleToken} />
  }

  return <Dashboard onLogout={handleLogout} />
}

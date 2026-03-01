import { Logo } from '@rollhook/ui'
import { useState } from 'react'

interface TokenGateProps {
  onToken: (token: string) => void
}

export function TokenGate({ onToken }: TokenGateProps) {
  const [value, setValue] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!value.trim())
      return
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/jobs?limit=1', {
        headers: { Authorization: `Bearer ${value.trim()}` },
      })
      if (res.ok) {
        onToken(value.trim())
      }
      else {
        setError('Invalid token')
      }
    }
    catch {
      setError('Connection failed')
    }
    finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6 px-4">
        <div className="space-y-1">
          <Logo size={36} className="mb-4" />
          <h1 className="text-lg font-semibold tracking-tight">RollHook</h1>
          <p className="text-sm text-muted-foreground">Enter your token to continue</p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-3">
          <label htmlFor="webhook-token" className="sr-only">Webhook token</label>
          <input
            id="webhook-token"
            type="password"
            value={value}
            onChange={e => setValue(e.target.value)}
            placeholder="webhook token"
            aria-invalid={!!error}
            aria-describedby={error ? 'token-error' : undefined}
            className="w-full bg-input border border-border rounded px-3 py-2 font-mono text-sm outline-none focus:ring-1 focus:ring-ring"
            autoFocus
          />
          {error && <p id="token-error" role="alert" className="text-destructive text-xs font-mono">{error}</p>}
          <button
            type="submit"
            disabled={loading || !value.trim()}
            className="w-full bg-primary text-primary-foreground rounded px-3 py-2 text-sm font-medium disabled:opacity-50 transition-opacity"
          >
            {loading ? 'Verifying…' : 'Continue'}
          </button>
        </form>
      </div>
    </div>
  )
}

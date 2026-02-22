import { useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import { createIsomorphicFn } from '@tanstack/react-start'
import { api } from '../lib/api'

// SSR: calls Elysia handler directly via treaty(app) — zero HTTP, file store shared across module contexts.
// Client navigation: HTTP via Eden Treaty.
// createIsomorphicFn (not createServerFn) has no RPC endpoint, so no /_server/* routing issues.
const getCounter = createIsomorphicFn()
  .server(async () => {
    const [{ treaty }, { app }] = await Promise.all([
      import('@elysiajs/eden'),
      import('@stackcommander/server/app'),
    ])
    const { data } = await treaty(app).api.counter.get()
    return data ?? { count: 0 }
  })
  .client(async () => {
    const { data } = await api.api.counter.get()
    return data ?? { count: 0 }
  })

export const Route = createFileRoute('/')({
  loader: () => getCounter(),
  component: CounterPage,
})

function CounterPage() {
  const initial = Route.useLoaderData()
  const [count, setCount] = useState(initial.count)

  async function handleIncrement() {
    const { data } = await api.api.counter.increment.post({ amount: 1 })
    if (data)
      setCount(data.count)
  }

  return (
    <div>
      <h1>Counter</h1>
      <p>
        Count:
        {' '}
        {count}
      </p>
      <button type="button" onClick={handleIncrement}>
        Increment
      </button>
    </div>
  )
}

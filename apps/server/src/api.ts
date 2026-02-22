import { Elysia, t } from 'elysia'
import { store } from './store.js'

export const api = new Elysia({ prefix: '/api' })
  .get('/health', () => ({ status: 'ok' }))
  .get('/counter', () => ({ count: store.getCounter() }), {
    response: t.Object({ count: t.Number() }),
    detail: { tags: ['Counter'], summary: 'Get counter value' },
  })
  .post('/counter/increment', ({ body }) => {
    const count = store.incrementCounter(body.amount)
    return { count }
  }, {
    body: t.Object({ amount: t.Optional(t.Number()) }),
    response: t.Object({ count: t.Number() }),
    detail: { tags: ['Counter'], summary: 'Increment counter' },
  })

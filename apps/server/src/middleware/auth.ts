import process from 'node:process'
import { Elysia } from 'elysia'

export type Role = 'admin' | 'webhook'

const ADMIN_TOKEN = process.env.ADMIN_TOKEN
const WEBHOOK_TOKEN = process.env.WEBHOOK_TOKEN

if (!ADMIN_TOKEN || !WEBHOOK_TOKEN) {
  throw new Error('ADMIN_TOKEN and WEBHOOK_TOKEN environment variables are required')
}

// No `name` on the Elysia instance to prevent plugin deduplication: Elysia
// deduplicates named plugins globally, so using requireRole('webhook') in both
// deployApi and jobsApi would silently skip the second registration.
// `{ as: 'local' }` keeps the hook scoped to THIS instance only — no upward
// propagation. Routes MUST be chained onto the requireRole(...) return value
// (not onto a parent after .use()) so the hook and routes share the same instance.
export function requireRole(role: Role) {
  return new Elysia()
    .onBeforeHandle({ as: 'local' }, ({ headers, set }) => {
      const authHeader = headers.authorization
      if (!authHeader?.startsWith('Bearer ')) {
        set.status = 401
        return { message: 'Missing or invalid Authorization header' }
      }

      const token = authHeader.slice(7)

      if (role === 'admin' && token !== ADMIN_TOKEN) {
        set.status = 403
        return { message: 'Admin token required' }
      }

      if (role === 'webhook' && token !== ADMIN_TOKEN && token !== WEBHOOK_TOKEN) {
        set.status = 403
        return { message: 'Valid token required' }
      }
    })
}

import type { JobResult } from 'rollhook'

export const BASE_URL = 'http://localhost:7700'
export const TRAEFIK_URL = 'http://localhost:9080'
export const REGISTRY_HOST = 'localhost:5001'

export const ADMIN_TOKEN = 'e2e-admin-token'
export const WEBHOOK_TOKEN = 'e2e-webhook-token'

export function adminHeaders(): HeadersInit {
  return { 'Authorization': `Bearer ${ADMIN_TOKEN}`, 'Content-Type': 'application/json' }
}

export function webhookHeaders(): HeadersInit {
  return { 'Authorization': `Bearer ${WEBHOOK_TOKEN}`, 'Content-Type': 'application/json' }
}

export type { JobResult }

// Poll /jobs/:id every second until status is success or failed.
// Timeout accounts for queue depth: each rollout takes ~16s, up to 4 queued = ~64s worst case.
export async function pollJobUntilDone(jobId: string, timeoutMs = 90_000): Promise<JobResult> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const res = await fetch(`${BASE_URL}/jobs/${jobId}`, { headers: adminHeaders() })
    const job = await res.json() as JobResult
    if (job.status === 'success' || job.status === 'failed')
      return job
    await new Promise(resolve => setTimeout(resolve, 1_000))
  }
  throw new Error(`Job ${jobId} did not complete within ${timeoutMs}ms`)
}

// Same as pollJobUntilDone but uses the webhook token — mirrors the real CI journey
// where rollhook-action only has a webhook token, not an admin token.
export async function webhookPollJobUntilDone(jobId: string, timeoutMs = 90_000): Promise<JobResult> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const res = await fetch(`${BASE_URL}/jobs/${jobId}`, { headers: webhookHeaders() })
    const job = await res.json() as JobResult
    if (job.status === 'success' || job.status === 'failed')
      return job
    await new Promise(resolve => setTimeout(resolve, 1_000))
  }
  throw new Error(`Job ${jobId} did not complete within ${timeoutMs}ms`)
}

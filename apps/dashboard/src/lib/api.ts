import type { JobResult, JobStatus } from 'rollhook'

const IS_DEMO = import.meta.env.DEV || import.meta.env.MODE === 'demo'

let _token = ''

export function configureApi(token: string) {
  _token = token
}

function authHeaders(): Record<string, string> {
  return { Authorization: `Bearer ${_token}` }
}

async function getDemoData() {
  if (!IS_DEMO)
    return null
  return (await import('../demo/data.json')).default
}

export async function fetchJobs(params: { app?: string, status?: JobStatus, limit?: number } = {}): Promise<JobResult[]> {
  if (IS_DEMO) {
    const data = await getDemoData()
    let jobs = (data?.jobs ?? []) as JobResult[]
    if (params.app)
      jobs = jobs.filter(j => j.app === params.app)
    if (params.status)
      jobs = jobs.filter(j => j.status === params.status)
    if (params.limit)
      jobs = jobs.slice(0, params.limit)
    return jobs
  }
  const search = new URLSearchParams()
  if (params.app)
    search.set('app', params.app)
  if (params.status)
    search.set('status', params.status)
  if (params.limit)
    search.set('limit', String(params.limit))
  const res = await fetch(`/jobs?${search}`, { headers: authHeaders() })
  if (!res.ok)
    throw new Error(`${res.status}`)
  return res.json() as Promise<JobResult[]>
}

export function streamLogs(jobId: string, onLine: (line: string) => void, signal: AbortSignal): Promise<void> {
  if (IS_DEMO) {
    return getDemoData().then((data) => {
      const lines = (data?.logs as Record<string, string[]> | undefined)?.[jobId] ?? []
      return new Promise<void>((resolve) => {
        let i = 0
        function next() {
          if (signal.aborted || i >= lines.length) {
            resolve()
            return
          }
          onLine(lines[i++])
          setTimeout(next, 8)
        }
        next()
      })
    })
  }
  return fetch(`/jobs/${jobId}/logs`, { headers: authHeaders(), signal })
    .then((res) => {
      if (!res.ok)
        throw new Error(`${res.status}`)
      if (!res.body)
        throw new Error('Response body is null')
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      function pump(): Promise<void> {
        return reader.read().then(({ done, value }) => {
          if (done)
            return
          buffer += decoder.decode(value, { stream: true })
          const parts = buffer.split('\n\n')
          buffer = parts.pop() ?? ''
          for (const part of parts) {
            const line = part.replace(/^data: /, '')
            if (line)
              onLine(line)
          }
          return pump()
        })
      }
      return pump()
    })
}

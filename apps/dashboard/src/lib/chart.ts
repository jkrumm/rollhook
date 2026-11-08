import type { ChartDataPoint, JobResult } from '@rollhook/ui'

const pad = (n: number) => String(n).padStart(2, '0')

// Local calendar day (YYYY-MM-DD). Used by both the chart grouping and the day filter so they agree.
export function dayKey(createdAt: string): string {
  const d = new Date(createdAt)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function filterByDay(jobs: JobResult[], day: string | undefined): JobResult[] {
  if (!day) return jobs
  return jobs.filter((j) => dayKey(j.created_at) === day)
}

export function computeChartData(jobs: JobResult[]): ChartDataPoint[] {
  const map = new Map<string, { success: number; failed: number }>()
  for (const job of jobs) {
    if (job.status !== 'success' && job.status !== 'failed') continue
    const day = dayKey(job.created_at)
    const entry = map.get(day) ?? { success: 0, failed: 0 }
    entry[job.status]++
    map.set(day, entry)
  }
  return [...map.entries()]
    .toSorted(([a], [b]) => a.localeCompare(b))
    .slice(-14)
    .map(([day, counts]) => ({ day, ...counts }))
}

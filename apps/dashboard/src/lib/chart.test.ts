import { describe, expect, test } from 'bun:test'
import type { JobResult } from '@rollhook/ui'
import { computeChartData, dayKey, filterByDay } from './chart'

function job(created_at: string, status: JobResult['status'] = 'success'): JobResult {
  return { id: created_at, app: 'a', status, created_at } as unknown as JobResult
}

// Noon local time avoids timezone day-boundary flakiness.
const iso = (y: number, m: number, d: number) => new Date(y, m - 1, d, 12).toISOString()

describe('chart helpers', () => {
  test('dayKey returns local YYYY-MM-DD', () => {
    expect(dayKey(iso(2026, 3, 5))).toBe('2026-03-05')
  })

  test('filterByDay distinguishes the same MM-DD in different years', () => {
    const jobs = [job(iso(2025, 3, 5)), job(iso(2026, 3, 5))]
    expect(filterByDay(jobs, '2026-03-05')).toEqual([jobs[1]])
    expect(filterByDay(jobs, undefined)).toBe(jobs)
  })

  test('computeChartData counts terminal jobs per day, sorted, last 14', () => {
    const jobs = [
      job(iso(2026, 3, 6), 'failed'),
      job(iso(2026, 3, 5)),
      job(iso(2026, 3, 5)),
      job(iso(2026, 3, 5), 'running'),
    ]
    expect(computeChartData(jobs)).toEqual([
      { day: '2026-03-05', success: 2, failed: 0 },
      { day: '2026-03-06', success: 0, failed: 1 },
    ])
    const many = Array.from({ length: 20 }, (_, i) => job(iso(2026, 1, i + 1)))
    expect(computeChartData(many)).toHaveLength(14)
  })

  test('chart day keys match filter keys', () => {
    const jobs = [job(iso(2026, 3, 5))]
    const [point] = computeChartData(jobs)
    expect(filterByDay(jobs, point.day)).toEqual(jobs)
  })
})

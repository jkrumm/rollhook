import { afterEach, describe, expect, it } from 'bun:test'
import { enqueue, setProcessor } from '../jobs/queue'

// Queue state is module-level — wait for drain between tests

async function drain(ms = 200): Promise<void> {
  await new Promise(resolve => setTimeout(resolve, ms))
}

afterEach(async () => {
  // Allow any in-progress jobs to complete before the next test
  await drain()
})

describe('Job queue', () => {
  it('processes jobs in FIFO order', async () => {
    const processed: string[] = []
    setProcessor(async (job) => {
      processed.push(job.jobId)
    })

    enqueue({ jobId: 'fifo-1', app: 'app', imageTag: 'img:1' })
    enqueue({ jobId: 'fifo-2', app: 'app', imageTag: 'img:2' })
    enqueue({ jobId: 'fifo-3', app: 'app', imageTag: 'img:3' })

    await drain(300)

    expect(processed).toEqual(['fifo-1', 'fifo-2', 'fifo-3'])
  })

  it('processes jobs sequentially (max 1 concurrent)', async () => {
    let concurrency = 0
    let maxConcurrency = 0

    setProcessor(async (_job) => {
      concurrency++
      maxConcurrency = Math.max(maxConcurrency, concurrency)
      await drain(20)
      concurrency--
    })

    enqueue({ jobId: 'seq-a', app: 'app', imageTag: 'img:1' })
    enqueue({ jobId: 'seq-b', app: 'app', imageTag: 'img:2' })

    await drain(300)

    expect(maxConcurrency).toBe(1)
  })

  it('continues processing after a job throws', async () => {
    const processed: string[] = []
    // Suppress the expected [queue] error log — this output is intentional, not a test failure
    const originalError = console.error
    console.error = () => {}

    setProcessor(async (job) => {
      if (job.jobId === 'err-bad')
        throw new Error('intentional failure')
      processed.push(job.jobId)
    })

    enqueue({ jobId: 'err-good-1', app: 'app', imageTag: 'img:1' })
    enqueue({ jobId: 'err-bad', app: 'app', imageTag: 'img:2' })
    enqueue({ jobId: 'err-good-2', app: 'app', imageTag: 'img:3' })

    await drain(400)
    console.error = originalError

    expect(processed).toContain('err-good-1')
    expect(processed).toContain('err-good-2')
  })
})

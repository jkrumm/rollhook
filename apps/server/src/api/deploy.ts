import { Elysia, t } from 'elysia'
import { scheduleJob, waitForJob } from '@/jobs/executor'
import { requireRole } from '@/middleware/auth'

export const deployApi = new Elysia({ prefix: '/deploy' })
  .use(
    requireRole('webhook')
      .post('/', async ({ body, query, set }) => {
        const app = body.image_tag.split('/').pop()!.split(':')[0]
        const job = scheduleJob(app, body.image_tag)

        if (query.async) {
          return { job_id: job.id, app: job.app, status: job.status }
        }

        const result = await waitForJob(job.id)
        if (result.status === 'failed') {
          set.status = 500
          return { job_id: result.id, app: result.app, status: result.status, error: result.error }
        }

        return { job_id: result.id, app: result.app, status: result.status }
      }, {
        body: t.Object({ image_tag: t.String() }),
        query: t.Object({ async: t.Optional(t.Boolean()) }),
        detail: { tags: ['Deploy'], summary: 'Trigger rolling deployment. App name is derived from the image tag. Blocks until complete by default; pass ?async=true for fire-and-forget.' },
      }),
  )

import type { Static } from '@sinclair/typebox'
import type { ServerConfigSchema } from './schema/config'

export type ServerConfig = Static<typeof ServerConfigSchema>

export type JobStatus = 'queued' | 'running' | 'success' | 'failed'

export interface JobResult {
  id: string
  app: string
  image_tag: string
  status: JobStatus
  created_at: string
  updated_at: string
  error?: string
}

import process from 'node:process'
import { app } from '@/app'
import { waitForQueueDrain } from '@/jobs/queue'
import { createRegistryManager } from '@/registry/manager'
import { startShutdown } from '@/state'

const secret = process.env.ROLLHOOK_SECRET
if (!secret || secret.length < 7) {
  console.error('ROLLHOOK_SECRET must be set and at least 7 characters long.')
  process.exit(1)
}

export const registryManager = createRegistryManager()

const port = Number(process.env.PORT ?? 7700)

registryManager.start().then(() => {
  app.listen(port, () => {
    process.stdout.write(`RollHook running on http://localhost:${port}\n`)
  })
}).catch((err: unknown) => {
  process.stderr.write(`Failed to start registry: ${err}\n`)
  process.exit(1)
})

// Graceful shutdown: return 503 from /health so Traefik deregisters us,
// drain the job queue so in-flight deployments complete cleanly, then exit.
process.on('SIGTERM', async () => {
  startShutdown()
  // Allow Traefik to observe the 503 and stop routing (healthcheck interval 1s + buffer)
  await new Promise(resolve => setTimeout(resolve, 3_000))
  // Wait for the current job to finish (up to 5 minutes)
  await waitForQueueDrain(5 * 60 * 1000)
  await registryManager.stop()
  app.stop(true)
  process.exit(0)
})

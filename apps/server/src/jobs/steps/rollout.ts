import { appendFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import process from 'node:process'

export async function rolloutApp(
  composePath: string,
  service: string,
  imageTag: string,
  logPath: string,
): Promise<void> {
  const log = (line: string) => appendFileSync(logPath, `[${new Date().toISOString()}] ${line}\n`)
  const cwd = dirname(composePath)

  // Write IMAGE_TAG to .env so docker compose uses the correct image.
  // Docker Compose v2 .env file takes precedence over shell env for compose
  // variable substitution — without this, IMAGE_TAG from a previous deploy
  // would persist in .env and override the new value.
  writeFileSync(join(cwd, '.env'), `IMAGE_TAG=${imageTag}\n`)

  log(`[rollout] Rolling out service: ${service} (IMAGE_TAG=${imageTag})`)

  // Bun intermittent bug: Bun.spawn with an explicit env object throws ENOENT
  // on its first invocation in the process lifetime, even with an absolute binary
  // path. Mutating process.env temporarily and spawning without an explicit env
  // avoids the bug — the child inherits the parent env (including IMAGE_TAG) at
  // fork time. Safe because the job queue is strictly sequential.
  const prevImageTag = process.env.IMAGE_TAG
  // Restore IMAGE_TAG in a finally block so env is always cleaned up even if
  // Bun.spawn throws synchronously (which would taint subsequent queue jobs).
  // Using an IIFE keeps `proc` as a const with the correct inferred type.
  const proc = (() => {
    try {
      process.env.IMAGE_TAG = imageTag
      return Bun.spawn(['docker', 'rollout', service, '-f', composePath], {
        cwd,
        stdout: 'pipe',
        stderr: 'pipe',
      })
    }
    finally {
      if (prevImageTag === undefined)
        delete process.env.IMAGE_TAG
      else
        process.env.IMAGE_TAG = prevImageTag
    }
  })()

  const [exitCode, , stderr] = await Promise.all([
    proc.exited,
    (async () => {
      const reader = proc.stdout.getReader()
      const decoder = new TextDecoder()
      while (true) {
        const { done, value } = await reader.read()
        if (done)
          break
        log(decoder.decode(value, { stream: true }))
      }
    })(),
    new Response(proc.stderr).text(),
  ])

  if (exitCode !== 0)
    throw new Error(`docker rollout failed for ${service} (exit ${exitCode}): ${stderr}`)

  log(`[rollout] Service ${service} rolled out successfully`)
}

import { appendFileSync } from 'node:fs'
import { dirname } from 'node:path'
import process from 'node:process'

// Note: steps always run sequentially (post-MVP: dependency graph)
export async function rolloutApp(
  composePath: string,
  steps: Array<{ service: string }>,
  imageTag: string,
  logPath: string,
): Promise<void> {
  const log = (line: string) => appendFileSync(logPath, `[${new Date().toISOString()}] ${line}\n`)
  const cwd = dirname(composePath)

  for (const step of steps) {
    log(`[rollout] Rolling out service: ${step.service} (IMAGE_TAG=${imageTag})`)

    // Bun intermittent bug: Bun.spawn with an explicit env object throws ENOENT
    // on its first invocation in the process lifetime, even with an absolute binary
    // path. Mutating process.env temporarily and spawning without an explicit env
    // avoids the bug — the child inherits the parent env (including IMAGE_TAG) at
    // fork time. Safe because the job queue is strictly sequential.
    const prevImageTag = process.env.IMAGE_TAG
    process.env.IMAGE_TAG = imageTag
    const proc = Bun.spawn(['docker', 'rollout', step.service, '-f', composePath], {
      cwd,
      stdout: 'pipe',
      stderr: 'pipe',
    })
    if (prevImageTag === undefined)
      delete process.env.IMAGE_TAG
    else
      process.env.IMAGE_TAG = prevImageTag

    const [exitCode, , stderr] = await Promise.all([
      proc.exited,
      (async () => {
        const reader = proc.stdout.getReader()
        const decoder = new TextDecoder()
        while (true) {
          const { done, value } = await reader.read()
          if (done)
            break
          log(decoder.decode(value))
        }
      })(),
      new Response(proc.stderr).text(),
    ])

    if (exitCode !== 0)
      throw new Error(`docker rollout failed for ${step.service} (exit ${exitCode}): ${stderr}`)

    log(`[rollout] Service ${step.service} rolled out successfully`)
  }
}

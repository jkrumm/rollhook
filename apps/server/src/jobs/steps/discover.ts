import { appendFileSync } from 'node:fs'

interface DockerPsEntry {
  ID: string
  Image: string
  Names: string
}

// Exported for unit testing — pure parsing helpers with no side effects.

export function extractImageName(imageTag: string): string {
  // Strip only the tag portion (after the last `/`, find the first `:`)
  // Preserves port numbers in registry hostnames, e.g. localhost:5001/image:tag → localhost:5001/image
  const lastSlash = imageTag.lastIndexOf('/')
  const afterLastSlash = imageTag.slice(lastSlash + 1)
  const tagStart = afterLastSlash.indexOf(':')
  if (tagStart < 0)
    return imageTag
  return imageTag.slice(0, lastSlash + 1 + tagStart)
}

export function findMatchingContainerId(
  psOutput: string,
  imageName: string,
): { id: string, name: string } | null {
  for (const line of psOutput.split('\n')) {
    if (!line.trim())
      continue
    let entry: DockerPsEntry
    try {
      entry = JSON.parse(line) as DockerPsEntry
    }
    catch {
      continue
    }
    // Match containers whose image is exactly `imageName` (bare) or `imageName:tag`
    if (entry.Image === imageName || entry.Image.startsWith(`${imageName}:`)) {
      return { id: entry.ID, name: entry.Names.replace(/^\//, '') }
    }
  }
  return null
}

export function extractComposeInfo(
  inspectOutput: string,
  containerName: string,
): { composePath: string, service: string } {
  let labels: Record<string, string> | null
  try {
    labels = JSON.parse(inspectOutput.trim()) as Record<string, string> | null
  }
  catch {
    throw new Error(`Container ${containerName} returned invalid JSON from docker inspect`)
  }

  if (!labels)
    throw new Error(`Container ${containerName} has no Docker labels — not started via docker compose`)

  // config_files may list multiple comma-separated paths when using -f overrides; take the first
  const composePath = (labels['com.docker.compose.project.config_files'] ?? '').split(',')[0]?.trim()
  const service = labels['com.docker.compose.service']

  if (!composePath)
    throw new Error(`Container ${containerName} is missing 'config_files' label — not started via docker compose`)

  if (!service)
    throw new Error(`Container ${containerName} is missing 'service' label — not started via docker compose`)

  return { composePath, service }
}

export async function discover(imageTag: string, app: string, logPath: string): Promise<{ composePath: string, service: string }> {
  const log = (line: string) => appendFileSync(logPath, `[${new Date().toISOString()}] ${line}\n`)
  const imageName = extractImageName(imageTag)

  log(`[discover] Searching for containers using image: ${imageName}`)

  const psProc = Bun.spawn(['docker', 'ps', '--format', '{{json .}}'], {
    stdout: 'pipe',
    stderr: 'pipe',
  })

  const [psExit, psStdout, psStderr] = await Promise.all([
    psProc.exited,
    new Response(psProc.stdout).text(),
    new Response(psProc.stderr).text(),
  ])

  if (psExit !== 0)
    throw new Error(`docker ps failed (exit ${psExit}): ${psStderr}`)

  const match = findMatchingContainerId(psStdout, imageName)

  if (!match) {
    log(`[discover] No running containers found matching image: ${imageName}`)
    log(`[discover] Hint: Ensure the service was started at least once before using rollhook`)
    log(`[discover] Hint: Run 'docker ps' on your server to verify the container is running`)
    log(`[discover] Hint: The image registry prefix must match exactly what Docker shows for the running container`)
    log(`[discover] Hint: To verify: docker ps --filter ancestor=${imageName} (check image names match)`)
    throw new Error(`No running container found matching image: ${imageName}`)
  }

  log(`[discover] Found container: ${match.name} (ID: ${match.id.slice(0, 12)})`)

  const inspectProc = Bun.spawn(['docker', 'inspect', match.id, '--format', '{{json .Config.Labels}}'], {
    stdout: 'pipe',
    stderr: 'pipe',
  })

  const [inspectExit, inspectStdout, inspectStderr] = await Promise.all([
    inspectProc.exited,
    new Response(inspectProc.stdout).text(),
    new Response(inspectProc.stderr).text(),
  ])

  if (inspectExit !== 0)
    throw new Error(`docker inspect failed (exit ${inspectExit}): ${inspectStderr}`)

  const { composePath, service } = extractComposeInfo(inspectStdout, match.name)

  log(`[discover] Compose file: ${composePath}`)
  log(`[discover] Service: ${service}`)
  log(`[discover] Discovery complete`)

  return { composePath, service }
}

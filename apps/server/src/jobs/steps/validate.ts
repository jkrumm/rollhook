import { appendFileSync, existsSync } from 'node:fs'
import { isAbsolute } from 'node:path'

export function validateCompose(composePath: string, logPath: string): void {
  const log = (line: string) => appendFileSync(logPath, `[${new Date().toISOString()}] ${line}\n`)
  log('[validate] Checking compose_path')
  if (!isAbsolute(composePath))
    throw new Error(`compose_path must be absolute, got: ${composePath}`)
  if (!existsSync(composePath))
    throw new Error(`Compose file not found: ${composePath}`)
  log(`[validate] OK — ${composePath}`)
}

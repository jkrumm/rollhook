import { appendFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

export function writeEnvFile(composePath: string, imageTag: string, logPath: string): void {
  const envPath = join(dirname(composePath), '.env')
  writeFileSync(envPath, `IMAGE_TAG=${imageTag}\n`)
  appendFileSync(logPath, `[env] IMAGE_TAG=${imageTag} → ${envPath}\n`)
}

import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { writeEnvFile } from '../jobs/steps/env'

const TMP_DIR = join(tmpdir(), `rollhook-env-test-${Date.now()}`)

beforeAll(() => {
  mkdirSync(TMP_DIR, { recursive: true })
})

afterAll(() => {
  rmSync(TMP_DIR, { recursive: true, force: true })
})

function makeLogPath(name: string): string {
  const logPath = join(TMP_DIR, `${name}.log`)
  writeFileSync(logPath, '')
  return logPath
}

describe('writeEnvFile', () => {
  it('writes IMAGE_TAG to .env next to compose file', () => {
    const composePath = join(TMP_DIR, 'compose.yml')
    writeFileSync(composePath, '')
    const logPath = makeLogPath('write-env')

    writeEnvFile(composePath, 'ghcr.io/user/app:sha256abc', logPath)

    const envContent = readFileSync(join(TMP_DIR, '.env'), 'utf-8')
    expect(envContent).toBe('IMAGE_TAG=ghcr.io/user/app:sha256abc\n')
  })

  it('overwrites existing .env content', () => {
    const composePath = join(TMP_DIR, 'compose.yml')
    writeFileSync(composePath, '')
    writeFileSync(join(TMP_DIR, '.env'), 'IMAGE_TAG=old-tag\nOTHER_VAR=value\n')
    const logPath = makeLogPath('overwrite-env')

    writeEnvFile(composePath, 'new-tag', logPath)

    const envContent = readFileSync(join(TMP_DIR, '.env'), 'utf-8')
    expect(envContent).toBe('IMAGE_TAG=new-tag\n')
  })

  it('appends IMAGE_TAG line to log', () => {
    const composePath = join(TMP_DIR, 'compose.yml')
    writeFileSync(composePath, '')
    const logPath = makeLogPath('log-env')

    writeEnvFile(composePath, 'my-image:v3', logPath)

    const logContent = readFileSync(logPath, 'utf-8')
    expect(logContent).toContain('[env] IMAGE_TAG=my-image:v3')
    expect(logContent).toContain('.env')
  })
})

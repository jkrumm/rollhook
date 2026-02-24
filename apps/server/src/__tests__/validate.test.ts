import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { validateApp } from '../jobs/steps/validate'

const TMP_DIR = join(tmpdir(), `rollhook-validate-test-${Date.now()}`)

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

function makeAppDir(name: string, files: Record<string, string>): string {
  const dir = join(TMP_DIR, name)
  mkdirSync(dir, { recursive: true })
  for (const [filename, content] of Object.entries(files))
    writeFileSync(join(dir, filename), content)
  return dir
}

describe('validateApp', () => {
  it('accepts valid rollhook.yaml', async () => {
    const dir = makeAppDir('valid-app', {
      'rollhook.yaml': `name: my-app\nsteps:\n  - service: backend\n`,
      'compose.yml': `services:\n  backend:\n    image: nginx\n`,
    })
    const result = await validateApp(dir, makeLogPath('valid-app'))
    expect(result.name).toBe('my-app')
    expect(result.steps).toHaveLength(1)
    expect(result.steps[0]!.service).toBe('backend')
  })

  it('throws when steps is missing', async () => {
    const dir = makeAppDir('no-steps-app', {
      'rollhook.yaml': `name: my-app\n`,
      'compose.yml': `services:\n  app:\n    image: nginx\n`,
    })
    expect(validateApp(dir, makeLogPath('no-steps'))).rejects.toThrow('Invalid rollhook.yaml')
  })

  it('throws when compose file is not found', async () => {
    const dir = makeAppDir('no-compose-app', {
      'rollhook.yaml': `name: my-app\nsteps:\n  - service: backend\n`,
      // no compose.yml
    })
    expect(validateApp(dir, makeLogPath('no-compose'))).rejects.toThrow('Compose file not found')
  })

  it('throws when rollhook.yaml is not found', async () => {
    const dir = makeAppDir('no-yaml-app', {
      'compose.yml': `services:\n  app:\n    image: nginx\n`,
      // no rollhook.yaml
    })
    expect(validateApp(dir, makeLogPath('no-yaml'))).rejects.toThrow('rollhook.yaml not found')
  })

  it('compose_file defaults to compose.yml when not specified', async () => {
    const dir = makeAppDir('default-compose-app', {
      'rollhook.yaml': `name: my-app\nsteps:\n  - service: backend\n`,
      'compose.yml': `services:\n  backend:\n    image: nginx\n`,
    })
    const result = await validateApp(dir, makeLogPath('default-compose'))
    expect(result.name).toBe('my-app')
    // compose_file is not set in YAML → validateApp checks for 'compose.yml' as fallback
    expect(result.compose_file).toBeUndefined()
  })
})

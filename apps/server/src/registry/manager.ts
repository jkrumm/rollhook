import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import process from 'node:process'
import { generateHtpasswd, generateZotConfig, getZotPassword, ZOT_USER } from './config'

const REGISTRY_DIR = join(process.cwd(), 'data', 'registry')
const CONFIG_PATH = join(REGISTRY_DIR, 'config.json')
const HTPASSWD_PATH = join(REGISTRY_DIR, '.htpasswd')
const ZOT_PORT = 5000
const ZOT_READY_TIMEOUT_MS = 10_000
const ZOT_POLL_INTERVAL_MS = 200

interface ZotProcess {
  stdout: ReadableStream<Uint8Array> | null
  stderr: ReadableStream<Uint8Array> | null
  exited: Promise<number>
  kill: () => void
}

export interface RegistryManager {
  start: () => Promise<void>
  stop: () => Promise<void>
  isRunning: () => boolean
  // Returns { user: 'rollhook', password: ROLLHOOK_SECRET } — deterministic, no random state
  getInternalCredentials: () => { user: string, password: string }
}

export function createRegistryManager(): RegistryManager {
  let zotProcess: ZotProcess | null = null
  let stopping = false

  async function start(): Promise<void> {
    mkdirSync(REGISTRY_DIR, { recursive: true })

    writeFileSync(
      CONFIG_PATH,
      generateZotConfig({ storageRoot: REGISTRY_DIR, htpasswdPath: HTPASSWD_PATH, port: ZOT_PORT }),
    )
    writeFileSync(HTPASSWD_PATH, await generateHtpasswd())

    const proc = Bun.spawn(['zot', 'serve', CONFIG_PATH], {
      stdout: 'pipe',
      stderr: 'pipe',
    })
    zotProcess = proc as unknown as ZotProcess

    pipeWithPrefix(proc.stdout as ReadableStream<Uint8Array> | null)
    pipeWithPrefix(proc.stderr as ReadableStream<Uint8Array> | null)

    void proc.exited.then((code) => {
      if (code !== 0 && !stopping)
        process.stderr.write(`[zot] process exited unexpectedly with code ${code}\n`)
    })

    await waitUntilReady()
  }

  async function stop(): Promise<void> {
    if (zotProcess) {
      stopping = true
      zotProcess.kill()
      zotProcess = null
    }
  }

  function isRunning(): boolean {
    return zotProcess !== null
  }

  function getInternalCredentials(): { user: string, password: string } {
    return { user: ZOT_USER, password: getZotPassword() }
  }

  return { start, stop, isRunning, getInternalCredentials }
}

function pipeWithPrefix(stream: ReadableStream<Uint8Array> | null): void {
  if (!stream)
    return
  void (async () => {
    const reader = stream.getReader()
    const decoder = new TextDecoder()
    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done)
          break
        process.stdout.write(`[zot] ${decoder.decode(value, { stream: true })}`)
      }
    }
    catch {
      // Stream closed on process exit — expected
    }
  })()
}

async function waitUntilReady(): Promise<void> {
  const deadline = Date.now() + ZOT_READY_TIMEOUT_MS
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://127.0.0.1:${ZOT_PORT}/v2/`)
      if (res.status === 200 || res.status === 401) {
        process.stdout.write('[zot] registry ready\n')
        return
      }
    }
    catch {
      // Not ready yet
    }
    await new Promise(resolve => setTimeout(resolve, ZOT_POLL_INTERVAL_MS))
  }
  throw new Error(`Zot registry failed to start within ${ZOT_READY_TIMEOUT_MS}ms`)
}

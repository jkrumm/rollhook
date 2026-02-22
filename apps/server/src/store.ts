import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

const STORE_PATH = join(process.cwd(), 'data/store.json')

interface Store {
  counter: number
}

function read(): Store {
  if (!existsSync(STORE_PATH))
    return { counter: 0 }
  return JSON.parse(readFileSync(STORE_PATH, 'utf-8')) as Store
}

function write(data: Store): void {
  mkdirSync(dirname(STORE_PATH), { recursive: true })
  writeFileSync(STORE_PATH, JSON.stringify(data, null, 2))
}

export const store = {
  getCounter(): number {
    return read().counter
  },
  incrementCounter(amount = 1): number {
    const data = read()
    data.counter += amount
    write(data)
    return data.counter
  },
}

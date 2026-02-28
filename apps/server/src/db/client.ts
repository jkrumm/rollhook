import { Database } from 'bun:sqlite'
import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import process from 'node:process'

const DATA_DIR = join(process.cwd(), 'data')
const DB_PATH = join(DATA_DIR, 'rollhook.db')

mkdirSync(DATA_DIR, { recursive: true })
mkdirSync(join(DATA_DIR, 'logs'), { recursive: true })

export const db = new Database(DB_PATH, { create: true })

// Exported for unit testing — accepts any Database instance
export function applyMigrations(database: Database): void {
  database.exec(`
    CREATE TABLE IF NOT EXISTS jobs (
      id TEXT PRIMARY KEY,
      app TEXT NOT NULL,
      image_tag TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'queued',
      error TEXT,
      created_at TEXT NOT NULL,
      updated_at TEXT NOT NULL
    );
  `)
  // Add new columns idempotently (ADD COLUMN IF NOT EXISTS not available in all SQLite versions)
  const cols = (database.prepare('PRAGMA table_info(jobs)').all() as Array<{ name: string }>).map(c => c.name)
  if (!cols.includes('compose_path'))
    database.exec('ALTER TABLE jobs ADD COLUMN compose_path TEXT')
  if (!cols.includes('service'))
    database.exec('ALTER TABLE jobs ADD COLUMN service TEXT')
}

applyMigrations(db)

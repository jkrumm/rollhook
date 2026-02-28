import { Database } from 'bun:sqlite'
import { describe, expect, it } from 'bun:test'
import { applyMigrations } from '../db/client'

function columnNames(db: Database): string[] {
  return (db.prepare('PRAGMA table_info(jobs)').all() as Array<{ name: string }>).map(c => c.name)
}

describe('DB migration', () => {
  it('adds compose_path and service columns to a fresh DB', () => {
    const db = new Database(':memory:')
    applyMigrations(db)
    const cols = columnNames(db)
    expect(cols).toContain('compose_path')
    expect(cols).toContain('service')
  })

  it('is idempotent — running twice does not throw', () => {
    const db = new Database(':memory:')
    expect(() => {
      applyMigrations(db)
      applyMigrations(db)
    }).not.toThrow()
  })

  it('preserves existing rows when columns are added', () => {
    const db = new Database(':memory:')
    // Create table without new columns, insert a row, then migrate
    db.exec(`
      CREATE TABLE jobs (
        id TEXT PRIMARY KEY, app TEXT NOT NULL, image_tag TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'queued', error TEXT,
        created_at TEXT NOT NULL, updated_at TEXT NOT NULL
      )
    `)
    db.prepare(`INSERT INTO jobs VALUES ('id-1', 'app', 'img:1', 'success', null, 'now', 'now')`).run()
    applyMigrations(db)
    const row = db.prepare('SELECT * FROM jobs WHERE id = ?').get('id-1') as Record<string, unknown>
    expect(row.app).toBe('app')
    expect(row.compose_path).toBeNull()
    expect(row.service).toBeNull()
  })

  it('new columns have TEXT type and are nullable', () => {
    const db = new Database(':memory:')
    applyMigrations(db)
    const info = db.prepare('PRAGMA table_info(jobs)').all() as Array<{
      name: string
      type: string
      notnull: number
    }>
    const composePath = info.find(c => c.name === 'compose_path')
    const service = info.find(c => c.name === 'service')
    expect(composePath?.type).toBe('TEXT')
    expect(composePath?.notnull).toBe(0)
    expect(service?.type).toBe('TEXT')
    expect(service?.notnull).toBe(0)
  })
})

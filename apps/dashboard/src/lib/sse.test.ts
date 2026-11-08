import { describe, expect, test } from 'bun:test'
import { parseSseChunk } from './sse'

describe('parseSseChunk', () => {
  test('parses data lines', () => {
    const r = parseSseChunk('', 'data: one\n\ndata: two\n\n')
    expect(r).toEqual({ lines: ['one', 'two'], rest: '', done: false })
  })

  test('stops at [DONE] without emitting it', () => {
    const r = parseSseChunk('', 'data: one\n\ndata: [DONE]\n\ndata: after\n\n')
    expect(r).toEqual({ lines: ['one'], rest: '', done: true })
  })

  test('ignores blank and comment events', () => {
    const r = parseSseChunk('', ': keepalive\n\n\n\ndata:\n\ndata: x\n\n')
    expect(r.lines).toEqual(['x'])
  })

  test('keeps partial event in rest across chunk boundaries', () => {
    const a = parseSseChunk('', 'data: hel')
    expect(a).toEqual({ lines: [], rest: 'data: hel', done: false })
    const b = parseSseChunk(a.rest, 'lo\n')
    expect(b.lines).toEqual([])
    const c = parseSseChunk(b.rest, '\ndata: next')
    expect(c).toEqual({ lines: ['hello'], rest: 'data: next', done: false })
  })

  test('handles [DONE] split across chunks', () => {
    const a = parseSseChunk('', 'data: [DO')
    const b = parseSseChunk(a.rest, 'NE]\n\n')
    expect(b.done).toBe(true)
    expect(b.lines).toEqual([])
  })
})

export const SSE_DONE = '[DONE]'

export interface SseChunkResult {
  lines: string[]
  rest: string
  done: boolean
}

// Pure SSE parser: `buffer` is the unconsumed tail from the previous call, `chunk` the new text.
// Events end at a blank line; comment (`:`) and non-data fields are ignored; the `[DONE]`
// sentinel ends parsing and is never returned as a line.
export function parseSseChunk(buffer: string, chunk: string): SseChunkResult {
  const parts = (buffer + chunk).split('\n\n')
  const rest = parts.pop() ?? ''
  const lines: string[] = []
  for (const part of parts) {
    const data = part
      .split('\n')
      .filter((l) => l.startsWith('data:'))
      .map((l) => l.slice(5).replace(/^ /, ''))
      .join('\n')
    if (!data) continue
    if (data === SSE_DONE) return { lines, rest: '', done: true }
    lines.push(data)
  }
  return { lines, rest, done: false }
}

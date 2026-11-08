import type { LogLine } from '@rollhook/ui'
import { LogDrawer } from '@rollhook/ui'
import { useEffect, useRef, useState } from 'react'
import { streamLogs } from '../lib/api'

const MAX_RECONNECTS = 5
const RECONNECT_BASE_MS = 1000
const TIMESTAMP_RE = /^\[(\d{4}-\d{2}-\d{2}T[\d:.]+Z)\]\s+(.*)/

interface LogContainerProps {
  jobId: string
}

export function LogContainer({ jobId }: LogContainerProps) {
  const [logLines, setLogLines] = useState<LogLine[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const startTimeRef = useRef<number | null>(null)
  // Tracks how many lines have already been rendered — used to skip duplicates on reconnect
  const seenCountRef = useRef(0)

  useEffect(() => {
    // Reset state for new job
    setLogLines([])
    setIsLoading(true)
    startTimeRef.current = null
    seenCountRef.current = 0

    const ctrl = new AbortController()

    // The server follows the log until the job is terminal, so one stream suffices.
    // Reconnect only after an error; already-rendered lines are skipped via seenCountRef.
    async function streamWithRetry(): Promise<void> {
      for (let attempt = 0; attempt <= MAX_RECONNECTS; attempt++) {
        const pass = { lineIndex: 0 }
        try {
          await streamLogs(
            jobId,
            (raw) => {
              if (pass.lineIndex++ < seenCountRef.current) return

              const match = raw.match(TIMESTAMP_RE)
              let text: string
              let elapsed: number | undefined

              if (match) {
                const lineMs = new Date(match[1]).getTime()
                if (startTimeRef.current === null) startTimeRef.current = lineMs
                elapsed = Math.round((lineMs - startTimeRef.current) / 1000)
                text = match[2]
              } else {
                text = raw
              }

              seenCountRef.current++
              setLogLines((prev) => [...prev, { text, elapsed }])
              setIsLoading(false)
            },
            ctrl.signal,
          )
          break
        } catch {
          if (ctrl.signal.aborted || attempt === MAX_RECONNECTS) break
          await new Promise<void>((resolve) => {
            const t = setTimeout(resolve, RECONNECT_BASE_MS * 2 ** attempt)
            function onAbort() {
              clearTimeout(t)
              resolve()
            }
            // eslint-disable-next-line react-web-api/no-leaked-event-listener -- ctrl.abort() fires this in cleanup; { once: true } auto-removes it
            ctrl.signal.addEventListener('abort', onAbort, { once: true })
          })
          if (ctrl.signal.aborted) break
        }
      }
      setIsLoading(false)
    }

    void streamWithRetry()
    return () => {
      ctrl.abort()
    }
  }, [jobId])

  return <LogDrawer logLines={logLines} isLoading={isLoading} />
}

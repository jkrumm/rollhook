import { cn } from '../lib/utils'

export interface LogLine {
  text: string
  elapsed?: number
}

const TAG_COLORS: Record<string, string> = {
  '[discover]': 'text-blue',
  '[validate]': 'text-yellow',
  '[pull]': 'text-purple',
  '[rollout]': 'text-green',
}

function parseLine(text: string): { tag: string | null, rest: string } {
  const match = text.match(/^(\[\w+\])\s?(.*)$/)
  return match ? { tag: match[1], rest: match[2] } : { tag: null, rest: text }
}

interface LogDrawerProps {
  logLines: LogLine[]
  isLoading: boolean
  className?: string
}

export function LogDrawer({ logLines, isLoading, className }: LogDrawerProps) {
  return (
    <div className={cn('bg-black/40 border-t border-border', className)}>
      {isLoading && logLines.length === 0
        ? (
            <span className="text-muted-foreground text-xs animate-pulse px-4 py-3 block">loading logs…</span>
          )
        : (
            <div className="font-mono text-[10px] sm:text-xs space-y-0.5 py-2">
              {logLines.length === 0
                ? <span className="text-muted-foreground px-4">(no log output)</span>
                : logLines.map((line, i) => {
                    const { tag, rest } = parseLine(line.text)
                    const tagColor = tag ? (TAG_COLORS[tag.toLowerCase()] ?? 'text-foreground/50') : null
                    return (
                      // eslint-disable-next-line react/no-array-index-key -- log lines are append-only, index is stable
                      <div key={i} className="flex gap-2 leading-relaxed pr-3">
                        {line.elapsed !== undefined && (
                          <span className="text-muted-foreground/40 w-6 shrink-0 text-right tabular-nums">
                            {line.elapsed}
                            s
                          </span>
                        )}
                        {tag && (
                          <span className={cn('shrink-0 font-semibold', tagColor)}>{tag}</span>
                        )}
                        <span className="text-foreground/70 break-all">{rest}</span>
                      </div>
                    )
                  })}
            </div>
          )}
    </div>
  )
}

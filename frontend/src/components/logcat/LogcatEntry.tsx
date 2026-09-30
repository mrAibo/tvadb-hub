import { memo } from 'react'
import { cn } from '@/lib/utils'
import { classifyLogcatIssue } from '@/lib/logcatAnalysis'
import type { LogcatEntry as LogcatEntryType } from '@/lib/types'

const LEVEL_COLORS: Record<string, string> = {
  V: 'text-muted-foreground',
  D: 'text-[var(--logcat-debug)]',
  I: 'text-[var(--logcat-info)]',
  W: 'text-[var(--logcat-warn)]',
  E: 'text-[var(--logcat-error)]',
  F: 'text-[var(--logcat-fatal)] font-bold',
}

const LEVEL_BG: Record<string, string> = {
  V: 'bg-muted-foreground/10',
  D: 'bg-[var(--logcat-debug)]/10',
  I: 'bg-[var(--logcat-info)]/10',
  W: 'bg-[var(--logcat-warn)]/10',
  E: 'bg-[var(--logcat-error)]/10',
  F: 'bg-[var(--logcat-error)]/20',
}

interface LogcatEntryProps {
  entry: LogcatEntryType
}

export const LogcatEntry = memo(function LogcatEntry({ entry }: LogcatEntryProps) {
  const levelColor = LEVEL_COLORS[entry.level] ?? LEVEL_COLORS.V
  const levelBg = LEVEL_BG[entry.level] ?? LEVEL_BG.V
  const issue = classifyLogcatIssue(entry)

  return (
    <div
      className={cn(
        'flex items-start gap-2 border-l-2 border-transparent px-3 py-1 font-mono text-[11px] leading-relaxed hover:bg-muted/30 transition-colors',
        issue === 'crash' && 'border-l-destructive/70 bg-destructive/5',
        issue === 'anr' && 'border-l-[var(--warning)]/70 bg-[var(--warning)]/5',
      )}
    >
      <span className={cn('shrink-0 rounded px-1 py-0.5 text-[10px] font-semibold tabular-nums', levelBg, levelColor)}>
        {entry.level}
      </span>
      <span className="shrink-0 text-muted-foreground tabular-nums select-none">
        {entry.time}
      </span>
      <span className="shrink-0 text-muted-foreground tabular-nums w-16 text-right select-none">
        {entry.pid}/{entry.tid}
      </span>
      <span className="shrink-0 text-cyan-400/80 truncate max-w-[180px]">
        {entry.tag}
      </span>
      {issue && (
        <span
          className={cn(
            'shrink-0 rounded px-1 py-0.5 text-[9px] font-bold tracking-wide',
            issue === 'crash'
              ? 'bg-destructive/15 text-destructive'
              : 'bg-[var(--warning)]/15 text-[var(--warning)]',
          )}
        >
          {issue === 'crash' ? 'CRASH' : 'ANR'}
        </span>
      )}
      <span className={cn('flex-1 truncate', levelColor)}>
        {entry.message}
      </span>
    </div>
  )
})

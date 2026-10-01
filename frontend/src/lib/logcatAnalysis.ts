import type {
  LogcatEntry,
  LogcatFilter,
  LogcatIssue,
} from '@/lib/types'

const ANR_PATTERNS = [
  /\banr in\b/i,
  /application not responding/i,
  /\bam_anr\b/i,
  /input dispatching timed out/i,
  /broadcast of intent .* timed out/i,
  /executing service .* timed out/i,
]

const CRASH_PATTERNS = [
  /fatal exception/i,
  /fatal signal\s+\d+/i,
  /uncaught exception/i,
  /native crash/i,
  /crash dump/i,
  /tombstone written to/i,
]

export function classifyLogcatIssue(entry: LogcatEntry): LogcatIssue | null {
  const searchable = `${entry.tag}\n${entry.message}\n${entry.raw}`

  if (ANR_PATTERNS.some((pattern) => pattern.test(searchable))) {
    return 'anr'
  }

  if (
    (entry.tag.toLowerCase() === 'androidruntime' &&
      (entry.level === 'E' || entry.level === 'F')) ||
    CRASH_PATTERNS.some((pattern) => pattern.test(searchable))
  ) {
    return 'crash'
  }

  return null
}

export function matchesLogcatFilter(entry: LogcatEntry, filter: LogcatFilter): boolean {
  if (!filter.levels.includes(entry.level)) {
    return false
  }

  if (
    filter.tag !== '' &&
    !entry.tag.toLowerCase().includes(filter.tag.toLowerCase())
  ) {
    return false
  }

  if (
    filter.text !== '' &&
    !entry.message.toLowerCase().includes(filter.text.toLowerCase())
  ) {
    return false
  }

  if (filter.pid !== '' && entry.pid !== filter.pid.trim()) {
    return false
  }

  if (
    filter.process !== '' &&
    !(entry.processName ?? '').toLowerCase().includes(filter.process.toLowerCase())
  ) {
    return false
  }

  if (filter.issue !== 'all' && classifyLogcatIssue(entry) !== filter.issue) {
    return false
  }

  return true
}

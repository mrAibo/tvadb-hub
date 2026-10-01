import { describe, expect, it } from 'vitest'
import type { LogcatEntry, LogcatFilter } from '@/lib/types'
import { classifyLogcatIssue, matchesLogcatFilter } from '../logcatAnalysis'

function makeEntry(overrides: Partial<LogcatEntry> = {}): LogcatEntry {
  return {
    id: '1',
    serial: 'device-1',
    date: '09-30',
    time: '09:30:00.000',
    pid: '123',
    tid: '123',
    level: 'I',
    tag: 'Example',
    message: 'normal message',
    raw: '09-30 09:30:00.000 123 123 I Example: normal message',
    timestamp: '2026-09-30T09:30:00.000',
    ...overrides,
  }
}

const baseFilter: LogcatFilter = {
  levels: ['V', 'D', 'I', 'W', 'E', 'F'],
  tag: '',
  text: '',
  pid: '',
  process: '',
  issue: 'all',
}

describe('logcat issue analysis', () => {
  it('classifies AndroidRuntime fatal output as a crash', () => {
    const entry = makeEntry({
      level: 'E',
      tag: 'AndroidRuntime',
      message: 'FATAL EXCEPTION: main',
    })

    expect(classifyLogcatIssue(entry)).toBe('crash')
  })

  it('classifies ActivityManager ANR output as an ANR', () => {
    const entry = makeEntry({
      level: 'E',
      tag: 'ActivityManager',
      message: 'ANR in com.example.app',
    })

    expect(classifyLogcatIssue(entry)).toBe('anr')
  })

  it('does not classify an ordinary error as a crash', () => {
    const entry = makeEntry({
      level: 'E',
      tag: 'MediaCodec',
      message: 'decoder returned an error',
    })

    expect(classifyLogcatIssue(entry)).toBeNull()
  })

  it('filters by exact PID and case-insensitive process name', () => {
    const entry = makeEntry({
      pid: '4242',
      processName: 'com.example.Player:remote',
    })

    expect(matchesLogcatFilter(entry, {
      ...baseFilter,
      pid: '4242',
      process: 'player',
    })).toBe(true)

    expect(matchesLogcatFilter(entry, {
      ...baseFilter,
      pid: '42',
    })).toBe(false)

    expect(matchesLogcatFilter(entry, {
      ...baseFilter,
      process: 'missing.app',
    })).toBe(false)
  })

  it('combines issue filtering with level, tag and message filters', () => {
    const crash = makeEntry({
      level: 'E',
      tag: 'AndroidRuntime',
      message: 'FATAL EXCEPTION: main com.example',
    })

    expect(matchesLogcatFilter(crash, {
      ...baseFilter,
      levels: ['E', 'F'],
      tag: 'android',
      text: 'example',
      issue: 'crash',
    })).toBe(true)

    expect(matchesLogcatFilter(crash, {
      ...baseFilter,
      issue: 'anr',
    })).toBe(false)
  })
})

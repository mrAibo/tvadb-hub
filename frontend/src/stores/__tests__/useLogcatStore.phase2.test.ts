import { beforeEach, describe, expect, it } from 'vitest'
import type { LogcatEntry } from '@/lib/types'
import { useLogcatStore } from '../useLogcatStore'

function entry(id: string): LogcatEntry {
  return {
    id,
    serial: 'device-1',
    date: '10-01',
    time: '12:00:00.000',
    pid: '123',
    tid: '123',
    processName: 'com.example.app',
    level: 'I',
    tag: 'Example',
    message: 'hello',
    raw: '10-01 12:00:00.000 123 123 I Example: hello',
    timestamp: '10-01 12:00:00.000',
  }
}

describe('useLogcatStore phase 2', () => {
  beforeEach(() => {
    useLogcatStore.getState().reset()
    useLogcatStore.getState().clearPinned()
  })

  it('pins and unpins immutable entry snapshots by id', () => {
    const first = entry('a')

    useLogcatStore.getState().togglePinned(first)
    expect(useLogcatStore.getState().pinnedEntries).toEqual([first])

    useLogcatStore.getState().togglePinned(first)
    expect(useLogcatStore.getState().pinnedEntries).toEqual([])
  })

  it('keeps pinned events when the rolling log buffer is cleared', () => {
    const first = entry('a')
    useLogcatStore.getState().appendLogs([first])
    useLogcatStore.getState().togglePinned(first)

    useLogcatStore.getState().clearLogs()

    expect(useLogcatStore.getState().logs).toEqual([])
    expect(useLogcatStore.getState().pinnedEntries).toEqual([first])
  })

  it('clearing pins also leaves pinned-only mode', () => {
    useLogcatStore.getState().togglePinned(entry('a'))
    useLogcatStore.getState().setPinnedOnly(true)

    useLogcatStore.getState().clearPinned()

    expect(useLogcatStore.getState().pinnedEntries).toEqual([])
    expect(useLogcatStore.getState().pinnedOnly).toBe(false)
  })
})

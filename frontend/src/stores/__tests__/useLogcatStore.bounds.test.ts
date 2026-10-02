import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LogcatEntry } from '@/lib/types'
import {
  DEFAULT_LOGCAT_BUFFER_LIMIT,
  resolveLogcatExport,
  resolveLogcatNotice,
  useLogcatStore,
} from '../useLogcatStore'

function entry(id: string, message = `line ${id}`, level: LogcatEntry['level'] = 'I'): LogcatEntry {
  return {
    id,
    serial: 'device-1',
    date: '10-01',
    time: '12:00:00.000',
    pid: '123',
    tid: '123',
    processName: 'com.example.app',
    level,
    tag: 'Example',
    message,
    raw: `10-01 12:00:00.000 123 123 ${level} Example: ${message}`,
    timestamp: '10-01 12:00:00.000',
  }
}

/**
 * The serialized entry carries the payload twice (message and raw), so a payload
 * of N characters costs roughly 2N bytes. These sizes are chosen against the
 * shipped ceilings — a 64 KiB per-entry ceiling, a 32 MiB buffer volume and an
 * 8 MiB ingress volume — and stay small enough that the suite never allocates
 * hundreds of megabytes to prove a bound.
 */
const OVERSIZED_ENTRY_CHARS = 100_000
/** ~64 KB serialized: comfortably below the 64 KiB per-entry ceiling. */
const LARGE_ENTRY_CHARS = 32_000
/** Mirrors LOGCAT_MAX_ENTRY_BYTES so the ceiling itself is asserted. */
const ENTRY_CEILING = 64 * 1024
/** Mirrors LOGCAT_MAX_BUFFER_BYTES so the ceiling itself is asserted. */
const BYTE_CEILING = 32 * 1024 * 1024

function hugeEntry(id: string, chars: number = OVERSIZED_ENTRY_CHARS): LogcatEntry {
  const payload = 'x'.repeat(chars)
  const raw = `10-01 12:00:00.000 123 123 E Example: ${payload}`

  return {
    ...entry(id, payload, 'E'),
    raw,
  }
}

/** Serialized volume of the entries currently retained, measured independently. */
function measuredLogBytes(): number {
  return useLogcatStore
    .getState()
    .logs.reduce((sum, item) => sum + JSON.stringify(item).length, 0)
}

function resetStore() {
  useLogcatStore.getState().reset()
  useLogcatStore.getState().clearPinned()
  useLogcatStore.getState().setExportMode('all')
  useLogcatStore.getState().setBufferLimit(DEFAULT_LOGCAT_BUFFER_LIMIT)
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('useLogcatStore logcat retention budgets', { timeout: 60_000 }, () => {
  beforeEach(() => {
    resetStore()
  })

  describe('byte budgets', () => {
    it('bounds each entry so one oversized line cannot hold the buffer hostage', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs([hugeEntry('huge-1')])

      const state = useLogcatStore.getState()
      const stored = state.logs[0]

      expect(state.clippedEntries).toBe(1)
      expect(stored.message.length).toBeLessThan(OVERSIZED_ENTRY_CHARS)
      expect(state.bufferBytes).toBeLessThan(ENTRY_CEILING)
      expect(state.bufferBytes).toBe(JSON.stringify(stored).length)
    })

    it('announces clipped lines the same way it announces omissions and evictions', () => {
      const notice = resolveLogcatNotice({
        droppedEntries: 0,
        droppedBytes: 0,
        evictedEntries: 0,
        evictedBytes: 0,
        evictedPinnedEntries: 0,
        clippedEntries: 4,
      })

      expect(notice).toContain('[4] lines clipped')
      expect(notice).toContain('clipped')
    })

    it('evicts on the volume budget, not only on the entry count', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      // 700 entries at ~64 KB serialized is ~45 MB: past the 32 MiB buffer
      // ceiling while the 1000-entry count ceiling is still satisfied.
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 700 }, (_, index) => hugeEntry(`volume-${index}`, LARGE_ENTRY_CHARS)),
      )

      const state = useLogcatStore.getState()

      expect(state.logs.length).toBeGreaterThan(0)
      expect(state.logs.length).toBeLessThan(700)
      expect(state.bufferBytes).toBeLessThanOrEqual(BYTE_CEILING)
      expect(state.evictedEntries).toBeGreaterThan(0)
      expect(state.clippedEntries).toBe(0)
      expect(state.logs[state.logs.length - 1].id).toBe('volume-699')
    })

    it('keeps the newest entries when the count budget is exceeded', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 1200 }, (_, index) => entry(`line-${index}`)),
      )

      const state = useLogcatStore.getState()

      expect(state.logs).toHaveLength(1000)
      expect(state.logs[state.logs.length - 1].id).toBe('line-1199')
      expect(state.bufferFull).toBe(true)
      expect(state.evictedEntries).toBe(200)
      expect(state.evictedBytes).toBeGreaterThan(0)
      expect(state.bufferBytes).toBeGreaterThan(0)
    })

    it('tracks the retained byte volume without re-measuring on every read', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs([entry('line-1'), entry('line-2')])

      const state = useLogcatStore.getState()

      expect(state.bufferBytes).toBe(measuredLogBytes())
    })

    it('does not re-serialize retained entries to enforce the volume on a flush', () => {
      // The worst case the hot path has to survive: a buffer at the 50k maximum
      // ceiling, flushed while nearly full. Flushes arrive at 10 Hz, so a
      // per-flush re-serialization of every retained row is the regression this
      // asserts against.
      useLogcatStore.getState().setBufferLimit(50000)
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 50000 }, (_, index) => entry(`cached-${index}`)),
      )
      expect(useLogcatStore.getState().logs).toHaveLength(50000)

      const serialize = vi.spyOn(JSON, 'stringify')
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 5 }, (_, index) => entry(`fresh-${index}`)),
      )

      // 50000 retained rows are trimmed on this flush. Only the five arriving
      // entries may be measured; a limit of 25 also leaves room for the small
      // amount of surrounding work that does not scale with the buffer size.
      expect(serialize.mock.calls.length).toBeLessThanOrEqual(25)

      const next = useLogcatStore.getState()

      // The count ceiling still holds exactly, and the newest rows are the ones
      // that arrived — the flush was budgeted, not skipped.
      expect(next.logs.length).toBe(50000)
      expect(next.logs[next.logs.length - 1].id).toBe('fresh-4')
      expect(next.logs[0].id).toBe('cached-5')
    })

    it('keeps the cached sizes exact as the volume ceiling evicts entries', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 700 }, (_, index) => hugeEntry(`exact-${index}`, LARGE_ENTRY_CHARS)),
      )

      const state = useLogcatStore.getState()

      // The independently measured volume still matches the cached/accumulated
      // total after hundreds of evictions, in both directions.
      expect(state.bufferBytes).toBe(measuredLogBytes())
      expect(state.bufferBytes).toBeLessThanOrEqual(BYTE_CEILING)
      expect(state.evictedBytes).toBeGreaterThan(0)

      const serialize = vi.spyOn(JSON, 'stringify')
      useLogcatStore.getState().appendLogs([hugeEntry('exact-next', LARGE_ENTRY_CHARS)])

      const next = useLogcatStore.getState()

      expect(serialize.mock.calls.length).toBeLessThanOrEqual(4)
      expect(next.logs).toHaveLength(state.logs.length)
      expect(next.bufferBytes).toBe(measuredLogBytes())
      expect(next.logs[next.logs.length - 1].id).toBe('exact-next')
    })

    it('drops from the ingress queue instead of letting a huge batch pile up', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      // Each of these entries is ~9.8 KB serialized and stays inside the 64 KiB
      // per-entry ceiling. 900 of them is ~8.3 MiB — past the 8 MiB ingress
      // volume, while the 1000-entry queue ceiling is still satisfied.
      const batch = Array.from({ length: 900 }, (_, index) =>
        hugeEntry(`queued-${index}`, 5_000),
      )
      expect(JSON.stringify(batch[0]).length).toBeLessThan(ENTRY_CEILING)

      useLogcatStore.getState().applyBatchEvent(batch)

      const queued = useLogcatStore.getState()

      expect(queued.droppedEntries).toBeGreaterThan(0)
      expect(queued.droppedBytes).toBeGreaterThan(0)
      expect(queued.clippedEntries).toBe(0)
      expect(queued.logs).toHaveLength(0)
    })

    it('counts queued entries discarded by the count ceiling', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().applyBatchEvent(
        Array.from({ length: 1500 }, (_, index) => entry(`queued-${index}`)),
      )

      expect(useLogcatStore.getState().droppedEntries).toBe(500)
      expect(useLogcatStore.getState().logs).toHaveLength(0)
    })

    it('never grows the retained buffer past the count ceiling while streaming', () => {
      useLogcatStore.getState().setBufferLimit(1000)

      for (let index = 0; index < 3000; index += 1) {
        useLogcatStore.getState().appendLogs([entry(`stream-${index}`)])
      }

      const state = useLogcatStore.getState()

      expect(state.logs.length).toBeLessThanOrEqual(1000)
      expect(state.logs.length).toBeGreaterThan(0)
      expect(state.logs[state.logs.length - 1].id).toBe('stream-2999')
    })

    it('resets the retention counters together with the buffer', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 1200 }, (_, index) => entry(`line-${index}`)),
      )
      expect(useLogcatStore.getState().evictedEntries).toBe(200)

      useLogcatStore.getState().clearLogs()

      expect(useLogcatStore.getState().evictedEntries).toBe(0)
      expect(useLogcatStore.getState().evictedBytes).toBe(0)
      expect(useLogcatStore.getState().bufferBytes).toBe(0)
      expect(useLogcatStore.getState().bufferFull).toBe(false)
    })

    it('keeps byte accounting exact after a clear and a fresh stream', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs([entry('before-clear')])
      useLogcatStore.getState().clearLogs()

      useLogcatStore.getState().appendLogs([
        entry('after-clear-1'),
        entry('after-clear-2'),
        entry('after-clear-3'),
      ])

      const state = useLogcatStore.getState()

      expect(state.logs.map((item) => item.id)).toEqual([
        'after-clear-1',
        'after-clear-2',
        'after-clear-3',
      ])
      expect(state.bufferBytes).toBe(measuredLogBytes())
    })

    it('clears counters on demand without touching the buffered entries', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 1100 }, (_, index) => entry(`line-${index}`)),
      )

      useLogcatStore.getState().clearCounters()

      expect(useLogcatStore.getState().evictedEntries).toBe(0)
      expect(useLogcatStore.getState().logs).toHaveLength(1000)
    })

    it('stays silent about retention when nothing was lost', () => {
      expect(
        resolveLogcatNotice({
          droppedEntries: 0,
          droppedBytes: 0,
          evictedEntries: 0,
          evictedBytes: 0,
          evictedPinnedEntries: 0,
          clippedEntries: 0,
        }),
      ).toBeNull()
    })

    it('describes omissions and evictions in plain English', () => {
      const notice = resolveLogcatNotice({
        droppedEntries: 12,
        droppedBytes: 4096,
        evictedEntries: 3,
        evictedBytes: 2048,
        evictedPinnedEntries: 0,
        clippedEntries: 0,
      })

      expect(notice).toContain('[12] lines omitted')
      expect(notice).toContain('[3] lines evicted')
    })
  })

  describe('pin retention', () => {
    it('announces pin eviction instead of silently slicing the list at 100', () => {
      useLogcatStore.getState().setBufferLimit(1000)
      const pins = Array.from({ length: 150 }, (_, index) => entry(`pin-${index}`))

      for (const pin of pins) {
        useLogcatStore.getState().togglePinned(pin)
      }

      const state = useLogcatStore.getState()

      expect(state.pinnedEntries).toHaveLength(100)
      expect(state.pinnedEntries[0].id).toBe('pin-50')
      expect(state.pinnedEntries[99].id).toBe('pin-149')
      expect(state.evictedPinnedEntries).toBe(50)
      expect(state.evictedPinnedBytes).toBeGreaterThan(0)
      expect(
        resolveLogcatNotice({
          droppedEntries: 0,
          droppedBytes: 0,
          evictedEntries: 0,
          evictedBytes: 0,
          evictedPinnedEntries: state.evictedPinnedEntries,
          clippedEntries: 0,
        }),
      ).toContain('[50] pins evicted')
    })

    it('evicts pins on the volume budget instead of only on the count', () => {
      useLogcatStore.getState().setBufferLimit(1000)

      // ~64 KB per pin against a 4 MiB pin volume: 100 pins would be over 6 MiB,
      // so the volume budget must retire pins the 100-entry ceiling would keep.
      const pins = Array.from({ length: 100 }, (_, index) =>
        hugeEntry(`bulk-pin-${index}`, LARGE_ENTRY_CHARS),
      )

      for (const pin of pins) {
        useLogcatStore.getState().togglePinned(pin)
      }

      const state = useLogcatStore.getState()

      expect(state.pinnedEntries.length).toBeLessThan(100)
      expect(state.pinnedBytes).toBeLessThanOrEqual(4 * 1024 * 1024)
      expect(state.evictedPinnedEntries).toBeGreaterThan(0)
      expect(state.pinnedEntries[state.pinnedEntries.length - 1].id).toBe('bulk-pin-99')
    })

    it('keeps pin toggles idempotent and drops the counter when the list shrinks', () => {
      useLogcatStore.getState().togglePinned(entry('pin-a'))
      useLogcatStore.getState().togglePinned(entry('pin-a'))

      expect(useLogcatStore.getState().pinnedEntries).toHaveLength(0)
      expect(useLogcatStore.getState().pinnedBytes).toBe(0)
      expect(useLogcatStore.getState().evictedPinnedEntries).toBe(0)
    })
  })

  describe('export scopes', () => {
    it('exports only the retained buffer in the default All scope', () => {
      useLogcatStore.getState().appendLogs([entry('line-1'), entry('line-2')])

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'text')

      expect(resolution.export?.entries).toHaveLength(2)
      expect(resolution.export?.content).toBe(`${state.logs[0].raw}\n${state.logs[1].raw}`)
      expect(resolution.export?.filename).toMatch(/^logcat-device-1-\d+\.txt$/)
    })

    it('exports only filter matches in the Filtered scope', () => {
      useLogcatStore.getState().appendLogs([
        entry('info-1', 'hello', 'I'),
        entry('error-1', 'fatal exception', 'E'),
        entry('info-2', 'again', 'I'),
      ])
      useLogcatStore.getState().setFilter({ levels: ['E', 'F'] })
      useLogcatStore.getState().setExportMode('filtered')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'json')

      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['error-1'])
    })

    it('refuses a filtered export when nothing matches', () => {
      useLogcatStore.getState().appendLogs([entry('info-1')])
      useLogcatStore.getState().setFilter({ tag: 'NoSuchTag' })
      useLogcatStore.getState().setExportMode('filtered')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'text')

      expect(resolution.export).toBeUndefined()
      expect(resolution.error).toBe('No logs match the current filter')
    })

    it('exports the full retained buffer in All even while the view shows pins', () => {
      const pinned = entry('pinned-1', 'kept pin', 'E')
      useLogcatStore.getState().appendLogs([
        entry('live-1', 'live one'),
        entry('live-2', 'live two'),
        pinned,
      ])
      useLogcatStore.getState().togglePinned(pinned)
      useLogcatStore.getState().setPinnedOnly(true)
      // All is an explicit buffer-wide scope; Filtered below follows the view.
      useLogcatStore.getState().setExportMode('all')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'text')

      expect(state.logs).toHaveLength(3)
      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['live-1', 'live-2', 'pinned-1'])
      expect(resolution.export?.content).toContain('live one')
      expect(resolution.export?.content).toContain(pinned.raw)
    })

    it('exports only the pinned view in Filtered even with the default filter', () => {
      const pinned = entry('pin-default')
      useLogcatStore.getState().appendLogs([entry('live-default'), pinned])
      useLogcatStore.getState().togglePinned(pinned)
      useLogcatStore.getState().setPinnedOnly(true)
      useLogcatStore.getState().setExportMode('filtered')
      const resolution = resolveLogcatExport(useLogcatStore.getState(), 'text')
      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['pin-default'])
    })

    it('applies the active filter to pins in the Filtered scope while pinned-only is on', () => {
      const pinnedInfo = entry('pin-info', 'routine tick', 'I')
      const pinnedError = entry('pin-error', 'crashed hard', 'E')
      useLogcatStore.getState().appendLogs([pinnedInfo, pinnedError, entry('live-error', 'live crash', 'E')])

      useLogcatStore.getState().togglePinned(pinnedInfo)
      useLogcatStore.getState().togglePinned(pinnedError)
      useLogcatStore.getState().setPinnedOnly(true)
      useLogcatStore.getState().setFilter({ levels: ['E', 'F'] })
      useLogcatStore.getState().setExportMode('filtered')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'json')

      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['pin-error'])
    })

    it('still honours an active filter over the live buffer when pinned-only is off', () => {
      useLogcatStore.getState().appendLogs([
        entry('live-info', 'routine tick', 'I'),
        entry('live-error', 'crashed hard', 'E'),
      ])
      useLogcatStore.getState().setFilter({ pid: '123' })
      useLogcatStore.getState().setPinnedOnly(false)
      useLogcatStore.getState().setExportMode('filtered')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'json')

      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['live-info', 'live-error'])
    })

    it('exports pinned events after the rolling buffer was cleared', () => {
      const pinned = entry('pinned-1', 'kept after clear', 'E')
      useLogcatStore.getState().appendLogs([entry('line-1'), pinned])
      useLogcatStore.getState().togglePinned(pinned)
      useLogcatStore.getState().clearLogs()
      useLogcatStore.getState().setExportMode('pinned')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: '' }, 'text')

      expect(state.logs).toHaveLength(0)
      expect(resolution.export?.entries).toEqual([pinned])
      expect(resolution.export?.content).toBe(pinned.raw)
      expect(resolution.export?.filename).toMatch(/^logcat-export-\d+\.txt$/)
    })

    it('exports every pin in the explicit Pinned scope as its label declares', () => {
      const pinnedInfo = entry('pin-info', 'routine tick', 'I')
      const pinnedError = entry('pin-error', 'crashed hard', 'E')
      useLogcatStore.getState().appendLogs([pinnedInfo, pinnedError])
      useLogcatStore.getState().togglePinned(pinnedInfo)
      useLogcatStore.getState().togglePinned(pinnedError)
      useLogcatStore.getState().setFilter({ levels: ['E', 'F'] })
      useLogcatStore.getState().setPinnedOnly(true)
      useLogcatStore.getState().setExportMode('pinned')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: 'device-1' }, 'json')

      expect(resolution.export?.entries.map((item) => item.id)).toEqual(['pin-info', 'pin-error'])
    })

    it('reports an empty pinned scope instead of exporting an empty file', () => {
      useLogcatStore.getState().setExportMode('pinned')

      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: '' }, 'text')

      expect(resolution.export).toBeUndefined()
      expect(resolution.error).toBe('No pinned events to export')
    })

    it('reports an empty All scope with the message the shortcut path expects', () => {
      const state = useLogcatStore.getState()
      const resolution = resolveLogcatExport({ ...state, streamingSerial: '' }, 'text')

      expect(resolution.export).toBeUndefined()
      expect(resolution.error).toBe('No logs to export')
    })
  })
})

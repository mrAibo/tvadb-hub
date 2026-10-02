import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, type RefObject } from 'react'
import { useVirtualizer, type ReactVirtualizer } from '@tanstack/react-virtual'
import { resolveLogcatFilteredLogs, useLogcatStore } from '@/stores/useLogcatStore'
import { LogcatEntry as LogcatEntryRow } from './LogcatEntry'

const ROW_HEIGHT = 24
const AUTO_SCROLL_THROTTLE_MS = 200

interface LogcatViewProps {
  scrollContainerRef: RefObject<HTMLDivElement | null>
}

export function LogcatView({ scrollContainerRef }: LogcatViewProps) {
  const logs = useLogcatStore((state) => state.logs)
  const pinnedEntries = useLogcatStore((state) => state.pinnedEntries)
  const pinnedOnly = useLogcatStore((state) => state.pinnedOnly)
  const togglePinned = useLogcatStore((state) => state.togglePinned)
  const filter = useLogcatStore((state) => state.filter)
  const autoScroll = useLogcatStore((state) => state.autoScroll)
  const virtualizerRef = useRef<ReactVirtualizer<HTMLDivElement, Element> | null>(null)
  const newestIndexRef = useRef(0)
  const autoScrollRef = useRef(autoScroll)
  const scrollTimerRef = useRef<number | null>(null)
  const scrollPendingRef = useRef(false)
  /** Newest row the last real scroll landed on, so idle windows can stay idle. */
  const scrolledNewestRef = useRef<string | undefined>(undefined)

  const filteredLogs = useMemo(
    () => resolveLogcatFilteredLogs(logs, pinnedEntries, pinnedOnly, filter),
    [logs, pinnedEntries, pinnedOnly, filter],
  )

  // The newest id participates in the auto-scroll dependency below on purpose:
  // a rolling replacement over a full buffer keeps the same length while the
  // tail changes, and that is exactly when a scroll is still required.
  const newestEntryId = filteredLogs[filteredLogs.length - 1]?.id
  const newestIdRef = useRef(newestEntryId)

  const pinnedIds = useMemo(
    () => new Set(pinnedEntries.map((entry) => entry.id)),
    [pinnedEntries],
  )

  const virtualizer = useVirtualizer({
    count: filteredLogs.length,
    getScrollElement: () => scrollContainerRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  })

  // The virtualizer instance and the newest row index are only read from the
  // scheduler, never captured by its dependency list, so re-renders cannot
  // restart the throttle window.
  virtualizerRef.current = virtualizer
  newestIndexRef.current = filteredLogs.length - 1
  newestIdRef.current = newestEntryId
  autoScrollRef.current = autoScroll

  const cancelPendingScroll = useCallback(() => {
    if (scrollTimerRef.current === null) {
      return
    }

    clearTimeout(scrollTimerRef.current)
    scrollTimerRef.current = null
    scrollPendingRef.current = false
  }, [])

  /** Reads the newest row at call time, so a stale closure can never scroll short. */
  const scrollToNewest = useCallback(() => {
    virtualizerRef.current?.scrollToIndex(newestIndexRef.current, { align: 'end' })
  }, [])

  /**
   * Scrolls to the newest row at most once per throttle window, plus one final
   * scroll when the window closes over rows that arrived inside it.
   *
   * A row that arrives inside a window only marks the pending flag: it neither
   * scrolls nor restarts the window. When the window expires, the pending flag
   * and a newest row that has moved past the one this scheduler last showed
   * produce one trailing scroll, so a burst that ends mid-window still lands on
   * the true tail. Every scroll re-reads the index and id refs at the moment it
   * runs, which is what makes a rolling replacement — same length, new tail —
   * land correctly.
   *
   * A window that closes with nothing newer than the last scroll stays shut, so
   * it cannot scroll on its own or hold the next update behind an empty
   * interval. Unmount clears the timer and the flag, and switching auto-scroll
   * off releases a window that is still open.
   *
   * The previous implementation set a boolean before scheduling its timer and
   * cancelled that timer in cleanup without ever clearing the boolean, so the
   * next effect run saw "throttled" and returned before scheduling anything —
   * auto-scroll stopped permanently after the first rapid update.
   */
  const scheduleAutoScroll = useCallback(() => {
    if (!autoScrollRef.current) {
      return
    }

    if (scrollTimerRef.current !== null) {
      scrollPendingRef.current = true
      return
    }

    scrollToNewest()
    scrolledNewestRef.current = newestIdRef.current
    scrollPendingRef.current = false
    scrollTimerRef.current = window.setTimeout(() => {
      scrollTimerRef.current = null

      if (!scrollPendingRef.current || !autoScrollRef.current) {
        scrollPendingRef.current = false
        return
      }

      scrollPendingRef.current = false

      if (newestIdRef.current !== scrolledNewestRef.current) {
        // Trailing edge: rows arrived inside the window that no scroll has shown
        // yet, so show the true tail. This is a real scroll, and it opens the
        // next window as any other scroll does.
        scheduleAutoScroll()
      }
      // Nothing newer arrived: the window closes and the next update scrolls at
      // once, rather than being held behind an interval with nothing to deliver.
    }, AUTO_SCROLL_THROTTLE_MS)
  }, [scrollToNewest])

  // Unmount releases the timer. Auto-scroll switching off is handled by its own
  // effect below, so a data-driven render neither restarts nor cancels the
  // throttle window that is already open.
  useEffect(() => cancelPendingScroll, [cancelPendingScroll])

  useLayoutEffect(() => {
    if (!autoScroll) {
      // Switching auto-scroll off drops a window that is in flight, so the
      // trailing scroll can never reach the container after the switch flipped.
      cancelPendingScroll()
      return
    }

    if (filteredLogs.length === 0) {
      return
    }

    scheduleAutoScroll()
  }, [autoScroll, filteredLogs.length, newestEntryId, scheduleAutoScroll, cancelPendingScroll])

  if (filteredLogs.length === 0 && pinnedOnly && pinnedEntries.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <div className="text-center">
          <p className="text-sm font-medium">No pinned events</p>
          <p className="text-xs mt-1">Pin an event from the live log to keep it here</p>
        </div>
      </div>
    )
  }

  if (!pinnedOnly && logs.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <div className="text-center">
          <p className="text-sm font-medium">No logcat output</p>
          <p className="text-xs mt-1">Start a logcat stream to see device logs here</p>
        </div>
      </div>
    )
  }

  if (filteredLogs.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <div className="text-center">
          <p className="text-sm font-medium">No matching logs</p>
          <p className="text-xs mt-1">Try adjusting your filter criteria</p>
        </div>
      </div>
    )
  }

  return (
    <div
      style={{
        height: `${virtualizer.getTotalSize()}px`,
        width: '100%',
        position: 'relative',
      }}
    >
      {virtualizer.getVirtualItems().map((virtualRow) => {
        const entry = filteredLogs[virtualRow.index]
        if (!entry) return null

        return (
          <div
            key={entry.id}
            data-index={virtualRow.index}
            ref={virtualizer.measureElement}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translate3d(0, ${virtualRow.start}px, 0)`,
            }}
          >
            <LogcatEntryRow
              entry={entry}
              pinned={pinnedIds.has(entry.id)}
              onTogglePin={togglePinned}
            />
          </div>
        )
      })}
    </div>
  )
}

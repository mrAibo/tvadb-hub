import { act, render } from '@testing-library/react'
import { useRef } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LogcatEntry } from '@/lib/types'
import { useLogcatStore } from '@/stores/useLogcatStore'
import { LogcatView } from '../LogcatView'

const virtualizer = vi.hoisted(() => ({
  scrollToIndex: vi.fn(),
  options: { count: 0 },
}))

vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: (options: { count: number }) => {
    virtualizer.options = options
    return {
      options,
      getTotalSize: () => options.count * 24,
      getVirtualItems: () =>
        Array.from({ length: Math.min(options.count, 12) }, (_, index) => ({
          index,
          key: index,
          start: index * 24,
          size: 24,
        })),
      measureElement: () => 24,
      scrollToIndex: virtualizer.scrollToIndex,
    }
  },
}))

const THROTTLE_MS = 200

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
    message: `line ${id}`,
    raw: `10-01 12:00:00.000 123 123 I Example: line ${id}`,
    timestamp: '10-01 12:00:00.000',
  }
}

function Harness() {
  const containerRef = useRef<HTMLDivElement | null>(null)

  return (
    <div ref={containerRef} style={{ height: 300, overflow: 'auto' }}>
      <LogcatView scrollContainerRef={containerRef} />
    </div>
  )
}

function append(count: number, from = 0) {
  act(() => {
    useLogcatStore.getState().appendLogs(
      Array.from({ length: count }, (_, index) => entry(`line-${from + index}`)),
    )
  })
}

function advance(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms)
  })
}

describe('LogcatView auto-scroll throttling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    virtualizer.scrollToIndex.mockClear()
    virtualizer.options = { count: 0 }
    useLogcatStore.getState().reset()
    useLogcatStore.getState().setBufferLimit(1000)
    useLogcatStore.getState().setAutoScroll(true)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('scrolls to the newest row as soon as entries arrive', () => {
    render(<Harness />)
    expect(virtualizer.scrollToIndex).not.toHaveBeenCalled()

    append(1)

    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledWith(0, { align: 'end' })
  })

  it('resumes auto-scroll after the throttle window instead of latching off', () => {
    render(<Harness />)
    append(1)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)

    // A second update inside the throttle window is coalesced, not lost.
    append(1, 1)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)
    append(1, 2)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)

    // Regression: the old boolean throttle stayed true after its timer was
    // cancelled in cleanup, so every later update returned early forever.
    // Regression: the old boolean throttle stayed true after its timer was
    // cancelled in cleanup, so every later update returned early forever.
    advance(THROTTLE_MS)
    // The window trails once to the row that arrived inside it.
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(2)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(2, { align: 'end' })

    // The next update lands in the next window and is delivered by its trailing
    // edge, so no update is ever silently dropped by the throttle.
    append(1, 3)
    advance(THROTTLE_MS)

    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(3)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(3, { align: 'end' })
  })

  it('still scrolls when a rolling buffer replaces lines without changing length', () => {
    render(<Harness />)
    append(20)
    advance(THROTTLE_MS)

    const before = virtualizer.options.count
    const scrollsBefore = virtualizer.scrollToIndex.mock.calls.length

    act(() => {
      useLogcatStore.getState().clearLogs()
      // Replace the whole tail: the buffer length stays put while the newest id
      // moves, which is exactly what a rolling replacement looks like.
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 20 }, (_, index) => entry(`tail-${index}`)),
      )
    })

    expect(virtualizer.options.count).toBe(before)
    expect(virtualizer.scrollToIndex.mock.calls.length).toBeGreaterThan(scrollsBefore)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(before - 1, { align: 'end' })
  })

  it('scrolls a burst that ends inside the throttle window, not just its first line', () => {
    render(<Harness />)

    // The burst opens the window with line 0 and then extends the tail twice
    // while the window is still open.
    append(1)
    append(1, 1)
    append(1, 2)

    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(0, { align: 'end' })

    // Before the trailing scroll lands the view is stranded two lines behind.
    advance(THROTTLE_MS)

    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(2)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(2, { align: 'end' })

    // A window that closes with no update schedules nothing further.
    advance(THROTTLE_MS)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(2)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('trails to the newest row when a rolling buffer replaces the tail at the same length', () => {
    render(<Harness />)
    append(20)
    advance(THROTTLE_MS)

    const scrollsBefore = virtualizer.scrollToIndex.mock.calls.length

    // Open a window on a fresh buffer, then extend the tail inside it. The
    // replacement keeps the count growing while the tail ids change, so the
    // trailing scroll has to read the newest index at fire time rather than the
    // index that opened the window.
    act(() => {
      useLogcatStore.getState().clearLogs()
      useLogcatStore.getState().appendLogs(
        Array.from({ length: 20 }, (_, index) => entry(`tail-${index}`)),
      )
    })
    append(10, 20)

    expect(virtualizer.scrollToIndex.mock.calls.length).toBe(scrollsBefore + 1)

    advance(THROTTLE_MS)

    expect(virtualizer.scrollToIndex.mock.calls.length).toBe(scrollsBefore + 2)
    expect(virtualizer.scrollToIndex).toHaveBeenLastCalledWith(29, { align: 'end' })
  })

  it('does not scroll while auto-scroll is switched off', () => {
    useLogcatStore.getState().setAutoScroll(false)
    render(<Harness />)

    append(1)
    advance(THROTTLE_MS)
    append(1, 1)

    expect(virtualizer.scrollToIndex).not.toHaveBeenCalled()
  })

  it('drops a pending trailing scroll when auto-scroll is switched off mid-window', () => {
    render(<Harness />)

    append(1)
    append(1, 1)

    act(() => {
      useLogcatStore.getState().setAutoScroll(false)
    })

    advance(THROTTLE_MS * 3)

    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('leaves the timer cleared on unmount', () => {
    const view = render(<Harness />)
    append(1)

    view.unmount()

    expect(vi.getTimerCount()).toBe(0)
  })

  it('leaves no trailing timer behind when a burst is unmounted mid-window', () => {
    const view = render(<Harness />)

    append(1)
    append(1, 1)

    view.unmount()

    expect(vi.getTimerCount()).toBe(0)
    advance(THROTTLE_MS * 3)
    expect(virtualizer.scrollToIndex).toHaveBeenCalledTimes(1)
  })
})

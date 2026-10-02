import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LogcatEntry } from '@/lib/types'
import { useLogcatStore } from '@/stores/useLogcatStore'

const mocks = vi.hoisted(() => ({
  startLogcat: vi.fn().mockResolvedValue(undefined),
  stopLogcat: vi.fn().mockResolvedValue(undefined),
  saveLogcatToFile: vi.fn().mockResolvedValue(undefined),
  onLogcatBatch: vi.fn(() => () => {}),
  onLogcatStatus: vi.fn(() => () => {}),
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

vi.mock('@/services/logcatService', () => ({
  startLogcat: mocks.startLogcat,
  stopLogcat: mocks.stopLogcat,
  saveLogcatToFile: mocks.saveLogcatToFile,
  onLogcatBatch: mocks.onLogcatBatch,
  onLogcatStatus: mocks.onLogcatStatus,
}))

vi.mock('sonner', () => ({ toast: mocks.toast }))

import { useLogcat } from '../useLogcat'

function entry(raw: string): LogcatEntry {
  return {
    id: raw,
    serial: 'SERIAL-1',
    date: '01-01',
    time: '00:00:00.000',
    pid: '1',
    tid: '1',
    level: 'I',
    tag: 'Test',
    message: raw,
    raw,
    timestamp: '0',
  }
}

function pressKey(init: KeyboardEventInit) {
  act(() => {
    window.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, ...init }))
  })
}

describe('useLogcat keyboard ownership', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window.navigator, 'platform', { value: 'Win32', configurable: true })
    useLogcatStore.getState().reset()
    useLogcatStore.getState().setExportMode('all')
  })

  it('leaves the buffer untouched when the unshifted palette chord is pressed', () => {
    useLogcatStore.getState().appendLogs([entry('line-1'), entry('line-2')])
    renderHook(() => useLogcat())

    pressKey({ key: 'k', ctrlKey: true })

    expect(useLogcatStore.getState().logs).toHaveLength(2)
    expect(mocks.toast.info).not.toHaveBeenCalled()
  })

  it('clears the buffer on the separate shifted chord', () => {
    useLogcatStore.getState().appendLogs([entry('line-1'), entry('line-2')])
    renderHook(() => useLogcat())

    pressKey({ key: 'K', ctrlKey: true, shiftKey: true })

    expect(useLogcatStore.getState().logs).toHaveLength(0)
    expect(mocks.toast.info).toHaveBeenCalledWith('Logcat cleared')
  })

  it('ignores the chord entirely when no platform modifier is held', () => {
    useLogcatStore.getState().appendLogs([entry('line-1')])
    renderHook(() => useLogcat())

    pressKey({ key: 'k', shiftKey: true })

    expect(useLogcatStore.getState().logs).toHaveLength(1)
  })

  it('exports json for the shifted export chord only once', async () => {
    useLogcatStore.getState().appendLogs([entry('line-1')])
    renderHook(() => useLogcat())

    pressKey({ key: 'E', ctrlKey: true, shiftKey: true })
    await act(async () => {})

    expect(mocks.saveLogcatToFile).toHaveBeenCalledTimes(1)
    expect(mocks.saveLogcatToFile.mock.calls[0][1]).toMatch(/\.json$/)
  })

  it('keeps the plain export chord on text output', async () => {
    useLogcatStore.getState().appendLogs([entry('line-1')])
    renderHook(() => useLogcat())

    pressKey({ key: 'e', ctrlKey: true })
    await act(async () => {})

    expect(mocks.saveLogcatToFile).toHaveBeenCalledTimes(1)
    expect(mocks.saveLogcatToFile.mock.calls[0][1]).toMatch(/\.txt$/)
  })

  it('exports pinned events over the chord after the buffer was cleared', async () => {
    useLogcatStore.getState().appendLogs([entry('pinned-line')])
    const pinned = useLogcatStore.getState().logs[0]
    useLogcatStore.getState().togglePinned(pinned)
    useLogcatStore.getState().clearLogs()
    useLogcatStore.getState().setExportMode('pinned')
    renderHook(() => useLogcat())

    pressKey({ key: 'E', ctrlKey: true, shiftKey: true })
    await act(async () => {})

    expect(mocks.saveLogcatToFile).toHaveBeenCalledTimes(1)
    expect(mocks.saveLogcatToFile.mock.calls[0][1]).toMatch(/\.json$/)
    expect(mocks.saveLogcatToFile.mock.calls[0][0]).toContain(pinned.raw)
    expect(mocks.toast.success).toHaveBeenCalledWith('Exported 1 log entries')
  })

  it('refuses the chord when the selected export scope is empty', async () => {
    renderHook(() => useLogcat())

    pressKey({ key: 'e', ctrlKey: true })
    await act(async () => {})

    expect(mocks.saveLogcatToFile).not.toHaveBeenCalled()
    expect(mocks.toast.error).toHaveBeenCalledWith('No logs to export')
  })
})

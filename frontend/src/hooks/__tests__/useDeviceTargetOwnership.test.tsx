import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { useFileExplorerStore } from '@/stores/useFileExplorerStore'
import { useMonitor } from '../useMonitor'
import { useFileExplorer } from '../useFileExplorer'

const fileMocks = vi.hoisted(() => ({
  listFiles: vi.fn().mockResolvedValue([]),
  getDirectorySize: vi.fn().mockResolvedValue('0 B'),
  pullFile: vi.fn().mockResolvedValue('OK'),
  pullMultipleFilesDetailed: vi.fn(),
  pushFile: vi.fn().mockResolvedValue('OK'),
  pushMultipleFilesDetailed: vi.fn(),
  deleteMultipleFiles: vi.fn().mockResolvedValue('OK'),
  createDirectory: vi.fn().mockResolvedValue('OK'),
  renameFile: vi.fn().mockResolvedValue('OK'),
  selectFile: vi.fn().mockResolvedValue(''),
  selectSaveFile: vi.fn().mockResolvedValue(''),
  selectMultipleFiles: vi.fn().mockResolvedValue([]),
  selectDirectory: vi.fn().mockResolvedValue(''),
  onFileTransferProgress: vi.fn(() => () => {}),
  cancelFileTransfer: vi.fn(),
}))

const deviceMocks = vi.hoisted(() => ({
  getPerformanceSnapshot: vi.fn(),
}))

vi.mock('@/services/fileService', () => fileMocks)
vi.mock('@/services/deviceService', () => deviceMocks)
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    promise: vi.fn(),
  },
}))

import { toast } from 'sonner'

const readyDevice = (serial: string) => ({ serial, mode: 'adb', state: 'device' }) as const

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('device-bound file listing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useFileExplorerStore.getState().reset()
    useDeviceStore.setState({ devices: [readyDevice('A')], activeSerial: 'A' })
    fileMocks.listFiles.mockResolvedValue([
      { name: 'x.bin', path: '/sdcard/x.bin', type: 'file' },
    ])
  })

  it('queries the confirmed serial and caches under the serial tuple', async () => {
    renderHook(() => useFileExplorer())

    await waitFor(() => expect(useFileExplorerStore.getState().listingSerial).toBe('A'))

    expect(fileMocks.listFiles).toHaveBeenCalledWith('A', '/sdcard', false)
    expect(Object.keys(useFileExplorerStore.getState().fileCache)).toEqual([
      JSON.stringify(['A', '/sdcard', false]),
    ])
  })

  it('refuses a stale action after the confirmed device changes', async () => {
    const { result } = renderHook(() => useFileExplorer())
    await waitFor(() => expect(useFileExplorerStore.getState().listingSerial).toBe('A'))

    fileMocks.listFiles.mockImplementation(() => new Promise(() => {}))
    act(() => {
      useDeviceStore.setState({ devices: [readyDevice('B')], activeSerial: 'B' })
    })

    await act(async () => {
      await result.current.removeFile('/sdcard/x.bin')
    })

    expect(fileMocks.deleteMultipleFiles).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('device changed'))
  })

  it('never serves one device cached listing for another device', async () => {
    renderHook(() => useFileExplorer())
    await waitFor(() => expect(useFileExplorerStore.getState().listingSerial).toBe('A'))

    act(() => {
      useDeviceStore.setState({ devices: [readyDevice('B')], activeSerial: 'B' })
    })
    await waitFor(() => expect(fileMocks.listFiles).toHaveBeenCalledWith('B', '/sdcard', false))

    const keys = Object.keys(useFileExplorerStore.getState().fileCache)
    expect(keys).toContain(JSON.stringify(['A', '/sdcard', false]))
    expect(keys).toContain(JSON.stringify(['B', '/sdcard', false]))
  })
})

describe('monitor stale reply handling', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useDeviceStore.setState({ activeSerial: 'A' })
  })

  it('drops a reply that arrives after the confirmed device changed', async () => {
    const gate = deferred<unknown>()
    deviceMocks.getPerformanceSnapshot.mockReturnValueOnce(gate.promise)

    renderHook(() => useMonitor('A', true))

    act(() => {
      useDeviceStore.setState({ activeSerial: 'B' })
    })

    await act(async () => {
      gate.resolve({ serial: 'A', cpuUsage: 10, ramUsage: 20, networkRxSec: 30 })
    })

    expect(useDeviceStore.getState().performance).toBeNull()
  })

  it('applies a reply for the serial it was requested for', async () => {
    deviceMocks.getPerformanceSnapshot.mockResolvedValue({
      serial: 'A',
      cpuUsage: 10,
      ramUsage: 20,
      networkRxSec: 30,
    })

    renderHook(() => useMonitor('A', true))

    await waitFor(() =>
      expect(useDeviceStore.getState().performance).toMatchObject({ cpuUsage: 10 }),
    )
    expect(deviceMocks.getPerformanceSnapshot).toHaveBeenCalledWith('A')
  })
})

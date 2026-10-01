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

// The generated bindings, so the real service wrappers can be exercised directly
// instead of through the hook-level module mock.
const bindingMocks = vi.hoisted(() => ({
  ListFilesForDevice: vi.fn(),
  GetDirectorySizeForDevice: vi.fn(),
  PullFileForDevice: vi.fn(),
  PushFileForDevice: vi.fn(),
  DeleteFileForDevice: vi.fn(),
  DeleteMultipleFilesForDevice: vi.fn(),
  CreateDirectoryForDevice: vi.fn(),
  RenameFileForDevice: vi.fn(),
  SelectFile: vi.fn(),
  SelectSavePath: vi.fn(),
  SelectDirectory: vi.fn(),
  SelectMultipleFiles: vi.fn(),
  CancelFileTransfer: vi.fn(),
  GetStorageInfoForDevice: vi.fn(),
  ListSdCardsForDevice: vi.fn(),
  UnblockPathForDevice: vi.fn(),
}))

vi.mock('../../../../bindings/ADBKit/internal/app/app', () => bindingMocks)

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
    const firstCache = useFileExplorerStore.getState().fileCache
    expect(Object.keys(firstCache)).toEqual([JSON.stringify(['A', '/sdcard', false])])

    act(() => {
      useDeviceStore.setState({ devices: [readyDevice('B')], activeSerial: 'B' })
    })
    await waitFor(() => expect(useFileExplorerStore.getState().listingSerial).toBe('B'))

    // The switch drops the machine-bound cache instead of reusing A's entry, and the
    // new listing is stored under B's own tuple.
    const secondCache = useFileExplorerStore.getState().fileCache
    expect(Object.keys(secondCache)).toEqual([JSON.stringify(['B', '/sdcard', false])])
    expect(secondCache[JSON.stringify(['A', '/sdcard', false])]).toBeUndefined()
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

describe('confirmed-device file queries', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useDeviceStore.setState({ devices: [readyDevice('tv-A')], activeSerial: 'tv-A' })
  })

  it('binds storage info, SD-card listing and unblock guidance to the captured serial', async () => {
    const real = await vi.importActual<typeof import('@/services/fileService')>(
      '@/services/fileService',
    )
    bindingMocks.GetStorageInfoForDevice.mockResolvedValue({ totalHuman: '64 GB' })
    bindingMocks.ListSdCardsForDevice.mockResolvedValue([
      { id: 'vol-1', description: 'SD card', mountPoint: '/storage/1234-5678', isExternal: true },
    ])
    bindingMocks.UnblockPathForDevice.mockResolvedValue({ guidance: 'grant access' })

    await real.getStorageInfoForDevice('tv-A')
    expect(bindingMocks.GetStorageInfoForDevice).toHaveBeenCalledTimes(1)
    expect(bindingMocks.GetStorageInfoForDevice).toHaveBeenCalledWith('tv-A')

    const cards = await real.listSdCardsForDevice('tv-A')
    expect(bindingMocks.ListSdCardsForDevice).toHaveBeenCalledTimes(1)
    expect(bindingMocks.ListSdCardsForDevice).toHaveBeenCalledWith('tv-A')
    expect(cards).toHaveLength(1)

    await real.unblockPathForDevice('tv-A', '/sdcard/blocked')
    expect(bindingMocks.UnblockPathForDevice).toHaveBeenCalledTimes(1)
    expect(bindingMocks.UnblockPathForDevice).toHaveBeenCalledWith('tv-A', '/sdcard/blocked')

    // The serial-less legacy wrappers are gone, so no caller can silently fall back.
    const legacy = real as unknown as Record<string, unknown>
    expect(legacy.getStorageInfo).toBeUndefined()
    expect(legacy.listSdCards).toBeUndefined()
    expect(legacy.unblockPath).toBeUndefined()
  })

  it('refuses a blank target without reaching the backend', async () => {
    const real = await vi.importActual<typeof import('@/services/fileService')>(
      '@/services/fileService',
    )

    await expect(real.getStorageInfoForDevice('   ')).rejects.toThrow('get_storage_info')
    await expect(real.listSdCardsForDevice('')).rejects.toThrow('list_sd_cards')
    await expect(real.unblockPathForDevice('', '/sdcard/blocked')).rejects.toThrow('unblock_path')

    expect(bindingMocks.GetStorageInfoForDevice).not.toHaveBeenCalled()
    expect(bindingMocks.ListSdCardsForDevice).not.toHaveBeenCalled()
    expect(bindingMocks.UnblockPathForDevice).not.toHaveBeenCalled()
  })

  it('normalises an empty SD-card answer to an empty list', async () => {
    const real = await vi.importActual<typeof import('@/services/fileService')>(
      '@/services/fileService',
    )
    bindingMocks.ListSdCardsForDevice.mockResolvedValue(null)

    await expect(real.listSdCardsForDevice('tv-A')).resolves.toEqual([])
  })
})

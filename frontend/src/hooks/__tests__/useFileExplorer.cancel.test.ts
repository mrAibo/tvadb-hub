import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { toast } from 'sonner'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { useFileExplorerStore } from '@/stores/useFileExplorerStore'
import { useFileExplorer } from '../useFileExplorer'

const mocks = vi.hoisted(() => ({
  listFiles: vi.fn().mockResolvedValue([]),
  getDirectorySize: vi.fn().mockResolvedValue('0 B'),
  pullFile: vi.fn().mockResolvedValue('OK'),
  pullMultipleFiles: vi.fn().mockResolvedValue('OK'),
  pullMultipleFilesDetailed: vi.fn(),
  pushFile: vi.fn().mockResolvedValue('OK'),
  pushMultipleFiles: vi.fn().mockResolvedValue('OK'),
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
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    promise: vi.fn(),
  },
}))

vi.mock('@/services/fileService', () => mocks)
vi.mock('sonner', () => ({ toast: mocks.toast }))

describe('useFileExplorer transfer cancellation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useDeviceStore.getState().setActiveSerial('test-device')
    useFileExplorerStore.getState().reset()
  })

  it('reports a cancelled batch push without a success toast', async () => {
    mocks.pushMultipleFilesDetailed.mockRejectedValueOnce(new Error('push_file: Push canceled by user'))
    const { result } = renderHook(() => useFileExplorer())

    let completed: boolean | undefined
    await act(async () => {
      completed = await result.current.pushMultipleToCurrentDir(['C:\\temp\\file.bin'])
    })

    expect(completed).toBe(false)
    expect(toast.info).toHaveBeenCalledWith(expect.stringContaining('Push batch cancelled'))
    expect(toast.success).not.toHaveBeenCalled()
    expect(result.current.transferProgress).toBeNull()
    expect(result.current.busyBatchAction).toBeNull()
  })

  it('keeps transfer progress visible until the cancelled operation settles', () => {
    const { result } = renderHook(() => useFileExplorer())

    act(() => {
      useFileExplorerStore.getState().setTransferProgress({
        fileName: 'file.bin',
        direction: 'push',
        percent: 42,
        active: true,
      })
      result.current.cancelTransfer()
    })

    expect(mocks.cancelFileTransfer).toHaveBeenCalledOnce()
    expect(result.current.transferProgress).toMatchObject({ fileName: 'file.bin', percent: 42, active: true })
  })

  it('uses the native save picker for a single-file export', async () => {
    mocks.selectSaveFile.mockResolvedValueOnce('/tmp/file.bin')
    const { result } = renderHook(() => useFileExplorer())

    let selectedPath = ''
    await act(async () => {
      selectedPath = await result.current.chooseLocalSaveFile('file.bin')
    })

    expect(selectedPath).toBe('/tmp/file.bin')
    expect(mocks.selectSaveFile).toHaveBeenCalledWith('file.bin')
  })

  it('reports actual partial counts and never claims every input succeeded', async () => {
    mocks.pushMultipleFilesDetailed.mockResolvedValueOnce({ operationId: 'op1', serial: 'test-device', completed: 1, failed: 1, cancelled: 0, skipped: 0, items: [
      { source: '/tmp/ok', destination: '/sdcard/ok', status: 'success', message: 'OK' },
      { source: '/tmp/fail', destination: '/sdcard/fail', status: 'failed', message: 'permission denied' },
    ] })
    const { result } = renderHook(() => useFileExplorer())
    await act(async () => { expect(await result.current.pushMultipleToCurrentDir(['/tmp/ok', '/tmp/fail'])).toBe(false) })
    expect(mocks.pushMultipleFilesDetailed).toHaveBeenCalledWith('test-device', ['/tmp/ok', '/tmp/fail'], '/sdcard')
    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('1 completed, 1 failed'))
    expect(result.current.lastTransferBatch?.failed).toBe(1)
  })

  it('rejects a second UI operation without replacing the first progress', async () => {
    const { result } = renderHook(() => useFileExplorer())
    await act(() => useFileExplorerStore.getState().setTransferProgress({ fileName: 'active.bin', direction: 'push', percent: 40, active: true, operationId: 'active' }))
    await act(async () => { expect(await result.current.pushMultipleToCurrentDir(['/tmp/other'])).toBe(false) })
    expect(mocks.pushMultipleFilesDetailed).not.toHaveBeenCalled()
    expect(result.current.transferProgress?.operationId).toBe('active')
    await act(() => result.current.cancelTransfer())
    expect(mocks.cancelFileTransfer).toHaveBeenCalledWith('active')
  })

  it('preserves a newer device selection when an old pull finishes', async () => {
    let resolve!: (value: unknown) => void
    mocks.pullMultipleFilesDetailed.mockReturnValueOnce(new Promise(r => { resolve = r }))
    const { result } = renderHook(() => useFileExplorer())
    await act(() => useFileExplorerStore.getState().setSelectedFiles(['/sdcard/old']))
    let pending!: Promise<boolean>
    await act(() => { pending = result.current.pullSelectedFiles('/tmp') })
    await act(() => {
      useDeviceStore.setState({
        devices: [{ serial: 'new-device', mode: 'adb', state: 'device' }],
        activeSerial: 'new-device',
      })
    })
    // The new device's selection is made after the switch, and the old transfer's
    // completion must leave it untouched.
    await act(() => useFileExplorerStore.getState().setSelectedFiles(['/sdcard/new']))
    await act(async () => { resolve({ operationId: 'old', serial: 'test-device', completed: 1, failed: 0, cancelled: 0, skipped: 0, items: [{ source: '/sdcard/old', destination: '/tmp/old', status: 'success', message: 'OK' }] }); await pending })
    expect(useFileExplorerStore.getState().selectedFiles).toEqual(['/sdcard/new'])
    expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('test-device: 1 completed'))
  })

  it('does not put an old transfer failure on the newly selected device', async () => {
    let reject!: (reason: Error) => void
    mocks.pushFile.mockReturnValueOnce(new Promise((_resolve, r) => { reject = r }))
    const { result } = renderHook(() => useFileExplorer())
    let pending!: Promise<boolean>
    await act(() => { pending = result.current.pushSingleFile('/tmp/file', '/sdcard/file') })
    await act(() => {
      useDeviceStore.setState({
        devices: [{ serial: 'new-device', mode: 'adb', state: 'device' }],
        activeSerial: 'new-device',
      })
    })
    await act(async () => { useFileExplorerStore.getState().setError('new-device error') })
    await act(async () => { reject(new Error('old transfer failed')); await pending })
    expect(useFileExplorerStore.getState().error).toBe('new-device error')
    expect(mocks.pushFile).toHaveBeenCalledWith('test-device', '/tmp/file', '/sdcard/file')
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import * as fastbootSvc from '@/services/fastbootService'
import { useFlasher } from '../useFlasher'
import { useFlasherStore } from '@/stores/useFlasherStore'

const flashPartitionForDevice = vi.fn<(serial: string, partition: string, filePath: string) => Promise<string>>()
const flashRomFolderForDevice = vi.fn()
const getFastbootDevices = vi.fn()
const getActiveSlot = vi.fn(async () => 'a')
const isUserspaceFastboot = vi.fn(async () => false)

vi.mock('@/services/fastbootService', () => ({
  flashPartitionForDevice: (serial: string, partition: string, filePath: string) =>
    flashPartitionForDevice(serial, partition, filePath),
  flashRomFolderForDevice: (...args: unknown[]) => flashRomFolderForDevice(...args),
  getFastbootDevices: () => getFastbootDevices(),
  getActiveSlot: () => getActiveSlot(),
  isUserspaceFastboot: () => isUserspaceFastboot(),
  onFlashStepStatus: () => () => {},
  flashPartition: vi.fn(),
  flashRomFolder: vi.fn(),
  wipeData: vi.fn(),
  sideloadPackage: vi.fn(),
  runCustomFastbootCommand: vi.fn(),
  fastbootContinue: vi.fn(),
  wakeScreen: vi.fn(),
  wakeAndUnlock: vi.fn(),
  setStayAwakeWhileCharging: vi.fn(),
  getStayAwakeWhileCharging: vi.fn(async () => false),
  scanRomFolder: vi.fn(),
  selectFlashImageFile: vi.fn(),
  selectSideloadFile: vi.fn(),
  selectRomFolder: vi.fn(),
}))

vi.mock('@/services/deviceService', () => ({
  getDevices: vi.fn(async () => [
    { serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' },
  ]),
}))

function primeConfirmedTarget() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setSelectedPartition('boot')
  store.setSelectedImagePath('/boot.img')
}

describe('useFlasher destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
    flashPartitionForDevice.mockResolvedValue('Flashed boot')
    primeConfirmedTarget()
  })

  it('dispatches the captured serial through the strict twin, never the legacy RPC', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.capturePartitionConsent()
    expect(consent.serial).toBe('F1')

    await act(async () => {
      await result.current.executeFlashPartition(consent)
    })

    expect(flashPartitionForDevice).toHaveBeenCalledTimes(1)
    expect(flashPartitionForDevice).toHaveBeenCalledWith('F1', 'boot', '/boot.img')
    expect(fastbootSvc.flashPartition).not.toHaveBeenCalled()
  })

  it('blocks a duplicate dispatch synchronously before the first await', async () => {
    let releaseFirst: (value: string) => void = () => {}
    flashPartitionForDevice.mockImplementationOnce(
      () => new Promise<string>((resolve) => {
        releaseFirst = resolve
      }),
    )

    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.capturePartitionConsent()
    let firstDispatch: Promise<void> = Promise.resolve()
    act(() => {
      firstDispatch = result.current.executeFlashPartition(consent)
    })

    // The second click happens while the first command is still awaiting.
    await act(async () => {
      await result.current.executeFlashPartition(consent)
    })

    expect(flashPartitionForDevice).toHaveBeenCalledTimes(1)

    await act(async () => {
      releaseFirst('Flashed boot')
      await firstDispatch
    })
  })

  it('refuses consent when the target changed and came back (A -> B -> A)', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.capturePartitionConsent()

    await act(async () => {
      const store = useFlasherStore.getState()
      store.setActiveFastbootSerial('F2')
      store.setActiveFastbootSerial('F1')
    })

    await act(async () => {
      await result.current.executeFlashPartition(consent)
    })

    expect(flashPartitionForDevice).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).toMatch(/review and confirm again/i)
  })

  it('refuses consent when the image changed after the confirmation was captured', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.capturePartitionConsent()
    await act(async () => {
      useFlasherStore.getState().setSelectedImagePath('/other.img')
    })

    await act(async () => {
      await result.current.executeFlashPartition(consent)
    })

    expect(flashPartitionForDevice).not.toHaveBeenCalled()
  })

  it('refuses consent when the device disappeared from the live fastboot list', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.capturePartitionConsent()
    await act(async () => {
      // Keep the serial but drop the device from the live list (unplugged).
      useFlasherStore.getState().setFastbootDevices([])
    })

    await act(async () => {
      await result.current.executeFlashPartition(consent)
    })

    expect(flashPartitionForDevice).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).toMatch(/no longer connected/i)
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import * as fastbootSvc from '@/services/fastbootService'
import { useFlasher } from '../useFlasher'
import { useFlasherStore } from '@/stores/useFlasherStore'

const flashPartitionForDevice = vi.fn<(serial: string, partition: string, filePath: string) => Promise<string>>()
const flashRomFolderForDevice = vi.fn()
const wipeData = vi.fn()
const sideloadPackage = vi.fn()
const getFastbootDevices = vi.fn()
const getDevices = vi.fn()
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
  wipeData: (...args: unknown[]) => wipeData(...args),
  sideloadPackage: (...args: unknown[]) => sideloadPackage(...args),
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
  getDevices: () => getDevices(),
}))

function primeConfirmedTarget() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setDeviceMode('fastboot')
  store.setSelectedPartition('boot')
  store.setSelectedImagePath('/boot.img')
}

describe('useFlasher destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
    getDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' }])
    flashPartitionForDevice.mockResolvedValue('Flashed boot')
    wipeData.mockResolvedValue('Wiped')
    sideloadPackage.mockResolvedValue('Sideloaded')
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

describe('wipe and sideload consent', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
    getDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' }])
    wipeData.mockResolvedValue('Wiped')
    sideloadPackage.mockResolvedValue('Sideloaded')
    primeConfirmedTarget()
  })

  it('wipe dispatches the captured serial once and blocks a duplicate dispatch', async () => {
    let releaseFirst: (value: string) => void = () => {}
    wipeData.mockImplementationOnce(
      () => new Promise<string>((resolve) => {
        releaseFirst = resolve
      }),
    )

    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureWipeConsent()
    expect(consent.serial).toBe('F1')

    let firstDispatch: Promise<void> = Promise.resolve()
    act(() => {
      firstDispatch = result.current.executeWipeData(consent)
    })
    await act(async () => {
      await result.current.executeWipeData(consent)
    })

    expect(wipeData).toHaveBeenCalledTimes(1)
    expect(wipeData).toHaveBeenCalledWith('F1')

    await act(async () => {
      releaseFirst('Wiped')
      await firstDispatch
    })
  })

  it('wipe refuses when the device is gone from the live fastboot list (zero RPC)', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureWipeConsent()
    await act(async () => {
      useFlasherStore.getState().setFastbootDevices([])
    })
    await act(async () => {
      await result.current.executeWipeData(consent)
    })

    expect(wipeData).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).toMatch(/no longer connected/i)
  })

  it('wipe refuses after an A -> B -> A serial round trip (zero RPC)', async () => {
    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureWipeConsent()
    await act(async () => {
      const store = useFlasherStore.getState()
      store.setActiveFastbootSerial('F2')
      store.setActiveFastbootSerial('F1')
    })
    await act(async () => {
      await result.current.executeWipeData(consent)
    })

    expect(wipeData).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).toMatch(/review and confirm again/i)
  })

  it('sideload accepts an ADB-recovery target whose fastboot list is empty and uses the captured ZIP', async () => {
    // A sideload target is an ADB-recovery device: the fastboot list is legitimately
    // empty, so membership must not be required.
    getFastbootDevices.mockResolvedValue([])
    getDevices.mockResolvedValue([{ serial: 'S1', state: 'sideload', mode: 'adb', model: 'Pixel' }])
    const store = useFlasherStore.getState()
    store.reset()
    store.setActiveFastbootSerial('S1')
    store.setSideloadFilePath('/update.zip')
    store.setDeviceMode('sideload')

    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureSideloadConsent()
    expect(consent.serial).toBe('S1')
    expect(consent.zipPath).toBe('/update.zip')

    await act(async () => {
      await result.current.executeSideload(consent)
    })

    expect(sideloadPackage).toHaveBeenCalledTimes(1)
    expect(sideloadPackage).toHaveBeenCalledWith('S1', '/update.zip')
    expect(useFlasherStore.getState().error).toBeNull()
  })

  it('sideload refuses a changed ZIP after the confirmation was captured (zero RPC)', async () => {
    getFastbootDevices.mockResolvedValue([])
    getDevices.mockResolvedValue([{ serial: 'S1', state: 'sideload', mode: 'adb', model: 'Pixel' }])
    const store = useFlasherStore.getState()
    store.reset()
    store.setActiveFastbootSerial('S1')
    store.setSideloadFilePath('/update.zip')
    store.setDeviceMode('sideload')

    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureSideloadConsent()
    await act(async () => {
      useFlasherStore.getState().setSideloadFilePath('/other.zip')
    })
    await act(async () => {
      await result.current.executeSideload(consent)
    })

    expect(sideloadPackage).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).toMatch(/review and confirm again/i)
  })

  it('sideload refuses when the device left sideload mode after the confirmation (zero RPC)', async () => {
    getFastbootDevices.mockResolvedValue([])
    getDevices.mockResolvedValue([{ serial: 'S1', state: 'sideload', mode: 'adb', model: 'Pixel' }])
    const store = useFlasherStore.getState()
    store.reset()
    store.setActiveFastbootSerial('S1')
    store.setSideloadFilePath('/update.zip')
    store.setDeviceMode('sideload')

    const { result } = renderHook(() => useFlasher())
    await waitFor(() => expect(getFastbootDevices).toHaveBeenCalled())

    const consent = result.current.captureSideloadConsent()
    await act(async () => {
      useFlasherStore.getState().setDeviceMode('fastboot')
    })
    await act(async () => {
      await result.current.executeSideload(consent)
    })

    expect(sideloadPackage).not.toHaveBeenCalled()
    expect(useFlasherStore.getState().error).not.toBeNull()
  })
})

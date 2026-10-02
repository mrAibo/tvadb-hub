import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { DeviceSummary } from '@/lib/types'
import { useDeviceStore } from '@/stores/useDeviceStore'

const serviceMocks = vi.hoisted(() => ({
  getDevices: vi.fn(),
  getActiveSerial: vi.fn(),
  setActiveSerial: vi.fn(),
  getDeviceInfo: vi.fn(),
  getDeviceMode: vi.fn(),
  getDeviceNicknames: vi.fn(),
}))

vi.mock('@/services/deviceService', () => serviceMocks)

import { refreshDeviceState } from '../useDeviceSync'

const device: DeviceSummary = {
  serial: 'ABC123',
  state: 'device',
  mode: 'adb',
}

describe('useDeviceSync', () => {
  beforeEach(() => {
    useDeviceStore.getState().reset()
    vi.clearAllMocks()
    serviceMocks.getDevices.mockResolvedValue([device])
    serviceMocks.getActiveSerial.mockResolvedValue(device.serial)
    serviceMocks.getDeviceNicknames.mockResolvedValue({})
    serviceMocks.getDeviceInfo.mockResolvedValue({ ...device, model: 'Pixel 7' })
    serviceMocks.getDeviceMode.mockResolvedValue('adb')
    serviceMocks.setActiveSerial.mockResolvedValue(undefined)
  })

  it('coalesces concurrent refreshes and updates device state once', async () => {
    let resolveDevices!: (devices: DeviceSummary[]) => void
    serviceMocks.getDevices.mockReturnValueOnce(new Promise((resolve) => {
      resolveDevices = resolve
    }))

    const firstRefresh = refreshDeviceState(false)
    const secondRefresh = refreshDeviceState(true)

    expect(secondRefresh).toBe(firstRefresh)
    resolveDevices([device])
    await firstRefresh

    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(1)
    expect(useDeviceStore.getState().activeSerial).toBe(device.serial)
    expect(useDeviceStore.getState().deviceInfo?.model).toBe('Pixel 7')
    expect(useDeviceStore.getState().loading).toBe(false)
    expect(useDeviceStore.getState().refreshing).toBe(false)
  })

  it('clears loading state and records sync errors', async () => {
    serviceMocks.getDevices.mockRejectedValueOnce(new Error('ADB unavailable'))

    await refreshDeviceState(false)

    expect(useDeviceStore.getState().error).toBe('ADB unavailable')
    expect(useDeviceStore.getState().loading).toBe(false)
    expect(useDeviceStore.getState().refreshing).toBe(false)
  })

  it('awaits an in-flight refresh and then reads again when a fresh snapshot is demanded', async () => {
    let resolveStale!: (devices: DeviceSummary[]) => void
    serviceMocks.getDevices
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveStale = resolve
        }),
      )
      .mockResolvedValueOnce([device])

    const staleRefresh = refreshDeviceState(true)
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(1)

    const freshRefresh = refreshDeviceState(true, { fresh: true })
    // The stale read is still the only one in flight: no parallel read yet.
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(1)

    resolveStale([])
    await freshRefresh

    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(2)
    expect(freshRefresh).not.toBe(staleRefresh)
    expect(useDeviceStore.getState().devices).toEqual([device])
    expect(useDeviceStore.getState().activeSerial).toBe(device.serial)
    expect(useDeviceStore.getState().refreshing).toBe(false)
  })

  it('does not spend an extra read on fresh when nothing is in flight', async () => {
    await refreshDeviceState(true, { fresh: true })

    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(1)
  })

  it('shares a queued fresh read without starting a parallel stale overwrite', async () => {
    let resolveStale!: (devices: DeviceSummary[]) => void
    serviceMocks.getDevices.mockReturnValueOnce(new Promise((resolve) => { resolveStale = resolve }))
    const stale = refreshDeviceState(true)
    const first = refreshDeviceState(true, { fresh: true })
    const second = refreshDeviceState(true, { fresh: true })
    expect(second).toBe(first)
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(1)
    resolveStale([])
    await Promise.all([stale, first, second])
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(2)
    expect(useDeviceStore.getState().devices).toEqual([device])
  })

  it('queues another read when the previously fresh read already started', async () => {
    let resolveOld!: (devices: DeviceSummary[]) => void
    let resolveMiddle!: (devices: DeviceSummary[]) => void
    let signalMiddle!: () => void
    const middleStarted = new Promise<void>((resolve) => { signalMiddle = resolve })
    serviceMocks.getDevices
      .mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve }))
      .mockImplementationOnce(() => {
        signalMiddle()
        return new Promise((resolve) => { resolveMiddle = resolve })
      })
      .mockResolvedValueOnce([device])
    const old = refreshDeviceState(true)
    const first = refreshDeviceState(true, { fresh: true })
    resolveOld([])
    await middleStarted
    const second = refreshDeviceState(true, { fresh: true })
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(2)
    resolveMiddle([])
    await Promise.all([old, first, second])
    expect(serviceMocks.getDevices).toHaveBeenCalledTimes(3)
    expect(useDeviceStore.getState().devices).toEqual([device])
  })
})

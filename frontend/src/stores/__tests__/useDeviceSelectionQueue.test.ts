import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  persistActiveSerial: vi.fn(),
  getDeviceInfo: vi.fn(),
  getDeviceMode: vi.fn(),
}))

vi.mock('@/services/deviceService', () => ({
  setActiveSerial: mocks.persistActiveSerial,
  getDeviceInfo: mocks.getDeviceInfo,
  getDeviceMode: mocks.getDeviceMode,
}))

import { requestDeviceSelection, useDeviceStore } from '../useDeviceStore'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('shared device selection queue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    mocks.getDeviceInfo.mockResolvedValue({ serial: 'A' })
    mocks.getDeviceMode.mockResolvedValue('device')
    mocks.persistActiveSerial.mockResolvedValue(undefined)
  })

  it('publishes the latest intent when selections arrive back to back', async () => {
    const gate = deferred<void>()
    mocks.persistActiveSerial.mockReturnValueOnce(gate.promise).mockResolvedValue(undefined)

    const first = requestDeviceSelection('A', 'user')
    const second = requestDeviceSelection('B', 'user')

    gate.resolve()
    await Promise.all([first, second])

    // Both RPCs ran serialized (never concurrently) but only the newest intent
    // was allowed to publish.
    expect(mocks.persistActiveSerial.mock.calls.map((call) => call[0])).toEqual(['A', 'B'])
    expect(useDeviceStore.getState().activeSerial).toBe('B')
    expect(useDeviceStore.getState().refreshing).toBe(false)
  })

  it('never lets a background restore override or retarget a user selection', async () => {
    await requestDeviceSelection('A', 'user')
    expect(useDeviceStore.getState().activeSerial).toBe('A')

    mocks.persistActiveSerial.mockClear()
    await requestDeviceSelection('B', 'restore')

    expect(useDeviceStore.getState().activeSerial).toBe('A')
    // The restore must not even reach the backend, or the persisted selection would
    // drift away from the device the user confirmed.
    expect(mocks.persistActiveSerial).not.toHaveBeenCalled()
  })

  it('adopts a restore target when nothing is confirmed yet', async () => {
    await requestDeviceSelection('A', 'restore')

    expect(mocks.persistActiveSerial).toHaveBeenCalledWith('A')
    expect(useDeviceStore.getState().activeSerial).toBe('A')
  })

  it('shares one worker across every mounted caller', async () => {
    const gate = deferred<void>()
    mocks.persistActiveSerial.mockReturnValueOnce(gate.promise).mockResolvedValue(undefined)

    // Two hook instances (top bar and devices page) selecting different devices.
    const fromTopBar = requestDeviceSelection('A', 'user')
    const fromDevicesPage = requestDeviceSelection('B', 'user')

    expect(mocks.persistActiveSerial).toHaveBeenCalledTimes(1)

    gate.resolve()
    await Promise.all([fromTopBar, fromDevicesPage])

    expect(mocks.persistActiveSerial).toHaveBeenCalledTimes(2)
    expect(useDeviceStore.getState().activeSerial).toBe('B')
  })

  it('ignores an empty target instead of falling back to a global serial', async () => {
    await requestDeviceSelection('   ', 'user')

    expect(mocks.persistActiveSerial).not.toHaveBeenCalled()
    expect(useDeviceStore.getState().activeSerial).toBe('')
  })
})

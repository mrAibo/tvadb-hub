import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { SdCardPicker } from '../SdCardPicker'

const mocks = vi.hoisted(() => ({
  listSdCardsForDevice: vi.fn(),
}))

// Only the device-bound reader exists on the service, so a serial-less call cannot
// even be expressed here.
vi.mock('@/services/fileService', () => ({
  listSdCardsForDevice: mocks.listSdCardsForDevice,
}))

// Isolate the store's service boundary: the real deviceService re-exports
// @wailsio/runtime, whose import-time 50 ms poll is not owned by the jsdom
// environment (PHASE0_RUNTIME_DIAGNOSIS.md). The runtime mock is a fail-fast
// sentinel: it is only evaluated if an import still reaches the runtime.
vi.mock('@/services/deviceService', () => ({
  getDeviceInfo: vi.fn(),
  getDeviceMode: vi.fn(),
  setActiveSerial: vi.fn(),
}))
vi.mock('@wailsio/runtime', () => {
  throw new Error('Unexpected Wails runtime import in isolated unit test')
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

function openPicker() {
  const trigger = screen.getByText('Storage').closest('button')
  expect(trigger).not.toBeNull()
  fireEvent.click(trigger as HTMLButtonElement)
}

const externalCard = {
  id: 'vol-1',
  description: 'SD card',
  mountPoint: '/storage/1234-5678',
  isExternal: true,
}

describe('SdCardPicker device binding', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
  })

  it('reads the volumes of the serial it was given', async () => {
    useDeviceStore.setState({ activeSerial: 'tv-A' })
    mocks.listSdCardsForDevice.mockResolvedValue([externalCard])
    const onSelect = vi.fn()

    render(<SdCardPicker serial="tv-A" onSelect={onSelect} />)
    openPicker()

    await waitFor(() => expect(mocks.listSdCardsForDevice).toHaveBeenCalledWith('tv-A'))

    fireEvent.click(await screen.findByText('SD card'))
    expect(onSelect).toHaveBeenCalledWith('/storage/1234-5678')
  })

  it('uses the confirmed selection from the store when no serial prop is given', async () => {
    useDeviceStore.setState({ activeSerial: 'tv-B' })
    mocks.listSdCardsForDevice.mockResolvedValue([])

    render(<SdCardPicker onSelect={vi.fn()} />)
    openPicker()

    await waitFor(() => expect(mocks.listSdCardsForDevice).toHaveBeenCalledWith('tv-B'))
  })

  it('drops a reply that arrives after the confirmed device changed', async () => {
    useDeviceStore.setState({ activeSerial: 'tv-A' })
    const gate = deferred<unknown>()
    mocks.listSdCardsForDevice.mockReturnValueOnce(gate.promise)

    render(<SdCardPicker serial="tv-A" onSelect={vi.fn()} />)
    openPicker()
    await waitFor(() => expect(mocks.listSdCardsForDevice).toHaveBeenCalledWith('tv-A'))

    useDeviceStore.setState({ activeSerial: 'tv-B' })
    gate.resolve([{ ...externalCard, description: 'Stale SD card', mountPoint: '/storage/old' }])

    await act(async () => {})
    expect(screen.queryByText('Stale SD card')).toBeNull()
    expect(screen.queryByText('/storage/old')).toBeNull()
  })

  it('refuses locally when no device is confirmed', async () => {
    render(<SdCardPicker onSelect={vi.fn()} />)
    openPicker()

    await waitFor(() =>
      expect(screen.getByText('No storage volumes detected.')).toBeInTheDocument(),
    )
    expect(mocks.listSdCardsForDevice).not.toHaveBeenCalled()
  })
})

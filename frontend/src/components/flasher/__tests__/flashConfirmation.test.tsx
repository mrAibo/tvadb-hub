import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { PartitionFlashCard } from '../cards/PartitionFlashCard'
import { RomFlashCard } from '../cards/RomFlashCard'
import { useFlasherStore } from '@/stores/useFlasherStore'

const flashPartitionForDevice = vi.fn()
const flashRomFolderForDevice = vi.fn()
const getFastbootDevices = vi.fn()

vi.mock('@/services/fastbootService', () => ({
  flashPartitionForDevice: (...args: unknown[]) => flashPartitionForDevice(...args),
  flashRomFolderForDevice: (...args: unknown[]) => flashRomFolderForDevice(...args),
  getFastbootDevices: () => getFastbootDevices(),
  getActiveSlot: vi.fn(async () => 'a'),
  isUserspaceFastboot: vi.fn(async () => false),
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

function primePartitionCard() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setSelectedPartition('boot')
  store.setSelectedImagePath('/images/boot.img')
}

function primeBatchCard() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setRomFolderPath('/rom')
  store.setFlashPlan({
    steps: [
      { partition: 'boot', image_file: '/rom/boot.img' },
      { partition: 'vbmeta', image_file: '/rom/vbmeta.img' },
    ],
  })
}

describe('PartitionFlashCard destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
    flashPartitionForDevice.mockResolvedValue('Flashed boot')
    primePartitionCard()
  })

  it('shows the captured serial, model, partition and image, and cancels without dispatching', async () => {
    render(<PartitionFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash partition/i }))

    expect(await screen.findByTestId('partition-confirm-serial')).toHaveTextContent('F1')
    expect(screen.getByTestId('partition-confirm-partition')).toHaveTextContent('boot')
    expect(screen.getByTestId('partition-confirm-image')).toHaveTextContent('/images/boot.img')
    // The model is shown once the device poll reported it.
    await waitFor(() => expect(screen.getByText(/Chromecast HD/)).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(flashPartitionForDevice).not.toHaveBeenCalled()
  })

  it('dispatches exactly the captured target and inputs through the strict twin', async () => {
    render(<PartitionFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash partition/i }))
    await screen.findByTestId('partition-confirm-serial')

    const confirm = screen.getAllByRole('button', { name: /^flash partition$/i })
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })

    await waitFor(() => expect(flashPartitionForDevice).toHaveBeenCalledTimes(1))
    expect(flashPartitionForDevice).toHaveBeenCalledWith('F1', 'boot', '/images/boot.img')
  })

  it('invalidates consent for an A -> B -> A serial change and blocks the dispatch', async () => {
    render(<PartitionFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash partition/i }))
    await screen.findByTestId('partition-confirm-serial')

    act(() => {
      const store = useFlasherStore.getState()
      store.setActiveFastbootSerial('F2')
      store.setActiveFastbootSerial('F1')
    })

    expect(await screen.findByTestId('partition-confirm-stale')).toBeInTheDocument()
    const confirm = screen.getAllByRole('button', { name: /^flash partition$/i })
    expect(confirm[confirm.length - 1]).toBeDisabled()

    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })
    expect(flashPartitionForDevice).not.toHaveBeenCalled()
  })

  it('invalidates consent when the image changes while the dialog is open', async () => {
    render(<PartitionFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash partition/i }))
    await screen.findByTestId('partition-confirm-serial')

    act(() => {
      useFlasherStore.getState().setSelectedImagePath('/images/other.img')
    })

    expect(await screen.findByTestId('partition-confirm-stale')).toBeInTheDocument()
  })
})

describe('RomFlashCard destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
    flashRomFolderForDevice.mockResolvedValue('Flashed 2 partition(s)')
    primeBatchCard()
  })

  it('shows the captured serial, folder and step list, and cancels without dispatching', async () => {
    render(<RomFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash 2 partition\(s\)/i }))

    expect(await screen.findByTestId('batch-confirm-serial')).toHaveTextContent('F1')
    expect(screen.getByTestId('batch-confirm-folder')).toHaveTextContent('/rom')
    expect(screen.getByTestId('batch-confirm-steps')).toHaveTextContent('boot, vbmeta')
    await waitFor(() => expect(screen.getByText(/Chromecast HD/)).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(flashRomFolderForDevice).not.toHaveBeenCalled()
  })

  it('dispatches the captured folder and step selection through the batch twin', async () => {
    render(<RomFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash 2 partition\(s\)/i }))
    await screen.findByTestId('batch-confirm-serial')

    const confirm = screen.getAllByRole('button', { name: /^flash 2 partition\(s\)$/i })
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })

    await waitFor(() => expect(flashRomFolderForDevice).toHaveBeenCalledTimes(1))
    const [serial, folder, plan] = flashRomFolderForDevice.mock.calls[0] as [string, string, { steps: unknown[] }]
    expect(serial).toBe('F1')
    expect(folder).toBe('/rom')
    expect(plan.steps).toEqual([
      { partition: 'boot', image_file: '/rom/boot.img' },
      { partition: 'vbmeta', image_file: '/rom/vbmeta.img' },
    ])
  })

  it('invalidates consent when the selection changes while the dialog is open', async () => {
    render(<RomFlashCard />)
    fireEvent.click(screen.getByRole('button', { name: /flash 2 partition\(s\)/i }))
    await screen.findByTestId('batch-confirm-serial')

    act(() => {
      useFlasherStore.getState().togglePartitionSelection('vbmeta')
    })

    expect(await screen.findByTestId('batch-confirm-stale')).toBeInTheDocument()
    // The dialog keeps showing the captured snapshot (2 steps) and disables the action.
    const confirm = screen.getAllByRole('button', { name: /^flash 2 partition\(s\)$/i })
    expect(confirm[confirm.length - 1]).toBeDisabled()
  })
})

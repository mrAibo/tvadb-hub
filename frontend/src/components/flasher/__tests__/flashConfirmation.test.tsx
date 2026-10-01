import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { PartitionFlashCard } from '../cards/PartitionFlashCard'
import { RomFlashCard } from '../cards/RomFlashCard'
import { WipeDataCard } from '../cards/WipeDataCard'
import { SideloadCard } from '../cards/SideloadCard'
import { flashDeviceLabel } from '@/hooks/useFlasher'
import { useFlasherStore } from '@/stores/useFlasherStore'

const flashPartitionForDevice = vi.fn()
const flashRomFolderForDevice = vi.fn()
const wipeData = vi.fn()
const sideloadPackage = vi.fn()
const getFastbootDevices = vi.fn()
const getDevices = vi.fn()

vi.mock('@/services/fastbootService', () => ({
  flashPartitionForDevice: (...args: unknown[]) => flashPartitionForDevice(...args),
  flashRomFolderForDevice: (...args: unknown[]) => flashRomFolderForDevice(...args),
  getFastbootDevices: () => getFastbootDevices(),
  getActiveSlot: vi.fn(async () => 'a'),
  isUserspaceFastboot: vi.fn(async () => false),
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

const FASTBOOT_DEVICE = { serial: 'F1', state: 'fastboot', mode: 'fastboot' }

function primePartitionCard() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setDeviceMode('fastboot')
  store.setSelectedPartition('boot')
  store.setSelectedImagePath('/images/boot.img')
}

function primeBatchCard() {
  const store = useFlasherStore.getState()
  store.reset()
  store.setFastbootDevices([{ serial: 'F1', state: 'fastboot', mode: 'fastboot' }])
  store.setActiveFastbootSerial('F1')
  store.setDeviceMode('fastboot')
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
    getFastbootDevices.mockResolvedValue([FASTBOOT_DEVICE])
    getDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' }])
    flashPartitionForDevice.mockResolvedValue('Flashed boot')
    primePartitionCard()
  })

  it('shows the captured serial, model, partition and image, and cancels without dispatching', async () => {
    render(<PartitionFlashCard />)
    // The model label is part of the captured consent, so wait until the device poll
    // has reported it before opening the dialog.
    await waitFor(() => expect(flashDeviceLabel('F1')).toBe('Chromecast HD'))
    fireEvent.click(screen.getByRole('button', { name: /flash partition/i }))

    expect(await screen.findByTestId('partition-confirm-serial')).toHaveTextContent('F1')
    expect(screen.getByTestId('partition-confirm-partition')).toHaveTextContent('boot')
    expect(screen.getByTestId('partition-confirm-image')).toHaveTextContent('/images/boot.img')
    expect(screen.getByText(/Chromecast HD/)).toBeInTheDocument()

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
    getFastbootDevices.mockResolvedValue([FASTBOOT_DEVICE])
    getDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' }])
    flashRomFolderForDevice.mockResolvedValue('Flashed 2 partition(s)')
    primeBatchCard()
  })

  it('shows the captured serial, folder and step list, and cancels without dispatching', async () => {
    render(<RomFlashCard />)
    await waitFor(() => expect(flashDeviceLabel('F1')).toBe('Chromecast HD'))
    fireEvent.click(screen.getByRole('button', { name: /flash 2 partition\(s\)/i }))

    expect(await screen.findByTestId('batch-confirm-serial')).toHaveTextContent('F1')
    expect(screen.getByTestId('batch-confirm-folder')).toHaveTextContent('/rom')
    expect(screen.getByTestId('batch-confirm-steps')).toHaveTextContent('boot, vbmeta')
    expect(screen.getByText(/Chromecast HD/)).toBeInTheDocument()

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

describe('WipeDataCard destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getFastbootDevices.mockResolvedValue([FASTBOOT_DEVICE])
    getDevices.mockResolvedValue([{ serial: 'F1', state: 'fastboot', mode: 'adb', model: 'Chromecast HD' }])
    wipeData.mockResolvedValue('Wiped')
    primePartitionCard()
  })

  it('shows the captured serial and model, and cancels without dispatching', async () => {
    render(<WipeDataCard />)
    await waitFor(() => expect(flashDeviceLabel('F1')).toBe('Chromecast HD'))
    fireEvent.click(screen.getByRole('button', { name: /wipe everything \(factory reset\)/i }))

    expect(await screen.findByTestId('wipe-confirm-serial')).toHaveTextContent('F1')
    expect(screen.getByText(/Chromecast HD/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(wipeData).not.toHaveBeenCalled()
  })

  it('dispatches the captured serial once', async () => {
    render(<WipeDataCard />)
    fireEvent.click(screen.getByRole('button', { name: /wipe everything \(factory reset\)/i }))
    await screen.findByTestId('wipe-confirm-serial')

    const confirm = screen.getAllByRole('button', { name: /^wipe everything$/i })
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })

    await waitFor(() => expect(wipeData).toHaveBeenCalledTimes(1))
    expect(wipeData).toHaveBeenCalledWith('F1')
  })

  it('invalidates consent for an A -> B -> A serial change and blocks the dispatch', async () => {
    render(<WipeDataCard />)
    fireEvent.click(screen.getByRole('button', { name: /wipe everything \(factory reset\)/i }))
    await screen.findByTestId('wipe-confirm-serial')

    act(() => {
      const store = useFlasherStore.getState()
      store.setActiveFastbootSerial('F2')
      store.setActiveFastbootSerial('F1')
    })

    expect(await screen.findByTestId('wipe-confirm-stale')).toBeInTheDocument()
    const confirm = screen.getAllByRole('button', { name: /^wipe everything$/i })
    expect(confirm[confirm.length - 1]).toBeDisabled()
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })
    expect(wipeData).not.toHaveBeenCalled()
  })
})

describe('SideloadCard destructive confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Sideload targets an ADB-recovery device: the fastboot list is empty by design.
    getFastbootDevices.mockResolvedValue([])
    getDevices.mockResolvedValue([{ serial: 'S1', state: 'sideload', mode: 'adb', model: 'Pixel' }])
    sideloadPackage.mockResolvedValue('Sideloaded')
    const store = useFlasherStore.getState()
    store.reset()
    store.setActiveFastbootSerial('S1')
    store.setSideloadFilePath('/update.zip')
    store.setDeviceMode('sideload')
  })

  it('shows the captured serial, model and ZIP, and cancels without dispatching', async () => {
    render(<SideloadCard />)
    await waitFor(() => expect(flashDeviceLabel('S1')).toBe('Pixel'))
    fireEvent.click(screen.getByRole('button', { name: /^sideload package$/i }))

    expect(await screen.findByTestId('sideload-confirm-serial')).toHaveTextContent('S1')
    expect(screen.getByTestId('sideload-confirm-file')).toHaveTextContent('/update.zip')
    expect(screen.getByText(/Pixel/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(sideloadPackage).not.toHaveBeenCalled()
  })

  it('dispatches the captured serial and ZIP once', async () => {
    render(<SideloadCard />)
    fireEvent.click(screen.getByRole('button', { name: /^sideload package$/i }))
    await screen.findByTestId('sideload-confirm-serial')

    const confirm = screen.getAllByRole('button', { name: /^start sideload$/i })
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })

    await waitFor(() => expect(sideloadPackage).toHaveBeenCalledTimes(1))
    expect(sideloadPackage).toHaveBeenCalledWith('S1', '/update.zip')
  })

  it('invalidates consent when the ZIP changes while the dialog is open (zero RPC)', async () => {
    render(<SideloadCard />)
    fireEvent.click(screen.getByRole('button', { name: /^sideload package$/i }))
    await screen.findByTestId('sideload-confirm-serial')

    act(() => {
      useFlasherStore.getState().setSideloadFilePath('/other.zip')
    })

    expect(await screen.findByTestId('sideload-confirm-stale')).toBeInTheDocument()
    const confirm = screen.getAllByRole('button', { name: /^start sideload$/i })
    expect(confirm[confirm.length - 1]).toBeDisabled()
    await act(async () => {
      fireEvent.click(confirm[confirm.length - 1])
    })
    expect(sideloadPackage).not.toHaveBeenCalled()
  })

  it('invalidates consent when the device context leaves sideload mode (zero RPC)', async () => {
    render(<SideloadCard />)
    fireEvent.click(screen.getByRole('button', { name: /^sideload package$/i }))
    await screen.findByTestId('sideload-confirm-serial')

    act(() => {
      useFlasherStore.getState().setDeviceMode('fastboot')
    })

    expect(await screen.findByTestId('sideload-confirm-stale')).toBeInTheDocument()
  })
})

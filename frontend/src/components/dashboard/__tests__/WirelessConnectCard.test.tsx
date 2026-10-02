import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { WirelessConnectCard } from '../WirelessConnectCard'

const mocks = vi.hoisted(() => ({
  discoverWirelessDevices: vi.fn(),
  getRememberedWirelessDevices: vi.fn(),
  enableWirelessTCPIP: vi.fn(),
  refreshDevices: vi.fn(),
  toast: { success: vi.fn(), info: vi.fn(), error: vi.fn(), warning: vi.fn() },
  devices: [
    {
      serial: '192.168.178.99:38219',
      state: 'device',
      mode: 'adb',
      model: 'BRAVIA 4K GB',
      product: 'BRAVIA_ATV3_4K',
    },
  ] as Array<{ serial: string; state: string; mode: string; model?: string; product?: string }>,
}))

vi.mock('sonner', () => ({ toast: mocks.toast }))

vi.mock('@/services/deviceService', () => ({
  discoverWirelessDevices: mocks.discoverWirelessDevices,
  getRememberedWirelessDevices: mocks.getRememberedWirelessDevices,
  enableWirelessTCPIP: mocks.enableWirelessTCPIP,
}))

vi.mock('@/hooks/useDevices', () => ({
  useDevices: () => ({
    refreshDevices: mocks.refreshDevices,
  }),
}))

vi.mock('@/stores/useDeviceStore', () => ({
  useDeviceStore: Object.assign(
    (selector: (state: { devices: typeof mocks.devices }) => unknown) =>
      selector({ devices: mocks.devices }),
    { getState: () => ({ devices: mocks.devices }) },
  ),
}))

vi.mock('@/components/devices/WirelessConnectDialog', () => ({
  WirelessConnectDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="wireless-dialog">Wireless dialog open</div> : null,
}))

describe('WirelessConnectCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.devices = [
      {
        serial: '192.168.178.99:38219',
        state: 'device',
        mode: 'adb',
        model: 'BRAVIA 4K GB',
        product: 'BRAVIA_ATV3_4K',
      },
    ]
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:38219',
        preferredAddress: '192.168.178.99:38219',
        secureConnect: true,
      },
    ])
    mocks.getRememberedWirelessDevices.mockResolvedValue([
      {
        key: 'tv-1',
        host: '192.168.178.99',
        name: 'Living room TV',
        auto_connect: true,
      },
    ])
    mocks.enableWirelessTCPIP.mockResolvedValue('restarting in TCP mode port: 5555')
    mocks.refreshDevices.mockResolvedValue(undefined)
  })

  it('shows connected, discovered and remembered counts', async () => {
    render(<WirelessConnectCard />)

    await waitFor(() => {
      expect(mocks.discoverWirelessDevices).toHaveBeenCalled()
      expect(mocks.getRememberedWirelessDevices).toHaveBeenCalled()
      expect(screen.getByText('Discovered').nextElementSibling).toHaveTextContent('1')
    })

    expect(screen.getByText('Connected').nextElementSibling).toHaveTextContent('1')
    expect(screen.getByText('Remembered').nextElementSibling).toHaveTextContent('1')
  })

  it('opens the automatic wireless TV dialog', async () => {
    render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Discover / Pair / Connect' }))

    expect(await screen.findByTestId('wireless-dialog')).toBeInTheDocument()
  })

  it('enables legacy TCP/IP only on the confirmed serial, after explicit consent', async () => {
    render(<WirelessConnectCard />)

    expect(screen.queryByText(/Confirm on this serial/)).toBeNull()
    expect(mocks.enableWirelessTCPIP).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))

    expect(screen.getByText(/Enable legacy TCP\/IP on BRAVIA 4K GB/)).toBeInTheDocument()
    expect(screen.getByText(/unencrypted ADB/)).toBeInTheDocument()
    expect(mocks.enableWirelessTCPIP).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: /Confirm on this serial/ }))

    await waitFor(() => {
      expect(mocks.enableWirelessTCPIP).toHaveBeenCalledWith('5555', '192.168.178.99:38219')
    })
    expect(mocks.enableWirelessTCPIP).toHaveBeenCalledTimes(1)
  })

  it('sends nothing when the user cancels the confirmation, even if the target is still ready', async () => {
    render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))
    expect(screen.getByText(/Enable legacy TCP\/IP on BRAVIA 4K GB/)).toBeInTheDocument()
    expect(mocks.enableWirelessTCPIP).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    // The consent is gone, so the confirm action no longer exists and no command ran.
    expect(screen.queryByText(/Confirm on this serial/)).toBeNull()
    expect(screen.queryByText(/Enable legacy TCP\/IP on/)).toBeNull()
    expect(mocks.enableWirelessTCPIP).not.toHaveBeenCalled()
  })

  it('still reports the confirmed device when adbd restarts and drops it mid-command', async () => {
    let resolveEnable: (value: string) => void = () => {}
    mocks.enableWirelessTCPIP.mockImplementation(
      () => new Promise<string>((resolve) => { resolveEnable = resolve }),
    )
    const view = render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))
    fireEvent.click(screen.getByRole('button', { name: /Confirm on this serial/ }))
    expect(mocks.enableWirelessTCPIP).toHaveBeenCalledWith('5555', '192.168.178.99:38219')

    // adbd already restarted: the derived ready list no longer contains this device.
    // That must not rebump the consent epoch of the command that is running.
    mocks.devices = []
    view.rerender(<WirelessConnectCard />)

    await act(async () => {
      resolveEnable('restarting in TCP mode port: 5555')
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    // The command reports the exact device it was sent to, even though adbd has
    // already taken that device out of the ready list.
    expect(mocks.toast.info).toHaveBeenCalledWith(
      'Legacy ADB TCP/IP command finished; the device is no longer listed',
      expect.objectContaining({
        description: expect.stringContaining('192.168.178.99:38219'),
      }),
    )
    const description = mocks.toast.info.mock.calls[0][1]?.description ?? ''
    expect(description).toContain('BRAVIA 4K GB')
    expect(description).toContain('restarting in TCP mode port: 5555')
    expect(mocks.toast.error).not.toHaveBeenCalled()
    // A successful completion is followed by the real device refresh.
    await waitFor(() => {
      expect(mocks.refreshDevices).toHaveBeenCalled()
    })
  })

  it('reports the failed command with its captured model and serial when the device dropped', async () => {
    let rejectEnable: (error: Error) => void = () => {}
    mocks.enableWirelessTCPIP.mockImplementation(
      () => new Promise<string>((_resolve, reject) => { rejectEnable = reject }),
    )
    const view = render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))
    fireEvent.click(screen.getByRole('button', { name: /Confirm on this serial/ }))

    mocks.devices = []
    view.rerender(<WirelessConnectCard />)

    await act(async () => {
      rejectEnable(new Error('adb: device offline'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    // The failure is not hidden by the disconnect, and it names the device it was
    // actually sent to rather than a replacement.
    expect(mocks.toast.error).toHaveBeenCalledWith(
      'Could not enable legacy TCP/IP',
      expect.objectContaining({
        description: expect.stringContaining('192.168.178.99:38219'),
      }),
    )
    const description = mocks.toast.error.mock.calls[0][1]?.description ?? ''
    expect(description).toContain('BRAVIA 4K GB')
    expect(description).toContain('adb: device offline')
    expect(mocks.toast.success).not.toHaveBeenCalled()
  })

  it('never sends the command to a replacement device that appears during confirmation', async () => {
    const view = render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))
    expect(screen.getByText(/Enable legacy TCP\/IP on BRAVIA 4K GB/)).toBeInTheDocument()

    // The confirmed TV is replaced by a different device before the user confirms.
    mocks.devices = [{ serial: '192.168.178.50:40001', state: 'device', mode: 'adb', model: 'Phone' }]
    view.rerender(<WirelessConnectCard />)

    // The replacement is never confirmed implicitly; the panel names the real target.
    expect(screen.getByText(/Legacy TCP\/IP was confirmed for 192\.168\.178\.99:38219/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Confirm on this serial/ })).toBeDisabled()
    expect(mocks.enableWirelessTCPIP).not.toHaveBeenCalled()
  })
})

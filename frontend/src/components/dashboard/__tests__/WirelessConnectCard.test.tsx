import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { WirelessConnectCard } from '../WirelessConnectCard'

const mocks = vi.hoisted(() => ({
  discoverWirelessDevices: vi.fn(),
  getRememberedWirelessDevices: vi.fn(),
  enableWirelessTCPIP: vi.fn(),
  refreshDevices: vi.fn(),
  devices: [
    {
      serial: '192.168.178.99:38219',
      state: 'device',
      mode: 'adb',
    },
  ],
}))

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
  useDeviceStore: (selector: (state: { devices: typeof mocks.devices }) => unknown) =>
    selector({ devices: mocks.devices }),
}))

vi.mock('@/components/devices/WirelessConnectDialog', () => ({
  WirelessConnectDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="wireless-dialog">Wireless dialog open</div> : null,
}))

describe('WirelessConnectCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    })

    expect(screen.getByText('Connected').nextElementSibling).toHaveTextContent('1')
    expect(screen.getByText('Discovered').nextElementSibling).toHaveTextContent('1')
    expect(screen.getByText('Remembered').nextElementSibling).toHaveTextContent('1')
  })

  it('opens the automatic wireless TV dialog', async () => {
    render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Discover / Pair / Connect' }))

    expect(await screen.findByTestId('wireless-dialog')).toBeInTheDocument()
  })

  it('keeps the legacy tcpip fallback available', async () => {
    render(<WirelessConnectCard />)

    fireEvent.click(screen.getByRole('button', { name: 'Legacy TCP/IP' }))

    await waitFor(() => {
      expect(mocks.enableWirelessTCPIP).toHaveBeenCalledWith('5555')
    })
  })
})

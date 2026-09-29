import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/hooks/useDevices', () => ({
  useDevices: () => ({
    devices: [
      {
        serial: 'ABC123',
        state: 'device',
        mode: 'adb',
        model: 'Pixel 9',
      },
    ],
    activeSerial: 'ABC123',
    deviceInfo: {
      serial: 'ABC123',
      state: 'device',
      mode: 'adb',
      model: 'Pixel 9',
      ipAddress: '192.168.1.25',
      isTV: false,
    },
    nicknames: {},
    refreshing: false,
    refreshDevices: vi.fn().mockResolvedValue(undefined),
    selectDevice: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/services/deviceService', () => ({
  getRememberedWirelessDevices: vi.fn().mockResolvedValue([]),
  autoConnectWireless: vi.fn().mockResolvedValue({
    service: { address: '192.168.1.25:37121' },
  }),
}))

import { CurrentPlusTopBar } from '../CurrentPlusTopBar'

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

Object.defineProperty(globalThis, 'ResizeObserver', {
  configurable: true,
  value: TestResizeObserver,
})

Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
  configurable: true,
  value: vi.fn(),
})

describe('CurrentPlusTopBar device menu', () => {
  it('opens grouped device labels without Base UI MenuGroupContext errors', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CurrentPlusTopBar />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    const deviceName = screen.getByText('Pixel 9')
    const trigger = deviceName.closest('button')
    expect(trigger).not.toBeNull()

    fireEvent.click(trigger!)
    expect(await screen.findByText('Connected devices')).toBeInTheDocument()
    expect(screen.getByText('ABC123')).toBeInTheDocument()
  })
})

// test-utils registers the deviceService mock, so it must be evaluated before the
// component under test pulls the service in.
import { renderRoute } from '@/test-utils'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CurrentPlusTopBar } from '../CurrentPlusTopBar'
import { getDevices } from '@/services/deviceService'

vi.mock('@/components/devices/WirelessConnectDialog', () => ({
  WirelessConnectDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="wireless-dialog">Wireless dialog open</div> : null,
}))

describe('CurrentPlusTopBar', () => {
  it('opens the device dropdown without Base UI group context errors', async () => {
    renderRoute(<CurrentPlusTopBar />)

    const trigger = screen.getByText('No device selected').closest('button')
    expect(trigger).not.toBeNull()

    fireEvent.click(trigger!)

    await waitFor(() => {
      expect(screen.getByText('Connected devices')).toBeInTheDocument()
    })
    expect(screen.getByText('No connected devices.')).toBeInTheDocument()
  })

  it('opens the shared wireless dialog from the explicit Find / Pair entry', async () => {
    renderRoute(<CurrentPlusTopBar />)

    expect(screen.queryByTestId('wireless-dialog')).toBeNull()

    fireEvent.click(screen.getByLabelText('Find or pair a wireless device'))

    expect(await screen.findByTestId('wireless-dialog')).toBeInTheDocument()
  })

  it('keeps Refresh on adb device refresh and never opens the wireless dialog', async () => {
    renderRoute(<CurrentPlusTopBar />)

    const before = vi.mocked(getDevices).mock.calls.length
    fireEvent.click(screen.getByLabelText('Refresh devices'))

    await waitFor(() => {
      expect(vi.mocked(getDevices).mock.calls.length).toBeGreaterThan(before)
    })
    expect(screen.queryByTestId('wireless-dialog')).toBeNull()
    expect(screen.getByLabelText('Refresh devices').getAttribute('title')).toContain('adb devices')
  })
})

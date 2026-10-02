import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { WirelessConnectDialog } from '../WirelessConnectDialog'

const mocks = vi.hoisted(() => ({
  pairWireless: vi.fn(),
  pairAndConnectWireless: vi.fn(),
  discoverWirelessDevices: vi.fn(),
  getRememberedWirelessDevices: vi.fn(),
  connectWireless: vi.fn(),
  autoConnectWireless: vi.fn(),
  autoReconnectRememberedWireless: vi.fn(),
  forgetRememberedWirelessDevice: vi.fn(),
  getWirelessDiagnostics: vi.fn(),
  refreshDeviceState: vi.fn(),
  devices: [] as Array<{ serial: string; state: string; mode: string; model?: string }>,
}))

vi.mock('@/services/deviceService', () => ({
  pairWireless: mocks.pairWireless,
  pairAndConnectWireless: mocks.pairAndConnectWireless,
  discoverWirelessDevices: mocks.discoverWirelessDevices,
  getRememberedWirelessDevices: mocks.getRememberedWirelessDevices,
  connectWireless: mocks.connectWireless,
  autoConnectWireless: mocks.autoConnectWireless,
  autoReconnectRememberedWireless: mocks.autoReconnectRememberedWireless,
  forgetRememberedWirelessDevice: mocks.forgetRememberedWirelessDevice,
  getWirelessDiagnostics: mocks.getWirelessDiagnostics,
}))

vi.mock('@/hooks/useDeviceSync', () => ({
  refreshDeviceState: mocks.refreshDeviceState,
}))

vi.mock('@/stores/useDeviceStore', () => ({
  useDeviceStore: Object.assign(
    (selector: (state: { devices: typeof mocks.devices }) => unknown) =>
      selector({ devices: mocks.devices }),
    { getState: () => ({ devices: mocks.devices }) },
  ),
}))

function renderDialog(open = true) {
  const onOpenChange = vi.fn()
  const onConnected = vi.fn().mockResolvedValue(undefined)
  const view = render(
    <WirelessConnectDialog open={open} onOpenChange={onOpenChange} onConnected={onConnected} />,
  )
  return { view, onOpenChange, onConnected }
}

const pairInputs = () => ({
  host: screen.getByLabelText('TV IP address or hostname') as HTMLInputElement,
  port: screen.getByLabelText('Pairing port') as HTMLInputElement,
  pin: screen.getByLabelText('Six-digit pairing code') as HTMLInputElement,
})

function fillManualPair(host: string, port: string, pin: string) {
  const inputs = pairInputs()
  fireEvent.change(inputs.host, { target: { value: host } })
  fireEvent.change(inputs.port, { target: { value: port } })
  fireEvent.change(inputs.pin, { target: { value: pin } })
}

async function submitPair() {
  fireEvent.click(screen.getByRole('button', { name: 'Pair' }))
  await waitFor(() => {
    expect(mocks.pairWireless).toHaveBeenCalled()
  })
}

describe('WirelessConnectDialog manual pairing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.devices = []
    mocks.discoverWirelessDevices.mockResolvedValue([])
    mocks.getRememberedWirelessDevices.mockResolvedValue([])
    mocks.refreshDeviceState.mockResolvedValue(undefined)
    mocks.pairWireless.mockResolvedValue('Successfully paired to 192.168.178.99:37755')
    mocks.connectWireless.mockResolvedValue('connected to 192.168.178.99:37121')
  })

  it('keeps a leading-zero PIN and never connects on the pairing port when mDNS is blocked', async () => {
    mocks.discoverWirelessDevices.mockRejectedValue(new Error('mdns discovery unavailable'))
    const { onConnected } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    expect(mocks.pairWireless).toHaveBeenCalledWith('192.168.178.99:37755', '042915')

    const panel = await screen.findByTestId('paired-panel')
    expect(within(panel).getByText(/Paired: 192\.168\.178\.99/)).toBeInTheDocument()
    expect(screen.getByTestId('pair-notice')).toHaveTextContent(/discovery is unavailable/i)
    // The PIN is cleared after completion and the pairing port is never dialled.
    expect(pairInputs().pin.value).toBe('')
    expect(mocks.connectWireless).not.toHaveBeenCalled()

    mocks.devices = [{ serial: '192.168.178.99:37121', state: 'device', mode: 'adb' }]
    fireEvent.change(screen.getByLabelText('Connect port'), { target: { value: '37121' } })
    fireEvent.click(within(panel).getByRole('button', { name: 'Connect' }))

    await waitFor(() => {
      expect(mocks.connectWireless).toHaveBeenCalledWith('192.168.178.99:37121')
    })
    await waitFor(() => {
      expect(onConnected).toHaveBeenCalled()
    })
  })

  it('refuses to dial the pairing port as the connect port', async () => {
    mocks.discoverWirelessDevices.mockRejectedValue(new Error('mdns discovery unavailable'))
    renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()
    await screen.findByTestId('paired-panel')

    fireEvent.change(screen.getByLabelText('Connect port'), { target: { value: '37755' } })
    fireEvent.click(within(screen.getByTestId('paired-panel')).getByRole('button', { name: 'Connect' }))

    await waitFor(() => {
      expect(screen.getByTestId('dialog-error')).toHaveTextContent(/pairing port/i)
    })
    expect(mocks.connectWireless).not.toHaveBeenCalled()
  })

  it('connects through the single distinct TLS connect service and requires adb authorization', async () => {
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        pairingAddress: '192.168.178.99:37755',
        connectAddress: '192.168.178.99:37121',
        pairingPort: 37755,
        connectPort: 37121,
        needsPairing: true,
        secureConnect: true,
      },
    ])
    const { onConnected, onOpenChange } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    await waitFor(() => {
      expect(mocks.connectWireless).toHaveBeenCalledWith('192.168.178.99:37121')
    })
    // Connect reported success, but adb devices does not list the device yet.
    await waitFor(() => {
      expect(screen.getByTestId('dialog-error')).toHaveTextContent(/not authorized/i)
    })
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(screen.queryByTestId('paired-panel')).not.toBeNull()
  })

  it('reports success only after the authorized device list contains the paired endpoint', async () => {
    mocks.devices = [{ serial: '192.168.178.99:37121', state: 'device', mode: 'adb' }]
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:37121',
        connectPort: 37121,
        secureConnect: true,
      },
    ])
    const { onConnected, onOpenChange } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    await waitFor(() => {
      expect(onConnected).toHaveBeenCalled()
    })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('does not submit twice while a pairing attempt is pending', async () => {
    let resolvePair: (value: string) => void = () => {}
    mocks.pairWireless.mockImplementation(
      () => new Promise<string>((resolve) => { resolvePair = resolve }),
    )
    renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    const pairButton = screen.getByRole('button', { name: 'Pair' })
    fireEvent.click(pairButton)
    expect(pairButton).toBeDisabled()
    fireEvent.click(pairButton)

    expect(mocks.pairWireless).toHaveBeenCalledTimes(1)

    resolvePair('Successfully paired')
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Pair' })).toBeEnabled()
    })
  })

  it('differentiates a wrong code and clears the PIN without claiming success', async () => {
    mocks.pairWireless.mockRejectedValue(new Error('Failed to pair: incorrect code'))
    const { onConnected } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    await waitFor(() => {
      expect(screen.getByTestId('pair-notice')).toHaveTextContent(/rejected that pairing code/i)
    })
    expect(pairInputs().pin.value).toBe('')
    expect(mocks.connectWireless).not.toHaveBeenCalled()
    expect(onConnected).not.toHaveBeenCalled()
  })

  it('auto-fills the discovered pairing endpoint and focuses the PIN', async () => {
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        pairingAddress: '192.168.178.99:37755',
        pairingPort: 37755,
        needsPairing: true,
        secureConnect: true,
      },
    ])
    renderDialog()

    fireEvent.click(await screen.findByRole('button', { name: /Pair with code/i }))

    expect(pairInputs().host.value).toBe('192.168.178.99')
    expect(pairInputs().port.value).toBe('37755')
    expect(document.activeElement).toBe(pairInputs().pin)
  })

  it('drops the PIN and discards a pending result when the dialog closes', async () => {
    // Discovery advertises a unique TLS endpoint, so a late pair result could
    // auto-select it and connect — unless the stale run is really discarded.
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:37121',
        connectPort: 37121,
        secureConnect: true,
      },
    ])
    let resolvePair: (value: string) => void = () => {}
    mocks.pairWireless.mockImplementation(
      () => new Promise<string>((resolve) => { resolvePair = resolve }),
    )
    const onOpenChange = vi.fn()
    const onConnected = vi.fn().mockResolvedValue(undefined)
    const view = render(
      <WirelessConnectDialog open onOpenChange={onOpenChange} onConnected={onConnected} />,
    )

    fillManualPair('192.168.178.99', '37755', '042915')
    fireEvent.click(screen.getByRole('button', { name: 'Pair' }))

    view.rerender(
      <WirelessConnectDialog open={false} onOpenChange={onOpenChange} onConnected={onConnected} />,
    )
    view.rerender(
      <WirelessConnectDialog open onOpenChange={onOpenChange} onConnected={onConnected} />,
    )

    expect(pairInputs().pin.value).toBe('')
    expect(pairInputs().host.value).toBe('')

    await waitFor(() => {
      expect(mocks.discoverWirelessDevices).toHaveBeenCalled()
    })
    await act(async () => {
      resolvePair('Successfully paired to 192.168.178.99:37755')
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(mocks.pairWireless).toHaveBeenCalledTimes(1)
    expect(mocks.connectWireless).not.toHaveBeenCalled()
    expect(screen.queryByTestId('paired-panel')).toBeNull()
    expect(onConnected).not.toHaveBeenCalled()
  })

  it('never treats a pre-connect snapshot as authorization and reports success from the fresh read', async () => {
    // A background refresh was already in flight when the connect command ran. The
    // dialog used to await exactly that read and then call the stale snapshot truth.
    let resolveStale: () => void = () => {}
    let resolveFresh: () => void = () => {}
    mocks.refreshDeviceState.mockImplementation(
      (_background?: boolean, options?: { fresh?: boolean }) => {
        if (options?.fresh) {
          return new Promise<void>((resolve) => { resolveFresh = resolve })
        }
        return new Promise<void>((resolve) => { resolveStale = resolve })
      },
    )
    let resolveConnect: () => void = () => {}
    mocks.connectWireless.mockImplementation(
      () => new Promise<string>((resolve) => { resolveConnect = () => resolve('connected to 192.168.178.99:37121') }),
    )
    const { onConnected, onOpenChange } = renderDialog()

    fireEvent.change(screen.getByLabelText('Connect address'), {
      target: { value: '192.168.178.99:37121' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => {
      expect(mocks.connectWireless).toHaveBeenCalledWith('192.168.178.99:37121')
    })

    // The connect finished. Joining the in-flight background read is not enough: the
    // caller must demand a read that starts after the command.
    await act(async () => {
      resolveConnect()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    expect(mocks.refreshDeviceState).toHaveBeenCalledWith(true, { fresh: true })

    // The older background read resolves with a pre-connect snapshot: no TV yet.
    // It is not the read that may prove authorization, so nothing is reported.
    mocks.devices = []
    await act(async () => {
      resolveStale()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    expect(onConnected).not.toHaveBeenCalled()
    expect(screen.queryByTestId('paired-panel')).toBeNull()

    // Only the read requested after the connect reports the TV as authorized.
    mocks.devices = [{ serial: '192.168.178.99:37121', state: 'device', mode: 'adb' }]
    await act(async () => {
      resolveFresh()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    await waitFor(() => {
      expect(onConnected).toHaveBeenCalled()
    })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('keeps the modal open and reports a not-authorized read instead of claiming success', async () => {
    let resolveFresh: () => void = () => {}
    mockDeferredRefresh((resolve) => {
      resolveFresh = resolve
    })
    mocks.connectWireless.mockResolvedValue('connected to 192.168.178.99:37121')
    const { onConnected, onOpenChange } = renderDialog()

    fireEvent.change(screen.getByLabelText('Connect address'), {
      target: { value: '192.168.178.99:37121' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => {
      expect(mocks.refreshDeviceState).toHaveBeenCalledWith(true, { fresh: true })
    })

    // The fresh read contains no such device: honest failure, modal stays open.
    mocks.devices = []
    await act(async () => {
      resolveFresh()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(screen.getByTestId('dialog-error')).toHaveTextContent(/not authorized/i)
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(screen.getByTestId('paired-panel')).toBeInTheDocument()
  })

  it('does not reselect or connect when the dialog closes while the fresh authorized read is in flight', async () => {
    let resolveFresh: () => void = () => {}
    mockDeferredRefresh((resolve) => {
      resolveFresh = resolve
    })
    mocks.connectWireless.mockResolvedValue('connected to 192.168.178.99:37121')
    const { view, onConnected, onOpenChange } = renderDialog()

    fireEvent.change(screen.getByLabelText('Connect address'), {
      target: { value: '192.168.178.99:37121' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
    await waitFor(() => {
      expect(mocks.refreshDeviceState).toHaveBeenCalledWith(true, { fresh: true })
    })

    view.rerender(
      <WirelessConnectDialog open={false} onOpenChange={onOpenChange} onConnected={onConnected} />,
    )

    // The device shows up only while the modal is closed: the late read must not
    // reselect anything or report a connection behind the user's back.
    mocks.devices = [{ serial: '192.168.178.99:37121', state: 'device', mode: 'adb' }]
    await act(async () => {
      resolveFresh()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(mocks.connectWireless).toHaveBeenCalledTimes(1)
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('does not reuse an authorized port on the host as proof for a fresh pairing', async () => {
    // The host already had an authorized transport on another port before this
    // pairing. That must not label the newly paired endpoint as connected.
    mocks.devices = [{ serial: '192.168.178.99:38000', state: 'device', mode: 'adb' }]
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:37121',
        connectPort: 37121,
        secureConnect: true,
      },
    ])
    const { onConnected, onOpenChange } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    // The fresh TLS endpoint is dialled, and the old port proves nothing.
    await waitFor(() => {
      expect(mocks.connectWireless).toHaveBeenCalledWith('192.168.178.99:37121')
    })
    await waitFor(() => {
      expect(screen.getByTestId('dialog-error')).toHaveTextContent(/not authorized/i)
    })
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)

    // Only the exact endpoint becoming authorized completes the pairing.
    mocks.devices = [
      { serial: '192.168.178.99:38000', state: 'device', mode: 'adb' },
      { serial: '192.168.178.99:37121', state: 'device', mode: 'adb' },
    ]
    fireEvent.change(screen.getByLabelText('Connect port'), { target: { value: '37121' } })
    fireEvent.click(
      within(screen.getByTestId('paired-panel')).getByRole('button', { name: 'Connect' }),
    )

    await waitFor(() => {
      expect(onConnected).toHaveBeenCalled()
    })
  })

  it('accepts the exact endpoint or its known mDNS instance name as fresh proof', async () => {
    // adb may report the network transport as the mDNS instance name rather
    // than ip:port; the discovery evidence ties that name to this endpoint.
    mocks.devices = [{ serial: 'adb-tv', state: 'device', mode: 'adb' }]
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:37121',
        connectPort: 37121,
        secureConnect: true,
      },
    ])
    const { onConnected, onOpenChange } = renderDialog()

    fillManualPair('192.168.178.99', '37755', '042915')
    await submitPair()

    await waitFor(() => {
      expect(onConnected).toHaveBeenCalled()
    })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('never accepts a different port on the host as a manual connect success', async () => {
    mocks.devices = [{ serial: '192.168.178.99:38000', state: 'device', mode: 'adb' }]
    const { onConnected, onOpenChange } = renderDialog()

    fireEvent.change(screen.getByLabelText('Connect address'), {
      target: { value: '192.168.178.99:37121' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))

    await waitFor(() => {
      expect(mocks.connectWireless).toHaveBeenCalledWith('192.168.178.99:37121')
    })
    await waitFor(() => {
      expect(screen.getByTestId('dialog-error')).toHaveTextContent(/not authorized/i)
    })
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('never reports a connect as connected for an unrelated authorized port', async () => {
    mocks.devices = [{ serial: '192.168.178.99:38000', state: 'device', mode: 'adb' }]
    mocks.discoverWirelessDevices.mockResolvedValue([
      {
        discoveryKey: 'adb-tv',
        host: '192.168.178.99',
        instanceNames: ['adb-tv'],
        connectAddress: '192.168.178.99:37121',
        connectPort: 37121,
        secureConnect: true,
      },
    ])
    mocks.autoConnectWireless.mockResolvedValue({
      service: { address: '192.168.178.99:37121' },
      message: 'connected to 192.168.178.99:37121',
    })
    const { onConnected, onOpenChange } = renderDialog()

    const card = await screen.findByText('adb-tv')
    expect(card).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Connect' })[0])

    await waitFor(() => {
      expect(mocks.autoConnectWireless).toHaveBeenCalledWith('192.168.178.99')
    })
    await waitFor(() => {
      expect(screen.getByTestId('dialog-error')).toHaveTextContent(/not authorized/i)
    })
    expect(onConnected).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })
})

function mockDeferredRefresh(onPending: (resolve: () => void) => void) {
  mocks.refreshDeviceState.mockImplementation(
    (_background?: boolean, options?: { fresh?: boolean }) => {
      if (options?.fresh) {
        return new Promise<void>((resolve) => {
          onPending(() => resolve())
        })
      }
      return Promise.resolve()
    },
  )
}

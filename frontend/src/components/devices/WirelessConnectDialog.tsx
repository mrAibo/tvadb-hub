import { useCallback, useEffect, useRef, useState } from 'react'
import {
  IconAlertTriangle as AlertTriangle,
  IconBug as Bug,
  IconCircleCheck as CheckCircle,
  IconCircleX as XCircle,
  IconInfoCircle as InfoCircle,
  IconKey as KeyRound,
  IconLoader2 as Loader2,
  IconRefresh as RefreshCw,
  IconTrash as Trash,
  IconWifi as Wifi,
} from '@tabler/icons-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  autoConnectWireless,
  autoReconnectRememberedWireless,
  connectWireless,
  discoverWirelessDevices,
  forgetRememberedWirelessDevice,
  getRememberedWirelessDevices,
  getWirelessDiagnostics,
  pairWireless,
} from '@/services/deviceService'
import type {
  DiscoveredWirelessDevice,
  RememberedWirelessDevice,
  WirelessDiagnosticCheck,
  WirelessDiagnosticsReport,
  WirelessReconnectReport,
} from '@/lib/types'
import { refreshDeviceState } from '@/hooks/useDeviceSync'
import {
  authorizedEndpointSerial,
  classifyPairFailure,
  connectAddressOf,
  describeConnectFailure,
  formatManualPairAddress,
  friendlyDeviceLabel,
  hostFromSerial,
  isCompletePairingCode,
  needsPairingOf,
  normalizePairingCode,
  pairingPortOf,
  parseManualPairInput,
  redactPairingCode,
  rememberedStatus,
  uniqueConnectAddress,
  type PairFailureKind,
} from '@/lib/wirelessPairing'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { toast } from 'sonner'

interface WirelessConnectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConnected: () => void | Promise<void>
}

type PairPhase = 'idle' | 'pairing'

interface PairNotice {
  kind: PairFailureKind | 'paired'
  message: string
}

function endpointLabel(device: DiscoveredWirelessDevice): string {
  if (device.connectAddress) return device.connectAddress
  if (device.legacyAddress) return device.legacyAddress
  if (device.pairingAddress) return device.pairingAddress
  return device.host
}

function noticeToneClass(kind: PairFailureKind | 'paired'): string {
  if (kind === 'paired') return 'text-green-500'
  if (kind === 'wrong-code') return 'text-destructive'
  return 'text-amber-500'
}

function DiagnosticIcon({ check }: { check: WirelessDiagnosticCheck }) {
  switch (check.status) {
    case 'pass':
      return <CheckCircle className="h-4 w-4 text-green-500" />
    case 'warning':
      return <AlertTriangle className="h-4 w-4 text-yellow-500" />
    case 'fail':
      return <XCircle className="h-4 w-4 text-destructive" />
    default:
      return <InfoCircle className="h-4 w-4 text-blue-400" />
  }
}

function formatLastSeen(value?: string): string {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function DiagnosticsPanel({
  report,
  onClose,
}: {
  report: WirelessDiagnosticsReport
  onClose: () => void
}) {
  return (
    <div className="rounded-lg border border-border/60 bg-muted/10 p-3">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold">Connection diagnostics</p>
          <p className="mt-0.5 text-[10px] text-muted-foreground">
            {report.selectedHost
              ? `Target: ${report.selectedHost}`
              : 'General Wireless ADB checks'}
          </p>
        </div>
        <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px]" onClick={onClose}>
          Hide
        </Button>
      </div>

      <div className="space-y-2">
        {report.checks.map((check) => (
          <div key={check.id} className="flex items-start gap-2 rounded-md border border-border/40 bg-card/60 p-2.5">
            <DiagnosticIcon check={check} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center justify-between gap-2">
                <p className="text-[11px] font-semibold">{check.label}</p>
                <span className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                  {check.status}
                </span>
              </div>
              <p className="mt-0.5 break-words text-[10px] leading-relaxed text-muted-foreground">
                {check.detail}
              </p>
              {check.recommendation && (
                <p className="mt-1 text-[10px] leading-relaxed text-foreground/80">
                  {check.recommendation}
                </p>
              )}
            </div>
          </div>
        ))}
      </div>

      {report.adbVersion && (
        <div className="mt-2 border-t border-border/40 pt-2 text-[9px] text-muted-foreground">
          <span className="font-medium">ADB:</span> {report.adbVersion}
        </div>
      )}
    </div>
  )
}

export function WirelessConnectDialog({
  open,
  onOpenChange,
  onConnected,
}: WirelessConnectDialogProps) {
  const adbDevices = useDeviceStore((state) => state.devices)
  const [devices, setDevices] = useState<DiscoveredWirelessDevice[]>([])
  const [remembered, setRemembered] = useState<RememberedWirelessDevice[]>([])
  const [scanning, setScanning] = useState(false)
  const [busyHost, setBusyHost] = useState('')
  const [pairHost, setPairHost] = useState('')
  const [pairPort, setPairPort] = useState('')
  const [pairCode, setPairCode] = useState('')
  const [pairPhase, setPairPhase] = useState<PairPhase>('idle')
  const [pairNotice, setPairNotice] = useState<PairNotice | null>(null)
  const [pairedHost, setPairedHost] = useState('')
  const [connectPort, setConnectPort] = useState('')
  const [manualAddress, setManualAddress] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [diagnosing, setDiagnosing] = useState(false)
  const [diagnostics, setDiagnostics] = useState<WirelessDiagnosticsReport | null>(null)
  const [reconnecting, setReconnecting] = useState(false)
  const [reconnectReport, setReconnectReport] = useState<WirelessReconnectReport | null>(null)

  // Every async flow takes a run id. Closing the dialog or cancelling bumps the
  // id, so a late RPC result can never write state or connect behind the user.
  const runRef = useRef(0)
  const pinRef = useRef<HTMLInputElement | null>(null)

  const authorizedAdb = adbDevices.filter(
    (device) => device.mode === 'adb' && device.state === 'device',
  )

  /** Drops the pairing PIN and everything derived from it. */
  const resetPairing = useCallback(() => {
    runRef.current += 1
    setPairHost('')
    setPairPort('')
    setPairCode('')
    setPairPhase('idle')
    setPairNotice(null)
    setPairedHost('')
    setConnectPort('')
    setManualAddress('')
  }, [])

  const loadDiscovery = useCallback(async () => {
    try {
      const [discovered, rememberedDevices] = await Promise.all([
        discoverWirelessDevices(),
        getRememberedWirelessDevices(),
      ])
      setDevices(discovered)
      setRemembered(rememberedDevices)
      return { ok: true as const, devices: discovered, error: null }
    } catch (scanError) {
      setDevices([])
      return { ok: false as const, devices: [] as DiscoveredWirelessDevice[], error: scanError }
    }
  }, [])

  const scan = useCallback(async () => {
    setScanning(true)
    setError(null)
    const result = await loadDiscovery()
    if (!result.ok) {
      setError(result.error instanceof Error ? result.error.message : 'Wireless discovery failed')
    }
    setScanning(false)
  }, [loadDiscovery])

  useEffect(() => {
    if (!open) return
    resetPairing()
    setError(null)
    setDiagnostics(null)
    setReconnectReport(null)
    void scan()
    return () => {
      // Close cleanup: no PIN, no half-filled pair target survives the dialog.
      resetPairing()
    }
  }, [open, scan, resetPairing])

  async function finishConnection(message: string) {
    toast.success('TV connected', { description: message })
    await onConnected()
    onOpenChange(false)
  }

  /**
   * Re-reads `adb devices`; only that list can prove authorization.
   *
   * The read must *start* after the connect command: a background refresh that was
   * already in flight when the command ran resolves with a pre-connect snapshot and
   * would report a freshly connected TV as not authorized. The fresh option awaits
   * that older read and then performs one new read; it never polls.
   */
  async function refreshAuthorized() {
    try {
      await refreshDeviceState(true, { fresh: true })
    } catch {
      // refreshDeviceState records its own error; keep the previous device state.
    }
  }

  function handlePairWithCode(device: DiscoveredWirelessDevice) {
    if (pairPhase === 'pairing' || busyHost !== '') return
    setPairHost(device.host)
    setPairPort(pairingPortOf(device))
    setPairNotice(null)
    setError(null)
    pinRef.current?.focus()
  }

  /**
   * Manual pairing: `adb pair` against the explicit endpoint, then a fresh
   * discovery/authorized-device read. No mDNS is required to pair, and the PIN
   * is dropped as soon as the attempt completes.
   */
  async function handleManualPair() {
    if (pairPhase === 'pairing') return
    const parsed = parseManualPairInput(pairHost, pairPort)
    if (!parsed.ok) {
      setError(parsed.error)
      return
    }
    const code = normalizePairingCode(pairCode)
    if (!isCompletePairingCode(code)) {
      setError('Enter the six ASCII digits from the TV exactly as shown — leading zeros matter.')
      return
    }

    const run = ++runRef.current
    setPairPhase('pairing')
    setError(null)
    setPairNotice(null)
    setPairedHost('')

    let pairError: unknown = null
    try {
      await pairWireless(parsed.address, code)
    } catch (attemptError) {
      pairError = attemptError
    }
    if (run !== runRef.current) return

    setPairCode('')
    setPairPhase('idle')
    if (pairError) {
      setPairNotice(classifyPairFailure(pairError))
      return
    }

    const discovery = await loadDiscovery()
    await refreshAuthorized()
    if (run !== runRef.current) return

    // A serial already authorized for another port on this host is not proof
    // that the endpoint just paired is connected. Only a fresh unique TLS
    // connect advertisement may be dialled, and only that exact endpoint (or a
    // known mDNS instance name for it) may later prove authorization.
    if (!discovery.ok) {
      setPairNotice({
        kind: 'discovery-unavailable',
        message:
          'Paired. mDNS discovery is unavailable here, so the TLS connect port cannot be found automatically. Enter the connect port the TV shows under Wireless debugging and press Connect.',
      })
      setPairedHost(parsed.host)
      return
    }

    const connectTarget = uniqueConnectAddress(discovery.devices, parsed.host)
    if (!connectTarget) {
      setPairNotice({
        kind: 'paired',
        message: `Paired with ${parsed.host}. No single TLS connect port was advertised — enter the connect port the TV shows under Wireless debugging and press Connect.`,
      })
      setPairedHost(parsed.host)
      return
    }

    // A distinct TLS connect service exists: connect through it, then require
    // that exact endpoint in the authorized-device list before reporting success.
    await connectTo(connectTarget, discovery.devices, run)
  }

  /**
   * Connects to an explicit connect endpoint and only reports success once
   * `adb devices` lists that exact endpoint (or one of the mDNS instance names
   * discovery ties to it). Never used with a pairing port, and never satisfied
   * by a different port that happens to share the host.
   */
  async function connectTo(
    address: string,
    knownDevices: DiscoveredWirelessDevice[],
    existingRun?: number,
  ) {
    const run = existingRun ?? ++runRef.current
    setBusyHost('manual')
    setError(null)
    try {
      const message = await connectWireless(address)
      await refreshAuthorized()
      if (run !== runRef.current) return
      const serial = authorizedEndpointSerial(
        address,
        useDeviceStore.getState().devices,
        knownDevices,
      )
      if (!serial) {
        setError(
          `The connect command reported “${redactPairingCode(message || 'no message')}”, but ${address} is not authorized in adb devices yet. Confirm the connect address from the TV's Wireless debugging screen and retry.`,
        )
        setPairedHost(hostFromSerial(address) || address)
        return
      }
      await finishConnection(message || `Authorized as ${serial}`)
    } catch (connectError) {
      if (run !== runRef.current) return
      setError(describeConnectFailure(connectError))
      setPairedHost(hostFromSerial(address) || address)
    } finally {
      if (run === runRef.current) setBusyHost('')
    }
  }

  async function handlePairedConnect() {
    if (busyHost !== '') return
    if (!pairedHost) return
    if (!/^\d{1,5}$/.test(connectPort)) {
      setError('Enter the connect port the TV shows under Wireless debugging.')
      return
    }
    if (pairHost.trim() === pairedHost && connectPort === pairPort) {
      setError(
        'That is the pairing port. Enter the connect port shown next to the TV IP under Wireless debugging.',
      )
      return
    }
    await connectTo(formatManualPairAddress(pairedHost, connectPort), devices)
  }

  async function handleManualConnect() {
    if (busyHost !== '') return
    const address = manualAddress.trim()
    if (!address) {
      setError('Enter an address such as 192.168.1.100:37121.')
      return
    }
    await connectTo(address, devices)
  }

  async function handleAutoConnect(device: DiscoveredWirelessDevice) {
    const run = ++runRef.current
    setBusyHost(device.host)
    setError(null)
    try {
      const result = await autoConnectWireless(device.host)
      await refreshAuthorized()
      if (run !== runRef.current) return
      // The backend resolved and dialled one exact endpoint; only that endpoint
      // may prove the fresh connection, never an unrelated port on the host.
      const endpoint = (result.service?.address || device.connectAddress || '').trim()
      const serial = authorizedEndpointSerial(
        endpoint,
        useDeviceStore.getState().devices,
        devices,
      )
      if (!serial) {
        setError(
          `The connect command reported “${redactPairingCode(result.message || 'no message')}”, but ${endpoint || device.host} is not authorized in adb devices yet.`,
        )
        return
      }
      await finishConnection(result.message || `Connected to ${result.service.address}`)
    } catch (connectError) {
      if (run !== runRef.current) return
      setError(describeConnectFailure(connectError))
    } finally {
      if (run === runRef.current) setBusyHost('')
    }
  }

  async function handleDiagnose(selector: string = '') {
    setDiagnosing(true)
    setError(null)
    try {
      const report = await getWirelessDiagnostics(selector)
      setDiagnostics(report)
    } catch (diagnosticError) {
      setDiagnostics(null)
      setError(
        diagnosticError instanceof Error ? diagnosticError.message : 'Diagnostics failed',
      )
    } finally {
      setDiagnosing(false)
    }
  }

  async function handleReconnectRemembered() {
    setReconnecting(true)
    setError(null)
    try {
      const report = await autoReconnectRememberedWireless()
      setReconnectReport(report)
      await onConnected()
      await scan()

      const newlyConnected = report.connected.length
      const alreadyConnected = report.alreadyConnected.length
      const failed = Object.keys(report.failed).length
      if (newlyConnected > 0) {
        toast.success(`Reconnected ${newlyConnected} remembered TV${newlyConnected === 1 ? '' : 's'}`)
      } else if (alreadyConnected > 0 && failed === 0) {
        toast.success('Remembered TV is already connected')
      } else if (failed > 0) {
        toast.error('Some remembered TVs could not reconnect')
      } else {
        toast.info('No remembered TV endpoint is currently available')
      }
    } catch (reconnectError) {
      setReconnectReport(null)
      setError(reconnectError instanceof Error ? reconnectError.message : 'Reconnect failed')
    } finally {
      setReconnecting(false)
    }
  }

  async function handleForgetRemembered(key: string) {
    setError(null)
    try {
      await forgetRememberedWirelessDevice(key)
      setRemembered((current) => current.filter((entry) => entry.key !== key))
      toast.success('Automatic reconnect disabled for this TV')
    } catch (forgetError) {
      setError(forgetError instanceof Error ? forgetError.message : 'Could not forget TV')
    }
  }

  const pairing = pairPhase === 'pairing'

  return (
    <Dialog open={open} onOpenChange={(next) => busyHost === '' && onOpenChange(next)}>
      <DialogContent className="sm:max-w-[620px] max-h-[88vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Wireless Android TV</DialogTitle>
          <DialogDescription>
            Discovered devices are advertised over ADB mDNS. “Pairable” means the TV currently
            offers a pairing window — that offer is not trust. A TV is only connected once it is
            authorized in adb devices. If your router blocks multicast, pair manually with the IP,
            pairing port and code from the TV.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-xs font-semibold">Discovered devices</p>
              <p className="text-[11px] text-muted-foreground">
                Pairing and connect ports may change whenever Wireless debugging restarts.
              </p>
            </div>
            <div className="flex flex-wrap justify-end gap-1.5">
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleReconnectRemembered()}
                disabled={reconnecting || scanning || busyHost !== '' || remembered.length === 0}
              >
                {reconnecting ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Wifi className="mr-1.5 h-3.5 w-3.5" />
                )}
                Reconnect
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void handleDiagnose('')}
                disabled={diagnosing || scanning || busyHost !== ''}
              >
                {diagnosing ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Bug className="mr-1.5 h-3.5 w-3.5" />
                )}
                Diagnose
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void scan()}
                disabled={scanning || busyHost !== '' || pairing}
              >
                {scanning ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                )}
                Scan
              </Button>
            </div>
          </div>

          <div className="max-h-64 space-y-2 overflow-y-auto">
            {!scanning && devices.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border/70 px-4 py-6 text-center">
                <Wifi className="mx-auto mb-2 h-5 w-5 text-muted-foreground" />
                <p className="text-xs font-medium">No Wireless ADB service found</p>
                <p className="mt-1 text-[11px] text-muted-foreground">
                  Press Scan. If discovery stays empty, your router is probably blocking mDNS —
                  use “Pair with a code” below.
                </p>
              </div>
            ) : (
              devices.map((device) => {
                const pairable = needsPairingOf(device)
                const connectable = Boolean(connectAddressOf(device))
                // Authorization is claimed for this exact endpoint, never for
                // "some device on this host".
                const authorizedSerial = connectable
                  ? authorizedEndpointSerial(connectAddressOf(device), adbDevices, [device])
                  : undefined
                const busy = busyHost === device.host

                return (
                  <div key={device.discoveryKey} className="rounded-lg border border-border/60 bg-card p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-xs font-semibold">{friendlyDeviceLabel(device)}</p>
                        <p className="truncate font-mono text-[10px] text-muted-foreground">
                          {endpointLabel(device)}
                        </p>
                        <div className="mt-1.5 flex flex-wrap gap-1.5 text-[9px] font-medium uppercase tracking-wide">
                          {authorizedSerial && (
                            <span className="rounded bg-green-500/10 px-1.5 py-0.5 text-green-500">
                              Authorized
                            </span>
                          )}
                          {pairable && (
                            <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-500">
                              Pairable · pairing offer
                            </span>
                          )}
                          {device.connectAddress && (
                            <span className="rounded bg-primary/10 px-1.5 py-0.5 text-primary">
                              TLS connect
                            </span>
                          )}
                          {device.legacyAddress && (
                            <span className="rounded bg-muted px-1.5 py-0.5 text-muted-foreground">
                              Legacy 5555
                            </span>
                          )}
                        </div>
                      </div>

                      <div className="flex shrink-0 gap-1.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 px-2 text-[10px]"
                          onClick={() => void handleDiagnose(device.host)}
                          disabled={diagnosing || busyHost !== ''}
                        >
                          <Bug className="mr-1 h-3 w-3" />
                          Diagnose
                        </Button>
                        {pairable && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 text-[11px]"
                            onClick={() => handlePairWithCode(device)}
                            disabled={busyHost !== '' || pairing}
                          >
                            <KeyRound className="mr-1 h-3 w-3" />
                            Pair with code
                          </Button>
                        )}
                        {connectable && (
                          <Button
                            size="sm"
                            className="h-7 text-[11px]"
                            onClick={() => void handleAutoConnect(device)}
                            disabled={busyHost !== '' || pairing}
                          >
                            {busy ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                            Connect
                          </Button>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })
            )}
          </div>

          <div className="rounded-lg border border-border/50 bg-muted/5 p-3">
            <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Authorized now (adb devices)
            </p>
            {authorizedAdb.length === 0 ? (
              <p className="mt-1 text-[11px] text-muted-foreground">
                Nothing authorized yet. A discovered or paired TV is not connected until it appears
                here with state “device”.
              </p>
            ) : (
              <div className="mt-1.5 space-y-1">
                {authorizedAdb.map((device) => (
                  <p key={device.serial} className="truncate font-mono text-[10px] text-foreground/80">
                    {device.serial}
                    {device.model || device.product ? ` · ${device.model || device.product}` : ''}
                  </p>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-lg border border-border/50 bg-muted/5 p-3">
            <p className="text-xs font-semibold">Pair with a code</p>
            <p className="mt-0.5 text-[10px] leading-relaxed text-muted-foreground">
              On the TV open Settings → Wireless debugging → “Pair device with pairing code”. Keep
              that window OPEN on the TV until pairing finishes, then copy the IP address, the
              pairing port and the six-digit code.
            </p>
            <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground/80">
              Manual pairing bypasses blocked multicast discovery only. It cannot open a firewall
              and it cannot reach a TV that is powered off or on another network.
            </p>

            <div className="mt-2 grid gap-2 sm:grid-cols-[1.3fr_0.8fr_1.1fr_auto]">
              <Input
                aria-label="TV IP address or hostname"
                value={pairHost}
                onChange={(event) => setPairHost(event.target.value)}
                placeholder="192.168.178.99"
                className="h-8 text-xs"
                disabled={pairing || busyHost !== ''}
              />
              <Input
                aria-label="Pairing port"
                value={pairPort}
                onChange={(event) =>
                  setPairPort(event.target.value.replace(/\D/g, '').slice(0, 5))
                }
                placeholder="37755"
                inputMode="numeric"
                className="h-8 font-mono text-xs"
                disabled={pairing || busyHost !== ''}
              />
              <Input
                ref={pinRef}
                aria-label="Six-digit pairing code"
                value={pairCode}
                onChange={(event) => setPairCode(normalizePairingCode(event.target.value))}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void handleManualPair()
                }}
                placeholder="042915"
                inputMode="numeric"
                autoComplete="one-time-code"
                className="h-8 font-mono tracking-[0.25em]"
                disabled={pairing || busyHost !== ''}
              />
              <Button
                size="sm"
                className="h-8"
                onClick={() => void handleManualPair()}
                disabled={pairing || busyHost !== ''}
              >
                {pairing ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <KeyRound className="mr-1.5 h-3.5 w-3.5" />
                )}
                {pairing ? 'Pairing…' : 'Pair'}
              </Button>
            </div>

            {pairing && (
              <p className="mt-1.5 text-[10px] text-muted-foreground">
                Waiting for the TV… keep the pairing-code window open until this finishes.
              </p>
            )}

            {!pairing && pairNotice && (
              <p
                data-testid="pair-notice"
                className={`mt-1.5 text-[10px] leading-relaxed ${noticeToneClass(pairNotice.kind)}`}
              >
                {pairNotice.message}
              </p>
            )}

            {!pairing && (pairHost || pairPort || pairCode) && (
              <Button
                variant="ghost"
                size="sm"
                className="mt-1 h-6 px-2 text-[10px]"
                onClick={resetPairing}
              >
                Cancel and clear code
              </Button>
            )}
          </div>

          {pairedHost && (
            <div
              data-testid="paired-panel"
              className="rounded-lg border border-primary/30 bg-primary/5 p-3"
            >
              <p className="text-xs font-semibold">Paired: {pairedHost}</p>
              <p className="mt-0.5 text-[10px] leading-relaxed text-muted-foreground">
                Pairing is not authorization. Enter the connect port the TV shows next to its IP
                under Wireless debugging (never the pairing port), then press Connect.
              </p>
              <div className="mt-2 flex items-end gap-2">
                <div className="w-28">
                  <label className="mb-1 block text-[10px] font-medium text-muted-foreground">
                    Connect port
                  </label>
                  <Input
                    aria-label="Connect port"
                    value={connectPort}
                    onChange={(event) =>
                      setConnectPort(event.target.value.replace(/\D/g, '').slice(0, 5))
                    }
                    placeholder="37121"
                    inputMode="numeric"
                    className="h-8 font-mono text-xs"
                    disabled={busyHost !== ''}
                  />
                </div>
                <Button
                  size="sm"
                  className="h-8"
                  onClick={() => void handlePairedConnect()}
                  disabled={busyHost !== '' || connectPort.length === 0}
                >
                  {busyHost === 'manual' ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Wifi className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  Connect
                </Button>
              </div>
            </div>
          )}

          {reconnectReport && (
            <div className="rounded-lg border border-border/50 bg-muted/5 p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                  Reconnect result
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 px-2 text-[10px]"
                  onClick={() => setReconnectReport(null)}
                >
                  Hide
                </Button>
              </div>
              <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
                <div className="rounded-md border border-border/40 bg-card/60 p-2">
                  <p className="text-[9px] uppercase text-muted-foreground">Attempted</p>
                  <p className="text-sm font-semibold">{reconnectReport.attempted}</p>
                </div>
                <div className="rounded-md border border-border/40 bg-card/60 p-2">
                  <p className="text-[9px] uppercase text-muted-foreground">Connected</p>
                  <p className="text-sm font-semibold text-green-500">{reconnectReport.connected.length}</p>
                </div>
                <div className="rounded-md border border-border/40 bg-card/60 p-2">
                  <p className="text-[9px] uppercase text-muted-foreground">Unavailable</p>
                  <p className="text-sm font-semibold">{reconnectReport.unavailable.length}</p>
                </div>
                <div className="rounded-md border border-border/40 bg-card/60 p-2">
                  <p className="text-[9px] uppercase text-muted-foreground">Failed</p>
                  <p className="text-sm font-semibold text-destructive">{Object.keys(reconnectReport.failed).length}</p>
                </div>
              </div>
            </div>
          )}

          {diagnostics && (
            <DiagnosticsPanel report={diagnostics} onClose={() => setDiagnostics(null)} />
          )}

          {remembered.length > 0 && (
            <div className="rounded-lg border border-border/50 bg-muted/5 p-3">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Remembered TVs · automatic reconnect
              </p>
              <div className="mt-2 space-y-1.5">
                {remembered.map((entry) => {
                  const status = rememberedStatus(entry, devices, adbDevices)
                  return (
                  <div
                    key={entry.key}
                    className="flex items-center justify-between gap-3 rounded-md border border-border/40 bg-card/60 px-2.5 py-2"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-[11px] font-medium">
                        {entry.name || entry.host}
                      </p>
                      {(entry.manufacturer || entry.model || entry.android_version) && (
                        <p className="truncate text-[9px] text-muted-foreground">
                          {[entry.manufacturer, entry.model, entry.android_version && `Android ${entry.android_version}`]
                            .filter(Boolean)
                            .join(' · ')}
                        </p>
                      )}
                      <p className="truncate font-mono text-[9px] text-muted-foreground">
                        {entry.last_address || entry.host}
                      </p>
                      <p className="mt-0.5 text-[9px] text-muted-foreground">
                        Last seen: {formatLastSeen(entry.last_seen_at)}
                      </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <span
                        className={
                          status === 'connected'
                            ? 'rounded bg-green-500/10 px-1.5 py-0.5 text-[9px] font-medium text-green-500'
                            : status === 'available'
                              ? 'rounded bg-blue-500/10 px-1.5 py-0.5 text-[9px] font-medium text-blue-400'
                              : 'rounded bg-muted px-1.5 py-0.5 text-[9px] font-medium text-muted-foreground'
                        }
                      >
                        {status === 'connected' ? 'Connected' : status === 'available' ? 'Available' : 'Offline'}
                      </span>
                      {entry.auto_connect && (
                        <span className="rounded bg-green-500/10 px-1.5 py-0.5 text-[9px] font-medium text-green-500">
                          Auto
                        </span>
                      )}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 text-muted-foreground hover:text-destructive"
                        onClick={() => void handleForgetRemembered(entry.key)}
                        title="Forget this TV"
                      >
                        <Trash className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                  )
                })}
              </div>
            </div>
          )}

          <div className="border-t border-border/50 pt-3">
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Manual connect
            </p>
            <p className="mb-2 text-[10px] leading-relaxed text-muted-foreground">
              For an already-authorized TV or the legacy 5555 port: enter the connect address
              (`host:port`). The pairing port never works here.
            </p>
            <div className="flex gap-2">
              <Input
                aria-label="Connect address"
                value={manualAddress}
                onChange={(event) => setManualAddress(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void handleManualConnect()}
                placeholder="192.168.178.99:37121"
                className="h-8 text-xs"
                disabled={busyHost !== ''}
              />
              <Button
                variant="outline"
                size="sm"
                className="h-8"
                onClick={() => void handleManualConnect()}
                disabled={busyHost !== ''}
              >
                {busyHost === 'manual' && !pairedHost ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Wifi className="mr-1.5 h-3.5 w-3.5" />
                )}
                Connect
              </Button>
            </div>
          </div>

          {error && (
            <div
              data-testid="dialog-error"
              className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-[11px] text-destructive"
            >
              {error}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

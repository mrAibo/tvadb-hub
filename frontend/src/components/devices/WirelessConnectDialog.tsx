import { useCallback, useEffect, useState } from 'react'
import {
  IconAlertTriangle as AlertTriangle,
  IconBug as Bug,
  IconCircleCheck as CheckCircle,
  IconCircleX as XCircle,
  IconInfoCircle as InfoCircle,
  IconTrash as Trash,
  IconLoader2 as Loader2,
  IconRefresh as RefreshCw,
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
  connectWireless,
  discoverWirelessDevices,
  forgetRememberedWirelessDevice,
  getRememberedWirelessDevices,
  getWirelessDiagnostics,
  pairAndConnectWireless,
} from '@/services/deviceService'
import type {
  DiscoveredWirelessDevice,
  RememberedWirelessDevice,
  WirelessDiagnosticCheck,
  WirelessDiagnosticsReport,
} from '@/lib/types'
import { toast } from 'sonner'

interface WirelessConnectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConnected: () => void | Promise<void>
}

function endpointLabel(device: DiscoveredWirelessDevice): string {
  if (device.connectAddress) return device.connectAddress
  if (device.legacyAddress) return device.legacyAddress
  if (device.pairingAddress) return device.pairingAddress
  return device.host
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
  const [devices, setDevices] = useState<DiscoveredWirelessDevice[]>([])
  const [remembered, setRemembered] = useState<RememberedWirelessDevice[]>([])
  const [scanning, setScanning] = useState(false)
  const [busyHost, setBusyHost] = useState('')
  const [pairHost, setPairHost] = useState('')
  const [pairCode, setPairCode] = useState('')
  const [manualAddress, setManualAddress] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [diagnosing, setDiagnosing] = useState(false)
  const [diagnostics, setDiagnostics] = useState<WirelessDiagnosticsReport | null>(null)

  const scan = useCallback(async () => {
    setScanning(true)
    setError(null)
    try {
      const [discovered, rememberedDevices] = await Promise.all([
        discoverWirelessDevices(),
        getRememberedWirelessDevices(),
      ])
      setDevices(discovered)
      setRemembered(rememberedDevices)
    } catch (scanError) {
      setDevices([])
      setError(scanError instanceof Error ? scanError.message : 'Wireless discovery failed')
    } finally {
      setScanning(false)
    }
  }, [])

  useEffect(() => {
    if (!open) return
    setPairHost('')
    setPairCode('')
    setError(null)
    setDiagnostics(null)
    void scan()
  }, [open, scan])

  async function finishConnection(message: string) {
    toast.success('TV connected', { description: message })
    await onConnected()
    onOpenChange(false)
  }

  async function handleAutoConnect(device: DiscoveredWirelessDevice) {
    setBusyHost(device.host)
    setError(null)
    try {
      const result = await autoConnectWireless(device.host)
      await finishConnection(result.message || `Connected to ${result.service.address}`)
    } catch (connectError) {
      setError(connectError instanceof Error ? connectError.message : 'Connection failed')
    } finally {
      setBusyHost('')
    }
  }

  async function handlePairAndConnect(device: DiscoveredWirelessDevice) {
    const code = pairCode.trim()
    if (!/^\d{6}$/.test(code)) {
      setError('Enter the six-digit pairing code shown on the TV.')
      return
    }

    setBusyHost(device.host)
    setError(null)
    try {
      const result = await pairAndConnectWireless(device.host, code)
      await finishConnection(
        result.connectMessage || `Paired and connected to ${result.connectService.address}`,
      )
    } catch (pairError) {
      setError(pairError instanceof Error ? pairError.message : 'Pairing failed')
    } finally {
      setBusyHost('')
    }
  }

  async function handleManualConnect() {
    const address = manualAddress.trim()
    if (!address) {
      setError('Enter an address such as 192.168.1.100:5555.')
      return
    }

    setBusyHost('manual')
    setError(null)
    try {
      const message = await connectWireless(address)
      setManualAddress('')
      await finishConnection(message)
    } catch (connectError) {
      setError(connectError instanceof Error ? connectError.message : 'Connection failed')
    } finally {
      setBusyHost('')
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

  return (
    <Dialog open={open} onOpenChange={(next) => busyHost === '' && onOpenChange(next)}>
      <DialogContent className="sm:max-w-[620px] max-h-[88vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Wireless Android TV</DialogTitle>
          <DialogDescription>
            TVADB Hub discovers the current dynamic ADB ports automatically. For a new TV,
            open Developer options → Wireless debugging → Pair device with pairing code.
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
            <div className="flex gap-1.5">
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
                disabled={scanning || busyHost !== ''}
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
                  Enable Wireless debugging on the TV, then press Scan.
                </p>
              </div>
            ) : (
              devices.map((device) => {
                const pairing = Boolean(device.pairingAddress)
                const connectable = Boolean(device.connectAddress || device.legacyAddress)
                const busy = busyHost === device.host
                const pairingSelected = pairHost === device.host

                return (
                  <div key={device.host} className="rounded-lg border border-border/60 bg-card p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-xs font-semibold">{device.host}</p>
                        <p className="truncate font-mono text-[10px] text-muted-foreground">
                          {endpointLabel(device)}
                        </p>
                        <div className="mt-1.5 flex gap-1.5 text-[9px] font-medium uppercase tracking-wide">
                          {device.connectAddress && (
                            <span className="rounded bg-primary/10 px-1.5 py-0.5 text-primary">
                              TLS connect
                            </span>
                          )}
                          {pairing && (
                            <span className="rounded bg-muted px-1.5 py-0.5 text-muted-foreground">
                              Pairing
                            </span>
                          )}
                          {device.legacyAddress && (
                            <span className="rounded bg-muted px-1.5 py-0.5 text-muted-foreground">
                              Legacy
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
                        {connectable && (
                          <Button
                            size="sm"
                            className="h-7 text-[11px]"
                            onClick={() => void handleAutoConnect(device)}
                            disabled={busyHost !== ''}
                          >
                            {busy ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                            Connect
                          </Button>
                        )}
                        {pairing && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 text-[11px]"
                            onClick={() => {
                              setPairHost(pairingSelected ? '' : device.host)
                              setPairCode('')
                              setError(null)
                            }}
                            disabled={busyHost !== ''}
                          >
                            Pair
                          </Button>
                        )}
                      </div>
                    </div>

                    {pairing && pairingSelected && (
                      <div className="mt-3 flex items-end gap-2 border-t border-border/50 pt-3">
                        <div className="flex-1">
                          <label className="mb-1 block text-[10px] font-medium text-muted-foreground">
                            Six-digit code from TV
                          </label>
                          <Input
                            value={pairCode}
                            onChange={(event) =>
                              setPairCode(event.target.value.replace(/\D/g, '').slice(0, 6))
                            }
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' && pairCode.length === 6) {
                                void handlePairAndConnect(device)
                              }
                            }}
                            inputMode="numeric"
                            autoComplete="one-time-code"
                            placeholder="123456"
                            className="h-8 font-mono tracking-[0.25em]"
                            autoFocus
                            disabled={busy}
                          />
                        </div>
                        <Button
                          size="sm"
                          className="h-8"
                          onClick={() => void handlePairAndConnect(device)}
                          disabled={busyHost !== '' || pairCode.length !== 6}
                        >
                          {busy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
                          Pair & connect
                        </Button>
                      </div>
                    )}
                  </div>
                )
              })
            )}
          </div>

          {diagnostics && (
            <DiagnosticsPanel report={diagnostics} onClose={() => setDiagnostics(null)} />
          )}

          {remembered.length > 0 && (
            <div className="rounded-lg border border-border/50 bg-muted/5 p-3">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Remembered TVs · automatic reconnect
              </p>
              <div className="mt-2 space-y-1.5">
                {remembered.map((entry) => (
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
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
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
                ))}
              </div>
            </div>
          )}

          <div className="border-t border-border/50 pt-3">
            <p className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Manual fallback
            </p>
            <div className="flex gap-2">
              <Input
                value={manualAddress}
                onChange={(event) => setManualAddress(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void handleManualConnect()}
                placeholder="192.168.1.100:5555"
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
                {busyHost === 'manual' ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Wifi className="mr-1.5 h-3.5 w-3.5" />
                )}
                Connect
              </Button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-[11px] text-destructive">
              {error}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

import { useCallback, useEffect, useState } from 'react'
import {
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
  pairAndConnectWireless,
} from '@/services/deviceService'
import type { DiscoveredWirelessDevice } from '@/lib/types'
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

export function WirelessConnectDialog({
  open,
  onOpenChange,
  onConnected,
}: WirelessConnectDialogProps) {
  const [devices, setDevices] = useState<DiscoveredWirelessDevice[]>([])
  const [scanning, setScanning] = useState(false)
  const [busyHost, setBusyHost] = useState('')
  const [pairHost, setPairHost] = useState('')
  const [pairCode, setPairCode] = useState('')
  const [manualAddress, setManualAddress] = useState('')
  const [error, setError] = useState<string | null>(null)

  const scan = useCallback(async () => {
    setScanning(true)
    setError(null)
    try {
      const discovered = await discoverWirelessDevices()
      setDevices(discovered)
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

  return (
    <Dialog open={open} onOpenChange={(next) => busyHost === '' && onOpenChange(next)}>
      <DialogContent className="sm:max-w-[560px]">
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
            <Button variant="outline" size="sm" onClick={() => void scan()} disabled={scanning || busyHost !== ''}>
              {scanning ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              )}
              Scan
            </Button>
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

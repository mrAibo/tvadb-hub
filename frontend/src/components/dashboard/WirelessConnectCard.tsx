import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  IconRefresh as RefreshCw,
  IconUsb as Usb,
  IconWifi as Wifi,
} from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { WirelessConnectDialog } from '@/components/devices/WirelessConnectDialog'
import {
  discoverWirelessDevices,
  enableWirelessTCPIP,
  getRememberedWirelessDevices,
} from '@/services/deviceService'
import { useDevices } from '@/hooks/useDevices'
import { useDeviceStore } from '@/stores/useDeviceStore'
import type {
  DiscoveredWirelessDevice,
  RememberedWirelessDevice,
} from '@/lib/types'
import { toast } from 'sonner'

function isWirelessADBSerial(serial: string): boolean {
  const value = serial.trim()
  if (!value) return false
  if (value.startsWith('[')) return value.includes(']:')
  const separator = value.lastIndexOf(':')
  return separator > 0 && /^\d+$/.test(value.slice(separator + 1))
}

export function WirelessConnectCard() {
  const { refreshDevices } = useDevices()
  const adbDevices = useDeviceStore((state) => state.devices)
  const [wirelessOpen, setWirelessOpen] = useState(false)
  const [discovered, setDiscovered] = useState<DiscoveredWirelessDevice[]>([])
  const [remembered, setRemembered] = useState<RememberedWirelessDevice[]>([])
  const [scanning, setScanning] = useState(false)
  const [enablingLegacy, setEnablingLegacy] = useState(false)

  const connectedWirelessCount = useMemo(
    () =>
      adbDevices.filter(
        (device) =>
          device.mode === 'adb' &&
          device.state === 'device' &&
          isWirelessADBSerial(device.serial),
      ).length,
    [adbDevices],
  )

  const hasReadyAdbDevice = adbDevices.some(
    (device) => device.mode === 'adb' && device.state === 'device',
  )

  const scan = useCallback(async () => {
    setScanning(true)
    try {
      const [nextDiscovered, nextRemembered] = await Promise.all([
        discoverWirelessDevices(),
        getRememberedWirelessDevices(),
      ])
      setDiscovered(nextDiscovered)
      setRemembered(nextRemembered)
    } catch {
      // The full connection dialog exposes detailed diagnostics. Keep the
      // dashboard summary quiet if discovery is temporarily unavailable.
    } finally {
      setScanning(false)
    }
  }, [])

  useEffect(() => {
    void scan()
  }, [scan])

  async function handleLegacyTCPIP() {
    setEnablingLegacy(true)
    try {
      const message = await enableWirelessTCPIP('5555')
      toast.success('Legacy ADB TCP/IP enabled', { description: message })
    } catch (error) {
      toast.error('Could not enable legacy TCP/IP', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setEnablingLegacy(false)
    }
  }

  async function handleConnected() {
    await refreshDevices()
    await scan()
  }

  return (
    <>
      <Card className="border border-border/60 bg-card shadow-[var(--shadow-card)]">
        <CardHeader className="flex flex-row items-start justify-between gap-3 px-4 pb-2 pt-4">
          <div>
            <CardTitle className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">
              <Wifi className="h-3.5 w-3.5" />
              Wireless Android TV
            </CardTitle>
            <p className="mt-1 text-[11px] leading-relaxed text-muted-foreground">
              Dynamic pairing and connect ports are discovered automatically through ADB mDNS.
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 shrink-0"
            aria-label="Scan wireless ADB"
            onClick={() => void scan()}
            disabled={scanning}
          >
            <RefreshCw className={`h-3.5 w-3.5 ${scanning ? 'animate-spin' : ''}`} />
          </Button>
        </CardHeader>

        <CardContent className="px-4 pb-4">
          <div className="grid grid-cols-3 gap-2">
            <div className="rounded-lg border border-border/40 bg-muted/10 p-2.5">
              <p className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                Connected
              </p>
              <p className="mt-1 text-lg font-semibold tabular-nums">
                {connectedWirelessCount}
              </p>
            </div>
            <div className="rounded-lg border border-border/40 bg-muted/10 p-2.5">
              <p className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                Discovered
              </p>
              <p className="mt-1 text-lg font-semibold tabular-nums">
                {discovered.length}
              </p>
            </div>
            <div className="rounded-lg border border-border/40 bg-muted/10 p-2.5">
              <p className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                Remembered
              </p>
              <p className="mt-1 text-lg font-semibold tabular-nums">
                {remembered.length}
              </p>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="sm"
              className="h-8 flex-1 text-xs font-medium"
              onClick={() => setWirelessOpen(true)}
            >
              <Wifi className="mr-1.5 h-3.5 w-3.5" />
              Discover / Pair / Connect
            </Button>

            {hasReadyAdbDevice && (
              <Button
                size="sm"
                variant="outline"
                className="h-8 text-xs font-medium"
                onClick={() => void handleLegacyTCPIP()}
                disabled={enablingLegacy}
                title="Legacy fallback for devices that use adb tcpip 5555"
              >
                <Usb className="mr-1.5 h-3.5 w-3.5" />
                {enablingLegacy ? 'Enabling…' : 'Legacy TCP/IP'}
              </Button>
            )}
          </div>

          <p className="mt-2 text-[9px] leading-relaxed text-muted-foreground/70">
            Modern Android / Google TV normally does not use port 5555. TVADB Hub resolves
            the current TLS endpoint automatically.
          </p>
        </CardContent>
      </Card>

      <WirelessConnectDialog
        open={wirelessOpen}
        onOpenChange={setWirelessOpen}
        onConnected={handleConnected}
      />
    </>
  )
}

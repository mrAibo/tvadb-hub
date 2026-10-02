import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import {
  IconAlertTriangle as AlertTriangle,
  IconLoader2 as Loader2,
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
import { isWirelessTransportSerial, LEGACY_TCPIP_PORT } from '@/lib/wirelessPairing'
import type {
  DeviceSummary,
  DiscoveredWirelessDevice,
  RememberedWirelessDevice,
} from '@/lib/types'
import { toast } from 'sonner'

function targetLabel(device: DeviceSummary): string {
  return device.model || device.product || device.serial
}

export function WirelessConnectCard() {
  const { refreshDevices } = useDevices()
  const adbDevices = useDeviceStore((state) => state.devices)
  const [wirelessOpen, setWirelessOpen] = useState(false)
  const [discovered, setDiscovered] = useState<DiscoveredWirelessDevice[]>([])
  const [remembered, setRemembered] = useState<RememberedWirelessDevice[]>([])
  const [scanning, setScanning] = useState(false)
  const [enablingLegacy, setEnablingLegacy] = useState(false)
  // Consent is held as a serial, not a captured device. The consent epoch is bumped
  // by an explicit selection change only — never by the derived ready-target list:
  // enabling TCP/IP restarts adbd, so this very device legitimately disappears from
  // that list mid-command, and that must not invalidate the command's own feedback.
  const [pendingSerial, setPendingSerial] = useState<string | null>(null)
  // Kept after the target drops so the in-flight command can still name the serial
  // and model it was confirmed for. It never reselects a replacement device.
  const [confirmedSerial, setConfirmedSerial] = useState<string | null>(null)
  const [pendingDropped, setPendingDropped] = useState(false)
  const consentEpochRef = useRef(0)
  const confirmedSerialRef = useRef<string | null>(null)

  const readyTargets = useMemo(
    () => adbDevices.filter((device) => device.mode === 'adb' && device.state === 'device'),
    [adbDevices],
  )

  const pendingTarget = readyTargets.find((device) => device.serial === pendingSerial) ?? null
  // `pendingDropped` is the observable "the confirmed device left adb" flag. It is
  // set below, so reading it here never races the in-flight `enablingLegacy` state.
  const targetDropped = pendingDropped || (confirmedSerial !== null && pendingSerial === null)

  // A device that drops while it is confirmed is never re-confirmed automatically:
  // only the user may re-arm the command on whatever appears afterwards. The epoch
  // is deliberately NOT bumped here — a command that already ran must still report
  // its own result even though adbd disconnected the target it ran on.
  useLayoutEffect(() => {
    if (pendingSerial === null || pendingTarget) return
    setPendingDropped(true)
    setPendingSerial(null)
  }, [pendingSerial, pendingTarget])

  // The lone ready device is confirmed explicitly; several require an explicit choice.
  function confirmTarget(serial: string) {
    consentEpochRef.current += 1
    confirmedSerialRef.current = serial
    setPendingSerial(serial)
    setConfirmedSerial(serial)
    setPendingDropped(false)
  }

  function dropConsent() {
    consentEpochRef.current += 1
    confirmedSerialRef.current = null
    setPendingSerial(null)
    setConfirmedSerial(null)
    setPendingDropped(false)
  }

  const connectedWirelessCount = useMemo(
    () =>
      adbDevices.filter(
        (device) =>
          device.mode === 'adb' &&
          device.state === 'device' &&
          isWirelessTransportSerial(device.serial),
      ).length,
    [adbDevices],
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

  function requestLegacyTCPIP() {
    // Never pick a target for the user: one ready device may be implied, several
    // require an explicit choice, and nothing runs before the confirmation. A device
    // that dropped while it was confirmed is never re-armed automatically.
    if (readyTargets.length === 1 && !targetDropped) {
      confirmTarget(readyTargets[0].serial)
      return
    }
    // Collapsing the confirmation is an explicit change of the confirmed selection,
    // so a command that is still in flight is discarded instead of being reported
    // for a target the user just took back.
    dropConsent()
  }

  async function handleConfirmLegacyTCPIP() {
    const target = pendingTarget
    if (!target || enablingLegacy) return
    const serial = target.serial
    // The command is pinned to the explicitly confirmed serial before it runs: a
    // replacement device is never selected, and this guard never reads the derived
    // ready-target list.
    const consentEpoch = consentEpochRef.current
    const capturedLabel = targetLabel(target)
    setEnablingLegacy(true)
    try {
      const message = await enableWirelessTCPIP(LEGACY_TCPIP_PORT, serial)
      // Only a change of the user's explicit selection discards the result. adbd
      // restarting and dropping this very device from `adb devices` is expected and
      // must not hide the completion feedback.
      if (consentEpochRef.current !== consentEpoch || confirmedSerialRef.current !== serial) {
        return
      }
      const stillListed = useDeviceStore
        .getState()
        .devices.some((device) => device.serial === serial)
      if (stillListed) {
        toast.success(`Legacy ADB TCP/IP enabled on ${capturedLabel}`, {
          description: `${serial} · ${message || `adbd restarted, reconnect on port ${LEGACY_TCPIP_PORT} when it drops.`}`,
        })
      } else {
        toast.info('Legacy ADB TCP/IP command finished; the device is no longer listed', {
          description: `${capturedLabel} · ${serial} · ${message || `adbd restarted on port ${LEGACY_TCPIP_PORT}.`}`,
        })
      }
      dropConsent()
      await refreshDevices()
      await scan()
    } catch (error) {
      if (consentEpochRef.current !== consentEpoch || confirmedSerialRef.current !== serial) {
        return
      }
      toast.error('Could not enable legacy TCP/IP', {
        description: `${capturedLabel} · ${serial} · ${error instanceof Error ? error.message : String(error)}`,
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

            {readyTargets.length > 0 && (
              <Button
                size="sm"
                variant="outline"
                className="h-8 text-xs font-medium"
                onClick={requestLegacyTCPIP}
                disabled={enablingLegacy}
                title="Legacy fallback for devices that use adb tcpip 5555"
              >
                <Usb className="mr-1.5 h-3.5 w-3.5" />
                {enablingLegacy ? 'Enabling…' : 'Legacy TCP/IP'}
              </Button>
            )}
          </div>

          {readyTargets.length > 1 && !pendingTarget && !targetDropped && (
            <div className="mt-2.5 rounded-lg border border-border/50 bg-muted/5 p-2.5">
              <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Legacy TCP/IP target
              </p>
              <p className="mt-0.5 text-[10px] leading-relaxed text-muted-foreground">
                Port {LEGACY_TCPIP_PORT} restarts adbd on one device only. Choose which connected
                serial to switch.
              </p>
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {readyTargets.map((device) => (
                  <Button
                    key={device.serial}
                    size="sm"
                    variant="outline"
                    className="h-7 text-[11px]"
                    onClick={() => confirmTarget(device.serial)}
                  >
                    {targetLabel(device)}
                  </Button>
                ))}
              </div>
            </div>
          )}

          {(pendingTarget || targetDropped) && (
            <div className="mt-2.5 rounded-lg border border-amber-500/40 bg-amber-500/5 p-2.5">
              <p className="flex items-center gap-1.5 text-[11px] font-medium">
                <AlertTriangle className="h-3.5 w-3.5 text-amber-500" />
                {pendingTarget
                  ? `Enable legacy TCP/IP on ${targetLabel(pendingTarget)}?`
                  : `Legacy TCP/IP was confirmed for ${confirmedSerial}`}
              </p>
              <p className="mt-0.5 truncate font-mono text-[10px] text-muted-foreground">
                {pendingTarget
                  ? `${pendingTarget.serial}${pendingTarget.product ? ` · ${pendingTarget.product}` : ''}`
                  : confirmedSerial}
              </p>
              <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
                {pendingTarget ? (
                  <>
                    Warning: port {LEGACY_TCPIP_PORT} is unencrypted ADB and adbd restarts, so this
                    exact device drops off adb until you connect to it again. No pairing is
                    attempted automatically.
                  </>
                ) : (
                  <>
                    This confirmed serial is no longer a ready ADB device, so nothing is sent to any
                    other device. Reconnect it and confirm again, or dismiss this.
                  </>
                )}
              </p>
              <div className="mt-1.5 flex gap-1.5">
                <Button
                  size="sm"
                  className="h-7 text-[11px]"
                  onClick={() => void handleConfirmLegacyTCPIP()}
                  disabled={enablingLegacy || !pendingTarget}
                >
                  {enablingLegacy ? (
                    <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                  ) : (
                    <Usb className="mr-1 h-3 w-3" />
                  )}
                  Confirm on this serial
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-[11px]"
                  onClick={dropConsent}
                  disabled={enablingLegacy}
                >
                  {pendingTarget ? 'Cancel' : 'Dismiss'}
                </Button>
              </div>
            </div>
          )}

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

import { useState } from 'react'
import {
  IconRefresh as RefreshCw,
  IconWifi as Wifi,
} from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useDevices } from '@/hooks/useDevices'
import { WirelessConnectDialog } from '@/components/devices/WirelessConnectDialog'
import type { DeviceSummary } from '@/lib/types'

function DeviceRow({ device, nickname, isActive, onSelect }: {
  device: DeviceSummary
  nickname: string
  isActive: boolean
  onSelect: () => void
}) {
  const stateColor = device.state === 'device'
    ? 'text-green-500'
    : device.state === 'unauthorized'
      ? 'text-yellow-500'
      : device.state === 'offline'
        ? 'text-muted-foreground'
        : 'text-blue-400'

  const displayName = nickname || device.model || device.product || device.serial

  return (
    <button
      onClick={onSelect}
      className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-xs transition-colors ${
        isActive
          ? 'border-primary/40 bg-primary/5'
          : 'border-transparent hover:bg-accent/40'
      }`}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-medium">{displayName}</span>
        <span className="truncate text-muted-foreground">{device.serial}</span>
      </div>
      <span className={`text-[10px] font-medium ${stateColor}`}>
        {device.state}
      </span>
    </button>
  )
}

export function DeviceSidebar() {
  const {
    devices,
    activeSerial,
    nicknames,
    loading,
    refreshing,
    refreshDevices,
    selectDevice,
  } = useDevices()
  const [wirelessOpen, setWirelessOpen] = useState(false)

  const onlineCount = devices.filter((device) => device.state === 'device').length

  return (
    <>
      <div className="flex h-full w-56 flex-col overflow-hidden rounded-xl border border-border/50 bg-card">
        <div className="flex items-center justify-between border-b border-border/40 px-3 py-2">
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Devices
            </span>
            {devices.length > 0 && (
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[9px] font-medium text-muted-foreground">
                {onlineCount}/{devices.length}
              </span>
            )}
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={() => void refreshDevices()}
            disabled={refreshing}
          >
            <RefreshCw className={`h-3 w-3 ${refreshing ? 'animate-spin' : ''}`} />
          </Button>
        </div>

        <div className="flex-1 overflow-y-auto p-2">
          {loading && devices.length === 0 ? (
            <div className="flex flex-col gap-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : devices.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-8">
              <p className="text-center text-[11px] text-muted-foreground">No devices</p>
              <p className="text-center text-[10px] text-muted-foreground/60">
                Connect via USB or pair wirelessly
              </p>
            </div>
          ) : (
            <div className="flex flex-col gap-0.5">
              {devices.map((device) => (
                <DeviceRow
                  key={device.serial}
                  device={device}
                  nickname={nicknames[device.serial] ?? ''}
                  isActive={device.serial === activeSerial}
                  onSelect={() => void selectDevice(device.serial)}
                />
              ))}
            </div>
          )}
        </div>

        <div className="border-t border-border/40 p-2.5">
          <Button
            variant="outline"
            size="sm"
            className="h-8 w-full justify-start text-[11px]"
            onClick={() => setWirelessOpen(true)}
          >
            <Wifi className="mr-2 h-3.5 w-3.5" />
            Wireless TV
          </Button>
        </div>
      </div>

      <WirelessConnectDialog
        open={wirelessOpen}
        onOpenChange={setWirelessOpen}
        onConnected={() => refreshDevices()}
      />
    </>
  )
}

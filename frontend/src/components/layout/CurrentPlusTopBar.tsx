import { useMemo, useState } from 'react'
import {
  IconCheck as Check,
  IconChevronDown as ChevronDown,
  IconDeviceMobile as DeviceMobile,
  IconDeviceTv as DeviceTv,
  IconHistory as History,
  IconLoader2 as Loader2,
  IconRefresh as RefreshCw,
  IconWifi as Wifi,
} from '@tabler/icons-react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useDevices } from '@/hooks/useDevices'
import {
  autoConnectWireless,
  getRememberedWirelessDevices,
} from '@/services/deviceService'
import { cn } from '@/lib/utils'

function hostFromSerial(serial: string) {
  const trimmed = serial.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('[')) {
    const end = trimmed.indexOf(']')
    return end > 1 ? trimmed.slice(1, end) : trimmed
  }
  const idx = trimmed.lastIndexOf(':')
  return idx > 0 ? trimmed.slice(0, idx) : trimmed
}

export function CurrentPlusTopBar() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const {
    devices,
    activeSerial,
    deviceInfo,
    nicknames,
    refreshing,
    refreshDevices,
    selectDevice,
  } = useDevices()
  const [connectingKey, setConnectingKey] = useState<string | null>(null)

  const rememberedQuery = useQuery({
    queryKey: ['wireless', 'remembered-devices'],
    queryFn: getRememberedWirelessDevices,
    staleTime: 15_000,
  })

  const displayName = activeSerial
    ? nicknames[activeSerial] || deviceInfo?.model || deviceInfo?.product || activeSerial
    : 'No device selected'
  const connected = deviceInfo?.state === 'device'
  const secondary = deviceInfo?.ipAddress || activeSerial || 'Connect an Android device to begin'

  const connectedHosts = useMemo(
    () => new Set(devices.map((device) => hostFromSerial(device.serial).toLowerCase())),
    [devices],
  )

  const rememberedOffline = (rememberedQuery.data ?? []).filter((entry) => {
    const host = entry.host.trim().toLowerCase()
    return host && !connectedHosts.has(host)
  })

  async function handleRememberedConnect(key: string, selector: string) {
    setConnectingKey(key)
    try {
      const result = await autoConnectWireless(selector)
      await refreshDevices()
      await selectDevice(result.service.address)
      await queryClient.invalidateQueries({ queryKey: ['wireless', 'remembered-devices'] })
      toast.success('Remembered device connected', {
        description: result.service.address,
      })
    } catch (error) {
      toast.error('Could not reconnect remembered device', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setConnectingKey(null)
    }
  }

  return (
    <header className="shrink-0 px-6 pt-4">
      <div className="flex min-h-12 items-center gap-3 rounded-2xl border border-border/60 bg-card/75 px-3.5 py-2 shadow-[var(--shadow-card)] backdrop-blur-xl">
        <button
          type="button"
          onClick={() => navigate('/')}
          className="flex shrink-0 items-center gap-2.5 rounded-xl px-1.5 py-1 transition-colors hover:bg-muted/40"
          aria-label="Open dashboard"
        >
          <img src="/logo.png" alt="" className="h-7 w-7 rounded-lg object-contain" />
          <div className="hidden text-left sm:block">
            <div className="text-xs font-bold leading-none text-foreground">DroidSphere</div>
            <div className="mt-1 text-[9px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
              Current+
            </div>
          </div>
        </button>

        <div className="h-7 w-px bg-border/60" />

        <DropdownMenu>
          <DropdownMenuTrigger className="flex min-w-0 flex-1 items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-left outline-none transition-colors hover:bg-muted/35">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-primary/15 bg-primary/10 text-primary">
              {deviceInfo?.isTV ? <DeviceTv className="h-4 w-4" /> : <DeviceMobile className="h-4 w-4" />}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="truncate text-xs font-semibold text-foreground">{displayName}</span>
                <span
                  className={cn(
                    'h-1.5 w-1.5 shrink-0 rounded-full',
                    connected ? 'bg-emerald-500' : 'bg-muted-foreground/35',
                  )}
                />
                <span
                  className={cn(
                    'hidden text-[10px] font-medium md:inline',
                    connected ? 'text-emerald-500' : 'text-muted-foreground',
                  )}
                >
                  {connected ? 'Connected' : deviceInfo?.state ?? 'Idle'}
                </span>
              </div>
              <div className="mt-0.5 flex items-center gap-1.5 text-[10px] text-muted-foreground">
                {connected && <Wifi className="h-3 w-3 text-primary" />}
                <span className="truncate font-mono">{secondary}</span>
              </div>
            </div>
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          </DropdownMenuTrigger>

          <DropdownMenuContent align="start" className="w-[330px]">
            <DropdownMenuGroup>
              <DropdownMenuLabel className="text-[10px] uppercase tracking-wider text-muted-foreground">
                Connected devices
              </DropdownMenuLabel>

              {devices.length === 0 ? (
                <div className="px-2 py-2 text-xs text-muted-foreground">No connected devices.</div>
              ) : (
                devices.map((device) => {
                  const label = nicknames[device.serial] || device.model || device.product || device.serial
                  const selected = device.serial === activeSerial
                  return (
                    <DropdownMenuItem
                      key={device.serial}
                      onClick={() => void selectDevice(device.serial)}
                      className="flex items-center gap-2"
                    >
                      <DeviceMobile className="h-3.5 w-3.5 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-xs font-medium">{label}</div>
                        <div className="truncate font-mono text-[9px] text-muted-foreground">{device.serial}</div>
                      </div>
                      {selected && <Check className="h-3.5 w-3.5 text-primary" />}
                    </DropdownMenuItem>
                  )
                })
              )}
            </DropdownMenuGroup>

            {rememberedOffline.length > 0 && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuLabel className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <History className="h-3 w-3" />
                    Remembered devices
                  </DropdownMenuLabel>
                  {rememberedOffline.map((entry) => {
                    const selector = entry.instance_name || entry.host
                    const loading = connectingKey === entry.key
                    return (
                      <DropdownMenuItem
                        key={entry.key}
                        disabled={Boolean(connectingKey)}
                        onClick={() => void handleRememberedConnect(entry.key, selector)}
                        className="flex items-center gap-2"
                      >
                        {loading ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
                        ) : (
                          <Wifi className="h-3.5 w-3.5 text-muted-foreground" />
                        )}
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-xs font-medium">
                            {entry.name || entry.model || entry.host}
                          </div>
                          <div className="truncate font-mono text-[9px] text-muted-foreground">
                            {entry.host}
                            {entry.android_version ? ` · Android ${entry.android_version}` : ''}
                          </div>
                        </div>
                        <span className="text-[9px] font-medium text-primary">Reconnect</span>
                      </DropdownMenuItem>
                    )
                  })}
                </DropdownMenuGroup>
              </>
            )}

            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => navigate('/devices')}>
              <DeviceTv className="mr-2 h-3.5 w-3.5" />
              Open Device Manager
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <button
          type="button"
          onClick={() => void refreshDevices()}
          disabled={refreshing}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/60 text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground disabled:opacity-50"
          aria-label="Refresh devices"
          title="Refresh devices"
        >
          <RefreshCw className={cn('h-3.5 w-3.5', refreshing && 'animate-spin')} />
        </button>
      </div>
    </header>
  )
}

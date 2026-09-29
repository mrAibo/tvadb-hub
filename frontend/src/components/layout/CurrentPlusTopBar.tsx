import {
  IconDeviceTv as DeviceTv,
  IconRefresh as RefreshCw,
  IconWifi as Wifi,
} from '@tabler/icons-react'
import { useNavigate } from 'react-router-dom'
import { useDevices } from '@/hooks/useDevices'
import { cn } from '@/lib/utils'

export function CurrentPlusTopBar() {
  const navigate = useNavigate()
  const {
    activeSerial,
    deviceInfo,
    nicknames,
    refreshing,
    refreshDevices,
  } = useDevices()

  const displayName = activeSerial
    ? nicknames[activeSerial] || deviceInfo?.model || deviceInfo?.product || activeSerial
    : 'No device selected'
  const connected = deviceInfo?.state === 'device'
  const secondary = deviceInfo?.ipAddress || activeSerial || 'Connect a TV to begin'

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
            <div className="text-xs font-bold leading-none text-foreground">TVADB Hub</div>
            <div className="mt-1 text-[9px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
              Current+
            </div>
          </div>
        </button>

        <div className="h-7 w-px bg-border/60" />

        <button
          type="button"
          onClick={() => navigate('/devices')}
          className="flex min-w-0 flex-1 items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-left transition-colors hover:bg-muted/35"
        >
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-primary/15 bg-primary/10 text-primary">
            <DeviceTv className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="truncate text-xs font-semibold text-foreground">{displayName}</span>
              <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', connected ? 'bg-emerald-500' : 'bg-muted-foreground/35')} />
              <span className={cn('hidden text-[10px] font-medium md:inline', connected ? 'text-emerald-500' : 'text-muted-foreground')}>
                {connected ? 'Connected' : deviceInfo?.state ?? 'Idle'}
              </span>
            </div>
            <div className="mt-0.5 flex items-center gap-1.5 text-[10px] text-muted-foreground">
              {connected && <Wifi className="h-3 w-3 text-primary" />}
              <span className="truncate font-mono">{secondary}</span>
            </div>
          </div>
        </button>

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

import { useState } from 'react'
import {
  IconArrowBackUp as Back,
  IconChevronDown as ChevronDown,
  IconChevronLeft as ChevronLeft,
  IconChevronRight as ChevronRight,
  IconChevronUp as ChevronUp,
  IconCircleDot as Select,
  IconHome as Home,
  IconMenu2 as Menu,
  IconMoon as Sleep,
  IconPlayerPlayFilled as PlayPause,
  IconPower as Power,
  IconVolume2 as VolumeUp,
  IconVolume3 as Mute,
  IconVolumeOff as VolumeDown,
  IconSun as Wake,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { sendTVRemoteKey } from '@/services/deviceService'
import type { TVRemoteKey } from '@/lib/types'

interface TVRemotePanelProps {
  serial: string
}

interface RemoteButtonProps {
  label: string
  remoteKey: TVRemoteKey
  busyKey: TVRemoteKey | null
  onPress: (key: TVRemoteKey) => void
  icon: React.ElementType
  variant?: 'default' | 'outline' | 'secondary' | 'ghost' | 'destructive'
  className?: string
}

function RemoteButton({
  label,
  remoteKey,
  busyKey,
  onPress,
  icon: Icon,
  variant = 'outline',
  className = '',
}: RemoteButtonProps) {
  return (
    <Button
      type="button"
      variant={variant}
      size="icon"
      className={`h-10 w-10 rounded-xl ${className}`}
      aria-label={label}
      title={label}
      disabled={busyKey !== null}
      onClick={() => onPress(remoteKey)}
    >
      <Icon className={busyKey === remoteKey ? 'animate-pulse' : ''} />
    </Button>
  )
}

export function TVRemotePanel({ serial }: TVRemotePanelProps) {
  const [busyKey, setBusyKey] = useState<TVRemoteKey | null>(null)

  async function press(key: TVRemoteKey) {
    setBusyKey(key)
    try {
      await sendTVRemoteKey(key, serial)
    } catch (error) {
      toast.error('Remote command failed', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setBusyKey(null)
    }
  }

  return (
    <Card className="border border-border/60 bg-card shadow-[var(--shadow-card)]">
      <CardHeader className="flex flex-row items-start justify-between px-4 pb-2 pt-4">
        <div>
          <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">
            TV Remote
          </CardTitle>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Controls the selected TV through ADB key events.
          </p>
        </div>
        <div className="flex gap-1.5">
          <RemoteButton label="Wake" remoteKey="wake" busyKey={busyKey} onPress={press} icon={Wake} />
          <RemoteButton
            label="Power"
            remoteKey="power"
            busyKey={busyKey}
            onPress={press}
            icon={Power}
            variant="destructive"
          />
        </div>
      </CardHeader>

      <CardContent className="grid gap-4 px-4 pb-4 pt-2 md:grid-cols-[1fr_auto_1fr] md:items-center">
        <div className="grid grid-cols-3 gap-2 justify-self-center">
          <div />
          <RemoteButton label="Up" remoteKey="up" busyKey={busyKey} onPress={press} icon={ChevronUp} />
          <div />
          <RemoteButton label="Left" remoteKey="left" busyKey={busyKey} onPress={press} icon={ChevronLeft} />
          <RemoteButton
            label="Select"
            remoteKey="select"
            busyKey={busyKey}
            onPress={press}
            icon={Select}
            variant="default"
          />
          <RemoteButton label="Right" remoteKey="right" busyKey={busyKey} onPress={press} icon={ChevronRight} />
          <div />
          <RemoteButton label="Down" remoteKey="down" busyKey={busyKey} onPress={press} icon={ChevronDown} />
          <div />
        </div>

        <div className="hidden h-24 w-px bg-border/60 md:block" />

        <div className="grid grid-cols-4 gap-2 justify-self-center">
          <RemoteButton label="Home" remoteKey="home" busyKey={busyKey} onPress={press} icon={Home} />
          <RemoteButton label="Back" remoteKey="back" busyKey={busyKey} onPress={press} icon={Back} />
          <RemoteButton label="Menu" remoteKey="menu" busyKey={busyKey} onPress={press} icon={Menu} />
          <RemoteButton label="Play or pause" remoteKey="play_pause" busyKey={busyKey} onPress={press} icon={PlayPause} />
          <RemoteButton label="Volume up" remoteKey="volume_up" busyKey={busyKey} onPress={press} icon={VolumeUp} />
          <RemoteButton label="Volume down" remoteKey="volume_down" busyKey={busyKey} onPress={press} icon={VolumeDown} />
          <RemoteButton label="Mute" remoteKey="mute" busyKey={busyKey} onPress={press} icon={Mute} />
          <RemoteButton label="Sleep" remoteKey="sleep" busyKey={busyKey} onPress={press} icon={Sleep} />
        </div>
      </CardContent>
    </Card>
  )
}

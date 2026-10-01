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
  IconKeyboard as Keyboard,
  IconSend as Send,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { LauncherWizardTrigger } from '@/components/launcher/LauncherWizard'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { sendTVRemoteKey, sendTVText } from '@/services/deviceService'
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
  const [textDraft, setTextDraft] = useState('')
  const [sendingText, setSendingText] = useState(false)

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

  async function submitText() {
    if (textDraft === '' || sendingText) return

    setSendingText(true)
    try {
      const result = await sendTVText(textDraft, serial)
      setTextDraft('')
      toast.success('Text sent to TV', {
        description: result.detail,
      })
    } catch (error) {
      toast.error('Text input failed', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setSendingText(false)
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
          <LauncherWizardTrigger />
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

        <div className="md:col-span-3 border-t border-border/60 pt-3">
          <div className="flex items-center gap-2">
            <Keyboard className="h-4 w-4 shrink-0 text-muted-foreground" />
            <div className="relative flex min-w-0 flex-1 items-center">
              <Input
                aria-label="TV text input"
                value={textDraft}
                onChange={(event) => setTextDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    void submitText()
                  }
                }}
                placeholder="Type text for the focused TV field..."
                maxLength={8192}
                disabled={sendingText}
                className="h-9 pr-20 text-xs"
              />
              <Button
                type="button"
                size="sm"
                className="absolute right-1 h-7 gap-1 px-2.5 text-[10px]"
                disabled={textDraft === '' || sendingText}
                onClick={() => void submitText()}
                aria-label="Send text to TV"
              >
                <Send className={sendingText ? 'h-3 w-3 animate-pulse' : 'h-3 w-3'} />
                Send
              </Button>
            </div>
          </div>
          <p className="mt-1.5 pl-6 text-[10px] leading-relaxed text-muted-foreground">
            Unicode requires a detected Android clipboard command with verified readback. Otherwise only printable ASCII is supported. You can also paste Unicode in scrcpy’s own window.
          </p>
        </div>
      </CardContent>
    </Card>
  )
}

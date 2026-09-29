import { useState } from 'react'
import {
  IconBookmark as Bookmark,
  IconTrash as Trash2,
  IconDeviceTv as Television
} from "@tabler/icons-react"
import type { ScrcpyOptions, ScrcpyPreset } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'

const TV_PRESETS: Array<{
  id: string
  name: string
  description: string
  options: ScrcpyOptions
}> = [
  {
    id: 'tv-balanced',
    name: 'TV Balanced',
    description: '1080p-class mirror, 60 fps target, audio and control enabled.',
    options: {
      max_size: 1920,
      bit_rate: 8000000,
      max_fps: 60,
      audio_bit_rate: 128000,
      audio_codec: 'opus',
      video_codec: 'h264',
      show_touches: false,
      no_audio: false,
      no_control: false,
      stay_awake: true,
      turn_screen_off: false,
      power_off_on_close: false,
      fullscreen: true,
      always_on_top: false,
      disable_screensaver: true,
      rotation: 0,
      display_id: 0,
      time_limit: 0,
    },
  },
  {
    id: 'tv-quality',
    name: 'TV High Quality',
    description: 'Higher resolution and bitrate for fast wired or Wi-Fi 6 networks.',
    options: {
      max_size: 2560,
      bit_rate: 16000000,
      max_fps: 60,
      audio_bit_rate: 192000,
      audio_codec: 'opus',
      video_codec: 'h264',
      show_touches: false,
      no_audio: false,
      no_control: false,
      stay_awake: true,
      turn_screen_off: false,
      power_off_on_close: false,
      fullscreen: true,
      always_on_top: false,
      disable_screensaver: true,
      rotation: 0,
      display_id: 0,
      time_limit: 0,
    },
  },
  {
    id: 'tv-low-bandwidth',
    name: 'TV Low Bandwidth',
    description: '720p-class mirror at 30 fps for weaker wireless links.',
    options: {
      max_size: 1280,
      bit_rate: 4000000,
      max_fps: 30,
      audio_bit_rate: 96000,
      audio_codec: 'opus',
      video_codec: 'h264',
      show_touches: false,
      no_audio: false,
      no_control: false,
      stay_awake: true,
      turn_screen_off: false,
      power_off_on_close: false,
      fullscreen: false,
      always_on_top: false,
      disable_screensaver: true,
      rotation: 0,
      display_id: 0,
      time_limit: 0,
    },
  },
]

interface PresetsManagerProps {
  currentOptions: ScrcpyOptions
  presets: ScrcpyPreset[]
  onApplyPreset: (options: ScrcpyOptions) => void
  onSavePreset: (name: string) => void
  onDeletePreset: (id: string) => void
}

export function PresetsManager({
  currentOptions,
  presets,
  onApplyPreset,
  onSavePreset,
  onDeletePreset,
}: PresetsManagerProps) {
  const [draftName, setDraftName] = useState('')

  const handleSave = () => {
    const trimmed = draftName.trim()
    if (!trimmed) return
    onSavePreset(trimmed)
    setDraftName('')
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/50 bg-background/50 p-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div className="flex-1 space-y-1.5">
            <label className="text-sm font-medium" htmlFor="preset-name">
              Save current options as preset
            </label>
            <Input
              id="preset-name"
              value={draftName}
              onChange={(e) => setDraftName(e.target.value)}
              placeholder="e.g. Low-latency, High quality"
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  handleSave()
                }
              }}
            />
          </div>
          <Button
            type="button"
            onClick={handleSave}
            disabled={!draftName.trim()}
            className="gap-2"
          >
            <Bookmark className="h-4 w-4" />
            Save
          </Button>
        </div>
        <p className="mt-2 text-xs text-muted-foreground">
          Captures max size, bitrate, fps, codecs, audio, and device toggles.
        </p>
      </Card>

      <Card className="border-primary/20 bg-primary/[0.03] p-4">
        <div className="mb-3 flex items-start gap-2">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Television className="h-4 w-4" />
          </div>
          <div>
            <h4 className="text-sm font-semibold">TVADB Hub TV presets</h4>
            <p className="mt-0.5 text-xs text-muted-foreground">
              One-click Scrcpy profiles tuned for Android TV and Google TV.
            </p>
          </div>
        </div>
        <div className="grid gap-2">
          {TV_PRESETS.map((preset) => (
            <div
              key={preset.id}
              className="flex items-center justify-between gap-3 rounded-lg border border-border/40 bg-background/50 p-3"
            >
              <div className="min-w-0">
                <p className="text-sm font-medium">{preset.name}</p>
                <p className="mt-0.5 text-[11px] leading-relaxed text-muted-foreground">
                  {preset.description}
                </p>
                <p className="mt-1 font-mono text-[10px] text-muted-foreground">
                  {preset.options.max_size}px · {(preset.options.bit_rate / 1_000_000).toFixed(0)} Mbps · {preset.options.max_fps} fps
                </p>
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="shrink-0"
                onClick={() => onApplyPreset({ ...preset.options })}
              >
                Apply
              </Button>
            </div>
          ))}
        </div>
      </Card>

      <Separator />

      <div className="space-y-2">
        <h4 className="text-sm font-semibold">Saved presets</h4>
        {presets.length === 0 ? (
          <p className="rounded-md border border-dashed border-border/40 p-4 text-center text-xs text-muted-foreground">
            No presets yet. Save your current options to create one.
          </p>
        ) : (
          <ScrollArea className="max-h-64 pr-2">
            <ul className="space-y-2">
              {presets.map((preset) => (
                <li
                  key={preset.id}
                  className="flex items-center justify-between gap-2 rounded-md border border-border/40 bg-background/40 p-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{preset.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {new Date(preset.createdAt).toLocaleString()}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => onApplyPreset(preset.options)}
                    >
                      Apply
                    </Button>
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="h-8 w-8 text-muted-foreground hover:text-destructive"
                      onClick={() => onDeletePreset(preset.id)}
                      aria-label={`Delete preset ${preset.name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </ScrollArea>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Active options: max {currentOptions.max_size === 0 ? '∞' : currentOptions.max_size}px ·{' '}
        {(currentOptions.bit_rate / 1_000_000).toFixed(1)} Mbps ·{' '}
        {currentOptions.max_fps === 0 ? '∞' : currentOptions.max_fps} fps
      </p>
    </div>
  )
}

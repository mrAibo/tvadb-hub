import type { ScrcpyAudioSource, ScrcpyOptions } from '@/lib/types'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface AudioControlsProps {
  options: ScrcpyOptions
  sdkVersion?: string
  onOptionChange: (key: string, value: number | string | boolean) => void
}

interface AudioChoice {
  value: string
  name: string
  description: string
}

const AUDIO_CODECS: AudioChoice[] = [
  { value: 'opus', name: 'Opus', description: 'Default, low latency' },
  { value: 'aac', name: 'AAC', description: 'Wide device support' },
  { value: 'flac', name: 'FLAC', description: 'Lossless, large files' },
  { value: 'raw', name: 'RAW', description: 'Uncompressed PCM' },
]

const AUDIO_SOURCES: Array<AudioChoice & { value: ScrcpyAudioSource; minSdk: number }> = [
  { value: 'output', name: 'Device output', description: 'Forward the complete device output', minSdk: 30 },
  { value: 'playback', name: 'Playback', description: 'Capture app playback where Android permits it', minSdk: 33 },
  { value: 'mic', name: 'Microphone', description: 'Capture the device microphone', minSdk: 30 },
]

export interface ScrcpyAudioCapability {
  detected: boolean
  sdk: number | null
  audioSupported: boolean
  playbackSupported: boolean
  hint: string
}

export function getScrcpyAudioCapability(sdkVersion?: string): ScrcpyAudioCapability {
  const sdk = Number.parseInt(sdkVersion ?? '', 10)
  if (!Number.isFinite(sdk)) {
    return {
      detected: false,
      sdk: null,
      audioSupported: true,
      playbackSupported: true,
      hint: 'Audio requires Android 11+; Playback source requires Android 13+.',
    }
  }
  if (sdk < 30) {
    return {
      detected: true,
      sdk,
      audioSupported: false,
      playbackSupported: false,
      hint: 'Audio forwarding requires Android 11 or newer on the selected device.',
    }
  }
  if (sdk === 30) {
    return {
      detected: true,
      sdk,
      audioSupported: true,
      playbackSupported: false,
      hint: 'Android 11 audio works when the device is unlocked; Playback source requires Android 13+.',
    }
  }
  if (sdk < 33) {
    return {
      detected: true,
      sdk,
      audioSupported: true,
      playbackSupported: false,
      hint: 'Output and microphone are available; Playback source requires Android 13+.',
    }
  }
  return {
    detected: true,
    sdk,
    audioSupported: true,
    playbackSupported: true,
    hint: 'Output, playback and microphone sources are available on this Android version.',
  }
}

export function AudioControls({ options, sdkVersion, onOptionChange }: AudioControlsProps) {
  const capability = getScrcpyAudioCapability(sdkVersion)
  const sourceDisabled = options.no_audio || !capability.audioSupported
  const audioOnlyDisabled = options.no_audio || !capability.audioSupported

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4 py-1">
        <div className="space-y-0.5">
          <Label htmlFor="no-audio" className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
            Disable Audio
          </Label>
          <p className="text-[10px] text-muted-foreground leading-relaxed">
            Disable audio forwarding for ordinary video mirroring.
          </p>
        </div>
        <Switch
          id="no-audio"
          checked={options.no_audio}
          disabled={options.audio_only}
          onCheckedChange={(checked) => onOptionChange('no_audio', checked)}
        />
      </div>

      <div className="flex items-center justify-between gap-4 border-t border-border/50 pt-3">
        <div className="space-y-0.5">
          <Label htmlFor="audio-only" className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
            Audio-only
          </Label>
          <p className="text-[10px] text-muted-foreground leading-relaxed">
            Start scrcpy with no video and no device control.
          </p>
        </div>
        <Switch
          id="audio-only"
          checked={options.audio_only}
          disabled={audioOnlyDisabled}
          onCheckedChange={(checked) => onOptionChange('audio_only', checked)}
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="audio-source" className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
          Audio Source
        </Label>
        <Select
          value={options.audio_source || 'output'}
          onValueChange={(value) => onOptionChange('audio_source', String(value ?? 'output'))}
          disabled={sourceDisabled}
        >
          <SelectTrigger id="audio-source" className="h-8.5 rounded-full text-xs bg-white dark:bg-zinc-900/60 border border-zinc-200 dark:border-zinc-800 focus:ring-1 focus:ring-zinc-400">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="min-w-[280px] rounded-2xl border-zinc-200 dark:border-zinc-800">
            {AUDIO_SOURCES.map((source) => (
              <SelectItem
                key={source.value}
                value={source.value}
                disabled={capability.detected && (capability.sdk ?? 0) < source.minSdk}
                className="py-2 cursor-pointer"
              >
                <div className="flex flex-col gap-0.5">
                  <span className="text-xs font-semibold">{source.name}</span>
                  <span className="text-[10px] text-muted-foreground whitespace-normal">
                    {source.description}{source.value === 'playback' ? ' · Android 13+' : ''}
                  </span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className={capability.audioSupported ? 'text-[10px] text-muted-foreground' : 'text-[10px] text-destructive'}>
          {capability.hint}
        </p>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="audio-codec" className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
          Audio Codec
        </Label>
        <Select
          value={options.audio_codec || 'opus'}
          onValueChange={(value) => onOptionChange('audio_codec', String(value ?? 'opus'))}
          disabled={sourceDisabled}
        >
          <SelectTrigger id="audio-codec" className="h-8.5 rounded-full text-xs bg-white dark:bg-zinc-900/60 border border-zinc-200 dark:border-zinc-800 focus:ring-1 focus:ring-zinc-400">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="min-w-[240px] rounded-2xl border-zinc-200 dark:border-zinc-800">
            {AUDIO_CODECS.map((codec) => (
              <SelectItem key={codec.value} value={codec.value} className="py-2 cursor-pointer">
                <div className="flex flex-col gap-0.5">
                  <span className="font-mono text-xs font-semibold">{codec.name}</span>
                  <span className="text-[10px] text-muted-foreground whitespace-normal">
                    {codec.description}
                  </span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label htmlFor="audio-bit-rate" className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
            Audio Bitrate
          </Label>
          <span className="text-[10px] font-bold text-muted-foreground font-mono">
            {options.audio_bit_rate === 0
              ? 'Default'
              : `${(options.audio_bit_rate / 1000).toFixed(0)} kbps`}
          </span>
        </div>
        <div className="relative flex items-center">
          <Input
            id="audio-bit-rate"
            type="number"
            min={0}
            step={8}
            value={options.audio_bit_rate === 0 ? 0 : Math.round(options.audio_bit_rate / 1000)}
            disabled={sourceDisabled}
            onChange={(e) => {
              const kbps = parseFloat(e.target.value) || 0
              onOptionChange('audio_bit_rate', Math.round(kbps * 1000))
            }}
            className="h-8.5 w-full rounded-full border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/60 focus-visible:ring-1 focus-visible:ring-zinc-400 dark:focus-visible:ring-zinc-700 text-xs pl-3.5 pr-8"
          />
          <span className="absolute right-3 text-[9px] text-zinc-400 dark:text-zinc-500 pointer-events-none font-mono">k</span>
        </div>
      </div>
    </div>
  )
}

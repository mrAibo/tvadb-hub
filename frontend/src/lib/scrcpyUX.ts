import type { ScrcpyOptions } from '@/lib/types'

export function describeScrcpyError(
  err: unknown,
): { title: string; description: string } {
  const raw = err instanceof Error ? err.message : String(err)
  const msg = raw.toLowerCase()

  if (
    msg.includes('audio-only') ||
    msg.includes('audio source') ||
    msg.includes('android 11') ||
    msg.includes('android 13') ||
    msg.includes('validate_scrcpy_audio')
  ) {
    return { title: 'Audio mode unavailable', description: raw }
  }

  if (
    msg.includes('binary') ||
    msg.includes('scrcpy executable') ||
    msg.includes("binary 'scrcpy'")
  ) {
    return {
      title: 'Scrcpy binary missing',
      description:
        'Scrcpy executable not found. Install scrcpy or configure the path in Settings.',
    }
  }

  if (msg.includes('unauthorized') || msg.includes('permission')) {
    return {
      title: 'Permission denied',
      description:
        'Device authorization required. Approve USB debugging on the device.',
    }
  }

  if (msg.includes('device') || msg.includes('adb')) {
    return { title: 'Device connection error', description: raw }
  }

  return { title: 'Operation failed', description: raw }
}

export interface ScrcpyRecordingDescriptor {
  prefix: string
  extension: string
  title: string
  description: string
}

export function getScrcpyRecordingDescriptor(
  options: Pick<ScrcpyOptions, 'audio_only' | 'audio_codec'>,
): ScrcpyRecordingDescriptor {
  if (!options.audio_only) {
    return {
      prefix: 'scrcpy-recording',
      extension: 'mp4',
      title: 'Recording started',
      description: 'Screen recording in progress',
    }
  }

  const extension =
    options.audio_codec === 'aac'
      ? 'aac'
      : options.audio_codec === 'flac'
        ? 'flac'
        : options.audio_codec === 'raw'
          ? 'wav'
          : 'opus'

  return {
    prefix: 'scrcpy-audio',
    extension,
    title: 'Audio recording started',
    description: 'Audio-only recording in progress',
  }
}

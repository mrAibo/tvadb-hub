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

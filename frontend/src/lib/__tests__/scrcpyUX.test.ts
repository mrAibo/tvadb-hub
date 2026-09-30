import { describe, expect, it } from 'vitest'
import {
  describeScrcpyError,
  getScrcpyRecordingDescriptor,
} from '../scrcpyUX'

describe('scrcpy UX helpers', () => {
  it('does not misclassify audio compatibility errors as a missing binary', () => {
    const result = describeScrcpyError(
      new Error('validate_scrcpy_audio: Playback audio source requires Android 13 or newer'),
    )
    expect(result.title).toBe('Audio mode unavailable')
    expect(result.description).toContain('Android 13')
  })

  it('still recognizes actual binary errors', () => {
    const result = describeScrcpyError(
      new Error("Required binary is not ready: binary 'scrcpy' is unavailable"),
    )
    expect(result.title).toBe('Scrcpy binary missing')
  })

  it('uses codec-appropriate extensions for audio-only recordings', () => {
    expect(getScrcpyRecordingDescriptor({ audio_only: true, audio_codec: 'opus' }).extension).toBe('opus')
    expect(getScrcpyRecordingDescriptor({ audio_only: true, audio_codec: 'aac' }).extension).toBe('aac')
    expect(getScrcpyRecordingDescriptor({ audio_only: true, audio_codec: 'flac' }).extension).toBe('flac')
    expect(getScrcpyRecordingDescriptor({ audio_only: true, audio_codec: 'raw' }).extension).toBe('wav')
    expect(getScrcpyRecordingDescriptor({ audio_only: false, audio_codec: 'opus' }).extension).toBe('mp4')
  })
})

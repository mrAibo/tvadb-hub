import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { AudioControls } from '../AudioControls'
import { getScrcpyAudioCapability } from '../audioCapabilities'
import type { ScrcpyOptions } from '@/lib/types'

const options: ScrcpyOptions = {
  max_size: 0,
  bit_rate: 8000000,
  max_fps: 0,
  audio_bit_rate: 128000,
  audio_codec: 'opus',
  audio_source: 'output',
  audio_only: false,
  video_codec: 'h264',
  show_touches: false,
  no_audio: false,
  no_control: false,
  stay_awake: true,
  turn_screen_off: false,
  power_off_on_close: false,
  fullscreen: false,
  always_on_top: false,
  disable_screensaver: false,
  rotation: 0,
  display_id: 0,
  time_limit: 0,
}

describe('AudioControls capabilities', () => {
  it('recognises Android audio and playback thresholds', () => {
    expect(getScrcpyAudioCapability('29').audioSupported).toBe(false)
    expect(getScrcpyAudioCapability('30').audioSupported).toBe(true)
    expect(getScrcpyAudioCapability('32').playbackSupported).toBe(false)
    expect(getScrcpyAudioCapability('33').playbackSupported).toBe(true)
  })

  it('disables audio-only below Android 11', () => {
    render(<AudioControls options={options} sdkVersion="29" onOptionChange={() => {}} />)
    expect(screen.getByRole('switch', { name: 'Audio-only' })).toBeDisabled()
    expect(screen.getByText(/requires Android 11 or newer/i)).toBeInTheDocument()
  })

  it('updates audio-only on supported devices', () => {
    const onOptionChange = vi.fn()
    render(<AudioControls options={options} sdkVersion="34" onOptionChange={onOptionChange} />)
    fireEvent.click(screen.getByRole('switch', { name: 'Audio-only' }))
    expect(onOptionChange).toHaveBeenCalledWith('audio_only', true)
  })

  it('prevents disabling audio while audio-only is active', () => {
    render(
      <AudioControls
        options={{ ...options, audio_only: true }}
        sdkVersion="34"
        onOptionChange={() => {}}
      />,
    )
    expect(screen.getByRole('switch', { name: 'Disable Audio' })).toBeDisabled()
  })
})

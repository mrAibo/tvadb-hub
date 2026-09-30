import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { PresetsManager } from '../PresetsManager'
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

describe('PresetsManager TV presets', () => {
  it('applies the balanced TV preset', () => {
    const onApplyPreset = vi.fn()

    render(
      <PresetsManager
        currentOptions={options}
        presets={[]}
        onApplyPreset={onApplyPreset}
        onSavePreset={() => {}}
        onDeletePreset={() => {}}
      />,
    )

    const card = screen.getByText('TV Balanced').closest('div.rounded-lg')
    expect(card).not.toBeNull()
    const applyButton = card?.querySelector('button')
    expect(applyButton).not.toBeNull()
    fireEvent.click(applyButton!)

    expect(onApplyPreset).toHaveBeenCalledTimes(1)
    const applied = onApplyPreset.mock.calls[0][0] as ScrcpyOptions
    expect(applied.max_size).toBe(1920)
    expect(applied.max_fps).toBe(60)
    expect(applied.bit_rate).toBe(8000000)
    expect(applied.fullscreen).toBe(true)
    expect(applied.disable_screensaver).toBe(true)
  })

  it('renders quality and low bandwidth TV profiles', () => {
    render(
      <PresetsManager
        currentOptions={options}
        presets={[]}
        onApplyPreset={() => {}}
        onSavePreset={() => {}}
        onDeletePreset={() => {}}
      />,
    )

    expect(screen.getByText('TV High Quality')).toBeInTheDocument()
    expect(screen.getByText('TV Low Bandwidth')).toBeInTheDocument()
  })
})

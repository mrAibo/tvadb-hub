import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppConfigSnapshot, ScrcpyOptions } from '@/lib/types'

const mocks = vi.hoisted(() => ({
  getAppConfig: vi.fn(),
  updatePreferences: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
  getActiveScrcpySession: vi.fn().mockResolvedValue(null),
  getScrcpyEncoderSupport: vi.fn().mockResolvedValue(null),
  onScrcpySessionStarted: vi.fn(() => () => {}),
  onScrcpySessionStopped: vi.fn(() => () => {}),
  onScrcpyError: vi.fn(() => () => {}),
  selectScrcpySaveFile: vi.fn().mockResolvedValue(''),
  startScrcpyRecording: vi.fn().mockResolvedValue(undefined),
  startScrcpySession: vi.fn().mockResolvedValue(undefined),
  stopScrcpyRecording: vi.fn().mockResolvedValue(undefined),
  stopScrcpySession: vi.fn().mockResolvedValue(undefined),
  takeScrcpyScreenshot: vi.fn().mockResolvedValue(undefined),
  getScrcpyClipboard: vi.fn().mockResolvedValue(''),
  pushScrcpyClipboard: vi.fn().mockResolvedValue(undefined),
  updateScrcpyOptions: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/services/settingsService', () => ({
  getAppConfig: mocks.getAppConfig,
  updatePreferences: mocks.updatePreferences,
}))

vi.mock('@/services/scrcpyService', () => ({
  getActiveScrcpySession: mocks.getActiveScrcpySession,
  getScrcpyEncoderSupport: mocks.getScrcpyEncoderSupport,
  onScrcpySessionStarted: mocks.onScrcpySessionStarted,
  onScrcpySessionStopped: mocks.onScrcpySessionStopped,
  onScrcpyError: mocks.onScrcpyError,
  selectScrcpySaveFile: mocks.selectScrcpySaveFile,
  startScrcpyRecording: mocks.startScrcpyRecording,
  startScrcpySession: mocks.startScrcpySession,
  stopScrcpyRecording: mocks.stopScrcpyRecording,
  stopScrcpySession: mocks.stopScrcpySession,
  takeScrcpyScreenshot: mocks.takeScrcpyScreenshot,
  getScrcpyClipboard: mocks.getScrcpyClipboard,
  pushScrcpyClipboard: mocks.pushScrcpyClipboard,
  updateScrcpyOptions: mocks.updateScrcpyOptions,
}))

vi.mock('sonner', () => ({ toast: mocks.toast }))

import { useScrcpy } from '../useScrcpy'
import { useScrcpyStore } from '@/stores/scrcpyStore'

const OPTIONS: ScrcpyOptions = {
  max_size: 1920,
  bit_rate: 8000000,
  max_fps: 60,
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
  fullscreen: true,
  always_on_top: false,
  disable_screensaver: true,
  rotation: 0,
  display_id: 0,
  time_limit: 0,
}

function savedConfig(overrides: Partial<AppConfigSnapshot> = {}): AppConfigSnapshot {
  return {
    adb_path: 'adb',
    fastboot_path: 'fastboot',
    scrcpy_path: 'scrcpy',
    setup_completed: true,
    theme: 'dark',
    binary_versions: {},
    device_nicknames: {},
    logcat_buffer_limit: 5000,
    scrcpy_options: OPTIONS,
    scrcpy_presets: [{ name: 'Saved TV', options: OPTIONS }],
    default_terminal_mode: 'shell',
    auto_refresh_devices: true,
    device_refresh_seconds: 8,
    audit_enabled: true,
    file_transfer_compression: 'off',
    verify_after_transfer: false,
    safe_tuning_feed_url: '',
    safe_tuning_feed_public_key: '',
    ...overrides,
  }
}

beforeEach(() => {
  mocks.getAppConfig.mockReset()
  mocks.updatePreferences.mockReset()
  mocks.toast.success.mockReset()
  mocks.toast.error.mockReset()
  mocks.getAppConfig.mockImplementation(async () => savedConfig())
  mocks.updatePreferences.mockImplementation(async () => savedConfig())
  useScrcpyStore.getState().reset()
})

describe('useScrcpy preset persistence', () => {
  it('hydrates saved presets and options on startup', async () => {
    renderHook(() => useScrcpy())

    await waitFor(() => {
      expect(useScrcpyStore.getState().presetsHydrated).toBe(true)
    })
    expect(useScrcpyStore.getState().presets.map((preset) => preset.name)).toEqual(['Saved TV'])
    expect(useScrcpyStore.getState().options.max_size).toBe(1920)
  })

  it('saves a preset through the durable preferences API', async () => {
    const { result } = renderHook(() => useScrcpy())
    await waitFor(() => {
      expect(useScrcpyStore.getState().presetsHydrated).toBe(true)
    })

    await act(async () => {
      await result.current.handleSavePreset('  New TV  ')
    })

    const payload = mocks.updatePreferences.mock.calls[0][0]
    expect(payload.auto_refresh_devices).toBe(true)
    expect(payload.scrcpy_presets).toEqual([
      { name: 'New TV', options: useScrcpyStore.getState().options },
      { name: 'Saved TV', options: OPTIONS },
    ])
    expect(useScrcpyStore.getState().presets[0].name).toBe('New TV')
    expect(mocks.toast.success).toHaveBeenCalledWith('Preset saved')
  })

  it('reports a failed save instead of claiming success', async () => {
    const { result } = renderHook(() => useScrcpy())
    await waitFor(() => {
      expect(useScrcpyStore.getState().presetsHydrated).toBe(true)
    })
    mocks.updatePreferences.mockImplementation(async () => {
      throw new Error('write failed')
    })

    await act(async () => {
      await result.current.handleSavePreset('Doomed')
    })

    expect(mocks.toast.success).not.toHaveBeenCalled()
    expect(mocks.toast.error).toHaveBeenCalled()
    expect(useScrcpyStore.getState().presetsPersistError).toBe('write failed')
  })

  it('keeps a preset added while the initial config is still loading', async () => {
    const pendingConfigs: Array<(config: AppConfigSnapshot) => void> = []
    mocks.getAppConfig.mockImplementation(
      () =>
        new Promise<AppConfigSnapshot>((resolve) => {
          pendingConfigs.push(resolve)
        }),
    )

    const { result } = renderHook(() => useScrcpy())

    let savePromise: Promise<void> = Promise.resolve()
    act(() => {
      savePromise = result.current.handleSavePreset('Added early')
    })

    // Release both the hydration read and the queued write read.
    await act(async () => {
      pendingConfigs.forEach((resolve) => resolve(savedConfig()))
      await Promise.resolve()
    })
    await act(async () => {
      await savePromise
    })

    const names = useScrcpyStore.getState().presets.map((preset) => preset.name)
    expect(names).toContain('Added early')
    // The snapshot that arrived later must not overwrite the live preset.
    expect(names).not.toContain('Saved TV')
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppConfigSnapshot, ScrcpyOptions } from '@/lib/types'

const mocks = vi.hoisted(() => ({
  getAppConfig: vi.fn(),
  updatePreferences: vi.fn(),
}))

vi.mock('@/services/settingsService', () => ({
  getAppConfig: mocks.getAppConfig,
  updatePreferences: mocks.updatePreferences,
}))

import { useScrcpyStore } from '../scrcpyStore'

const OPTIONS: ScrcpyOptions = {
  max_size: 1280,
  bit_rate: 4000000,
  max_fps: 30,
  audio_bit_rate: 96000,
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
    scrcpy_presets: [],
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
  mocks.getAppConfig.mockImplementation(async () => savedConfig())
  mocks.updatePreferences.mockImplementation(async () => savedConfig())
  useScrcpyStore.getState().reset()
})

describe('useScrcpyStore options', () => {
  it('updates the active Scrcpy options', () => {
    useScrcpyStore.getState().setOptions({
      ...useScrcpyStore.getState().options,
      max_size: 1440,
      max_fps: 60,
    })

    expect(useScrcpyStore.getState().options.max_size).toBe(1440)
    expect(useScrcpyStore.getState().options.max_fps).toBe(60)
  })

  it('restores the default options on reset', () => {
    useScrcpyStore.getState().setOptions({
      ...useScrcpyStore.getState().options,
      max_size: 1440,
      no_audio: true,
    })

    useScrcpyStore.getState().reset()

    expect(useScrcpyStore.getState().options.max_size).toBe(0)
    expect(useScrcpyStore.getState().options.no_audio).toBe(false)
    expect(useScrcpyStore.getState().options.audio_source).toBe('output')
    expect(useScrcpyStore.getState().options.audio_only).toBe(false)
    expect(useScrcpyStore.getState().options.stay_awake).toBe(true)
  })
})

describe('useScrcpyStore preset persistence', () => {
  it('hydrates saved snapshots and keeps client IDs stable across reloads', () => {
    const store = useScrcpyStore.getState()
    store.hydratePresets([{ name: 'TV', options: OPTIONS }], useScrcpyStore.getState().presetsRevision)

    const [first] = useScrcpyStore.getState().presets
    expect(first.name).toBe('TV')
    expect(first.id).toBeTruthy()
    expect(first.options).toEqual(OPTIONS)

    // A second hydration of the same saved entry must reuse the live client ID.
    useScrcpyStore
      .getState()
      .hydratePresets(
        [{ name: 'TV', options: OPTIONS }],
        useScrcpyStore.getState().presetsRevision,
      )
    expect(useScrcpyStore.getState().presets).toHaveLength(1)
    expect(useScrcpyStore.getState().presets[0].id).toBe(first.id)
    expect(useScrcpyStore.getState().presetsHydrated).toBe(true)
  })

  it('persists {name, options} while preserving the saved auto-refresh flag', async () => {
    mocks.getAppConfig.mockImplementation(async () =>
      savedConfig({ theme: 'light', auto_refresh_devices: true }),
    )

    const saved = await useScrcpyStore.getState().savePreset('Mine', OPTIONS)

    expect(saved).toBe(true)
    expect(mocks.updatePreferences).toHaveBeenCalledTimes(1)
    const payload = mocks.updatePreferences.mock.calls[0][0]
    expect(payload).toEqual({
      auto_refresh_devices: true,
      scrcpy_presets: [{ name: 'Mine', options: OPTIONS }],
    })
    expect(payload).not.toHaveProperty('theme')
    expect(useScrcpyStore.getState().presetsPersistError).toBeNull()
  })

  it('preserves a disabled auto-refresh flag instead of resetting it', async () => {
    mocks.getAppConfig.mockImplementation(async () =>
      savedConfig({ auto_refresh_devices: false }),
    )

    await useScrcpyStore.getState().savePreset('NoPolling', OPTIONS)

    expect(mocks.updatePreferences.mock.calls[0][0].auto_refresh_devices).toBe(false)
  })

  // Simulation only: the mock stands in for the Go handler semantics. Real
  // durability of scrcpy_presets across export/import is proven in Go
  // (internal/core/config.go:24,127-130,171; internal/app/settings.go:134-135,180,228;
  // internal/app/app_settings_backup.go:127-130, covered by config_concurrency_test.go
  // and app_settings_backup_test.go).
  it('does not send or revert the persisted theme when saving a preset', async () => {
    // Mirrors internal/app/settings.go: theme is applied only when non-empty
    // (:114-119), auto_refresh_devices is assigned unconditionally (:140),
    // scrcpy_presets only when non-nil (:134-136).
    const backend = {
      theme: 'light' as 'dark' | 'light',
      auto_refresh_devices: true,
      scrcpy_presets: [] as Array<{ name: string; options: ScrcpyOptions }>,
    }
    mocks.getAppConfig.mockImplementation(async () => savedConfig(backend))
    mocks.updatePreferences.mockImplementation(async (payload: never) => {
      const patch = payload as {
        theme?: string
        auto_refresh_devices?: boolean
        scrcpy_presets?: Array<{ name: string; options: ScrcpyOptions }>
      }
      if (typeof patch.theme === 'string' && patch.theme !== '') {
        backend.theme = patch.theme === 'light' ? 'light' : 'dark'
      }
      backend.auto_refresh_devices = patch.auto_refresh_devices ?? false
      if (patch.scrcpy_presets) {
        backend.scrcpy_presets = patch.scrcpy_presets
      }
      return savedConfig(backend)
    })

    const saved = await useScrcpyStore.getState().savePreset('After light', OPTIONS)

    expect(saved).toBe(true)
    const payload = mocks.updatePreferences.mock.calls[0][0]
    expect(payload).not.toHaveProperty('theme')
    expect(backend.theme).toBe('light')
    expect(backend.scrcpy_presets).toEqual([{ name: 'After light', options: OPTIONS }])
  })

  it('reports a failed write without dropping the local preset', async () => {
    mocks.updatePreferences.mockImplementation(async () => {
      throw new Error('config write failed')
    })

    const saved = await useScrcpyStore.getState().savePreset('Kept', OPTIONS)

    expect(saved).toBe(false)
    expect(useScrcpyStore.getState().presetsPersistError).toBe('config write failed')
    expect(useScrcpyStore.getState().presets.map((preset) => preset.name)).toEqual(['Kept'])
  })

  it('serializes rapid add/remove actions without losing the latest list', async () => {
    const store = useScrcpyStore.getState()
    const firstSave = store.savePreset('A', OPTIONS)
    const secondSave = store.savePreset('B', OPTIONS)

    // Remove the first preset while both writes are still queued.
    const firstId = useScrcpyStore.getState().presets[1].id
    const removal = useScrcpyStore.getState().deletePreset(firstId)

    await Promise.all([firstSave, secondSave, removal])

    expect(useScrcpyStore.getState().presets.map((preset) => preset.name)).toEqual(['B'])
    const lastPayload =
      mocks.updatePreferences.mock.calls[mocks.updatePreferences.mock.calls.length - 1][0]
    expect(lastPayload.scrcpy_presets).toEqual([{ name: 'B', options: OPTIONS }])
  })

  it('ignores a stale hydration that arrives after a live change', async () => {
    const revisionBeforeSave = useScrcpyStore.getState().presetsRevision
    await useScrcpyStore.getState().savePreset('Live', OPTIONS)

    useScrcpyStore
      .getState()
      .hydratePresets([{ name: 'Stale', options: OPTIONS }], revisionBeforeSave)

    expect(useScrcpyStore.getState().presets.map((preset) => preset.name)).toEqual(['Live'])
  })

  it('does not write when the removed preset is unknown', async () => {
    const removed = await useScrcpyStore.getState().deletePreset('missing')

    expect(removed).toBe(true)
    expect(mocks.updatePreferences).not.toHaveBeenCalled()
  })
})

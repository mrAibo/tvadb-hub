import { beforeEach, describe, expect, it } from 'vitest'
import type { AppConfigSnapshot, ScrcpyOptions } from '@/lib/types'
import { useSettingsStore } from '@/stores/useSettingsStore'

// The draft hydration never reads scrcpy options, so the nested options object only
// has to satisfy the snapshot type.
function testConfig(overrides: Partial<AppConfigSnapshot> = {}): AppConfigSnapshot {
  return {
    adb_path: '',
    fastboot_path: '',
    scrcpy_path: '',
    setup_completed: true,
    theme: 'dark',
    binary_versions: {},
    device_nicknames: {},
    logcat_buffer_limit: 5000,
    scrcpy_options: {} as ScrcpyOptions,
    scrcpy_presets: [],
    default_terminal_mode: 'adb-shell',
    auto_refresh_devices: true,
    device_refresh_seconds: 8,
    audit_enabled: false,
    file_transfer_compression: 'auto',
    verify_after_transfer: false,
    safe_tuning_feed_url: '',
    safe_tuning_feed_public_key: '',
    ...overrides,
  }
}

describe('useSettingsStore draft dirty tracking', () => {
  beforeEach(() => {
    useSettingsStore.getState().reset()
  })

  it('treats a fresh store as clean', () => {
    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
  })

  it('marks the draft dirty on an edit and clears it on hydration', () => {
    useSettingsStore.getState().setPreferencesDraft({ logcat_buffer_limit: 9000 })

    expect(useSettingsStore.getState().preferencesDirty).toBe(true)
    expect(useSettingsStore.getState().preferencesDraft.logcat_buffer_limit).toBe(9000)

    useSettingsStore.getState().hydratePreferencesDraft(testConfig({ logcat_buffer_limit: 4000 }))

    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
    expect(useSettingsStore.getState().preferencesDraft.logcat_buffer_limit).toBe(4000)
  })

  it('clears the dirty flag on reset', () => {
    useSettingsStore.getState().setPreferencesDraft({ theme: 'light' })
    expect(useSettingsStore.getState().preferencesDirty).toBe(true)

    useSettingsStore.getState().reset()

    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
    expect(useSettingsStore.getState().preferencesDraft.theme).toBe('dark')
  })
})

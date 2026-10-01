import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppConfigSnapshot, ScrcpyOptions } from '@/lib/types'
import { useSettingsStore } from '@/stores/useSettingsStore'

const serviceMocks = vi.hoisted(() => ({
  getAppConfig: vi.fn(),
  updatePreferences: vi.fn(),
  getWindowState: vi.fn().mockResolvedValue('maximised'),
  setWindowState: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@/services/settingsService', () => ({
  getAppConfig: serviceMocks.getAppConfig,
  updatePreferences: serviceMocks.updatePreferences,
  getWindowState: serviceMocks.getWindowState,
  setWindowState: serviceMocks.setWindowState,
}))

import { useSettings } from '../useSettings'

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

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useSettings(), { wrapper })
  return { client, ...hook }
}

describe('useSettings unsaved draft protection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSettingsStore.getState().reset()
  })

  it('hydrates the draft on the initial load', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    const { result } = setup()

    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(5000)
    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
  })

  it('preserves an unsaved draft across an unrelated refetch while publishing the new snapshot', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    const { result, client } = setup()
    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    act(() => {
      result.current.setPreferencesDraft({ logcat_buffer_limit: 9999 })
    })
    expect(useSettingsStore.getState().preferencesDirty).toBe(true)

    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 4000 }))
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['settings', 'config'] })
    })

    await waitFor(() => expect(result.current.appConfig?.logcat_buffer_limit).toBe(4000))
    expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(9999)
    expect(useSettingsStore.getState().preferencesDirty).toBe(true)
  })

  it('re-hydrates the draft when it is not dirty', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    const { result, client } = setup()
    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 4100 }))
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['settings', 'config'] })
    })

    await waitFor(() => expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(4100))
  })

  it('discards the draft back to the persisted snapshot on request', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    const { result } = setup()
    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    act(() => {
      result.current.setPreferencesDraft({ logcat_buffer_limit: 9999 })
    })
    act(() => {
      result.current.resetPreferencesDraft()
    })

    expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(5000)
    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
  })

  it('clears the dirty flag after a successful save', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    serviceMocks.updatePreferences.mockResolvedValue(testConfig({ logcat_buffer_limit: 7777 }))
    const { result } = setup()
    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    act(() => {
      result.current.setPreferencesDraft({ logcat_buffer_limit: 7777 })
    })
    await act(async () => {
      await result.current.savePreferences()
    })

    expect(serviceMocks.updatePreferences).toHaveBeenCalledTimes(1)
    expect(result.current.appConfig?.logcat_buffer_limit).toBe(7777)
    expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(7777)
    expect(useSettingsStore.getState().preferencesDirty).toBe(false)
  })

  it('keeps the draft dirty when a save fails', async () => {
    serviceMocks.getAppConfig.mockResolvedValue(testConfig({ logcat_buffer_limit: 5000 }))
    serviceMocks.updatePreferences.mockRejectedValue(new Error('write failed'))
    const { result } = setup()
    await waitFor(() => expect(result.current.appConfig).not.toBeNull())

    act(() => {
      result.current.setPreferencesDraft({ logcat_buffer_limit: 8888 })
    })
    await act(async () => {
      await result.current.savePreferences()
    })

    expect(result.current.preferencesDraft.logcat_buffer_limit).toBe(8888)
    expect(useSettingsStore.getState().preferencesDirty).toBe(true)
  })
})

import { create } from 'zustand'
import type {
  AppConfigSnapshot,
  PreferencesPayload,
  ScrcpyEncoderSupport,
  ScrcpyOptions,
  ScrcpyPreset,
  ScrcpyPresetSnapshot,
  ScrcpySession,
  ScrcpySessionEvent,
  ScrcpyState,
} from '@/lib/types'
import { getAppConfig, updatePreferences } from '@/services/settingsService'

const DEFAULT_OPTIONS: ScrcpyOptions = {
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

interface ScrcpyActions {
  setSession: (session: ScrcpySession | null) => void
  setOptions: (options: ScrcpyOptions) => void
  setEncoderSupport: (support: ScrcpyEncoderSupport | null) => void
  setIsStarting: (isStarting: boolean) => void
  setIsStopping: (isStopping: boolean) => void
  setIsRecording: (isRecording: boolean) => void
  setRecordingStartedAt: (timestamp: number | null) => void
  setIsFetchingEncoder: (isFetching: boolean) => void
  setError: (error: string | null) => void
  applyStartedEvent: (event: ScrcpySessionEvent) => void
  applyStoppedEvent: (event: ScrcpySessionEvent) => void
  applyErrorEvent: (event: ScrcpySessionEvent) => void
  reset: () => void
}

/**
 * Preset persistence is part of the shared store, not of one hook instance: every
 * caller adds/removes through these actions, so several mounted useScrcpy hooks
 * cannot race each other or lose a preset.
 */
interface ScrcpyPresetPersistence {
  presetsRevision: number
  presetsHydrated: boolean
  presetsPersistError: string | null
  hydratePresets: (snapshot: ScrcpyPresetSnapshot[], revisionAtRequest: number) => void
  savePreset: (name: string, options: ScrcpyOptions) => Promise<boolean>
  deletePreset: (id: string) => Promise<boolean>
}

type ScrcpyStore = ScrcpyState & ScrcpyActions & ScrcpyPresetPersistence

// Only state lives here: reset() must restore every state field without replacing
// the real actions with placeholders.
const INITIAL_STATE: ScrcpyState &
  Pick<ScrcpyPresetPersistence, 'presetsRevision' | 'presetsHydrated' | 'presetsPersistError'> = {
  session: null,
  options: { ...DEFAULT_OPTIONS },
  encoderSupport: null,
  presets: [],
  isStarting: false,
  isStopping: false,
  isRecording: false,
  recordingStartedAt: null,
  isFetchingEncoder: false,
  error: null,
  lastEventAt: null,
  presetsRevision: 0,
  presetsHydrated: false,
  presetsPersistError: null,
}

function mergeSessionFromEvent(
  current: ScrcpySession | null,
  event: ScrcpySessionEvent,
): ScrcpySession {
  return {
    id: event.sessionId,
    serial: event.serial,
    status: event.status,
    pid: event.pid ?? current?.pid ?? 0,
    startedAt: current?.startedAt ?? Date.now(),
  }
}

function clientPresetId(): string {
  return `preset-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

/** The persisted DTO is the existing {name, options} snapshot: no backend ID is invented. */
function toPresetSnapshots(presets: ScrcpyPreset[]): ScrcpyPresetSnapshot[] {
  return presets.map((preset) => ({ name: preset.name, options: preset.options }))
}

function optionsEqual(left: ScrcpyOptions, right: ScrcpyOptions): boolean {
  return JSON.stringify(left) === JSON.stringify(right)
}

/**
 * Rebuilds the local list from the saved snapshots. IDs are client-side and stay
 * stable for the life of a session: an entry that is already loaded keeps its ID
 * instead of being replaced by a new one on every reload.
 */
function hydrateFromSnapshots(
  snapshot: ScrcpyPresetSnapshot[],
  existing: ScrcpyPreset[],
): ScrcpyPreset[] {
  return snapshot.map((entry) => {
    const match = existing.find(
      (preset) => preset.name === entry.name && optionsEqual(preset.options, entry.options),
    )
    return {
      id: match?.id ?? clientPresetId(),
      name: entry.name,
      options: entry.options,
      createdAt: match?.createdAt ?? Date.now(),
    }
  })
}

function persistErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Failed to save presets'
}

/**
 * Reads the freshly saved configuration and writes only the preset list, so the
 * unconditionally assigned auto_refresh_devices (and the theme) keep their saved
 * values. Dirty local preference drafts are never sent from here.
 */
async function persistCurrentPresets(): Promise<boolean> {
  const presets = useScrcpyStore.getState().presets
  try {
    const saved: AppConfigSnapshot = await getAppConfig()
    const payload: PreferencesPayload = {
      theme: saved.theme === 'light' ? 'light' : 'dark',
      auto_refresh_devices: saved.auto_refresh_devices,
      scrcpy_presets: toPresetSnapshots(presets),
    }
    await updatePreferences(payload)
    useScrcpyStore.setState({ presetsPersistError: null })
    return true
  } catch (error) {
    useScrcpyStore.setState({ presetsPersistError: persistErrorMessage(error) })
    return false
  }
}

// One serialized writer per process: a burst of add/remove calls cannot interleave
// their writes, and every job persists the latest list, so none is dropped.
let persistQueue: Promise<unknown> = Promise.resolve()

function enqueuePresetPersist(): Promise<boolean> {
  const job = persistQueue.then(persistCurrentPresets, persistCurrentPresets)
  persistQueue = job.catch(() => undefined)
  return job
}

export const useScrcpyStore = create<ScrcpyStore>()((set) => ({
  ...INITIAL_STATE,
  options: { ...DEFAULT_OPTIONS },
  setSession: (session) => set({ session }),
  setOptions: (options) => set({ options }),
  setEncoderSupport: (encoderSupport) => set({ encoderSupport }),
  setIsStarting: (isStarting) => set({ isStarting }),
  setIsStopping: (isStopping) => set({ isStopping }),
  setIsRecording: (isRecording) => set({ isRecording }),
  setRecordingStartedAt: (recordingStartedAt) => set({ recordingStartedAt }),
  setIsFetchingEncoder: (isFetchingEncoder) => set({ isFetchingEncoder }),
  setError: (error) => set({ error }),
  applyStartedEvent: (event) =>
    set((state) => ({
      session: mergeSessionFromEvent(state.session, event),
      isStarting: false,
      isStopping: false,
      error: null,
      lastEventAt: Date.now(),
    })),
  applyStoppedEvent: (event) =>
    set((state) => {
      if (state.session?.id !== event.sessionId) {
        return state
      }
      return {
        session: null,
        isStarting: false,
        isStopping: false,
        error: null,
        lastEventAt: Date.now(),
      }
    }),
  applyErrorEvent: (event) =>
    set((state) => ({
      session:
        state.session?.id === event.sessionId
          ? { ...state.session, status: 'error' }
          : state.session,
      isStarting: false,
      isStopping: false,
      error: event.message ?? 'Scrcpy session failed unexpectedly',
      lastEventAt: Date.now(),
    })),
  hydratePresets: (snapshot, revisionAtRequest) =>
    set((state) => {
      // A preset was added or removed while the config was loading: that newer
      // local state wins, and it is persisted by its own queued write.
      if (state.presetsRevision !== revisionAtRequest) {
        return state
      }
      return {
        presets: hydrateFromSnapshots(snapshot, state.presets),
        presetsHydrated: true,
      }
    }),
  savePreset: async (name, options) => {
    const trimmed = name.trim()
    if (!trimmed) return false
    set((state) => ({
      presets: [
        { id: clientPresetId(), name: trimmed, options: { ...options }, createdAt: Date.now() },
        ...state.presets,
      ],
      presetsRevision: state.presetsRevision + 1,
      presetsHydrated: true,
    }))
    return enqueuePresetPersist()
  },
  deletePreset: async (id) => {
    let removed = false
    set((state) => {
      const presets = state.presets.filter((preset) => preset.id !== id)
      removed = presets.length !== state.presets.length
      if (!removed) return state
      return { presets, presetsRevision: state.presetsRevision + 1, presetsHydrated: true }
    })
    if (!removed) return true
    return enqueuePresetPersist()
  },
  reset: () => set({ ...INITIAL_STATE, options: { ...DEFAULT_OPTIONS } }),
}))

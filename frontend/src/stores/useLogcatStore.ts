import { create } from 'zustand'
import type {
  LogcatEntry,
  LogcatFilter,
  LogcatLevel,
  LogcatState,
  LogcatStatusEvent,
  SavedLogcatFilter,
} from '@/lib/types'

const MIN_LOGCAT_BUFFER_LIMIT = 1000
const DEFAULT_LOGCAT_BUFFER_LIMIT = 5000
const MAX_LOGCAT_BUFFER_LIMIT = 50000
const LOGCAT_FLUSH_INTERVAL = 100
const SAVED_FILTERS_STORAGE_KEY = 'droidsphere.logcat.saved-filters.v1'
const MAX_SAVED_FILTERS = 20
const LOGCAT_LEVELS: LogcatLevel[] = ['V', 'D', 'I', 'W', 'E', 'F']

let currentBufferLimit = DEFAULT_LOGCAT_BUFFER_LIMIT
let queuedEntries: LogcatEntry[] = []
let flushTimer: number | null = null

interface LogcatActions {
  setStreamingSerial: (serial: string) => void
  setIsStreaming: (isStreaming: boolean) => void
  setAutoScroll: (autoScroll: boolean) => void
  setFilter: (filter: Partial<LogcatFilter>) => void
  saveCurrentFilter: (name: string) => void
  applySavedFilter: (id: string) => void
  deleteSavedFilter: (id: string) => void
  setError: (error: string | null) => void
  setLastUpdatedAt: (timestamp: number | null) => void
  setBufferLimit: (limit: number) => void
  setPinnedOnly: (pinnedOnly: boolean) => void
  togglePinned: (entry: LogcatEntry) => void
  clearPinned: () => void
  clearLogs: () => void
  appendLogs: (entries: LogcatEntry[]) => void
  applyLineEvent: (entry: LogcatEntry) => void
  applyBatchEvent: (entries: LogcatEntry[]) => void
  applyStatusEvent: (event: LogcatStatusEvent) => void
  reset: () => void
}

interface LogcatExtraState {
  bufferLimit: number
  bufferFull: boolean
  savedFilters: SavedLogcatFilter[]
  pinnedEntries: LogcatEntry[]
  pinnedOnly: boolean
}

type LogcatStore = LogcatState & LogcatExtraState & LogcatActions

function normalizeLogLevel(level: string): LogcatLevel {
  if (level === 'D' || level === 'I' || level === 'W' || level === 'E' || level === 'F') {
    return level
  }

  return 'V'
}

function flushQueuedEntries() {
  if (queuedEntries.length === 0) {
    flushTimer = null
    return
  }

  const entries = queuedEntries
  queuedEntries = []
  flushTimer = null

  useLogcatStore.getState().appendLogs(entries)
}

function queueLogEntries(entries: LogcatEntry[]) {
  if (entries.length === 0) {
    return
  }

  queuedEntries.push(...entries)

  if (queuedEntries.length > currentBufferLimit) {
    queuedEntries = queuedEntries.slice(-currentBufferLimit)
  }

  if (flushTimer === null && typeof window !== 'undefined') {
    flushTimer = window.setTimeout(flushQueuedEntries, LOGCAT_FLUSH_INTERVAL)
  }
}

function cloneFilter(filter: LogcatFilter): LogcatFilter {
  return {
    ...filter,
    levels: [...filter.levels],
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function normalizeStoredFilter(value: unknown): SavedLogcatFilter | null {
  if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string') {
    return null
  }

  if (!isRecord(value.filter)) {
    return null
  }

  const levels = Array.isArray(value.filter.levels)
    ? value.filter.levels.filter(
        (level): level is LogcatLevel =>
          typeof level === 'string' && LOGCAT_LEVELS.includes(level as LogcatLevel),
      )
    : []

  if (levels.length === 0) {
    return null
  }

  const issue =
    value.filter.issue === 'crash' || value.filter.issue === 'anr'
      ? value.filter.issue
      : 'all'

  return {
    id: value.id,
    name: value.name.trim().slice(0, 40),
    filter: {
      levels,
      tag: typeof value.filter.tag === 'string' ? value.filter.tag : '',
      text: typeof value.filter.text === 'string' ? value.filter.text : '',
      pid: typeof value.filter.pid === 'string' ? value.filter.pid : '',
      process: typeof value.filter.process === 'string' ? value.filter.process : '',
      issue,
    },
  }
}

function loadSavedFilters(): SavedLogcatFilter[] {
  if (typeof window === 'undefined') {
    return []
  }

  try {
    const raw = window.localStorage.getItem(SAVED_FILTERS_STORAGE_KEY)
    if (!raw) {
      return []
    }

    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }

    return parsed
      .map(normalizeStoredFilter)
      .filter((saved): saved is SavedLogcatFilter => saved !== null && saved.name !== '')
      .slice(-MAX_SAVED_FILTERS)
  } catch {
    return []
  }
}

function persistSavedFilters(filters: SavedLogcatFilter[]) {
  if (typeof window === 'undefined') {
    return
  }

  try {
    window.localStorage.setItem(SAVED_FILTERS_STORAGE_KEY, JSON.stringify(filters))
  } catch {
    // A disabled or quota-limited storage backend should not break live Logcat.
  }
}

const initialState: LogcatState & LogcatExtraState = {
  logs: [],
  streamingSerial: '',
  isStreaming: false,
  autoScroll: true,
  filter: {
    levels: ['V', 'D', 'I', 'W', 'E', 'F'],
    tag: '',
    text: '',
    pid: '',
    process: '',
    issue: 'all',
  },
  error: null,
  lastUpdatedAt: null,
  bufferLimit: currentBufferLimit,
  bufferFull: false,
  savedFilters: loadSavedFilters(),
  pinnedEntries: [],
  pinnedOnly: false,
}

export const useLogcatStore = create<LogcatStore>()((set) => ({
  ...initialState,
  setStreamingSerial: (streamingSerial) => set({ streamingSerial }),
  setIsStreaming: (isStreaming) => set({ isStreaming }),
  setAutoScroll: (autoScroll) => set({ autoScroll }),
  setFilter: (filter) =>
    set((state) => ({
      filter: {
        ...state.filter,
        ...filter,
      },
    })),
  saveCurrentFilter: (name) =>
    set((state) => {
      const normalizedName = name.trim().slice(0, 40)
      if (!normalizedName) {
        return {}
      }

      const existingIndex = state.savedFilters.findIndex(
        (saved) => saved.name.toLowerCase() === normalizedName.toLowerCase(),
      )
      const saved: SavedLogcatFilter = {
        id:
          existingIndex >= 0
            ? state.savedFilters[existingIndex].id
            : `filter-${Date.now().toString(36)}-${state.savedFilters.length.toString(36)}`,
        name: normalizedName,
        filter: cloneFilter(state.filter),
      }

      const next =
        existingIndex >= 0
          ? state.savedFilters.map((item, index) => (index === existingIndex ? saved : item))
          : [...state.savedFilters, saved].slice(-MAX_SAVED_FILTERS)

      persistSavedFilters(next)
      return { savedFilters: next }
    }),
  applySavedFilter: (id) =>
    set((state) => {
      const saved = state.savedFilters.find((item) => item.id === id)
      return saved ? { filter: cloneFilter(saved.filter) } : {}
    }),
  deleteSavedFilter: (id) =>
    set((state) => {
      const next = state.savedFilters.filter((saved) => saved.id !== id)
      persistSavedFilters(next)
      return { savedFilters: next }
    }),
  setError: (error) => set({ error }),
  setLastUpdatedAt: (lastUpdatedAt) => set({ lastUpdatedAt }),
  setBufferLimit: (limit) => {
    const bounded = Math.min(MAX_LOGCAT_BUFFER_LIMIT, Math.max(MIN_LOGCAT_BUFFER_LIMIT, limit))
    currentBufferLimit = bounded
    set((state) => ({
      bufferLimit: bounded,
      logs: state.logs.length > bounded ? state.logs.slice(-bounded) : state.logs,
      bufferFull: state.logs.length >= bounded,
    }))
  },
  setPinnedOnly: (pinnedOnly) => set({ pinnedOnly }),
  togglePinned: (entry) =>
    set((state) => {
      const exists = state.pinnedEntries.some((pinned) => pinned.id === entry.id)
      return {
        pinnedEntries: exists
          ? state.pinnedEntries.filter((pinned) => pinned.id !== entry.id)
          : [...state.pinnedEntries, entry].slice(-100),
      }
    }),
  clearPinned: () => set({ pinnedEntries: [], pinnedOnly: false }),
  clearLogs: () => {
    queuedEntries = []
    if (flushTimer !== null && typeof window !== 'undefined') {
      window.clearTimeout(flushTimer)
      flushTimer = null
    }
    set({ logs: [], lastUpdatedAt: Date.now(), bufferFull: false })
  },
  appendLogs: (entries) =>
    set((state) => {
      const combined = [...state.logs, ...entries]
      const trimmed = combined.length > currentBufferLimit
        ? combined.slice(-currentBufferLimit)
        : combined
      return {
        logs: trimmed,
        lastUpdatedAt: entries.length > 0 ? Date.now() : state.lastUpdatedAt,
        bufferFull: trimmed.length >= currentBufferLimit,
      }
    }),
  applyLineEvent: (entry) => {
    queueLogEntries([{
      ...entry,
      level: normalizeLogLevel(entry.level),
    }])
  },
  applyBatchEvent: (entries) => {
    queueLogEntries(entries.map((entry) => ({
      ...entry,
      level: normalizeLogLevel(entry.level),
    })))
  },
  applyStatusEvent: (event) =>
    set((state) => {
      if (state.streamingSerial !== '' && state.streamingSerial !== event.serial) {
        return state
      }

      return {
        streamingSerial: event.status === 'started' ? event.serial : '',
        isStreaming: event.status === 'started',
        error:
          event.status === 'error'
            ? 'Logcat stream stopped unexpectedly'
            : event.status === 'started'
              ? null
              : state.error,
        lastUpdatedAt: Date.now(),
      }
    }),
  reset: () => {
    queuedEntries = []
    if (flushTimer !== null && typeof window !== 'undefined') {
      window.clearTimeout(flushTimer)
      flushTimer = null
    }
    set({
      ...initialState,
      savedFilters: useLogcatStore.getState().savedFilters,
    })
  },
}))

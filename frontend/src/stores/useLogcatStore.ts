import { create } from 'zustand'
import { matchesLogcatFilter } from '@/lib/logcatAnalysis'
import type {
  LogcatEntry,
  LogcatFilter,
  LogcatLevel,
  LogcatState,
  LogcatStatusEvent,
  SavedLogcatFilter,
} from '@/lib/types'

export const MIN_LOGCAT_BUFFER_LIMIT = 1000
export const DEFAULT_LOGCAT_BUFFER_LIMIT = 5000
export const MAX_LOGCAT_BUFFER_LIMIT = 50000
/** Entries waiting for the next flush are bounded by the same count ceiling. */
export const LOGCAT_FLUSH_INTERVAL = 100
const SAVED_FILTERS_STORAGE_KEY = 'droidsphere.logcat.saved-filters.v1'
const MAX_SAVED_FILTERS = 20
const LOGCAT_LEVELS: LogcatLevel[] = ['V', 'D', 'I', 'W', 'E', 'F']

/**
 * Retention is bounded twice over: by entry count (the user-visible buffer
 * limit) and by serialized volume, so a few enormous entries can never pin an
 * unbounded amount of memory. A single entry that alone exceeds the per-entry
 * ceiling is clipped and counted instead of being accepted on faith.
 *
 * These ceilings bound SERIALIZED VOLUME, not live heap: every measured number
 * in this store is the length of the JSON the entry would export as. The
 * retained byte total is the sum of those lengths, so it can only ever
 * overstate the JS heap a little (escaping, object overhead), never understate
 * the export volume it is there to bound.
 */
const LOGCAT_MAX_ENTRY_BYTES = 64 * 1024
const LOGCAT_MAX_BUFFER_BYTES = 32 * 1024 * 1024
const LOGCAT_MAX_QUEUE_BYTES = 8 * 1024 * 1024
const MAX_PINNED_ENTRIES = 100
const LOGCAT_MAX_PINNED_BYTES = 4 * 1024 * 1024

const CLIPPED_SUFFIX = '…[clipped]'

export type LogcatExportMode = 'all' | 'filtered' | 'pinned'

export interface LogcatExport {
  entries: LogcatEntry[]
  content: string
  filename: string
}

interface LogcatUsage {
  retainedBytes: number
  evictedEntries: number
  evictedBytes: number
}

interface LogcatMutation {
  logs: LogcatEntry[]
  /** Serialized size per retained entry, positional with `logs`. */
  sizes: number[]
  usage: LogcatUsage
}

export interface LogcatExportResolution {
  export?: LogcatExport
  error?: string
}

let currentBufferLimit = DEFAULT_LOGCAT_BUFFER_LIMIT
let queuedEntries: LogcatEntry[] = []
/** Serialized size of each queued entry, measured once on ingress. */
let queuedSizes: number[] = []
/**
 * Serialized size of every retained entry, positionally parallel to the live
 * `logs` array. Entries are immutable snapshots, so a size measured on the way
 * in stays true for as long as the entry is retained: a flush measures only the
 * entries arriving in that batch and slides the window by subtracting the
 * cached sizes of the evicted ones. Re-serializing the whole retained buffer on
 * every update is exactly the cost this cache exists to remove.
 *
 * The retained byte total itself is not kept here: `state.bufferBytes` already
 * holds it, accumulated by the same eviction that moves this window.
 */
let retainedSizes: number[] = []
let flushTimer: number | null = null

/**
 * Counters are accumulated here and handed to state through explicit
 * increments, so no O(n) re-measurement happens per render or per update.
 */
let droppedEntryCount = 0
let droppedByteCount = 0
let evictedEntryCount = 0
let evictedByteCount = 0
let evictedPinCount = 0
let evictedPinByteCount = 0
let clippedEntryCount = 0

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
  setExportMode: (mode: LogcatExportMode) => void
  togglePinned: (entry: LogcatEntry) => void
  clearPinned: () => void
  clearCounters: () => void
  clearLogs: () => void
  appendLogs: (entries: LogcatEntry[], sizes?: number[]) => void
  applyLineEvent: (entry: LogcatEntry) => void
  applyBatchEvent: (entries: LogcatEntry[]) => void
  applyStatusEvent: (event: LogcatStatusEvent) => void
  reset: () => void
}

interface LogcatExtraState {
  bufferLimit: number
  bufferFull: boolean
  bufferBytes: number
  savedFilters: SavedLogcatFilter[]
  pinnedEntries: LogcatEntry[]
  pinnedOnly: boolean
  pinnedBytes: number
  exportMode: LogcatExportMode
  /**
   * Entries that never reached a retained buffer: the ingress queue discarded
   * them because the count or byte budget was already spent. These are real
   * frontend discards — backend backpressure is not reported here.
   */
  droppedEntries: number
  droppedBytes: number
  /**
   * Entries that were retained once and then rolled out of the buffer or the
   * pin list by the budgets below. Kept separate from drops so an eviction is
   * never reported as backpressure.
   */
  evictedEntries: number
  evictedBytes: number
  evictedPinnedEntries: number
  evictedPinnedBytes: number
  clippedEntries: number
}

type LogcatStore = LogcatState & LogcatExtraState & LogcatActions

function normalizeLogLevel(level: string): LogcatLevel {
  if (level === 'D' || level === 'I' || level === 'W' || level === 'E' || level === 'F') {
    return level
  }

  return 'V'
}

/**
 * Budgets measure the serialized entry, because that is also what export
 * writes out. JSON.stringify walks the exact wire shape, so the accounting
 * cannot drift from what the user actually holds or saves.
 */
function encodedBytes(entry: LogcatEntry): number {
  return JSON.stringify(entry).length
}

function clipText(value: string, budget: number): string {
  if (budget <= 0) {
    return ''
  }

  if (value.length <= budget) {
    return value
  }

  const head = value.slice(0, Math.max(1, budget - CLIPPED_SUFFIX.length))
  return `${head}${CLIPPED_SUFFIX}`
}

/**
 * Hard ceiling for one entry. Entries at or below the ceiling keep their exact
 * identity so pin toggles keep matching by id; only an oversized entry is
 * replaced by a clipped copy, and that is counted.
 */
function boundEntry(entry: LogcatEntry): LogcatEntry {
  if (encodedBytes(entry) <= LOGCAT_MAX_ENTRY_BYTES) {
    return entry
  }

  const largestField = Math.max(
    entry.serial.length,
    entry.message.length,
    entry.raw.length,
    entry.tag.length,
    entry.processName?.length ?? 0,
  )
  // The clipped copy keeps roughly the largest single field; the other text
  // fields keep their heads under the same budget, so the copy lands inside the
  // ceiling as long as one field dominated the original entry.
  const budget = Math.max(
    1,
    LOGCAT_MAX_ENTRY_BYTES - Math.max(0, encodedBytes(entry) - largestField),
  )
  const bounded: LogcatEntry = {
    ...entry,
    message: clipText(entry.message, budget),
    raw: clipText(entry.raw, budget),
    tag: clipText(entry.tag, budget),
  }

  if (bounded.processName !== undefined) {
    bounded.processName = clipText(bounded.processName, budget)
  }

  if (encodedBytes(bounded) > LOGCAT_MAX_ENTRY_BYTES) {
    bounded.message = clipText(bounded.message, Math.max(1, Math.floor(budget / 2)))
    bounded.raw = clipText(bounded.raw, Math.max(1, Math.floor(budget / 2)))
  }

  clippedEntryCount += 1
  return bounded
}

function queueByteBudget(): number {
  return Math.min(LOGCAT_MAX_QUEUE_BYTES, currentBufferLimit * LOGCAT_MAX_ENTRY_BYTES)
}

interface BudgetFit {
  kept: LogcatEntry[]
  keptSizes: number[]
  keptBytes: number
  evictedEntries: number
  evictedBytes: number
}

/**
 * Keeps the newest entries that fit both budgets, the one place both the
 * retained buffer and the ingress queue are trimmed. Sizes travel with the
 * entries, so trimming an already-measured window is a single backwards walk
 * over counts — no payload is serialized to enforce a budget.
 */
function fitNewest(
  entries: LogcatEntry[],
  sizes: number[],
  byteLimit: number,
  entryLimit: number,
): BudgetFit {
  if (entryLimit <= 0 || byteLimit <= 0) {
    let evictedBytes = 0
    for (const size of sizes) {
      evictedBytes += size
    }

    return {
      kept: [],
      keptSizes: [],
      keptBytes: 0,
      evictedEntries: entries.length,
      evictedBytes,
    }
  }

  let keptCount = 0
  let keptBytes = 0

  for (let index = entries.length - 1; index >= 0; index -= 1) {
    if (keptCount >= entryLimit) {
      break
    }

    const size = sizes[index]
    if (keptCount > 0 && keptBytes + size > byteLimit) {
      break
    }

    keptBytes += size
    keptCount += 1
  }

  const retainedFrom = entries.length - keptCount
  const evictedEntries = retainedFrom
  let evictedBytes = 0
  for (let index = 0; index < evictedEntries; index += 1) {
    evictedBytes += sizes[index]
  }

  return {
    kept: retainedFrom === 0 ? entries : entries.slice(retainedFrom),
    keptSizes: retainedFrom === 0 ? sizes : sizes.slice(retainedFrom),
    keptBytes,
    evictedEntries,
    evictedBytes,
  }
}

/**
 * Applies the retained-buffer budgets to a candidate list. Eviction is counted
 * from what was actually discarded, never guessed, and the sizes that arrive
 * with the candidates are handed straight back so the caller can keep its
 * positional size window without re-serializing anything.
 */
function trimToBudget(entries: LogcatEntry[], sizes: number[]): LogcatMutation {
  const fit = fitNewest(entries, sizes, LOGCAT_MAX_BUFFER_BYTES, currentBufferLimit)

  return {
    logs: fit.kept,
    sizes: fit.keptSizes,
    usage: {
      retainedBytes: fit.keptBytes,
      evictedEntries: fit.evictedEntries,
      evictedBytes: fit.evictedBytes,
    },
  }
}

/**
 * Ingress queue. Bounded on both axes, oldest first, so a burst of huge entries
 * can never accumulate without limit before the flush interval elapses. Drops
 * here are real discards and are counted.
 *
 * Returns true when the queue discarded anything, so the caller can publish the
 * updated counters to the store in the same tick as the drop.
 */
function enqueueEntries(entries: LogcatEntry[]): boolean {
  if (entries.length === 0) {
    return false
  }

  const bounded = entries.map(boundEntry)
  // The only place a new entry is serialized on the streaming hot path: each
  // arriving entry is measured once, and that measurement is reused by every
  // later trim, flush and pin toggle.
  const sizes = bounded.map(encodedBytes)
  const fit = fitNewest(
    [...queuedEntries, ...bounded],
    [...queuedSizes, ...sizes],
    queueByteBudget(),
    currentBufferLimit,
  )

  if (fit.evictedEntries > 0) {
    droppedEntryCount += fit.evictedEntries
    droppedByteCount += fit.evictedBytes
  }

  queuedEntries = fit.kept
  queuedSizes = fit.keptSizes

  if (flushTimer === null && typeof window !== 'undefined') {
    flushTimer = window.setTimeout(flushQueuedEntries, LOGCAT_FLUSH_INTERVAL)
  }

  return fit.evictedEntries > 0
}

function flushQueuedEntries() {
  flushTimer = null

  if (queuedEntries.length === 0) {
    queuedSizes = []
    return
  }

  const entries = queuedEntries
  const sizes = queuedSizes
  queuedEntries = []
  queuedSizes = []

  useLogcatStore.getState().appendLogs(entries, sizes)
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
  bufferBytes: 0,
  savedFilters: loadSavedFilters(),
  pinnedEntries: [],
  pinnedOnly: false,
  pinnedBytes: 0,
  exportMode: 'all',
  droppedEntries: 0,
  droppedBytes: 0,
  evictedEntries: 0,
  evictedBytes: 0,
  evictedPinnedEntries: 0,
  evictedPinnedBytes: 0,
  clippedEntries: 0,
}

function releaseFlushTimer() {
  queuedEntries = []
  queuedSizes = []

  if (flushTimer !== null && typeof window !== 'undefined') {
    window.clearTimeout(flushTimer)
  }

  flushTimer = null
}

function resetCounters() {
  droppedEntryCount = 0
  droppedByteCount = 0
  evictedEntryCount = 0
  evictedByteCount = 0
  evictedPinCount = 0
  evictedPinByteCount = 0
  clippedEntryCount = 0
}

/** Latest counter totals, applied to state by the next explicit set. */
function counterTotals() {
  return {
    droppedEntries: droppedEntryCount,
    droppedBytes: droppedByteCount,
    evictedEntries: evictedEntryCount,
    evictedBytes: evictedByteCount,
    evictedPinnedEntries: evictedPinCount,
    evictedPinnedBytes: evictedPinByteCount,
    clippedEntries: clippedEntryCount,
  }
}

/**
 * Pin retention. The 100-entry ceiling is a real policy, not a silent slice:
 * every pin that leaves the list is counted so the UI can say so out loud.
 *
 * Sizes arrive with the pins, measured once per pin and cached for as long as it
 * stays pinned, so re-pinning never re-serializes the rest of the list.
 */
function retainPins(
  entries: LogcatEntry[],
  sizes: number[],
): {
  pinnedEntries: LogcatEntry[]
  pinnedBytes: number
} {
  let pinnedEntries = entries
  let pinnedSizes = sizes
  let byteTotal = 0
  for (const size of pinnedSizes) {
    byteTotal += size
  }

  let evicted = 0
  let evictedBytes = 0

  if (pinnedEntries.length > MAX_PINNED_ENTRIES) {
    const overflow = pinnedEntries.length - MAX_PINNED_ENTRIES
    for (let index = 0; index < overflow; index += 1) {
      evictedBytes += pinnedSizes[index]
    }

    evicted += overflow
    pinnedEntries = pinnedEntries.slice(overflow)
    pinnedSizes = pinnedSizes.slice(overflow)
    byteTotal -= evictedBytes
  }

  while (byteTotal > LOGCAT_MAX_PINNED_BYTES && pinnedEntries.length > 1) {
    const size = pinnedSizes[0]
    pinnedEntries = pinnedEntries.slice(1)
    pinnedSizes = pinnedSizes.slice(1)
    byteTotal -= size
    evicted += 1
    evictedBytes += size
  }

  if (byteTotal > LOGCAT_MAX_PINNED_BYTES && pinnedEntries.length === 1) {
    const bounded = boundEntry(pinnedEntries[0])
    const boundedSize = encodedBytes(bounded)
    pinnedEntries = [bounded]
    pinnedSizes = [boundedSize]
    byteTotal = boundedSize
  }

  evictedPinCount += evicted
  evictedPinByteCount += evictedBytes

  return {
    pinnedEntries,
    pinnedBytes: byteTotal,
  }
}

/**
 * Size of each pinned entry. A pin that is still in the retained buffer reuses
 * the size cached for it there — matched by id, because a pin toggle can hand
 * in an equal-but-fresh snapshot. Only a pin kept after the buffer moved on, or
 * after a clear, is measured, and then only once, when it is pinned.
 */
function sizesForPins(pinned: LogcatEntry[], retained: LogcatEntry[]): number[] {
  const indexById = new Map<string, number>()
  for (let index = 0; index < retained.length; index += 1) {
    indexById.set(retained[index].id, index)
  }

  const sizes: number[] = []
  for (const entry of pinned) {
    const index = indexById.get(entry.id)
    sizes.push(index === undefined ? encodedBytes(entry) : retainedSizes[index])
  }

  return sizes
}

/** The default filter selects everything, so filtering can be skipped whole. */
export function isLogcatFilterDefault(filter: LogcatFilter): boolean {
  return (
    filter.levels.length === LOGCAT_LEVELS.length &&
    filter.tag === '' &&
    filter.text === '' &&
    filter.pid === '' &&
    filter.process === '' &&
    filter.issue === 'all'
  )
}

/** Single source of truth for what the log view renders and what it counts. */
export function resolveLogcatFilteredLogs(
  logs: LogcatEntry[],
  pinnedEntries: LogcatEntry[],
  pinnedOnly: boolean,
  filter: LogcatFilter,
): LogcatEntry[] {
  const source = pinnedOnly ? pinnedEntries : logs

  if (isLogcatFilterDefault(filter)) {
    return source
  }

  return source.filter((entry) => matchesLogcatFilter(entry, filter))
}

export interface LogcatExportState {
  logs: LogcatEntry[]
  pinnedEntries: LogcatEntry[]
  exportMode: LogcatExportMode
  filter: LogcatFilter
  streamingSerial: string
  /** Mirrors the view: pinned-only swaps the displayed source to the pin list. */
  pinnedOnly: boolean
}

/**
 * Export resolves against the source the user is actually looking at.
 *
 * `all` writes the entire retained buffer, regardless of view toggles.
 * `filtered` narrows the displayed source (pins when pinned-only is enabled).
 * `pinned` always writes every pin, regardless of the active filter.
 */
export function resolveLogcatExport(
  state: LogcatExportState,
  format: 'text' | 'json',
): LogcatExportResolution {
  const serial = state.streamingSerial || 'export'
  const stamp = Date.now()
  const extension = format === 'text' ? 'txt' : 'json'
  const filename = `logcat-${serial}-${stamp}.${extension}`
  const build = (entries: LogcatEntry[]): LogcatExport => ({
    entries,
    content: format === 'text'
      ? entries.map((entry) => entry.raw).join('\n')
      : JSON.stringify(entries, null, 2),
    filename,
  })

  if (state.exportMode === 'pinned') {
    if (state.pinnedEntries.length === 0) {
      return { error: 'No pinned events to export' }
    }

    return { export: build(state.pinnedEntries) }
  }

  const displayedSource = state.pinnedOnly ? state.pinnedEntries : state.logs

  if (state.exportMode === 'filtered') {
    const selected = isLogcatFilterDefault(state.filter)
      ? displayedSource
      : displayedSource.filter((entry) => matchesLogcatFilter(entry, state.filter))

    if (selected.length === 0) {
      return { error: 'No logs match the current filter' }
    }

    return { export: build(selected) }
  }

  if (state.logs.length === 0) {
    return { error: 'No logs to export' }
  }

  return { export: build(state.logs) }
}

export function formatLogcatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }

  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KiB`
  }

  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}

/**
 * Plain-English retention notice. Counts are only reported when the frontend
 * buffer actually discarded something, so an idle stream never claims a drop
 * that backpressure caused — the backend does not drop entries at all.
 */
export function resolveLogcatNotice(state: {
  droppedEntries: number
  droppedBytes: number
  evictedEntries: number
  evictedBytes: number
  evictedPinnedEntries: number
  clippedEntries: number
}): string | null {
  const parts: string[] = []

  if (state.droppedEntries > 0) {
    parts.push(
      `[${state.droppedEntries}] lines omitted (${formatLogcatBytes(state.droppedBytes)} over the buffer budget)`,
    )
  }

  if (state.evictedEntries > 0) {
    parts.push(
      `[${state.evictedEntries}] lines evicted (${formatLogcatBytes(state.evictedBytes)} over the buffer budget)`,
    )
  }

  if (state.evictedPinnedEntries > 0) {
    parts.push(`[${state.evictedPinnedEntries}] pins evicted by the pin limit`)
  }

  if (state.clippedEntries > 0) {
    parts.push(
      `[${state.clippedEntries}] lines clipped (single line over the ${formatLogcatBytes(LOGCAT_MAX_ENTRY_BYTES)} entry ceiling)`,
    )
  }

  return parts.length > 0 ? parts.join(' · ') : null
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
    set((state) => {
      // The cached size window is positional with the retained entries, so a
      // tighter limit re-trims both without serializing a single entry again.
      const retention = trimToBudget(state.logs, retainedSizes)
      retainedSizes = retention.sizes
      evictedEntryCount += retention.usage.evictedEntries
      evictedByteCount += retention.usage.evictedBytes
      return {
        bufferLimit: bounded,
        logs: retention.logs,
        bufferBytes: retention.usage.retainedBytes,
        bufferFull: retention.logs.length >= bounded,
        ...counterTotals(),
      }
    })
  },
  setPinnedOnly: (pinnedOnly) => set({ pinnedOnly }),
  setExportMode: (exportMode) => set({ exportMode }),
  togglePinned: (entry) =>
    set((state) => {
      const exists = state.pinnedEntries.some((pinned) => pinned.id === entry.id)
      const next = exists
        ? state.pinnedEntries.filter((pinned) => pinned.id !== entry.id)
        : [...state.pinnedEntries, entry]
      const retention = retainPins(next, sizesForPins(next, state.logs))

      return {
        pinnedEntries: retention.pinnedEntries,
        pinnedBytes: retention.pinnedBytes,
        ...counterTotals(),
      }
    }),
  clearPinned: () => set({ pinnedEntries: [], pinnedOnly: false, pinnedBytes: 0 }),
  clearCounters: () => {
    resetCounters()
    set(counterTotals())
  },
  clearLogs: () => {
    releaseFlushTimer()
    resetCounters()
    retainedSizes = []
    set({
      logs: [],
      bufferBytes: 0,
      lastUpdatedAt: Date.now(),
      bufferFull: false,
      ...counterTotals(),
    })
  },
  appendLogs: (entries, sizes) =>
    set((state) => {
      if (entries.length === 0) {
        return {}
      }

      // Only the arriving batch is measured. The retained entries keep the sizes
      // cached when they were admitted, so a flush over a full buffer walks the
      // size window instead of re-serializing every row it retains.
      const bounded = entries.map(boundEntry)
      const nextSizes = sizes ?? bounded.map(encodedBytes)
      const retention = trimToBudget(
        [...state.logs, ...bounded],
        [...retainedSizes, ...nextSizes],
      )
      retainedSizes = retention.sizes
      evictedEntryCount += retention.usage.evictedEntries
      evictedByteCount += retention.usage.evictedBytes

      return {
        logs: retention.logs,
        bufferBytes: retention.usage.retainedBytes,
        lastUpdatedAt: Date.now(),
        bufferFull: retention.logs.length >= currentBufferLimit,
        ...counterTotals(),
      }
    }),
  applyLineEvent: (entry) => {
    const dropped = enqueueEntries([{
      ...entry,
      level: normalizeLogLevel(entry.level),
    }])

    if (dropped) {
      set(counterTotals())
    }
  },
  applyBatchEvent: (entries) => {
    const dropped = enqueueEntries(entries.map((entry) => ({
      ...entry,
      level: normalizeLogLevel(entry.level),
    })))

    if (dropped) {
      set(counterTotals())
    }
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
    releaseFlushTimer()
    resetCounters()
    retainedSizes = []
    set({
      ...initialState,
      savedFilters: useLogcatStore.getState().savedFilters,
    })
  },
}))

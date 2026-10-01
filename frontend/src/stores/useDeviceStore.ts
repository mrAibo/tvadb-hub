import { create } from 'zustand'
import {
  getDeviceInfo,
  getDeviceMode,
  setActiveSerial as persistActiveSerial,
} from '@/services/deviceService'
import type {
  DeviceSummary,
  DeviceInfo,
  DeviceMode,
  PerformanceSnapshot,
  DeviceNicknames,
} from '@/lib/types'

interface DeviceState {
  devices: DeviceSummary[]
  activeSerial: string
  deviceInfo: DeviceInfo | null
  deviceMode: DeviceMode | null
  performance: PerformanceSnapshot | null
  nicknames: DeviceNicknames
  loading: boolean
  refreshing: boolean
  infoLoading: boolean
  perfLoading: boolean
  error: string | null
  lastUpdatedAt: number | null
}

interface DeviceActions {
  setDevices: (devices: DeviceSummary[]) => void
  setActiveSerial: (serial: string) => void
  setDeviceInfo: (info: DeviceInfo | null) => void
  setDeviceMode: (mode: DeviceMode | null) => void
  setPerformance: (perf: PerformanceSnapshot | null) => void
  setNicknames: (nicknames: DeviceNicknames) => void
  setNickname: (serial: string, nickname: string) => void
  getNickname: (serial: string) => string
  setLoading: (loading: boolean) => void
  setRefreshing: (refreshing: boolean) => void
  setInfoLoading: (infoLoading: boolean) => void
  setPerfLoading: (perfLoading: boolean) => void
  setError: (error: string | null) => void
  setLastUpdatedAt: (timestamp: number | null) => void
  reset: () => void
}

type DeviceStore = DeviceState & DeviceActions

const initialState: DeviceState = {
  devices: [],
  activeSerial: '',
  deviceInfo: null,
  deviceMode: null,
  performance: null,
  nicknames: {},
  loading: false,
  refreshing: false,
  infoLoading: false,
  perfLoading: false,
  error: null,
  lastUpdatedAt: null,
}

export const useDeviceStore = create<DeviceStore>()((set, get) => ({
  ...initialState,

  setDevices: (devices) => set({ devices }),
  setActiveSerial: (activeSerial) => set({ activeSerial }),
  setDeviceInfo: (deviceInfo) => set({ deviceInfo }),
  setDeviceMode: (deviceMode) => set({ deviceMode }),
  setPerformance: (performance) => set({ performance }),
  setNicknames: (nicknames) => set({ nicknames }),
  setNickname: (serial, nickname) =>
    set((state) => {
      const trimmed = serial.trim()
      if (!trimmed) return state
      const next = { ...state.nicknames }
      const trimmedNick = nickname.trim()
      if (trimmedNick) {
        next[trimmed] = trimmedNick
      } else {
        delete next[trimmed]
      }
      return { nicknames: next }
    }),
  getNickname: (serial) => get().nicknames[serial] ?? '',
  setLoading: (loading) => set({ loading }),
  setRefreshing: (refreshing) => set({ refreshing }),
  setInfoLoading: (infoLoading) => set({ infoLoading }),
  setPerfLoading: (perfLoading) => set({ perfLoading }),
  setError: (error) => set({ error }),
  setLastUpdatedAt: (lastUpdatedAt) => set({ lastUpdatedAt }),
  reset: () => set(initialState),
}))

/**
 * Shared, serialized device-selection worker.
 *
 * Every caller that wants to change the confirmed target - the device selector, the
 * dock, the sidebar and the background poll - goes through this single module-level
 * queue, so two mounted hook instances can never race two `SetActiveSerial` RPCs
 * against each other. The queue coalesces to the latest intent: a request enqueued
 * while another is in flight supersedes it, and only the newest generation is
 * allowed to write the store. A `restore` request (the poll re-anchoring a
 * disappeared serial) never overwrites a different serial the user picked.
 */
export type DeviceSelectionReason = 'user' | 'restore'

interface PendingDeviceSelection {
  serial: string
  reason: DeviceSelectionReason
  generation: number
}

let selectionGeneration = 0
let selectionWorker: Promise<void> | null = null
let pendingSelection: PendingDeviceSelection | null = null

export function requestDeviceSelection(
  serial: string,
  reason: DeviceSelectionReason = 'user',
): Promise<void> {
  const trimmed = serial.trim()
  if (!trimmed) {
    return Promise.resolve()
  }

  pendingSelection = { serial: trimmed, reason, generation: ++selectionGeneration }

  if (!selectionWorker) {
    selectionWorker = runDeviceSelectionWorker().finally(() => {
      selectionWorker = null
    })
  }

  return selectionWorker
}

async function runDeviceSelectionWorker(): Promise<void> {
  while (pendingSelection) {
    const request = pendingSelection
    pendingSelection = null

    // A newer intent arrived before this one started: never apply it.
    if (request.generation < selectionGeneration) {
      continue
    }

    // A background restore must never retarget, or even persist, over a serial the
    // user has already confirmed. Checked before the RPC so the backend selection
    // cannot drift away from the UI.
    const beforeState = useDeviceStore.getState()
    if (
      request.reason === 'restore' &&
      beforeState.activeSerial !== '' &&
      beforeState.activeSerial !== request.serial
    ) {
      continue
    }

    const store = useDeviceStore.getState()
    store.setRefreshing(true)

    try {
      await persistActiveSerial(request.serial)
      const [info, mode] = await Promise.all([
        getDeviceInfo(request.serial),
        getDeviceMode(request.serial),
      ])

      // Only the newest intent may publish.
      const superseded = request.generation !== selectionGeneration || pendingSelection !== null
      const state = useDeviceStore.getState()
      if (superseded) {
        continue
      }

      state.setActiveSerial(request.serial)
      state.setDeviceInfo(info)
      state.setDeviceMode(mode)
      state.setError(null)
    } catch (selectionError) {
      if (request.generation === selectionGeneration) {
        useDeviceStore
          .getState()
          .setError(
            selectionError instanceof Error
              ? selectionError.message
              : 'Failed to select device',
          )
      }
    } finally {
      if (!pendingSelection) {
        useDeviceStore.getState().setRefreshing(false)
      }
    }
  }
}

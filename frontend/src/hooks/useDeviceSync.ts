import { useEffect } from 'react'
import {
  getDevices,
  getActiveSerial,
  getDeviceInfo,
  getDeviceMode,
  getDeviceNicknames,
  autoReconnectRememberedWireless,
} from '@/services/deviceService'
import { useDeviceStore, requestDeviceSelection } from '@/stores/useDeviceStore'
import { useSettingsStore } from '@/stores/useSettingsStore'

const DEFAULT_DEVICE_POLL_SECONDS = 8
const WIRELESS_RECONNECT_INTERVAL = 20000

let syncPromise: Promise<void> | null = null
let freshPending = false

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Failed to sync devices'
}

// Read-only view of the persisted preferences. Polling must never hydrate or
// rewrite the settings draft, so it reads the published snapshot directly.
function devicePollIntervalMs(): number {
  const config = useSettingsStore.getState().appConfig
  if (!config || config.auto_refresh_devices === false) {
    return 0
  }
  const seconds = config.device_refresh_seconds
  const bounded = Number.isFinite(seconds) && seconds > 0 ? seconds : DEFAULT_DEVICE_POLL_SECONDS
  return bounded * 1000
}

async function syncDeviceState(isBackgroundRefresh: boolean) {
  const store = useDeviceStore.getState()

  if (isBackgroundRefresh) {
    store.setRefreshing(true)
  } else {
    store.setLoading(true)
  }

  try {
    const [nextDevices, persistedActiveSerial, persistedNicknames] = await Promise.all([
      getDevices(),
      getActiveSerial(),
      getDeviceNicknames(),
    ])

    store.setDevices(nextDevices)
    store.setNicknames(persistedNicknames)

    const firstDevice = nextDevices[0]
    const persistedExists = nextDevices.some(
      (device) => device.serial === persistedActiveSerial,
    )
    const currentSerial = useDeviceStore.getState().activeSerial

    if (!persistedActiveSerial && firstDevice) {
      // First run: adopt the first device through the shared queue.
      await requestDeviceSelection(firstDevice.serial, 'restore')
    } else if (!persistedExists) {
      // The confirmed serial disappeared: re-anchor once, through the same queue.
      if (firstDevice) {
        await requestDeviceSelection(firstDevice.serial, 'restore')
      } else {
        store.setActiveSerial('')
        store.setDeviceInfo(null)
        store.setDeviceMode('unknown')
      }
    } else if (currentSerial === '') {
      // First observation after a reload: adopt the persisted selection through the
      // shared queue, so it is serialized against any user intent.
      await requestDeviceSelection(persistedActiveSerial, 'restore')
    } else {
      // The poll never re-elects the persisted serial: the user's selection (or the
      // queue's latest intent) stays authoritative; only the facts are refreshed.
      const target = currentSerial
      const [nextDeviceInfo, nextDeviceMode] = await Promise.all([
        getDeviceInfo(target),
        getDeviceMode(target),
      ])
      if (useDeviceStore.getState().activeSerial === target) {
        store.setDeviceInfo(nextDeviceInfo)
        store.setDeviceMode(nextDeviceMode)
      }
    }

    store.setLastUpdatedAt(Date.now())
    store.setError(null)
  } catch (syncError) {
    store.setError(getErrorMessage(syncError))
  } finally {
    store.setLoading(false)
    store.setRefreshing(false)
  }
}

export interface RefreshDeviceStateOptions {
  /**
   * Guarantee a read that starts after this call, instead of joining a refresh
   * that was already in flight. Use it after a command that changes what
   * `adb devices` reports (pair/connect): an older in-flight refresh resolves with
   * a pre-command snapshot and would otherwise be mistaken for the new truth.
   * At most one extra read is performed, and only when a refresh is in flight.
   */
  fresh?: boolean
}

export function refreshDeviceState(
  isBackgroundRefresh = true,
  options: RefreshDeviceStateOptions = {},
): Promise<void> {
  // Concurrent fresh demands share one follow-up read instead of chaining reads.
  if (options.fresh && syncPromise) {
    if (freshPending) return syncPromise
    const inFlight = syncPromise
    const startFresh = () => {
      freshPending = false
      return runDeviceSync(isBackgroundRefresh, true)
    }
    const next = inFlight.then(startFresh, startFresh)
    freshPending = true
    syncPromise = next
    return next
  }
  return runDeviceSync(isBackgroundRefresh, options.fresh === true)
}

function runDeviceSync(isBackgroundRefresh: boolean, forceFresh: boolean): Promise<void> {
  if (syncPromise && !forceFresh) {
    return syncPromise
  }

  const next = syncDeviceState(isBackgroundRefresh).finally(() => {
    // Only the current owner clears the shared slot: a forced fresh read is
    // awaited by its own caller and must not truncate a newer refresh.
    if (syncPromise === next) {
      syncPromise = null
      freshPending = false
    }
  })
  syncPromise = next
  return next
}

export function useDeviceSync() {
  useEffect(() => {
    let reconnectRunning = false

    async function reconnectRememberedWireless() {
      if (reconnectRunning || document.hidden) return
      reconnectRunning = true
      try {
        await autoReconnectRememberedWireless()
        await refreshDeviceState(true)
      } catch {
        // Background recovery is best-effort. The explicit Wireless TV
        // diagnostics flow exposes actionable errors to the user.
      } finally {
        reconnectRunning = false
      }
    }

    void refreshDeviceState(false)
    void reconnectRememberedWireless()

    // The interval is re-read from the persisted config whenever it may have
    // changed, so a saved change applies without re-mounting and without ever
    // touching an unsaved settings draft.
    let deviceIntervalId = 0
    let currentIntervalMs = -1

    function applyPollInterval() {
      const interval = devicePollIntervalMs()
      if (interval === currentIntervalMs) return
      currentIntervalMs = interval
      if (deviceIntervalId) {
        window.clearInterval(deviceIntervalId)
        deviceIntervalId = 0
      }
      if (interval > 0) {
        deviceIntervalId = window.setInterval(() => {
          if (document.hidden) return
          applyPollInterval()
          void refreshDeviceState(true)
        }, interval)
      }
    }
    applyPollInterval()

    const reconnectIntervalId = window.setInterval(() => {
      void reconnectRememberedWireless()
    }, WIRELESS_RECONNECT_INTERVAL)

    function handleVisibilityChange() {
      if (!document.hidden) {
        applyPollInterval()
        void refreshDeviceState(true)
        void reconnectRememberedWireless()
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      if (deviceIntervalId) window.clearInterval(deviceIntervalId)
      window.clearInterval(reconnectIntervalId)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [])
}

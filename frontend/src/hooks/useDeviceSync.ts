import { useEffect } from 'react'
import {
  getDevices,
  getActiveSerial,
  setActiveSerial as persistActiveSerial,
  getDeviceInfo,
  getDeviceMode,
  getDeviceNicknames,
  autoReconnectRememberedWireless,
} from '@/services/deviceService'
import { useDeviceStore } from '@/stores/useDeviceStore'

const DEVICE_POLL_INTERVAL = 8000
const WIRELESS_RECONNECT_INTERVAL = 20000

let syncPromise: Promise<void> | null = null

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Failed to sync devices'
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

    let nextActiveSerial = persistedActiveSerial
    const firstDevice = nextDevices[0]

    if (!nextActiveSerial && firstDevice) {
      nextActiveSerial = firstDevice.serial
      await persistActiveSerial(nextActiveSerial)
    }

    const serialExists = nextDevices.some((device) => device.serial === nextActiveSerial)
    if (!serialExists) {
      nextActiveSerial = firstDevice?.serial ?? ''
      if (nextActiveSerial) {
        await persistActiveSerial(nextActiveSerial)
      }
    }

    store.setActiveSerial(nextActiveSerial)

    if (nextActiveSerial) {
      const [nextDeviceInfo, nextDeviceMode] = await Promise.all([
        getDeviceInfo(nextActiveSerial),
        getDeviceMode(nextActiveSerial),
      ])
      store.setDeviceInfo(nextDeviceInfo)
      store.setDeviceMode(nextDeviceMode)
    } else {
      store.setDeviceInfo(null)
      store.setDeviceMode('unknown')
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

export function refreshDeviceState(isBackgroundRefresh = true): Promise<void> {
  if (syncPromise) {
    return syncPromise
  }

  syncPromise = syncDeviceState(isBackgroundRefresh).finally(() => {
    syncPromise = null
  })

  return syncPromise
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

    const deviceIntervalId = window.setInterval(() => {
      if (document.hidden) return
      void refreshDeviceState(true)
    }, DEVICE_POLL_INTERVAL)

    const reconnectIntervalId = window.setInterval(() => {
      void reconnectRememberedWireless()
    }, WIRELESS_RECONNECT_INTERVAL)

    function handleVisibilityChange() {
      if (!document.hidden) {
        void refreshDeviceState(true)
        void reconnectRememberedWireless()
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      window.clearInterval(deviceIntervalId)
      window.clearInterval(reconnectIntervalId)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [])
}

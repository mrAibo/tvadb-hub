import { useEffect, useCallback, useRef } from 'react'
import { getPerformanceSnapshot } from '@/services/deviceService'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { useMetricsHistoryStore } from '@/stores/metricsHistoryStore'

const MONITOR_POLL_INTERVAL = 3000

export function useMonitor(serial: string, isActive: boolean) {
  const { performance, perfLoading, error, setPerformance, setPerfLoading, setError } = useDeviceStore()
  const { pushCPU, pushRAM, pushRX, reset } = useMetricsHistoryStore()

  // Every request carries the serial it was made for plus a generation. A reply is
  // only applied while it is still the newest request for the serial the store
  // currently confirms, so A->B->A can never resurrect an old device's sample.
  const generationRef = useRef(0)

  const refresh = useCallback(async () => {
    if (!serial || !isActive) {
      setPerformance(null)
      return
    }

    const requestSerial = serial
    const generation = ++generationRef.current
    setPerfLoading(true)
    setError(null)

    try {
      const snap = await getPerformanceSnapshot(requestSerial)
      const snapSerial = (snap as { serial?: string } | null)?.serial
      const stale =
        generation !== generationRef.current ||
        requestSerial !== useDeviceStore.getState().activeSerial ||
        snapSerial !== requestSerial
      if (stale) {
        return
      }
      setPerformance(snap)
      pushCPU(snap.cpuUsage)
      pushRAM(snap.ramUsage)
      pushRX(snap.networkRxSec)
    } catch (e) {
      const stale =
        generation !== generationRef.current ||
        requestSerial !== useDeviceStore.getState().activeSerial
      if (!stale) {
        setError(e instanceof Error ? e.message : 'Failed to fetch performance data')
        setPerformance(null)
      }
    } finally {
      if (generation === generationRef.current) {
        setPerfLoading(false)
      }
    }
  }, [serial, isActive, setPerformance, setPerfLoading, setError, pushCPU, pushRAM, pushRX])

  useEffect(() => {
    if (!serial || !isActive) {
      generationRef.current++
      setPerformance(null)
      reset()
      return
    }

    reset()
    void refresh()

    const intervalId = window.setInterval(() => {
      if (document.hidden) return
      void refresh()
    }, MONITOR_POLL_INTERVAL)

    function handleVisibilityChange() {
      if (!document.hidden) void refresh()
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      // Invalidate in-flight replies for the serial we are leaving.
      generationRef.current++
      window.clearInterval(intervalId)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [serial, isActive, refresh, setPerformance, reset])

  return {
    snapshot: performance,
    polling: perfLoading,
    error,
    refresh,
  }
}

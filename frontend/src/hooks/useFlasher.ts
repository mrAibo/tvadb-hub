import { useCallback, useEffect, useRef, useSyncExternalStore } from 'react'
import { toast } from 'sonner'
import * as fastbootSvc from '@/services/fastbootService'
import * as deviceSvc from '@/services/deviceService'
import { useFlasherStore, flashTargetRevision } from '@/stores/useFlasherStore'
import type { FlasherMode, FlasherState } from '@/lib/types'

const POLL_INTERVAL = 4000

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'An unexpected error occurred'
}

// ---------------------------------------------------------------------------
// Shared destructive-dispatch admission, common to every useFlasher() instance.
//
// The flag flips synchronously, before the first await, so a double click or a
// second card cannot start a second flash while one is still in flight. It is a
// module-level guard on purpose: it needs no store field, no context provider and
// no job framework, and every caller of the hook shares the same admission.
let flashDispatchBusy = false
const dispatchBusyListeners = new Set<() => void>()

function setFlashDispatchBusy(value: boolean) {
  flashDispatchBusy = value
  dispatchBusyListeners.forEach((listener) => listener())
}

export function isFlashDispatchBusy(): boolean {
  return flashDispatchBusy
}

export function subscribeFlashDispatchBusy(listener: () => void): () => void {
  dispatchBusyListeners.add(listener)
  return () => {
    dispatchBusyListeners.delete(listener)
  }
}

export function useFlashDispatchBusy(): boolean {
  return useSyncExternalStore(
    subscribeFlashDispatchBusy,
    isFlashDispatchBusy,
    isFlashDispatchBusy,
  )
}

// ---------------------------------------------------------------------------
// Serial -> model labels, refreshed with the device poll, so a destructive
// confirmation can name the captured device whenever a model is actually known.
const deviceLabels = new Map<string, string>()
const deviceLabelListeners = new Set<() => void>()

function replaceDeviceLabels(entries: Map<string, string>) {
  let changed = entries.size !== deviceLabels.size
  if (!changed) {
    for (const [serial, label] of entries) {
      if (deviceLabels.get(serial) !== label) {
        changed = true
        break
      }
    }
  }
  if (!changed) return
  deviceLabels.clear()
  entries.forEach((label, serial) => deviceLabels.set(serial, label))
  deviceLabelListeners.forEach((listener) => listener())
}

export function subscribeFlashDeviceLabels(listener: () => void): () => void {
  deviceLabelListeners.add(listener)
  return () => {
    deviceLabelListeners.delete(listener)
  }
}

export function flashDeviceLabel(serial: string): string | null {
  return deviceLabels.get(serial) ?? null
}

export function useFlashDeviceLabel(serial: string): string | null {
  return useSyncExternalStore(
    subscribeFlashDeviceLabels,
    () => deviceLabels.get(serial) ?? null,
    () => deviceLabels.get(serial) ?? null,
  )
}

// ---------------------------------------------------------------------------
// Destructive confirmation payload: the target and the inputs captured when the
// user opened the dialog. The accepted callback dispatches exactly this snapshot.
export interface FlashConsent {
  serial: string
  deviceLabel: string | null
  revision: number
  mode?: string
  partition?: string
  imagePath?: string
  folderPath?: string
  steps?: string[]
  zipPath?: string
}

export function capturePartitionConsent(state: FlasherState): FlashConsent {
  return {
    serial: state.activeFastbootSerial,
    deviceLabel: deviceLabels.get(state.activeFastbootSerial) ?? null,
    revision: flashTargetRevision(),
    mode: state.deviceMode ?? undefined,
    partition: state.selectedPartition,
    imagePath: state.selectedImagePath,
  }
}

export function captureBatchConsent(state: FlasherState): FlashConsent {
  return {
    serial: state.activeFastbootSerial,
    deviceLabel: deviceLabels.get(state.activeFastbootSerial) ?? null,
    revision: flashTargetRevision(),
    mode: state.deviceMode ?? undefined,
    folderPath: state.romFolderPath,
    steps: [...state.selectedPartitions].sort(),
  }
}

// captureWipeConsent captures the wipe target. Wipe runs on a fastboot device, so the
// live check requires fastboot presence.
export function captureWipeConsent(state: FlasherState): FlashConsent {
  return {
    serial: state.activeFastbootSerial,
    deviceLabel: deviceLabels.get(state.activeFastbootSerial) ?? null,
    revision: flashTargetRevision(),
    mode: state.deviceMode ?? undefined,
  }
}

// captureSideloadConsent captures the sideload target AND the chosen ZIP. Sideload
// runs on an ADB-recovery device: the fastboot device list is expected to be empty, so
// the live check uses the sideload mode plus the captured serial/path instead of
// fastboot-list membership.
export function captureSideloadConsent(state: FlasherState): FlashConsent {
  return {
    serial: state.activeFastbootSerial,
    deviceLabel: deviceLabels.get(state.activeFastbootSerial) ?? null,
    revision: flashTargetRevision(),
    mode: state.deviceMode ?? undefined,
    zipPath: state.sideloadFilePath,
  }
}

// consentRefusal re-checks the captured snapshot against the LIVE store right before
// dispatch. The revision comparison catches value-changing edits (including an
// A -> B -> A sequence that looks restored or a device-context switch), and the
// explicit field comparisons catch a target that changed without a revision move.
//
// The guard is mode specific on purpose: wipe/partition/batch require the device to be
// in the live fastboot list, while sideload must NOT require that membership because a
// sideload target is an ADB-recovery device whose fastboot list is legitimately empty.
function consentRefusal(
  kind: 'partition' | 'batch' | 'wipe' | 'sideload',
  consent: FlashConsent,
  live: FlasherState,
): string | null {
  if (!consent.serial) return 'No confirmed device'
  if (flashTargetRevision() !== consent.revision) {
    return 'The device or the inputs changed — review and confirm again'
  }
  if (live.activeFastbootSerial !== consent.serial) {
    return `The confirmed device changed to ${live.activeFastbootSerial || 'none'} — review and confirm again`
  }
  if (kind !== 'sideload' && !live.fastbootDevices.some((device) => device.serial === consent.serial)) {
    return `Fastboot device ${consent.serial} is no longer connected`
  }
  if (kind === 'sideload') {
    if (consent.mode !== 'sideload' || live.deviceMode !== 'sideload') {
      return 'The device is no longer in sideload mode — review and confirm again'
    }
    if (live.sideloadFilePath !== (consent.zipPath ?? '')) {
      return 'The selected ZIP changed — review and confirm again'
    }
    return null
  }
  if (kind === 'wipe') {
    return null
  }
  if (kind === 'partition') {
    if (live.selectedPartition !== (consent.partition ?? '')) {
      return 'The partition changed — review and confirm again'
    }
    if (live.selectedImagePath !== (consent.imagePath ?? '')) {
      return 'The image file changed — review and confirm again'
    }
    return null
  }
  if (live.romFolderPath !== (consent.folderPath ?? '')) {
    return 'The ROM folder changed — review and confirm again'
  }
  const liveSteps = [...live.selectedPartitions].sort().join('\u0000')
  const consentSteps = [...(consent.steps ?? [])].sort().join('\u0000')
  if (liveSteps !== consentSteps) {
    return 'The selected partitions changed — review and confirm again'
  }
  return null
}

function beginFlashDispatch(): boolean {
  if (flashDispatchBusy) {
    toast.error('A flash operation is already running')
    return false
  }
  setFlashDispatchBusy(true)
  return true
}

export function useFlasher() {
  const store = useFlasherStore()
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const slotLoadedRef = useRef<string>('')
  // Prevents polling from changing mode or showing error toasts during active operations
  const operationInProgressRef = useRef(false)

  const isAnyOperationRunning =
    store.runningFlash ||
    store.runningBatchFlash ||
    store.runningWipe ||
    store.runningSideload ||
    store.runningSlotChange

  useEffect(() => {
    operationInProgressRef.current = isAnyOperationRunning
  }, [isAnyOperationRunning])

  useEffect(() => {
    const unsub = fastbootSvc.onFlashStepStatus((event) => {
      const status = event.status === 'flashing' ? 'running' : event.status
      store.setFlashPlanStepStatus(event.partition, status, event.message || null)
    })
    return unsub
  }, [])

  const syncDevices = useCallback(
    async (isBackground = false) => {
      if (operationInProgressRef.current) return

      if (isBackground) {
        store.setRefreshingDevices(true)
      } else {
        store.setLoadingDevices(true)
      }

      try {
        const [fbDevices, adbDevices] = await Promise.allSettled([
          fastbootSvc.getFastbootDevices(),
          deviceSvc.getDevices(),
        ])

        const fastbootList = fbDevices.status === 'fulfilled' ? fbDevices.value : []
        const adbList = adbDevices.status === 'fulfilled' ? adbDevices.value : []

        store.setFastbootDevices(fastbootList)

        // Keep the serial -> model labels in step with the poll so a confirmation can
        // name the captured device when a model is known.
        const labels = new Map<string, string>()
        adbList.forEach((device) => {
          if (device.serial && device.model) labels.set(device.serial, device.model)
        })
        replaceDeviceLabels(labels)

        const currentSerial = store.activeFastbootSerial

        if (fastbootList.length > 0) {
          const stillConnected = currentSerial && fastbootList.some((d) => d.serial === currentSerial)
          if (!stillConnected) {
            store.setActiveFastbootSerial(fastbootList[0].serial)
          }
          const mode: FlasherMode = store.isUserspace ? 'fastbootd' : 'fastboot'
          store.setDeviceMode(mode)
        } else if (adbList.length > 0) {
          const sideloadDevice = adbList.find((d) => d.state === 'sideload')
          if (sideloadDevice) {
            store.setActiveFastbootSerial(sideloadDevice.serial)
            store.setDeviceMode('sideload')
          } else {
            store.setDeviceMode(null)
          }
        } else {
          store.setDeviceMode(null)
          store.setActiveFastbootSerial('')
        }

        store.setLastUpdatedAt(Date.now())
        store.setError(null)
      } catch (err) {
        // Suppress error state updates during active operations to avoid toast spam
        if (!operationInProgressRef.current) {
          store.setError(getErrorMessage(err))
        }
      } finally {
        store.setLoadingDevices(false)
        store.setRefreshingDevices(false)
      }
    },
    [store.activeFastbootSerial, store.isUserspace],
  )

  useEffect(() => {
    let active = true

    async function poll() {
      if (!active) return
      await syncDevices(false)
    }

    poll()
    pollRef.current = setInterval(() => {
      if (active) syncDevices(true)
    }, POLL_INTERVAL)

    return () => {
      active = false
      if (pollRef.current) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }
  }, [syncDevices])

  useEffect(() => {
    const serial = store.activeFastbootSerial
    const mode = store.deviceMode
    if (!serial || mode === 'sideload') {
      slotLoadedRef.current = ''
      return
    }
    if (slotLoadedRef.current === serial) return

    slotLoadedRef.current = serial
    fastbootSvc
      .getActiveSlot(serial)
      .then((slot) => store.setCurrentSlot(slot))
      .catch(() => store.setCurrentSlot(''))

    fastbootSvc
      .isUserspaceFastboot(serial)
      .then((isU) => {
        store.setIsUserspace(isU)
        if (isU) store.setDeviceMode('fastbootd')
        else store.setDeviceMode('fastboot')
      })
      .catch(() => store.setIsUserspace(false))
  }, [store.activeFastbootSerial, store.deviceMode])

  const chooseImageFile = useCallback(async () => {
    try {
      const path = await fastbootSvc.selectFlashImageFile()
      if (path) store.setSelectedImagePath(path)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [])

  const chooseSideloadFile = useCallback(async () => {
    try {
      const path = await fastbootSvc.selectSideloadFile()
      if (path) store.setSideloadFilePath(path)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [])

  const chooseRomFolder = useCallback(async () => {
    try {
      const path = await fastbootSvc.selectRomFolder()
      if (path) store.setRomFolderPath(path)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [])

  const loadActiveSlot = useCallback(async () => {
    const serial = store.activeFastbootSerial
    if (!serial) return
    try {
      const slot = await fastbootSvc.getActiveSlot(serial)
      store.setCurrentSlot(slot)
    } catch {
      store.setCurrentSlot('')
    }
  }, [store.activeFastbootSerial])

  const applyActiveSlot = useCallback(
    async (slot: string) => {
      const serial = store.activeFastbootSerial
      if (!serial) return
      if (operationInProgressRef.current) return
      store.setRunningSlotChange(true)
      try {
        await fastbootSvc.setActiveSlot(serial, slot)
        store.setCurrentSlot(slot)
        toast.success(`Active slot changed to ${slot.toUpperCase()}`)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setRunningSlotChange(false)
      }
    },
    [store.activeFastbootSerial],
  )

  const scanSelectedRomFolder = useCallback(async () => {
    const folderPath = store.romFolderPath
    if (!folderPath) {
      toast.error('Select a ROM folder first')
      return
    }
    store.setScanningPlan(true)
    try {
      const plan = await fastbootSvc.scanRomFolder(folderPath)
      store.setFlashPlan(plan)
      toast.success(`Found ${plan.steps.length} partition(s) in ROM folder`)
    } catch (err) {
      toast.error(getErrorMessage(err))
      store.setFlashPlan(null)
    } finally {
      store.setScanningPlan(false)
    }
  }, [store.romFolderPath])

  // Destructive single-partition flash. The caller passes the consent snapshot it
  // captured when the user opened the confirmation dialog; the target and the inputs
  // are re-checked against the LIVE store before the strict backend twin is called.
  const executeFlashPartition = useCallback(async (consent: FlashConsent) => {
    if (!beginFlashDispatch()) return
    store.setError(null)
    try {
      const refusal = consentRefusal('partition', consent, useFlasherStore.getState())
      if (refusal) {
        store.setError(refusal)
        toast.error(refusal)
        return
      }
      store.setRunningFlash(true)
      try {
        const result = await fastbootSvc.flashPartitionForDevice(
          consent.serial,
          consent.partition ?? '',
          consent.imagePath ?? '',
        )
        toast.success(result || `Flashed ${consent.partition} successfully`)
        store.setSelectedPartition('')
        store.setSelectedImagePath('')
      } catch (err) {
        const msg = getErrorMessage(err)
        store.setError(msg)
        toast.error(msg)
      } finally {
        store.setRunningFlash(false)
      }
    } finally {
      setFlashDispatchBusy(false)
    }
  }, [])

  // Destructive batch flash with the same consent contract: one captured serial,
  // folder and step selection, re-checked live before the batch twin is called.
  const executeBatchFlash = useCallback(async (consent: FlashConsent) => {
    if (!beginFlashDispatch()) return
    store.setError(null)
    try {
      const live = useFlasherStore.getState()
      const refusal = consentRefusal('batch', consent, live)
      if (refusal) {
        store.setError(refusal)
        toast.error(refusal)
        return
      }
      const plan = live.flashPlan
      if (!plan) {
        toast.error('Scan the ROM folder again before flashing')
        return
      }
      const selected = consent.steps ?? []
      const filteredSteps = live.flashPlanSteps.filter((s) => selected.includes(s.partition))
      if (filteredSteps.length === 0) {
        toast.error('No partitions to flash')
        return
      }

      store.setRunningBatchFlash(true)
      for (const step of filteredSteps) {
        store.setFlashPlanStepStatus(step.partition, 'pending')
      }

      try {
        const filteredPlan = { steps: plan.steps.filter((s) => selected.includes(s.partition)) }
        await fastbootSvc.flashRomFolderForDevice(
          consent.serial,
          consent.folderPath ?? '',
          filteredPlan,
        )
        toast.success(`Batch flash completed: ${filteredSteps.length} partition(s)`)
      } catch (err) {
        const msg = getErrorMessage(err)
        // Per-step error state already arrives via the flash_step_status event listener.
        store.setError(msg)
        toast.error(msg)
      } finally {
        store.setRunningBatchFlash(false)
      }
    } finally {
      setFlashDispatchBusy(false)
    }
  }, [])

  const executeWipeData = useCallback(async (consent: FlashConsent) => {
    if (!beginFlashDispatch()) return
    store.setError(null)
    try {
      const refusal = consentRefusal('wipe', consent, useFlasherStore.getState())
      if (refusal) {
        store.setError(refusal)
        toast.error(refusal)
        return
      }
      store.setRunningWipe(true)
      try {
        const result = await fastbootSvc.wipeData(consent.serial)
        toast.success(result || 'Device data wiped successfully')
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setRunningWipe(false)
      }
    } finally {
      setFlashDispatchBusy(false)
    }
  }, [])

  const executeSideload = useCallback(async (consent: FlashConsent) => {
    if (!beginFlashDispatch()) return
    store.setError(null)
    try {
      const refusal = consentRefusal('sideload', consent, useFlasherStore.getState())
      if (refusal) {
        store.setError(refusal)
        toast.error(refusal)
        return
      }
      const zipPath = consent.zipPath ?? ''
      if (!zipPath) {
        toast.error('Select a ZIP file to sideload')
        return
      }
      store.setRunningSideload(true)
      try {
        const result = await fastbootSvc.sideloadPackage(consent.serial, zipPath)
        toast.success(result || 'Sideload completed')
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setRunningSideload(false)
      }
    } finally {
      setFlashDispatchBusy(false)
    }
  }, [])

  const executeCustomCommand = useCallback(async () => {
    const serial = store.activeFastbootSerial
    const args = store.customCommand.trim()
    if (!serial) {
      toast.error('No fastboot device connected')
      return
    }
    if (!args) {
      toast.error('Enter a fastboot command')
      return
    }
    store.setRunningCommand(true)
    try {
      const output = await fastbootSvc.runCustomFastbootCommand(serial, args)
      store.setCustomCommandOutput(output)
    } catch (err) {
      store.setCustomCommandOutput(getErrorMessage(err))
    } finally {
      store.setRunningCommand(false)
    }
  }, [store.activeFastbootSerial, store.customCommand])

  const resetFlashPlan = useCallback(() => {
    store.setFlashPlan(null)
    store.setRomFolderPath('')
  }, [])

  // WOF (Wake on Fastboot): continue boot out of fastboot without a physical
  // Start/Power press. Falls back to the active serial when none passed.
  const continueBoot = useCallback(async () => {
    const serial = store.activeFastbootSerial
    if (!serial) {
      toast.error('No fastboot device connected')
      return
    }
    try {
      const result = await fastbootSvc.fastbootContinue(serial)
      toast.success(result || `Continuing boot on ${serial}`)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [store.activeFastbootSerial])

  // WOF: wake the device screen via KEYCODE_WAKEUP (power-button replacement).
  const wakeScreen = useCallback(async () => {
    const serial = store.activeFastbootSerial
    if (!serial) {
      toast.error('No device connected')
      return
    }
    try {
      const result = await fastbootSvc.wakeScreen(serial)
      toast.success(result || `Wake signal sent to ${serial}`)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [store.activeFastbootSerial])

  // WOF: wake + dismiss non-secure keyguard (one-tap turn-on).
  const wakeAndUnlock = useCallback(async () => {
    const serial = store.activeFastbootSerial
    if (!serial) {
      toast.error('No device connected')
      return
    }
    try {
      const result = await fastbootSvc.wakeAndUnlock(serial)
      toast.success(result || `Wake + unlock sent to ${serial}`)
    } catch (err) {
      toast.error(getErrorMessage(err))
    }
  }, [store.activeFastbootSerial])

  // WOF: toggle "Stay awake while charging" so the screen never sleeps on USB.
  const setStayAwake = useCallback(
    async (enabled: boolean) => {
      const serial = store.activeFastbootSerial
      if (!serial) {
        toast.error('No device connected')
        return
      }
      try {
        const result = await fastbootSvc.setStayAwakeWhileCharging(serial, enabled)
        toast.success(result)
      } catch (err) {
        toast.error(getErrorMessage(err))
        throw err
      }
    },
    [store.activeFastbootSerial],
  )

  const getStayAwake = useCallback(async (): Promise<boolean> => {
    const serial = store.activeFastbootSerial
    if (!serial) return false
    try {
      return await fastbootSvc.getStayAwakeWhileCharging(serial)
    } catch {
      return false
    }
  }, [store.activeFastbootSerial])

  return {
    ...store,
    syncFastbootDevices: syncDevices,
    dispatchBusy: flashDispatchBusy,
    capturePartitionConsent: () => capturePartitionConsent(useFlasherStore.getState()),
    captureBatchConsent: () => captureBatchConsent(useFlasherStore.getState()),
    captureWipeConsent: () => captureWipeConsent(useFlasherStore.getState()),
    captureSideloadConsent: () => captureSideloadConsent(useFlasherStore.getState()),
    chooseImageFile,
    chooseSideloadFile,
    chooseRomFolder,
    loadActiveSlot,
    applyActiveSlot,
    scanSelectedRomFolder,
    executeFlashPartition,
    executeBatchFlash,
    executeWipeData,
    executeSideload,
    executeCustomCommand,
    resetFlashPlan,
    continueBoot,
    wakeScreen,
    wakeAndUnlock,
    setStayAwake,
    getStayAwake,
  }
}

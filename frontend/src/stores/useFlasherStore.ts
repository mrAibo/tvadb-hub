import { create } from 'zustand'
import { immer } from 'zustand/middleware/immer'
import { useSyncExternalStore } from 'react'
import type {
  FlasherState,
  FlasherActions,
  FlashPlan,
  FlashPlanStepStatus,
  OperationStatus,
} from '@/lib/types'

const initialState: FlasherState = {
  fastbootDevices: [],
  activeFastbootSerial: '',
  deviceMode: null,
  isUserspace: false,
  selectedPartition: '',
  selectedImagePath: '',
  romFolderPath: '',
  flashPlan: null,
  flashPlanSteps: [],
  selectedPartitions: [],
  currentSlot: '',
  customCommand: '',
  customCommandOutput: '',
  sideloadFilePath: '',
  loadingDevices: false,
  refreshingDevices: false,
  scanningPlan: false,
  runningFlash: false,
  runningBatchFlash: false,
  runningWipe: false,
  runningSideload: false,
  runningSlotChange: false,
  runningCommand: false,
  error: null,
  lastUpdatedAt: null,
}

// Target revision registry.
//
// Every mutation that changes the flash TARGET (the confirmed fastboot serial) or the
// flash INPUTS (partition/image, ROM folder, plan, selected steps) bumps one
// monotonic revision. A destructive confirmation captures the revision when the user
// opens the dialog and compares it again right before dispatch, so a later change
// invalidates consent even when the values happen to look identical again (the
// A -> B -> A case a plain value comparison would miss).
let targetRevision = 0
const revisionListeners = new Set<() => void>()

function bumpTargetRevision() {
  targetRevision += 1
  revisionListeners.forEach((listener) => listener())
}

export function flashTargetRevision(): number {
  return targetRevision
}

export function subscribeFlashTargetRevision(listener: () => void): () => void {
  revisionListeners.add(listener)
  return () => {
    revisionListeners.delete(listener)
  }
}

export function useFlashTargetRevision(): number {
  return useSyncExternalStore(
    subscribeFlashTargetRevision,
    flashTargetRevision,
    flashTargetRevision,
  )
}

function createPlanSteps(plan: FlashPlan): FlashPlanStepStatus[] {
  return plan.steps.map((step) => ({
    partition: step.partition,
    imageFile: step.image_file,
    status: 'idle' as OperationStatus,
    detail: null,
  }))
}

export const useFlasherStore = create<FlasherState & FlasherActions>()(immer((set, get) => ({
  ...initialState,

  setFastbootDevices: (devices) => set({ fastbootDevices: devices }),
  // A repeated identical value (for example the device poll re-adopting the serial
  // that is already confirmed) must not bump the revision, otherwise a dialog would
  // go stale on its own. Real changes - including A -> B -> A - always bump.
  setActiveFastbootSerial: (serial) => {
    if (get().activeFastbootSerial === serial) return
    set({ activeFastbootSerial: serial })
    bumpTargetRevision()
  },
  // A device-context switch (fastboot / fastbootd / sideload) is part of the consent
  // context: capture a confirmation, switch the context and switch back, and the
  // revision has moved, so the stale dialog cannot be accepted.
  setDeviceMode: (mode) => {
    if (get().deviceMode === mode) return
    set({ deviceMode: mode })
    bumpTargetRevision()
  },
  setIsUserspace: (isUserspace) => set({ isUserspace }),
  setSelectedPartition: (partition) => {
    if (get().selectedPartition === partition) return
    set({ selectedPartition: partition })
    bumpTargetRevision()
  },
  setSelectedImagePath: (path) => {
    if (get().selectedImagePath === path) return
    set({ selectedImagePath: path })
    bumpTargetRevision()
  },
  setRomFolderPath: (path) => {
    if (get().romFolderPath === path) return
    set({ romFolderPath: path })
    bumpTargetRevision()
  },

  setFlashPlan: (plan) => {
    if (!plan) {
      set({ flashPlan: null, flashPlanSteps: [], selectedPartitions: [] })
      bumpTargetRevision()
      return
    }
    const steps = createPlanSteps(plan)
    set({
      flashPlan: plan,
      flashPlanSteps: steps,
      selectedPartitions: steps.map((s) => s.partition),
    })
    bumpTargetRevision()
  },

  setFlashPlanStepStatus: (partition, status, detail = null) => {
    set((state) => {
      const step = state.flashPlanSteps.find((s) => s.partition === partition)
      if (step) {
        step.status = status
        step.detail = detail
      }
    })
  },

  togglePartitionSelection: (partition) => {
    set((state) => {
      const idx = state.selectedPartitions.indexOf(partition)
      if (idx >= 0) {
        state.selectedPartitions.splice(idx, 1)
      } else {
        state.selectedPartitions.push(partition)
      }
    })
    bumpTargetRevision()
  },

  selectAllPartitions: () => {
    set((state) => {
      state.selectedPartitions = state.flashPlanSteps.map((s) => s.partition)
    })
    bumpTargetRevision()
  },

  deselectAllPartitions: () => {
    set({ selectedPartitions: [] })
    bumpTargetRevision()
  },

  setCurrentSlot: (slot) => set({ currentSlot: slot }),
  setCustomCommand: (command) => set({ customCommand: command }),
  setCustomCommandOutput: (output) => set({ customCommandOutput: output }),
  // The chosen ZIP is an input of the sideload confirmation, so it participates in
  // the same revision contract as the flash inputs.
  setSideloadFilePath: (path) => {
    if (get().sideloadFilePath === path) return
    set({ sideloadFilePath: path })
    bumpTargetRevision()
  },
  setLoadingDevices: (loading) => set({ loadingDevices: loading }),
  setRefreshingDevices: (refreshing) => set({ refreshingDevices: refreshing }),
  setScanningPlan: (scanning) => set({ scanningPlan: scanning }),
  setRunningFlash: (running) => set({ runningFlash: running }),
  setRunningBatchFlash: (running) => set({ runningBatchFlash: running }),
  setRunningWipe: (running) => set({ runningWipe: running }),
  setRunningSideload: (running) => set({ runningSideload: running }),
  setRunningSlotChange: (running) => set({ runningSlotChange: running }),
  setRunningCommand: (running) => set({ runningCommand: running }),
  setError: (error) => set({ error }),
  setLastUpdatedAt: (time) => set({ lastUpdatedAt: time }),

  reset: () => {
    set(initialState)
    bumpTargetRevision()
  },
})))

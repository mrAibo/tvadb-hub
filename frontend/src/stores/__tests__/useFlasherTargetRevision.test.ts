import { beforeEach, describe, expect, it } from 'vitest'
import { flashTargetRevision, useFlasherStore } from '../useFlasherStore'
import type { FlashPlan } from '@/lib/types'

const makePlan = (steps: { partition: string; image_file: string }[]): FlashPlan => ({ steps })

describe('flash target revision', () => {
  beforeEach(() => {
    useFlasherStore.getState().reset()
  })

  it('bumps on a serial change, on an input change and on the A -> B -> A case', () => {
    const store = useFlasherStore.getState()

    const start = flashTargetRevision()
    store.setActiveFastbootSerial('F1')
    const afterFirst = flashTargetRevision()
    expect(afterFirst).toBeGreaterThan(start)

    store.setActiveFastbootSerial('F2')
    const afterSecond = flashTargetRevision()
    expect(afterSecond).toBeGreaterThan(afterFirst)

    // A -> B -> A: the value looks restored but the revision moved, so a consent
    // captured before the round trip is still invalid.
    store.setActiveFastbootSerial('F1')
    const afterRoundTrip = flashTargetRevision()
    expect(afterRoundTrip).toBeGreaterThan(afterSecond)

    const beforeInput = flashTargetRevision()
    store.setSelectedImagePath('/boot.img')
    expect(flashTargetRevision()).toBeGreaterThan(beforeInput)
  })

  it('does not bump when the same value is re-applied', () => {
    const store = useFlasherStore.getState()
    store.setActiveFastbootSerial('F1')
    const afterFirst = flashTargetRevision()
    store.setActiveFastbootSerial('F1')
    expect(flashTargetRevision()).toBe(afterFirst)

    store.setSelectedPartition('boot')
    const afterPartition = flashTargetRevision()
    store.setSelectedPartition('boot')
    expect(flashTargetRevision()).toBe(afterPartition)
  })

  it('bumps on plan, selection and reset changes', () => {
    const store = useFlasherStore.getState()
    store.setFlashPlan(makePlan([
      { partition: 'boot', image_file: '/boot.img' },
      { partition: 'vbmeta', image_file: '/vbmeta.img' },
    ]))
    const afterPlan = flashTargetRevision()

    store.togglePartitionSelection('boot')
    const afterToggle = flashTargetRevision()
    expect(afterToggle).toBeGreaterThan(afterPlan)

    store.setRomFolderPath('/rom')
    expect(flashTargetRevision()).toBeGreaterThan(afterToggle)

    const beforeReset = flashTargetRevision()
    store.reset()
    expect(flashTargetRevision()).toBeGreaterThan(beforeReset)
  })
})

describe('sideload input and device-context revision', () => {
  beforeEach(() => {
    useFlasherStore.getState().reset()
  })

  it('bumps when the sideload ZIP changes and not when the same path is re-applied', () => {
    const store = useFlasherStore.getState()
    const start = flashTargetRevision()

    store.setSideloadFilePath('/update.zip')
    const afterFirst = flashTargetRevision()
    expect(afterFirst).toBeGreaterThan(start)

    store.setSideloadFilePath('/update.zip')
    expect(flashTargetRevision()).toBe(afterFirst)

    store.setSideloadFilePath('/other.zip')
    expect(flashTargetRevision()).toBeGreaterThan(afterFirst)
  })

  it('bumps on a device-context switch, including A -> B -> A', () => {
    const store = useFlasherStore.getState()
    const start = flashTargetRevision()

    store.setDeviceMode('sideload')
    const afterSideload = flashTargetRevision()
    expect(afterSideload).toBeGreaterThan(start)

    store.setDeviceMode('sideload')
    expect(flashTargetRevision()).toBe(afterSideload)

    store.setDeviceMode('fastboot')
    const afterFastboot = flashTargetRevision()
    expect(afterFastboot).toBeGreaterThan(afterSideload)

    // A -> B -> A: the context looks restored but the revision moved, so a consent
    // captured before the round trip stays invalid.
    store.setDeviceMode('sideload')
    expect(flashTargetRevision()).toBeGreaterThan(afterFastboot)
  })
})

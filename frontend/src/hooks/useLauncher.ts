import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDeviceStore } from '@/stores/useDeviceStore'
import {
  applyLauncher,
  cancelLauncherOperation,
  preflightLauncher,
  readLauncherRecovery,
  restoreLauncher,
  testLauncherCandidate,
} from '@/services/launcherService'
import type {
  ApplyResult,
  CandidateTestResult,
  HomeCandidate,
  Preflight,
  RecordSummary,
  RestoreResult,
} from '../../bindings/ADBKit/internal/launcher/models'

export type LauncherStep =
  | 'idle'
  | 'preflight'
  | 'candidate-test'
  | 'consent'
  | 'apply'
  | 'cancel'
  | 'restore'
  | 'recovery'

// Consent is the explicit confirmation for ONE target: the owned operation id, the
// serial it was granted for and the exact current/candidate pair the user saw. It is
// never inferred from the live selection, and it is dropped whenever the target
// changes.
export interface LauncherConsent {
  operationId: string
  serial: string
  generation: number
  expectedComponent: string
  candidateComponent: string
  testedAt: number
}

function newOperationId(): string {
  const globalCrypto = globalThis.crypto
  if (globalCrypto && typeof globalCrypto.randomUUID === 'function') {
    return globalCrypto.randomUUID()
  }
  // Deterministic-enough fallback for environments without crypto.randomUUID.
  return `launcher-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function countHomeForPackage(candidates: HomeCandidate[], component: string): number {
  const pkg = component.split('/')[0]
  if (!pkg) return 0
  return candidates.filter((candidate) => candidate.package === pkg).length
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

export function useLauncher() {
  const activeSerial = useDeviceStore((state) => state.activeSerial)

  const [preflight, setPreflight] = useState<Preflight | null>(null)
  const [testResult, setTestResult] = useState<CandidateTestResult | null>(null)
  const [applyResult, setApplyResult] = useState<ApplyResult | null>(null)
  const [restoreResult, setRestoreResult] = useState<RestoreResult | null>(null)
  const [consent, setConsent] = useState<LauncherConsent | null>(null)
  const [recovery, setRecovery] = useState<RecordSummary[]>([])
  const [recoveryError, setRecoveryError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState<LauncherStep | 'recovery' | null>(null)

  // Every request carries the serial and the generation it was issued for. A reply is
  // only applied while both still match, which also covers A -> B -> A.
  const generationRef = useRef(0)
  const applyInFlightRef = useRef(false)

  const liveSerial = useCallback(() => useDeviceStore.getState().activeSerial, [])

  // A target change invalidates consent and every device-bound result. Nothing is
  // retargeted: the stale values are simply dropped and must be re-confirmed.
  useEffect(() => {
    generationRef.current += 1
    setConsent(null)
    setPreflight(null)
    setTestResult(null)
    setApplyResult(null)
    setRestoreResult(null)
    setError(null)
  }, [activeSerial])

  const ambiguousPackages = useMemo(() => {
    if (!preflight) return []
    const counts = new Map<string, number>()
    for (const candidate of preflight.homeCandidates ?? []) {
      counts.set(candidate.package, (counts.get(candidate.package) ?? 0) + 1)
    }
    return [...counts.entries()]
      .filter(([, count]) => count > 1)
      .map(([pkg, count]) => `${pkg} (${count} HOME activities)`)
      .sort()
  }, [preflight])

  const blockedReason = useMemo(() => {
    if (!activeSerial) return 'No confirmed device is selected'
    if (!preflight) return 'Run the read-only check first'
    if (!preflight.supported) {
      return preflight.reason || 'This device is not supported for a guarded launcher change'
    }
    if (!preflight.restorable || !preflight.currentHome) {
      return preflight.reason || 'The current HOME cannot be restored exactly'
    }
    if (ambiguousPackages.length > 0) {
      return `Refused: ${ambiguousPackages.join(', ')}`
    }
    if (!consent) return 'Confirm the target, the current and the new launcher first'
    return null
  }, [activeSerial, preflight, ambiguousPackages, consent])

  const loadPreflight = useCallback(async () => {
    const serial = liveSerial()
    if (!serial) {
      setError('Select a device before checking the launcher state')
      return null
    }
    const generation = generationRef.current
    setBusy('preflight')
    setError(null)
    try {
      const next = await preflightLauncher(serial)
      if (generation !== generationRef.current || liveSerial() !== serial) return null
      setPreflight(next)
      return next
    } catch (preflightError) {
      if (generation === generationRef.current && liveSerial() === serial) {
        setError(messageOf(preflightError, 'Launcher check failed'))
        setPreflight(null)
      }
      return null
    } finally {
      if (generation === generationRef.current) setBusy(null)
    }
  }, [liveSerial])

  // Local, read-only and available with no online device: the recovery list is never
  // gated on a device, and a read failure is reported instead of shown as "empty".
  const loadRecovery = useCallback(async () => {
    const serial = liveSerial()
    setBusy('recovery')
    setRecoveryError(null)
    try {
      const listing = await readLauncherRecovery(serial)
      setRecovery(listing.records ?? [])
    } catch (recoveryFailure) {
      setRecoveryError(messageOf(recoveryFailure, 'Launcher recovery could not be read'))
    } finally {
      setBusy(null)
    }
  }, [liveSerial])

  const testCandidate = useCallback(
    async (candidateComponent: string) => {
      const serial = liveSerial()
      if (!serial) {
        setError('Select a device before testing a launcher candidate')
        return null
      }
      const generation = generationRef.current
      setBusy('candidate-test')
      setError(null)
      try {
        const result = await testLauncherCandidate(serial, candidateComponent)
        if (generation !== generationRef.current || liveSerial() !== serial) return null
        setTestResult(result)
        return result
      } catch (testError) {
        if (generation === generationRef.current && liveSerial() === serial) {
          setError(messageOf(testError, 'Candidate test failed'))
          setTestResult(null)
        }
        return null
      } finally {
        if (generation === generationRef.current) setBusy(null)
      }
    },
    [liveSerial],
  )

  // Consent is only granted for a candidate that was actually opened for inspection on
  // the confirmed target, and only while both packages declare exactly one HOME
  // activity - the same reversibility proof the backend enforces.
  const grantConsent = useCallback(
    (candidateComponent: string): string | null => {
      // Every refusal is returned to the caller AND stored in the hook error, so the
      // existing role="alert" line renders the reason instead of the wizard looking
      // like it silently ignored the confirmation.
      const refuse = (reason: string): string => {
        setError(reason)
        return reason
      }
      const serial = liveSerial()
      if (!serial) return refuse('No confirmed device is selected')
      if (!preflight || !preflight.supported) {
        return refuse(preflight?.reason || 'The read-only check has not confirmed this device')
      }
      const current = preflight.currentHome
      if (!current) return refuse('The current HOME is unknown')
      if (countHomeForPackage(preflight.homeCandidates ?? [], current) !== 1) {
        return refuse(`Refused: ${current.split('/')[0]} declares several HOME activities`)
      }
      if (countHomeForPackage(preflight.homeCandidates ?? [], candidateComponent) !== 1) {
        return refuse(
          `Refused: ${candidateComponent.split('/')[0]} declares several HOME activities`,
        )
      }
      if (!testResult || testResult.component !== candidateComponent || !testResult.launched) {
        return refuse('Open the candidate for inspection before confirming it')
      }
      setConsent({
        operationId: newOperationId(),
        serial,
        generation: generationRef.current,
        expectedComponent: current,
        candidateComponent,
        testedAt: Date.now(),
      })
      setApplyResult(null)
      setError(null)
      return null
    },
    [liveSerial, preflight, testResult],
  )

  // Synchronous, single-flight apply. It runs only with consent for the currently
  // confirmed target and generation, sends the pre-created operation id, and reports
  // exactly what the backend returned (never a self-declared success).
  const apply = useCallback(async () => {
    const serial = liveSerial()
    if (!serial) {
      setError('No confirmed device is selected')
      return null
    }
    if (!consent || consent.serial !== serial || consent.generation !== generationRef.current) {
      setError('Confirm the target again: the device or the launcher state changed')
      return null
    }
    if (applyInFlightRef.current) {
      setError('A launcher change is already running for this operation')
      return null
    }
    applyInFlightRef.current = true
    setBusy('apply')
    setError(null)
    try {
      const result = await applyLauncher({
        operationId: consent.operationId,
        expectedSerial: consent.serial,
        expectedComponent: consent.expectedComponent,
        candidateComponent: consent.candidateComponent,
      })
      const stale = consent.generation !== generationRef.current || liveSerial() !== consent.serial
      if (stale) {
        setError('The device changed while the launcher change ran; the recorded result is not applied to the new selection')
        return null
      }
      setApplyResult(result)
      return result
    } catch (applyError) {
      setError(messageOf(applyError, 'Launcher change failed'))
      return null
    } finally {
      applyInFlightRef.current = false
      setBusy(null)
    }
  }, [consent, liveSerial])

  // Cancel addresses the OWNED operation id and the serial it was created for, so a
  // device switch cannot retarget or block the cancellation.
  const cancel = useCallback(async () => {
    if (!consent) {
      setError('There is no owned launcher operation to cancel')
      return null
    }
    setBusy('cancel')
    setError(null)
    try {
      const result = await cancelLauncherOperation({
        operationId: consent.operationId,
        expectedSerial: consent.serial,
      })
      return result
    } catch (cancelError) {
      setError(messageOf(cancelError, 'Cancel failed'))
      return null
    } finally {
      setBusy(null)
    }
  }, [consent])

  // Restore is explicit and requires a live match: the record must belong to the
  // device that is confirmed right now.
  const restore = useCallback(
    async (recordId: string) => {
      const serial = liveSerial()
      if (!serial) {
        setError('Select a device before restoring')
        return null
      }
      const record = recovery.find((entry) => entry.id === recordId)
      if (!record) {
        setError('That launcher record is not in the recovery list')
        return null
      }
      if (record.serial !== serial) {
        setError(`That record belongs to ${record.serial}; confirm that device before restoring`)
        return null
      }
      setBusy('restore')
      setError(null)
      try {
        const result = await restoreLauncher({ recordId, expectedSerial: serial })
        if (liveSerial() !== serial) return null
        setRestoreResult(result)
        return result
      } catch (restoreError) {
        setError(messageOf(restoreError, 'Restore failed'))
        return null
      } finally {
        setBusy(null)
      }
    },
    [liveSerial, recovery],
  )

  return {
    activeSerial,
    preflight,
    testResult,
    applyResult,
    restoreResult,
    consent,
    recovery,
    recoveryError,
    error,
    busy,
    ambiguousPackages,
    blockedReason,
    canApply: blockedReason === null,
    loadPreflight,
    loadRecovery,
    testCandidate,
    grantConsent,
    apply,
    cancel,
    restore,
  }
}

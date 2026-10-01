import {
  ApplyLauncher,
  CancelLauncherOperation,
  PreflightLauncher,
  ReadLauncherRecovery,
  RestoreLauncher,
  TestLauncherCandidate,
} from '../../bindings/ADBKit/internal/app/app'
import type {
  ApplyRequest,
  ApplyResult,
  CancelRequest,
  CancelResult,
  CandidateTestResult,
  Preflight,
  Recovery,
  RestoreRequest,
  RestoreResult,
} from '../../bindings/ADBKit/internal/launcher/models'

// Every device-bound launcher RPC carries the caller-confirmed serial first and
// refuses an unanswered target here, before the request reaches the backend. The
// recovery listing is local and read-only, so it stays available with no device.

function confirmedSerial(operation: string, serial: string): string {
  const trimmed = serial.trim()
  if (!trimmed) {
    throw new Error(`${operation}: no confirmed device target is selected`)
  }
  return trimmed
}

export async function preflightLauncher(serial: string): Promise<Preflight> {
  return PreflightLauncher(confirmedSerial('launcher_preflight', serial))
}

export async function testLauncherCandidate(
  serial: string,
  candidateComponent: string,
): Promise<CandidateTestResult> {
  return TestLauncherCandidate(
    confirmedSerial('launcher_candidate_test', serial),
    candidateComponent,
  )
}

export async function applyLauncher(request: ApplyRequest): Promise<ApplyResult> {
  if (!request.operationId.trim()) {
    throw new Error('launcher_apply: the operation id must be created before the call')
  }
  return ApplyLauncher({
    ...request,
    expectedSerial: confirmedSerial('launcher_apply', request.expectedSerial),
  })
}

export async function cancelLauncherOperation(
  request: CancelRequest,
): Promise<CancelResult> {
  if (!request.operationId.trim()) {
    throw new Error('launcher_cancel: the owned operation id is required')
  }
  // The owned serial is sent as-is: the backend matches operation id plus serial, so
  // a device switch must never be able to retarget a cancellation.
  return CancelLauncherOperation({
    operationId: request.operationId,
    expectedSerial: request.expectedSerial.trim(),
  })
}

export async function restoreLauncher(request: RestoreRequest): Promise<RestoreResult> {
  return RestoreLauncher({
    ...request,
    recordId: request.recordId.trim(),
    expectedSerial: confirmedSerial('launcher_restore', request.expectedSerial),
  })
}

// ReadLauncherRecovery is a local, read-only listing. It needs no online device and
// deliberately sends no command, so an empty target is allowed and simply means
// "no device filter".
export async function readLauncherRecovery(serial = ''): Promise<Recovery> {
  return ReadLauncherRecovery(serial.trim())
}

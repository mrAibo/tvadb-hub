import { Call as WailsCall } from '@wailsio/runtime'
import type {
  SafeTuningAnalysis,
  SafeTuningApplyRequest,
  SafeTuningApplyResult,
  TuningRestoreResult,
  TuningSnapshotSummary,
} from '@/lib/types'

export async function analyzeSafeTuning(profileId = ''): Promise<SafeTuningAnalysis> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.AnalyzeSafeTuning', profileId)
  return raw as SafeTuningAnalysis
}

export async function applySafeTuning(
  request: SafeTuningApplyRequest,
): Promise<SafeTuningApplyResult> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.ApplySafeTuning', request)
  return raw as SafeTuningApplyResult
}

export async function listTuningSnapshots(): Promise<TuningSnapshotSummary[]> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.ListTuningSnapshots')
  return (raw as TuningSnapshotSummary[] | null) ?? []
}

export async function restoreTuningSnapshot(snapshotId: string): Promise<TuningRestoreResult> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.RestoreTuningSnapshot', snapshotId)
  return raw as TuningRestoreResult
}

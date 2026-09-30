import { Call as WailsCall } from '@wailsio/runtime'
import type {
  SafeTuningAnalysis,
  SafeTuningApplyRequest,
  SafeTuningApplyResult,
  SafeTuningFeedConfig,
  SafeTuningFeedStatus,
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

export async function getSafeTuningFeedConfig(): Promise<SafeTuningFeedConfig> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.GetSafeTuningFeedConfig')
  return raw as SafeTuningFeedConfig
}

export async function getSafeTuningFeedStatus(): Promise<SafeTuningFeedStatus> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.GetSafeTuningFeedStatus')
  return raw as SafeTuningFeedStatus
}

export async function configureSafeTuningFeed(
  config: SafeTuningFeedConfig,
): Promise<SafeTuningFeedStatus> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.ConfigureSafeTuningFeed', config)
  return raw as SafeTuningFeedStatus
}

export async function refreshSafeTuningFeed(): Promise<SafeTuningFeedStatus> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.RefreshSafeTuningFeed')
  return raw as SafeTuningFeedStatus
}

export async function rollbackSafeTuningFeed(): Promise<SafeTuningFeedStatus> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.RollbackSafeTuningFeed')
  return raw as SafeTuningFeedStatus
}

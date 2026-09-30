import { useEffect, useMemo, useState } from 'react'
import {
  IconAlertTriangle as AlertTriangle,
  IconArrowBackUp as ArrowBackUp,
  IconCloudDownload as CloudDownload,
  IconDatabase as Database,
  IconExternalLink as ExternalLink,
  IconKey as Key,
  IconLoader2 as Loader2,
  IconRefresh as RefreshCw,
  IconShieldCheck as ShieldCheck,
  IconSparkles as Sparkles,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useDevices } from '@/hooks/useDevices'
import {
  analyzeSafeTuning,
  applySafeTuning,
  configureSafeTuningFeed,
  getSafeTuningFeedConfig,
  getSafeTuningFeedStatus,
  listTuningSnapshots,
  refreshSafeTuningFeed,
  restoreTuningSnapshot,
  rollbackSafeTuningFeed,
} from '@/services/tuningService'
import { cn } from '@/lib/utils'
import type {
  SafeTuningActionMode,
  SafeTuningAnalysis,
  SafeTuningFeedConfig,
  SafeTuningFeedStatus,
  SafeTuningPackageMatch,
  TuningRisk,
  TuningSnapshotSummary,
} from '@/lib/types'

const riskStyle: Record<TuningRisk, string> = {
  safe: 'border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  caution: 'border-amber-500/20 bg-amber-500/10 text-amber-600 dark:text-amber-400',
  dangerous: 'border-red-500/20 bg-red-500/10 text-red-600 dark:text-red-400',
  blocked: 'border-border bg-muted/50 text-muted-foreground',
}

function PackageRow({
  item,
  selected,
  onToggle,
}: {
  item: SafeTuningPackageMatch
  selected: boolean
  onToggle: (packageName: string, checked: boolean) => void
}) {
  const disabled = !item.actionable || !item.isEnabled
  return (
    <label
      className={cn(
        'grid grid-cols-[22px_minmax(0,1fr)_90px_90px] items-center gap-3 px-4 py-3 transition-colors',
        disabled ? 'opacity-60' : 'hover:bg-muted/25 cursor-pointer',
      )}
    >
      <input
        type="checkbox"
        checked={selected}
        disabled={disabled}
        onChange={(event) => onToggle(item.packageName, event.target.checked)}
        className="h-4 w-4 accent-[var(--primary)]"
      />
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="truncate font-mono text-xs font-medium">{item.packageName}</span>
          <span className="text-[10px] text-muted-foreground">{item.label}</span>
        </div>
        <div className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
          {item.category} · {item.reason}
          {item.protected ? ' · Protected by profile keep-list' : ''}
          {!item.isEnabled ? ' · Already disabled' : ''}
        </div>
      </div>
      <span
        className={cn(
          'w-fit rounded-full border px-2 py-0.5 text-[9px] font-bold uppercase tracking-wide',
          riskStyle[item.risk],
        )}
      >
        {item.risk}
      </span>
      <span className="text-right text-[10px] text-muted-foreground">
        {item.isSystemApp ? 'System' : 'User'}
      </span>
    </label>
  )
}

export default function TuningPage() {
  const { activeSerial, deviceInfo } = useDevices()
  const [analysis, setAnalysis] = useState<SafeTuningAnalysis | null>(null)
  const [snapshots, setSnapshots] = useState<TuningSnapshotSummary[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [mode, setMode] = useState<SafeTuningActionMode>('disable')
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)
  const [restoring, setRestoring] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [feedConfig, setFeedConfig] = useState<SafeTuningFeedConfig>({ url: '', publicKey: '' })
  const [feedStatus, setFeedStatus] = useState<SafeTuningFeedStatus | null>(null)
  const [feedBusy, setFeedBusy] = useState(false)

  async function reload(profileId = '') {
    if (!activeSerial) {
      setAnalysis(null)
      setSnapshots([])
      setSelected(new Set())
      return
    }
    setLoading(true)
    setError(null)
    try {
      const [nextAnalysis, nextSnapshots] = await Promise.all([
        analyzeSafeTuning(profileId),
        listTuningSnapshots(),
      ])
      setAnalysis(nextAnalysis)
      setSnapshots(nextSnapshots)
      setSelected(new Set(nextAnalysis.defaultSelected))
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : String(loadError))
      setAnalysis(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload()
  }, [activeSerial])

  useEffect(() => {
    void (async () => {
      try {
        const [config, status] = await Promise.all([
          getSafeTuningFeedConfig(),
          getSafeTuningFeedStatus(),
        ])
        setFeedConfig(config)
        setFeedStatus(status)
      } catch (feedError) {
        console.error('Failed to load Safe Tuning feed state', feedError)
      }
    })()
  }, [])

  async function handleSaveFeedTrust() {
    setFeedBusy(true)
    try {
      const status = await configureSafeTuningFeed({
        url: feedConfig.url.trim(),
        publicKey: feedConfig.publicKey.trim(),
      })
      setFeedStatus(status)
      toast.success(status.configured ? 'Safe Tuning feed trust saved' : 'Signed metadata feed disabled', {
        description: status.message,
      })
      if (activeSerial) await reload()
    } catch (feedError) {
      toast.error('Could not save Safe Tuning feed trust', {
        description: feedError instanceof Error ? feedError.message : String(feedError),
      })
    } finally {
      setFeedBusy(false)
    }
  }

  async function handleRefreshFeed() {
    setFeedBusy(true)
    try {
      const status = await refreshSafeTuningFeed()
      setFeedStatus(status)
      toast.success('Signed Safe Tuning metadata verified', {
        description: status.version
          ? `${status.version} · revision ${status.revision} · ${status.profileCount} profile(s)`
          : status.message,
      })
      if (activeSerial) await reload()
    } catch (feedError) {
      toast.error('Signed metadata update rejected', {
        description: feedError instanceof Error ? feedError.message : String(feedError),
      })
    } finally {
      setFeedBusy(false)
    }
  }

  async function handleRollbackFeed() {
    setFeedBusy(true)
    try {
      const status = await rollbackSafeTuningFeed()
      setFeedStatus(status)
      toast.success('Safe Tuning metadata rolled back', {
        description: status.message,
      })
      if (activeSerial) await reload()
    } catch (feedError) {
      toast.error('Metadata rollback failed', {
        description: feedError instanceof Error ? feedError.message : String(feedError),
      })
    } finally {
      setFeedBusy(false)
    }
  }

  async function handleUseBuiltinsOnly() {
    setFeedConfig({ url: '', publicKey: '' })
    setFeedBusy(true)
    try {
      const status = await configureSafeTuningFeed({ url: '', publicKey: '' })
      setFeedStatus(status)
      toast.success('Using built-in Safe Tuning metadata only')
      if (activeSerial) await reload()
    } catch (feedError) {
      toast.error('Could not disable signed metadata feed', {
        description: feedError instanceof Error ? feedError.message : String(feedError),
      })
    } finally {
      setFeedBusy(false)
    }
  }

  const selectedMatches = useMemo(() => {
    if (!analysis) return []
    return analysis.matches.filter((item) => selected.has(item.packageName))
  }, [analysis, selected])

  const cautionSelected = selectedMatches.some((item) => item.risk === 'caution')

  function togglePackage(packageName: string, checked: boolean) {
    setSelected((current) => {
      const next = new Set(current)
      if (checked) next.add(packageName)
      else next.delete(packageName)
      return next
    })
  }

  async function handleApply() {
    if (!analysis || selectedMatches.length === 0) return

    const cautionText = cautionSelected
      ? '\n\nYour selection contains CAUTION packages. Their related features may stop working.'
      : ''
    const actionText =
      mode === 'disable'
        ? 'disable the selected packages for Android user 0'
        : 'uninstall the selected preinstalled packages for Android user 0'
    const ok = window.confirm(
      `TVADB Hub will create a restore snapshot first, then ${actionText}.${cautionText}\n\nContinue?`,
    )
    if (!ok) return

    setApplying(true)
    try {
      const result = await applySafeTuning({
        profileId: analysis.selectedProfile.id,
        packageNames: selectedMatches.map((item) => item.packageName),
        mode,
        acknowledgeCaution: cautionSelected,
      })
      const failed = Object.keys(result.failed ?? {}).length
      if (failed > 0) {
        toast.warning('Safe Tuning completed with warnings', {
          description: `${result.changed.length} changed, ${failed} failed. Snapshot: ${result.snapshotId || 'none'}`,
        })
      } else {
        toast.success('Safe Tuning applied', {
          description: `${result.changed.length} package(s) changed. Snapshot: ${result.snapshotId || 'none'}`,
        })
      }
      await reload(analysis.selectedProfile.id)
    } catch (applyError) {
      toast.error('Safe Tuning failed', {
        description: applyError instanceof Error ? applyError.message : String(applyError),
      })
    } finally {
      setApplying(false)
    }
  }

  async function handleRestore(snapshot: TuningSnapshotSummary) {
    if (!window.confirm(`Restore snapshot ${snapshot.id} for this device?\n\nTVADB Hub will reverse only changes recorded in that snapshot.`)) {
      return
    }
    setRestoring(snapshot.id)
    try {
      const result = await restoreTuningSnapshot(snapshot.id)
      const failed = Object.keys(result.failed ?? {}).length
      if (failed > 0) {
        toast.warning('Restore completed with warnings', {
          description: `${result.restored.length} restored, ${failed} failed.`,
        })
      } else {
        toast.success('Tuning snapshot restored', {
          description: `${result.restored.length} package(s) restored.`,
        })
      }
      await reload(analysis?.selectedProfile.id ?? '')
    } catch (restoreError) {
      toast.error('Restore failed', {
        description: restoreError instanceof Error ? restoreError.message : String(restoreError),
      })
    } finally {
      setRestoring(null)
    }
  }

  if (!activeSerial) {
    return (
      <div className="flex h-full flex-col gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Safe Tuning</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Reversible debloat profiles for Android TV, phones and tablets.
          </p>
        </div>
        <Card className="flex flex-1 items-center justify-center rounded-2xl border-border/60 bg-card/60">
          <CardContent className="max-w-md p-8 text-center">
            <ShieldCheck className="mx-auto h-10 w-10 text-muted-foreground" />
            <h2 className="mt-4 font-semibold">Connect an Android device first</h2>
            <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
              Safe Tuning only analyzes the currently selected ADB device and never runs changes automatically.
            </p>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-hidden">
      <div className="flex shrink-0 items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <ShieldCheck className="h-6 w-6 text-primary" />
            <h1 className="text-2xl font-bold tracking-tight">Safe Tuning</h1>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Analyze installed packages, disable safe optional components, and restore exact snapshots.
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          className="gap-1.5"
          onClick={() => void reload(analysis?.selectedProfile.id ?? '')}
          disabled={loading}
        >
          <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
          Analyze again
        </Button>
      </div>

      {error && (
        <div className="shrink-0 rounded-xl border border-destructive/25 bg-destructive/10 px-4 py-3 text-xs text-destructive">
          {error}
        </div>
      )}

      {loading && !analysis ? (
        <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin" />
          Analyzing installed packages…
        </div>
      ) : analysis ? (
        <div className="grid min-h-0 flex-1 gap-4 xl:grid-cols-[minmax(0,1fr)_310px]">
          <Card className="flex min-h-0 flex-col overflow-hidden rounded-2xl border-border/60 bg-card/60">
            <CardHeader className="shrink-0 border-b border-border/50 pb-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <CardTitle className="text-sm">Profile & package analysis</CardTitle>
                  <p className="mt-1 text-[10px] text-muted-foreground">
                    {analysis.manufacturer || 'Unknown manufacturer'} · {analysis.model || 'Unknown model'} ·
                    Android {analysis.androidVersion || 'unknown'} · {analysis.installedCount} packages
                  </p>
                </div>
                <Select
                  value={analysis.selectedProfile.id}
                  onValueChange={(value) => value && void reload(value)}
                >
                  <SelectTrigger className="w-[270px] border border-border/50 bg-muted/30 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="end">
                    {analysis.availableProfiles.map((profile) => (
                      <SelectItem key={profile.id} value={profile.id} className="text-xs">
                        {profile.name}{profile.recommended ? ' · Recommended' : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="mt-3 flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => setMode('disable')}
                  className={cn(
                    'rounded-full border px-3 py-1 text-[10px] font-semibold transition-colors',
                    mode === 'disable'
                      ? 'border-primary/30 bg-primary/10 text-primary'
                      : 'border-border/60 text-muted-foreground hover:text-foreground',
                  )}
                >
                  Disable user 0 · Recommended
                </button>
                <button
                  type="button"
                  onClick={() => setMode('uninstall-user')}
                  className={cn(
                    'rounded-full border px-3 py-1 text-[10px] font-semibold transition-colors',
                    mode === 'uninstall-user'
                      ? 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400'
                      : 'border-border/60 text-muted-foreground hover:text-foreground',
                  )}
                >
                  Uninstall user 0 · Advanced
                </button>
                <span className="ml-auto text-[10px] text-muted-foreground">
                  {selected.size} selected
                </span>
              </div>
            </CardHeader>

            <CardContent className="min-h-0 flex-1 overflow-y-auto p-0">
              <div className="border-b border-border/40 bg-primary/5 px-4 py-3">
                <div className="flex items-start gap-2">
                  <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                  <div className="min-w-0">
                    <div className="text-xs font-semibold">{analysis.selectedProfile.name}</div>
                    <div className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
                      {analysis.selectedProfile.description}
                    </div>
                    <a
                      href={analysis.selectedProfile.sourceUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="mt-1.5 inline-flex items-center gap-1 text-[10px] font-medium text-primary hover:underline"
                    >
                      Source: {analysis.selectedProfile.sourceName} · {analysis.selectedProfile.sourceLicense}
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                </div>
              </div>

              {analysis.matches.length === 0 ? (
                <div className="p-8 text-center text-xs text-muted-foreground">
                  None of the packages in this profile are installed on the active device.
                </div>
              ) : (
                <div className="divide-y divide-border/40">
                  {analysis.matches.map((item) => (
                    <PackageRow
                      key={item.packageName}
                      item={item}
                      selected={selected.has(item.packageName)}
                      onToggle={togglePackage}
                    />
                  ))}
                </div>
              )}
            </CardContent>

            <div className="shrink-0 border-t border-border/50 bg-muted/15 p-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-start gap-2 text-[10px] text-muted-foreground">
                  <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-500" />
                  <span className="max-w-xl leading-relaxed">
                    Dangerous/blocked packages are informational and cannot be selected. A restore snapshot is written before every change.
                  </span>
                </div>
                <Button
                  size="sm"
                  onClick={() => void handleApply()}
                  disabled={applying || selectedMatches.length === 0}
                  className="gap-1.5"
                >
                  {applying ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ShieldCheck className="h-3.5 w-3.5" />}
                  Apply {selectedMatches.length || ''} selected
                </Button>
              </div>
            </div>
          </Card>

          <div className="flex min-h-0 flex-col gap-4 overflow-y-auto">
            <Card className="rounded-2xl border-border/60 bg-card/60">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Safety model</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-[10px] leading-relaxed text-muted-foreground">
                <p><strong className="text-foreground">Safe</strong> packages are selected by default only when currently enabled.</p>
                <p><strong className="text-foreground">Caution</strong> packages require an explicit confirmation.</p>
                <p><strong className="text-foreground">Dangerous / blocked</strong> packages cannot be modified from Safe Tuning.</p>
                <p><strong className="text-foreground">Disable</strong> uses Android's reversible <code>pm disable-user --user 0</code>.</p>
              </CardContent>
            </Card>

            <Card className="rounded-2xl border-border/60 bg-card/60">
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Database className="h-4 w-4 text-muted-foreground" />
                  Signed metadata feed
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="rounded-lg border border-border/50 bg-muted/20 p-2.5 text-[9px] leading-relaxed text-muted-foreground">
                  <div className="flex items-center gap-1.5 font-semibold text-foreground">
                    <ShieldCheck className="h-3 w-3 text-primary" />
                    {feedStatus?.active ? 'Verified external metadata active' : 'Built-in metadata active'}
                  </div>
                  <p className="mt-1">{feedStatus?.message ?? 'Loading metadata status…'}</p>
                  {feedStatus?.active && (
                    <div className="mt-2 space-y-0.5 font-mono text-[8px]">
                      <div>Version {feedStatus.version} · rev {feedStatus.revision}</div>
                      <div>Key {feedStatus.keyId || 'unknown'}</div>
                      <div className="truncate" title={feedStatus.digest}>SHA-256 {feedStatus.digest}</div>
                      <div>{feedStatus.profileCount} signed profile(s)</div>
                    </div>
                  )}
                  {feedStatus?.sourceName && (
                    <a
                      href={feedStatus.sourceUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="mt-2 inline-flex items-center gap-1 font-medium text-primary hover:underline"
                    >
                      {feedStatus.sourceName} · {feedStatus.sourceLicense}
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  )}
                </div>

                <div className="space-y-2">
                  <div className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                    <CloudDownload className="h-3 w-3" />
                    HTTPS feed URL
                  </div>
                  <Input
                    value={feedConfig.url}
                    onChange={(event) => setFeedConfig((current) => ({ ...current, url: event.target.value }))}
                    placeholder="https://…/safe-tuning-feed.json"
                    className="h-7 text-[10px]"
                    disabled={feedBusy}
                  />
                  <div className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
                    <Key className="h-3 w-3" />
                    Ed25519 public key · base64
                  </div>
                  <Input
                    value={feedConfig.publicKey}
                    onChange={(event) => setFeedConfig((current) => ({ ...current, publicKey: event.target.value }))}
                    placeholder="Pinned public key"
                    className="h-7 font-mono text-[9px]"
                    disabled={feedBusy}
                  />
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-[9px]"
                    disabled={feedBusy}
                    onClick={() => void handleSaveFeedTrust()}
                  >
                    {feedBusy ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <Key className="mr-1 h-3 w-3" />}
                    Save trust
                  </Button>
                  <Button
                    size="sm"
                    className="h-7 text-[9px]"
                    disabled={feedBusy || !feedStatus?.configured}
                    onClick={() => void handleRefreshFeed()}
                  >
                    <CloudDownload className="mr-1 h-3 w-3" />
                    Check update
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-[9px]"
                    disabled={feedBusy || !feedStatus?.canRollback}
                    onClick={() => void handleRollbackFeed()}
                  >
                    <ArrowBackUp className="mr-1 h-3 w-3" />
                    Roll back
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 text-[9px]"
                    disabled={feedBusy || !feedStatus?.configured}
                    onClick={() => void handleUseBuiltinsOnly()}
                  >
                    Built-ins only
                  </Button>
                </div>

                <p className="text-[9px] leading-relaxed text-muted-foreground">
                  Updating metadata never applies package changes automatically. Every action still requires analysis, selection, confirmation and a restore snapshot.
                </p>
              </CardContent>
            </Card>

            <Card className="rounded-2xl border-border/60 bg-card/60">
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <ArrowBackUp className="h-4 w-4 text-muted-foreground" />
                  Restore snapshots
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {snapshots.length === 0 ? (
                  <p className="text-[10px] text-muted-foreground">No tuning snapshots for this device yet.</p>
                ) : (
                  snapshots.slice(0, 10).map((snapshot) => (
                    <div key={snapshot.id} className="rounded-xl border border-border/50 bg-muted/20 p-3">
                      <div className="text-[10px] font-semibold">{snapshot.profileName}</div>
                      <div className="mt-1 font-mono text-[9px] text-muted-foreground">{snapshot.createdAt}</div>
                      <div className="mt-1 text-[9px] text-muted-foreground">
                        {snapshot.applied} changed · {snapshot.mode}
                      </div>
                      <Button
                        size="sm"
                        variant="outline"
                        className="mt-2 h-7 w-full gap-1.5 text-[10px]"
                        disabled={restoring !== null || snapshot.applied === 0}
                        onClick={() => void handleRestore(snapshot)}
                      >
                        {restoring === snapshot.id ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                          <ArrowBackUp className="h-3 w-3" />
                        )}
                        Restore exact changes
                      </Button>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>

            {analysis.protectedInstalled.length > 0 && (
              <Card className="rounded-2xl border-border/60 bg-card/60">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">Protected by profile</CardTitle>
                </CardHeader>
                <CardContent className="space-y-1">
                  {analysis.protectedInstalled.slice(0, 20).map((pkg) => (
                    <div key={pkg} className="break-all font-mono text-[9px] text-muted-foreground">{pkg}</div>
                  ))}
                  {analysis.protectedInstalled.length > 20 && (
                    <div className="text-[9px] text-muted-foreground">
                      +{analysis.protectedInstalled.length - 20} more protected packages
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {deviceInfo?.isTV && (
              <div className="rounded-xl border border-primary/15 bg-primary/5 px-3 py-2 text-[10px] text-muted-foreground">
                TV profile protection keeps known launcher, DRM, input, remote and playback packages out of actionable lists.
              </div>
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}

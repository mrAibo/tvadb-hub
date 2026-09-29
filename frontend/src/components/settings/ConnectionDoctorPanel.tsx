import { useMemo, useState } from 'react'
import {
  IconActivity as Activity,
  IconCheck as Check,
  IconClipboard as Clipboard,
  IconInfoCircle as InfoCircle,
  IconLoader2 as Loader2,
  IconRefresh as RefreshCw,
  IconServerBolt as ServerBolt,
  IconStethoscope as Stethoscope,
  IconAlertTriangle as AlertTriangle,
  IconX as X,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  getConnectionDoctorReport,
  restartADBServer,
} from '@/services/connectionDoctorService'
import type {
  ConnectionDoctorCheck,
  ConnectionDoctorReport,
  ConnectionDoctorStatus,
} from '@/lib/types'
import { cn } from '@/lib/utils'

const statusClasses: Record<ConnectionDoctorStatus, string> = {
  pass: 'border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  warning: 'border-amber-500/20 bg-amber-500/10 text-amber-600 dark:text-amber-400',
  fail: 'border-destructive/20 bg-destructive/10 text-destructive',
  info: 'border-blue-500/20 bg-blue-500/10 text-blue-600 dark:text-blue-400',
}

function StatusIcon({ status }: { status: ConnectionDoctorStatus }) {
  const className = 'h-3.5 w-3.5'
  if (status === 'pass') return <Check className={className} />
  if (status === 'warning') return <AlertTriangle className={className} />
  if (status === 'fail') return <X className={className} />
  return <InfoCircle className={className} />
}

function CheckRow({ check }: { check: ConnectionDoctorCheck }) {
  return (
    <div className="grid gap-2 rounded-xl border border-border/45 bg-muted/15 p-3 md:grid-cols-[150px_120px_minmax(0,1fr)]">
      <div className="text-[11px] font-semibold text-foreground">{check.label}</div>
      <div>
        <span
          className={cn(
            'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[9px] font-bold uppercase tracking-wide',
            statusClasses[check.status],
          )}
        >
          <StatusIcon status={check.status} />
          {check.status}
        </span>
      </div>
      <div className="min-w-0">
        <p className="break-words text-[10px] leading-relaxed text-muted-foreground">
          {check.detail}
        </p>
        {check.recommendation && (
          <p className="mt-1.5 text-[10px] leading-relaxed text-foreground/85">
            <span className="font-semibold">Fix:</span> {check.recommendation}
          </p>
        )}
      </div>
    </div>
  )
}

export function ConnectionDoctorPanel() {
  const [report, setReport] = useState<ConnectionDoctorReport | null>(null)
  const [running, setRunning] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const grouped = useMemo(() => {
    if (!report) return []
    const order: string[] = []
    const groups = new Map<string, ConnectionDoctorCheck[]>()
    for (const check of report.checks) {
      if (!groups.has(check.group)) {
        groups.set(check.group, [])
        order.push(check.group)
      }
      groups.get(check.group)?.push(check)
    }
    return order.map((group) => ({ group, checks: groups.get(group) ?? [] }))
  }, [report])

  const canRestartADB = report?.checks.some((check) => check.action === 'restart-adb') ?? false

  async function runDoctor() {
    setRunning(true)
    setError(null)
    try {
      const next = await getConnectionDoctorReport()
      setReport(next)
      if (next.failCount > 0) {
        toast.error('Connection Doctor found blocking issues', {
          description: String(next.failCount) + ' failed · ' + String(next.warningCount) + ' warning(s)',
        })
      } else if (next.warningCount > 0) {
        toast.warning('Connection Doctor completed with warnings', {
          description: String(next.passCount) + ' passed · ' + String(next.warningCount) + ' warning(s)',
        })
      } else {
        toast.success('Connection Doctor passed', {
          description: String(next.passCount) + ' checks passed.',
        })
      }
    } catch (doctorError) {
      const message = doctorError instanceof Error ? doctorError.message : String(doctorError)
      setError(message)
      toast.error('Connection Doctor failed to run', { description: message })
    } finally {
      setRunning(false)
    }
  }

  async function restartAndRerun() {
    setRestarting(true)
    try {
      const message = await restartADBServer()
      toast.success(message)
      await runDoctor()
    } catch (restartError) {
      toast.error('Could not restart ADB server', {
        description: restartError instanceof Error ? restartError.message : String(restartError),
      })
    } finally {
      setRestarting(false)
    }
  }

  async function copyReport() {
    if (!report) return
    try {
      await navigator.clipboard.writeText(JSON.stringify(report, null, 2))
      toast.success('Connection Doctor report copied')
    } catch (copyError) {
      toast.error('Could not copy report', {
        description: copyError instanceof Error ? copyError.message : String(copyError),
      })
    }
  }

  return (
    <Card className="rounded-2xl border border-border/50 bg-card/50 shadow-sm">
      <CardHeader className="pb-3">
        <CardTitle className="flex flex-wrap items-center justify-between gap-3 text-sm font-semibold">
          <span className="flex items-center gap-2">
            <Stethoscope className="h-4 w-4 text-primary" />
            Connection Doctor
          </span>
          {report && (
            <Badge variant={report.failCount > 0 ? 'destructive' : 'secondary'}>
              {report.failCount > 0
                ? String(report.failCount) + ' blocking issue' + (report.failCount === 1 ? '' : 's')
                : report.warningCount > 0
                  ? String(report.warningCount) + ' warning' + (report.warningCount === 1 ? '' : 's')
                  : 'Healthy'}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="max-w-3xl">
            <p className="text-xs leading-relaxed text-muted-foreground">
              Diagnose DroidSphere's Android toolchain, ADB daemon, device authorization,
              active target and USB/Wireless ADB transport. No device settings are changed.
            </p>
            <p className="mt-1 text-[10px] text-muted-foreground">
              Wireless targets also get mDNS and live TCP endpoint checks.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button size="sm" className="h-8 gap-1.5 text-xs" onClick={() => void runDoctor()} disabled={running || restarting}>
              {running ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Activity className="h-3.5 w-3.5" />}
              {running ? 'Diagnosing…' : report ? 'Run again' : 'Diagnose connection'}
            </Button>

            {canRestartADB && (
              <Button
                size="sm"
                variant="outline"
                className="h-8 gap-1.5 text-xs"
                onClick={() => void restartAndRerun()}
                disabled={running || restarting}
              >
                {restarting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ServerBolt className="h-3.5 w-3.5" />}
                Restart ADB server
              </Button>
            )}

            {report && (
              <Button size="sm" variant="outline" className="h-8 gap-1.5 text-xs" onClick={() => void copyReport()}>
                <Clipboard className="h-3.5 w-3.5" />
                Copy report
              </Button>
            )}
          </div>
        </div>

        {error && (
          <div className="rounded-xl border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {error}
          </div>
        )}

        {!report && !error && (
          <div className="rounded-xl border border-dashed border-border/60 bg-muted/10 px-4 py-5 text-center">
            <Stethoscope className="mx-auto h-7 w-7 text-muted-foreground" />
            <p className="mt-2 text-xs font-medium">Ready to diagnose the current connection</p>
            <p className="mt-1 text-[10px] text-muted-foreground">
              Select the intended device first when more than one Android/Fastboot target is connected.
            </p>
          </div>
        )}

        {report && (
          <>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-6">
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="text-lg font-bold tabular-nums text-emerald-600 dark:text-emerald-400">{report.passCount}</div>
                <div className="text-[9px] uppercase tracking-wide text-muted-foreground">Passed</div>
              </div>
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="text-lg font-bold tabular-nums text-amber-600 dark:text-amber-400">{report.warningCount}</div>
                <div className="text-[9px] uppercase tracking-wide text-muted-foreground">Warnings</div>
              </div>
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="text-lg font-bold tabular-nums text-destructive">{report.failCount}</div>
                <div className="text-[9px] uppercase tracking-wide text-muted-foreground">Failed</div>
              </div>
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="text-lg font-bold tabular-nums text-blue-600 dark:text-blue-400">{report.infoCount}</div>
                <div className="text-[9px] uppercase tracking-wide text-muted-foreground">Info</div>
              </div>
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="truncate text-xs font-bold">{report.transport || '—'}</div>
                <div className="mt-1 text-[9px] uppercase tracking-wide text-muted-foreground">Transport</div>
              </div>
              <div className="rounded-xl border border-border/45 bg-muted/15 px-3 py-2">
                <div className="truncate text-xs font-bold">{report.os}/{report.arch}</div>
                <div className="mt-1 text-[9px] uppercase tracking-wide text-muted-foreground">Host</div>
              </div>
            </div>

            {(report.targetModel || report.targetSerial) && (
              <div className="rounded-xl border border-primary/15 bg-primary/5 px-3 py-2 text-[10px]">
                <span className="font-semibold">Diagnostic target:</span>{' '}
                {report.targetModel || report.targetSerial}
                {report.targetSerial && report.targetModel ? ' · ' + report.targetSerial : ''}
                {report.targetState ? ' · ' + report.targetMode + '/' + report.targetState : ''}
              </div>
            )}

            <div className="space-y-4">
              {grouped.map(({ group, checks }) => (
                <section key={group}>
                  <div className="mb-2 flex items-center gap-2">
                    <RefreshCw className="h-3 w-3 text-muted-foreground" />
                    <h3 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{group}</h3>
                  </div>
                  <div className="space-y-2">
                    {checks.map((check) => <CheckRow key={check.id} check={check} />)}
                  </div>
                </section>
              ))}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

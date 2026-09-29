import { useState } from 'react'
import {
  IconClipboard as Clipboard,
  IconDeviceMobile as DeviceMobile,
  IconLoader2 as Loader2,
  IconPlayerPlay as Play,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { getDeviceInfo, getWirelessDiagnostics } from '@/services/deviceService'
import {
  buildDeviceValidationReport,
  selectorFromDeviceInfo,
  type DeviceValidationReport,
  type DeviceValidationStatus,
} from '@/lib/deviceValidation'

const STATUS_CLASSES: Record<DeviceValidationStatus, string> = {
  pass: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-300',
  warning: 'bg-amber-500/10 text-amber-600 dark:text-amber-300',
  fail: 'bg-destructive/10 text-destructive',
}

export function PhysicalDeviceValidationPanel() {
  const [report, setReport] = useState<DeviceValidationReport | null>(null)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function runValidation() {
    setRunning(true)
    setError(null)
    try {
      const info = await getDeviceInfo()
      const selector = selectorFromDeviceInfo(info)
      const diagnostics = await getWirelessDiagnostics(selector)
      const next = buildDeviceValidationReport(info, diagnostics)
      setReport(next)

      if (next.healthy) {
        toast.success('Physical Android device validation passed')
      } else {
        toast.error('Physical Android device validation found blocking checks')
      }
    } catch (err) {
      setReport(null)
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      toast.error('Physical Android device validation failed', { description: message })
    } finally {
      setRunning(false)
    }
  }

  async function copyReport() {
    if (!report) return
    try {
      await navigator.clipboard.writeText(JSON.stringify(report, null, 2))
      toast.success('Validation report copied')
    } catch (err) {
      toast.error('Could not copy validation report', {
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  return (
    <Card className="rounded-2xl border border-border/50 bg-card/50 shadow-sm">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center justify-between gap-3 text-sm font-semibold">
          <span className="flex items-center gap-2">
            <DeviceMobile className="h-4 w-4 text-muted-foreground" />
            Physical Android device validation
          </span>
          {report && (
            <Badge variant={report.healthy ? 'secondary' : 'destructive'}>
              {report.healthy ? 'Passed' : 'Needs attention'}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>

      <CardContent className="space-y-3">
        <p className="text-xs leading-relaxed text-muted-foreground">
          Run this with a real Android phone, tablet, TV or streaming device connected by USB or
          Wireless ADB. The check is read-only and validates ADB state, transport diagnostics,
          device classification and metadata.
        </p>

        <div className="flex flex-wrap gap-2">
          <Button size="sm" className="h-8 text-xs" onClick={() => void runValidation()} disabled={running}>
            {running ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Play className="mr-1.5 h-3.5 w-3.5" />
            )}
            {running ? 'Validating…' : 'Validate connected device'}
          </Button>

          {report && (
            <Button size="sm" variant="outline" className="h-8 text-xs" onClick={() => void copyReport()}>
              <Clipboard className="mr-1.5 h-3.5 w-3.5" />
              Copy JSON report
            </Button>
          )}
        </div>

        {error && (
          <p className="rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {error}
          </p>
        )}

        {report && (
          <div className="space-y-2">
            <div className="rounded-xl border border-border/40 bg-muted/20 p-3 text-xs">
              <p className="font-medium">
                {report.manufacturer || 'Unknown manufacturer'} {report.model || report.serial}
              </p>
              <p className="mt-1 font-mono text-[10px] text-muted-foreground">
                {report.serial}
                {report.androidVersion ? ` · Android ${report.androidVersion}` : ''}
                {report.isTV ? ' · TV' : ''}
              </p>
            </div>

            {report.checks.map((check) => (
              <div key={check.id} className="flex items-start justify-between gap-3 rounded-xl border border-border/40 px-3 py-2.5">
                <div className="min-w-0">
                  <p className="text-xs font-medium">{check.label}</p>
                  <p className="mt-0.5 text-[11px] leading-relaxed text-muted-foreground">{check.detail}</p>
                </div>
                <span className={`shrink-0 rounded-full px-2 py-0.5 text-[9px] font-semibold uppercase tracking-wide ${STATUS_CLASSES[check.status]}`}>
                  {check.status}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

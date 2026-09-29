import { useMemo } from 'react'
import {
  IconClipboard as Clipboard,
  IconExternalLink as ExternalLink,
  IconFolder as Folder,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { openPathLocation } from '@/services/settingsService'
import type { BinaryInfo } from '@/lib/types'

interface ToolLocationsSummaryProps {
  adb?: BinaryInfo
  fastboot?: BinaryInfo
  scrcpy?: BinaryInfo
  managedDir?: string
}

interface ToolRow {
  label: string
  path: string
  ready: boolean
  version: string
  source: string
}

function compactVersion(value?: string) {
  if (!value) return '—'
  const scrcpy = value.match(/scrcpy\s+([^\s]+)/i)
  if (scrcpy) return scrcpy[1]
  const platformTools = value.match(/Version\s+([^\s)]+)/i)
  if (platformTools) return platformTools[1]
  const firstVersion = value.match(/\d+(?:\.\d+){1,3}(?:[-+][\w.-]+)?/)
  return firstVersion?.[0] ?? value
}

function sourceLabel(source?: string) {
  switch (source) {
    case 'config':
      return 'Custom'
    case 'system-path':
      return 'System PATH'
    case 'app-data':
      return 'Managed'
    case 'common-path':
      return 'Common path'
    default:
      return source || '—'
  }
}

export function ToolLocationsSummary({
  adb,
  fastboot,
  scrcpy,
  managedDir,
}: ToolLocationsSummaryProps) {
  const rows = useMemo<ToolRow[]>(
    () => [
      {
        label: 'ADB',
        path: adb?.path ?? '',
        ready: adb?.status === 'ready',
        version: compactVersion(adb?.version),
        source: sourceLabel(adb?.source),
      },
      {
        label: 'Fastboot',
        path: fastboot?.path ?? '',
        ready: fastboot?.status === 'ready',
        version: compactVersion(fastboot?.version),
        source: sourceLabel(fastboot?.source),
      },
      {
        label: 'scrcpy',
        path: scrcpy?.path ?? '',
        ready: scrcpy?.status === 'ready',
        version: compactVersion(scrcpy?.version),
        source: sourceLabel(scrcpy?.source),
      },
      {
        label: 'Managed tools',
        path: managedDir ?? '',
        ready: Boolean(managedDir),
        version: '—',
        source: 'TVADB Hub data',
      },
    ],
    [adb, fastboot, scrcpy, managedDir],
  )

  async function copyPath(label: string, path: string) {
    if (!path) return
    try {
      await navigator.clipboard.writeText(path)
      toast.success(`${label} path copied`)
    } catch {
      toast.error('Could not copy path')
    }
  }

  async function openPath(label: string, path: string) {
    if (!path) return
    try {
      await openPathLocation(path)
    } catch (error) {
      toast.error(`Could not open ${label} location`, {
        description: error instanceof Error ? error.message : String(error),
      })
    }
  }

  return (
    <div className="mx-4 rounded-xl border border-border/50 bg-muted/20 p-3">
      <div className="mb-2.5 flex items-center gap-2">
        <Folder className="h-4 w-4 text-muted-foreground" />
        <div>
          <p className="text-xs font-semibold text-foreground">Tool locations</p>
          <p className="text-[11px] text-muted-foreground">
            Exact executables and directories TVADB Hub is using on this computer.
          </p>
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border border-border/40 bg-background/45">
        <div className="hidden grid-cols-[90px_86px_100px_1fr_58px] gap-2 border-b border-border/40 bg-muted/20 px-3 py-1.5 text-[9px] font-bold uppercase tracking-wider text-muted-foreground/60 lg:grid">
          <span>Tool</span>
          <span>Version</span>
          <span>Source</span>
          <span>Resolved path</span>
          <span className="text-right">Open</span>
        </div>

        <div className="divide-y divide-border/40">
          {rows.map((row) => (
            <div
              key={row.label}
              className="grid gap-2 px-3 py-2.5 lg:grid-cols-[90px_86px_100px_1fr_58px] lg:items-center"
            >
              <div className="flex items-center gap-2">
                <span
                  className={`h-1.5 w-1.5 shrink-0 rounded-full ${
                    row.ready ? 'bg-emerald-500' : 'bg-muted-foreground/35'
                  }`}
                />
                <span className="text-[11px] font-semibold text-foreground">{row.label}</span>
              </div>

              <span className="font-mono text-[10px] text-muted-foreground">{row.version}</span>

              <span className="w-fit rounded-full border border-border/50 bg-muted/25 px-2 py-0.5 text-[9px] font-medium text-muted-foreground">
                {row.source}
              </span>

              <div
                className="min-w-0 break-all font-mono text-[10px] leading-relaxed text-foreground/85"
                title={row.path || 'Not resolved'}
              >
                {row.path || 'Not resolved'}
              </div>

              <div className="flex items-center justify-end gap-0.5">
                <button
                  type="button"
                  disabled={!row.path}
                  onClick={() => void copyPath(row.label, row.path)}
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-30"
                  aria-label={`Copy ${row.label} path`}
                  title={row.path ? `Copy ${row.label} path` : 'Path not available'}
                >
                  <Clipboard className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  disabled={!row.path}
                  onClick={() => void openPath(row.label, row.path)}
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-30"
                  aria-label={`Open ${row.label} location`}
                  title={row.path ? `Open ${row.label} location` : 'Path not available'}
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

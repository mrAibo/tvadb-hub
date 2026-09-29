import { useMemo } from 'react'
import {
  IconClipboard as Clipboard,
  IconFolder as Folder,
  IconTerminal2 as Terminal2,
} from '@tabler/icons-react'
import { toast } from 'sonner'
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
}

function shortenMiddle(value: string, max = 72) {
  if (value.length <= max) return value
  const keep = Math.floor((max - 3) / 2)
  return `${value.slice(0, keep)}...${value.slice(-keep)}`
}

export function ToolLocationsSummary({
  adb,
  fastboot,
  scrcpy,
  managedDir,
}: ToolLocationsSummaryProps) {
  const rows = useMemo<ToolRow[]>(
    () => [
      { label: 'ADB', path: adb?.path ?? '', ready: adb?.status === 'ready' },
      { label: 'Fastboot', path: fastboot?.path ?? '', ready: fastboot?.status === 'ready' },
      { label: 'scrcpy', path: scrcpy?.path ?? '', ready: scrcpy?.status === 'ready' },
      { label: 'Managed tools', path: managedDir ?? '', ready: Boolean(managedDir) },
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

  return (
    <div className="mx-4 rounded-xl border border-border/50 bg-muted/20 p-3">
      <div className="mb-2.5 flex items-center gap-2">
        <Folder className="h-4 w-4 text-muted-foreground" />
        <div>
          <p className="text-xs font-semibold text-foreground">Tool locations</p>
          <p className="text-[11px] text-muted-foreground">
            Resolved paths used by TVADB Hub on this computer.
          </p>
        </div>
      </div>

      <div className="divide-y divide-border/40 overflow-hidden rounded-lg border border-border/40 bg-background/40">
        {rows.map((row) => (
          <div key={row.label} className="grid grid-cols-[92px_1fr_auto] items-center gap-2 px-3 py-2">
            <div className="flex items-center gap-2">
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  row.ready ? 'bg-emerald-500' : 'bg-muted-foreground/35'
                }`}
              />
              <span className="text-[11px] font-medium text-muted-foreground">{row.label}</span>
            </div>

            <div
              className="min-w-0 truncate font-mono text-[11px] text-foreground"
              title={row.path || 'Not resolved'}
            >
              {row.path ? shortenMiddle(row.path) : 'Not resolved'}
            </div>

            <button
              type="button"
              disabled={!row.path}
              onClick={() => void copyPath(row.label, row.path)}
              className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-30"
              aria-label={`Copy ${row.label} path`}
              title={row.path ? `Copy ${row.label} path` : 'Path not available'}
            >
              {row.label === 'Managed tools' ? (
                <Folder className="h-3.5 w-3.5" />
              ) : (
                <Terminal2 className="h-3.5 w-3.5" />
              )}
              <Clipboard className="sr-only" />
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

import { useEffect, useState } from 'react'
import {
  IconBox as PackageBox,
  IconCircleCheck as CheckCircle2,
  IconCircleXFilled as XCircle,
  IconLoader2 as Loader2,
  IconTrash as Trash,
} from '@tabler/icons-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { PackageInstallMode } from '@/lib/types'

interface InstallSplitApkDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onInstall: (filePaths: string[], mode: PackageInstallMode) => Promise<boolean>
  onSelectFiles: () => Promise<string[]>
  initialFilePaths?: string[]
}

type InstallStatus = 'idle' | 'installing' | 'success' | 'error'

export function InstallSplitApkDialog({
  open,
  onOpenChange,
  onInstall,
  onSelectFiles,
  initialFilePaths,
}: InstallSplitApkDialogProps) {
  const [filePaths, setFilePaths] = useState<string[]>([])
  const [installMode, setInstallMode] = useState<PackageInstallMode>('replace')
  const [status, setStatus] = useState<InstallStatus>('idle')
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (!open) return
    setFilePaths(initialFilePaths ?? [])
    setInstallMode('replace')
    setStatus('idle')
    setErrorMessage('')
  }, [open, initialFilePaths])

  async function browse() {
    if (status === 'installing') return
    const selected = await onSelectFiles()
    if (selected.length > 0) {
      setFilePaths(selected)
      setStatus('idle')
      setErrorMessage('')
    }
  }

  async function install() {
    if (filePaths.length < 2 || status === 'installing') return
    setStatus('installing')
    const success = await onInstall(filePaths, installMode)
    if (success) {
      setStatus('success')
    } else {
      setStatus('error')
      setErrorMessage(
        'Split APK installation failed. Verify that base.apk and all required splits belong to the same application.',
      )
    }
  }

  const done = status === 'success' || status === 'error'

  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (status !== 'installing') onOpenChange(value)
      }}
    >
      <DialogContent className="sm:max-w-[520px] rounded-2xl border border-border/80 bg-card/95 backdrop-blur-md">
        <DialogHeader>
          <DialogTitle className="text-sm font-semibold uppercase tracking-tight text-muted-foreground/80">
            Install split APKs
          </DialogTitle>
          <DialogDescription className="text-xs text-muted-foreground">
            Select base.apk together with every required configuration split.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          <div className="rounded-xl border border-border/40 bg-muted/10 p-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs font-semibold">
                  {filePaths.length > 0
                    ? `${filePaths.length} APK files selected`
                    : 'No split APK set selected'}
                </p>
                <p className="mt-0.5 text-[10px] text-muted-foreground">
                  At least two APK files are required.
                </p>
              </div>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={browse}
                disabled={status === 'installing'}
              >
                <PackageBox className="mr-1.5 h-3.5 w-3.5" />
                Select APKs
              </Button>
            </div>

            {filePaths.length > 0 && (
              <div className="mt-3 max-h-40 space-y-1 overflow-y-auto rounded-lg border border-border/40 bg-background/60 p-2">
                {filePaths.map((path) => (
                  <div
                    key={path}
                    className="flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-[10px]"
                  >
                    <span className="min-w-0 flex-1 truncate font-mono" title={path}>
                      {path.split(/[\\/]/).pop()}
                    </span>
                    <Button
                      type="button"
                      size="icon-xs"
                      variant="ghost"
                      aria-label={`Remove ${path.split(/[\\/]/).pop()}`}
                      disabled={status === 'installing'}
                      onClick={() => setFilePaths((current) => current.filter((item) => item !== path))}
                    >
                      <Trash />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {filePaths.length >= 2 && status === 'idle' && (
            <div className="rounded-xl border border-border/40 bg-muted/10 p-3">
              <p className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                Install mode
              </p>
              <div className="grid grid-cols-3 gap-2">
                {([
                  ['install', 'Install'],
                  ['replace', 'Update'],
                  ['downgrade', 'Downgrade'],
                ] as const).map(([value, label]) => (
                  <button
                    key={value}
                    type="button"
                    onClick={() => setInstallMode(value)}
                    className={cn(
                      'rounded-lg border px-3 py-2 text-[11px] font-semibold transition-colors',
                      installMode === value
                        ? 'border-primary/50 bg-primary/10'
                        : 'border-border/50 bg-background hover:bg-muted/40',
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {status === 'installing' && (
            <div className="flex items-center gap-2 rounded-xl border border-border/40 bg-muted/10 px-3 py-2.5 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
              Installing split APK set...
            </div>
          )}

          {status === 'success' && (
            <div className="flex items-center gap-2 rounded-xl border border-success/30 bg-success/5 px-3 py-2.5 text-xs text-success">
              <CheckCircle2 className="h-4 w-4" />
              Split APK set installed successfully.
            </div>
          )}

          {status === 'error' && (
            <div className="flex items-start gap-2 rounded-xl border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-xs text-destructive">
              <XCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}
        </div>

        <div className="flex gap-3 pt-1">
          {done ? (
            <>
              {status === 'error' && (
                <Button
                  type="button"
                  variant="outline"
                  className="flex-1"
                  onClick={() => {
                    setStatus('idle')
                    setErrorMessage('')
                  }}
                >
                  Try again
                </Button>
              )}
              <Button
                type="button"
                className="flex-1"
                onClick={() => onOpenChange(false)}
              >
                {status === 'success' ? 'Done' : 'Close'}
              </Button>
            </>
          ) : (
            <>
              <Button
                type="button"
                variant="outline"
                className="flex-1"
                disabled={status === 'installing'}
                onClick={() => onOpenChange(false)}
              >
                Cancel
              </Button>
              <Button
                type="button"
                className="flex-1"
                disabled={filePaths.length < 2 || status === 'installing'}
                onClick={install}
              >
                {filePaths.length === 0
                  ? 'Install APKs'
                  : `Install ${filePaths.length} APK${filePaths.length === 1 ? '' : 's'}`}
              </Button>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

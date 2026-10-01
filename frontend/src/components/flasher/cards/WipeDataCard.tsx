import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  useFlasher,
  useFlashDispatchBusy,
  type FlashConsent,
} from '@/hooks/useFlasher'
import { useFlashTargetRevision } from '@/stores/useFlasherStore'
import {
  IconAlertTriangle as AlertTriangle,
  IconTrash as Trash2,
  IconLoader2 as Loader2,
  IconCpu as Cpu
} from "@tabler/icons-react"

interface WipeDataCardProps {
  disabled?: boolean
}

export function WipeDataCard({ disabled }: WipeDataCardProps) {
  const { activeFastbootSerial, runningWipe, executeWipeData, captureWipeConsent } = useFlasher()
  const dispatchBusy = useFlashDispatchBusy()
  const targetRevision = useFlashTargetRevision()
  const [pendingConsent, setPendingConsent] = useState<FlashConsent | null>(null)
  // A serial or device-context change while the dialog is open (including A -> B -> A)
  // invalidates the captured consent and the user has to confirm again.
  const consentStale = pendingConsent !== null && pendingConsent.revision !== targetRevision
  const hasDevice = !!activeFastbootSerial && !disabled

  function handleOpenConfirm() {
    setPendingConsent(captureWipeConsent())
  }

  function handleConfirmWipe() {
    if (!pendingConsent) return
    const consent = pendingConsent
    setPendingConsent(null)
    void executeWipeData(consent)
  }

  return (
    <Card className="relative overflow-hidden border-[var(--destructive)]/30 dark:border-[var(--destructive)]/20 bg-[var(--destructive)]/[0.08] dark:bg-[var(--destructive)]/[0.04] rounded-2xl shadow-[var(--shadow-card)] h-full flex flex-col">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-sm font-semibold text-[var(--destructive)] dark:text-[var(--destructive)]">
          <Trash2 className="h-4 w-4 text-[var(--destructive)] dark:text-[var(--destructive)]" />
          Danger Zone: Wipe Data
        </CardTitle>
      </CardHeader>
      
      <CardContent className="space-y-4 flex-1 flex flex-col justify-between">
        <div className="space-y-4 flex-1">
          <p className="text-xs text-muted-foreground dark:text-muted-foreground leading-relaxed font-medium">
            Erase all user data, installed apps, and accounts. This action is permanent and cannot be undone.
          </p>
        </div>

        <Button
          variant="destructive"
          className="w-full rounded-full bg-[var(--destructive)] hover:bg-[var(--destructive)]/90 text-white border-0 transition-[colors,transform] active:scale-[0.97] cursor-pointer text-xs font-semibold shadow-sm h-9 mt-4"
          onClick={handleOpenConfirm}
          disabled={!hasDevice || runningWipe || dispatchBusy}
        >
          {runningWipe ? (
            <>
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              Wiping device...
            </>
          ) : (
            'Wipe Everything (Factory Reset)'
          )}
        </Button>
      </CardContent>

      {disabled && (
        <div className="absolute inset-0 z-20 flex flex-col items-center justify-center bg-card/80 dark:bg-[var(--terminal-surface)]/85 backdrop-blur-[3px] select-none transition-colors duration-300">
          <div className="flex items-center gap-1.5 rounded-full border border-[var(--border)] dark:border-[var(--border)] bg-card dark:bg-[var(--muted)]/90 px-3 py-1.5 shadow-sm text-[11px] font-semibold text-muted-foreground dark:text-muted-foreground">
            <Cpu className="h-3.5 w-3.5 text-muted-foreground dark:text-muted-foreground" />
            Fastboot Mode Required
          </div>
        </div>
      )}

      <AlertDialog
        open={pendingConsent !== null}
        onOpenChange={(open) => {
          if (!open) setPendingConsent(null)
        }}
      >
        <AlertDialogContent className="rounded-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle className="text-[var(--destructive)] dark:text-[var(--destructive)] flex items-center gap-2">
              <Trash2 className="h-4 w-4" />
              Factory Reset Warning
            </AlertDialogTitle>
            <AlertDialogDescription>
              All user files, downloaded apps, system accounts, and encryption keys on the device will be permanently deleted. This action cannot be reversed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-1.5 text-[11px] leading-relaxed text-muted-foreground">
            <div>
              Target device:{' '}
              <span data-testid="wipe-confirm-serial" className="font-mono text-foreground">
                {pendingConsent?.serial || 'none'}
              </span>
              {pendingConsent?.deviceLabel ? <> ({pendingConsent.deviceLabel})</> : null}
            </div>
          </div>
          {consentStale && (
            <Alert variant="destructive" className="rounded-xl py-2 px-3">
              <AlertTriangle className="h-3.5 w-3.5" />
              <AlertDescription data-testid="wipe-confirm-stale">
                The device changed. Close this dialog and confirm again.
              </AlertDescription>
            </Alert>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel className="rounded-full">Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmWipe}
              disabled={consentStale || dispatchBusy}
              className="rounded-full bg-[var(--destructive)] hover:bg-[var(--destructive)]/90 text-white border-0 shadow-sm"
            >
              Wipe Everything
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}

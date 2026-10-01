import { useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
import { PartitionChips } from '@/components/flasher/shared/PartitionChips'
import { FilePicker } from '@/components/flasher/shared/FilePicker'
import {
  useFlasher,
  useFlashDispatchBusy,
  type FlashConsent,
} from '@/hooks/useFlasher'
import { useFlashTargetRevision } from '@/stores/useFlasherStore'
import {
  IconAlertTriangle as AlertTriangle,
  IconBolt as Zap,
  IconCpu as Cpu
} from "@tabler/icons-react"

const LOGICAL_PARTITIONS = ['system', 'system_ext', 'vendor', 'product', 'odm', 'super', 'userdata']

interface PartitionFlashCardProps {
  disabled?: boolean
}

export function PartitionFlashCard({ disabled }: PartitionFlashCardProps) {
  const {
    activeFastbootSerial,
    selectedPartition,
    setSelectedPartition,
    selectedImagePath,
    isUserspace,
    runningFlash,
    chooseImageFile,
    executeFlashPartition,
    capturePartitionConsent,
  } = useFlasher()

  const dispatchBusy = useFlashDispatchBusy()
  const targetRevision = useFlashTargetRevision()
  const [pendingConsent, setPendingConsent] = useState<FlashConsent | null>(null)
  // A serial or input change while the dialog is open (including A -> B -> A)
  // invalidates the captured consent; the user has to confirm again.
  const consentStale = pendingConsent !== null && pendingConsent.revision !== targetRevision

  const needsUserspace =
    LOGICAL_PARTITIONS.includes(selectedPartition) && !isUserspace && !!activeFastbootSerial
  const canFlash =
    !!activeFastbootSerial && !!selectedPartition && !!selectedImagePath && !needsUserspace && !disabled

  function handleOpenConfirm() {
    setPendingConsent(capturePartitionConsent())
  }

  function handleConfirmFlash() {
    if (!pendingConsent) return
    const consent = pendingConsent
    setPendingConsent(null)
    void executeFlashPartition(consent)
  }

  return (
    <Card className="relative overflow-hidden border-[var(--border)] dark:border-[var(--border)] bg-card dark:bg-[var(--terminal-bg)]/40 rounded-2xl shadow-[var(--shadow-card)] h-full flex flex-col">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <Zap className="h-4 w-4 text-muted-foreground dark:text-muted-foreground" />
          Flash Partition
        </CardTitle>
      </CardHeader>
      
      <CardContent className="space-y-4 flex-1 flex flex-col justify-between">
        <div className="space-y-4 flex-1">
          <PartitionChips
            selected={selectedPartition}
            onSelect={setSelectedPartition}
            disabled={disabled || runningFlash}
          />

          <div className="relative flex items-center">
            <Input
              placeholder="Or type partition name..."
              value={selectedPartition}
              onChange={(e) => setSelectedPartition(e.target.value)}
              disabled={disabled || runningFlash}
              className="h-8 rounded-full border border-[var(--border)] dark:border-[var(--border)] bg-card dark:bg-[var(--muted)]/60 focus-visible:ring-1 focus-visible:ring-muted-foreground dark:focus-visible:ring-muted-foreground text-xs pl-3.5"
            />
          </div>

          <div className="h-px bg-[var(--muted)] dark:bg-[var(--muted)]/80" />

          <FilePicker
            value={selectedImagePath}
            placeholder="Select .img or .bin file..."
            variant="file-image"
            onBrowse={chooseImageFile}
            disabled={disabled || runningFlash}
          />

          {needsUserspace && (
            <Alert variant="destructive" className="rounded-xl py-2 px-3 border-[var(--destructive)]/20 dark:border-[var(--destructive)]/10 bg-[var(--destructive)]/10 dark:bg-[var(--destructive)]/20 text-[var(--destructive)]">
              <AlertTriangle className="h-3.5 w-3.5 text-[var(--destructive)]" />
              <AlertDescription className="text-[11px] leading-relaxed">
                Logical partitions need fastbootd. Run{' '}
                <code className="rounded bg-[var(--destructive)]/10 dark:bg-[var(--destructive)]/60 px-1 py-0.5 font-mono text-[10px] text-[var(--destructive)] dark:text-[var(--destructive)]">
                  fastboot reboot fastboot
                </code>{' '}
                first.
              </AlertDescription>
            </Alert>
          )}
        </div>

        <Button
          className="w-full rounded-full bg-primary hover:bg-primary/95 text-primary-foreground border-0 transition-[colors,transform] active:scale-[0.97] cursor-pointer text-xs font-semibold shadow-sm h-9 mt-4"
          onClick={handleOpenConfirm}
          disabled={!canFlash || runningFlash || dispatchBusy}
        >
          {runningFlash ? 'Flashing...' : 'Flash Partition'}
        </Button>
      </CardContent>

      <AlertDialog
        open={pendingConsent !== null}
        onOpenChange={(open) => {
          if (!open) setPendingConsent(null)
        }}
      >
        <AlertDialogContent className="rounded-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>Flash {pendingConsent?.partition || 'partition'}?</AlertDialogTitle>
            <AlertDialogDescription>
              This writes the selected image to one partition on the captured fastboot
              device. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-1.5 text-[11px] leading-relaxed text-muted-foreground">
            <div>
              Target device:{' '}
              <span data-testid="partition-confirm-serial" className="font-mono text-foreground">
                {pendingConsent?.serial || 'none'}
              </span>
              {pendingConsent?.deviceLabel ? <> ({pendingConsent.deviceLabel})</> : null}
            </div>
            <div data-testid="partition-confirm-partition">
              Partition: <span className="font-mono text-foreground">{pendingConsent?.partition || 'none'}</span>
            </div>
            <div data-testid="partition-confirm-image" className="break-all">
              Image: <span className="font-mono text-foreground">{pendingConsent?.imagePath || 'none'}</span>
            </div>
          </div>
          {consentStale && (
            <Alert variant="destructive" className="rounded-xl py-2 px-3">
              <AlertTriangle className="h-3.5 w-3.5" />
              <AlertDescription data-testid="partition-confirm-stale">
                The device or the flash inputs changed. Close this dialog and confirm again.
              </AlertDescription>
            </Alert>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel className="rounded-full">Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmFlash}
              disabled={consentStale || dispatchBusy}
              className="rounded-full bg-primary hover:bg-primary/95 text-primary-foreground border-0 shadow-sm"
            >
              Flash Partition
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {disabled && (
        <div className="absolute inset-0 z-20 flex flex-col items-center justify-center bg-card/80 dark:bg-[var(--terminal-surface)]/85 backdrop-blur-[3px] select-none transition-colors duration-300">
          <div className="flex items-center gap-1.5 rounded-full border border-[var(--border)] dark:border-[var(--border)] bg-card dark:bg-[var(--muted)]/90 px-3 py-1.5 shadow-sm text-[11px] font-semibold text-muted-foreground dark:text-muted-foreground">
            <Cpu className="h-3.5 w-3.5 text-muted-foreground dark:text-muted-foreground" />
            Fastboot Mode Required
          </div>
        </div>
      )}
    </Card>
  )
}

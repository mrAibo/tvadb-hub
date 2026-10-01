import { useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useLauncher } from '@/hooks/useLauncher'

// Durable launcher records are read locally, so this panel stays reachable with no
// online device at all. It never promises a state it cannot see: an unreadable record,
// an unknown/pending outcome and a read failure are all shown as such, and restore is
// only offered for a record that belongs to the currently confirmed device.
export function LauncherRecoveryPanel() {
  const launcher = useLauncher()

  useEffect(() => {
    void launcher.loadRecovery()
    // One local, read-only read when the panel mounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [launcher.activeSerial])

  const records = launcher.recovery
  const needsRecovery = records.filter((record) => record.needsRecovery)

  return (
    <Card className="border border-border/60 bg-card shadow-[var(--shadow-card)]">
      <CardHeader className="flex flex-row items-start justify-between px-4 pb-2 pt-4">
        <div>
          <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">
            Launcher recovery
          </CardTitle>
          <p className="mt-1 text-[10px] text-muted-foreground">
            Local record of guarded HOME changes. Read-only, works offline, sends no command.
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 text-xs"
          disabled={launcher.busy !== null}
          onClick={() => void launcher.loadRecovery()}
        >
          Refresh
        </Button>
      </CardHeader>

      <CardContent className="space-y-2 px-4 pb-4 pt-2 text-xs">
        {launcher.recoveryError && (
          <p role="alert" className="text-red-500">
            {launcher.recoveryError}
          </p>
        )}

        {!launcher.recoveryError && records.length === 0 && (
          <p className="text-muted-foreground">No launcher changes were recorded on this host.</p>
        )}

        {records.map((record) => (
          <div
            key={record.id || record.file}
            className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/60 px-3 py-2"
          >
            <div className="min-w-0 space-y-0.5">
              <p className="font-mono text-[11px]">
                {record.unreadable
                  ? `unreadable record: ${record.file || 'unknown file'}`
                  : `${record.id} · ${record.serial} · ${record.state}`}
              </p>
              {!record.unreadable && (
                <p className="text-[10px] text-muted-foreground">
                  {record.needsRecovery ? 'needs recovery' : 'no recovery needed'}
                  {record.blocksApply ? ' · blocks a new change' : ''}
                  {record.lastError ? ` · ${record.lastError}` : ''}
                </p>
              )}
              {record.manualCommand && (
                <p className="font-mono break-all text-[10px] text-muted-foreground">
                  {record.manualCommand}
                </p>
              )}
            </div>
            {!record.unreadable && record.needsRecovery && (
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-8 text-xs"
                disabled={launcher.busy !== null}
                onClick={() => void launcher.restore(record.id)}
              >
                Restore original
              </Button>
            )}
          </div>
        ))}

        {launcher.restoreResult && (
          <p role="status">
            Restore {launcher.restoreResult.state}: {launcher.restoreResult.restoredComponent}{' '}
            (verified: {launcher.restoreResult.verified ? 'yes' : 'no'})
          </p>
        )}

        {needsRecovery.length > 0 && !launcher.activeSerial && (
          <p className="text-amber-500">
            {needsRecovery.length} record(s) need recovery but no device is confirmed; the
            restore button becomes available for the recorded device only.
          </p>
        )}

        {launcher.error && (
          <p role="alert" className="text-red-500">
            {launcher.error}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

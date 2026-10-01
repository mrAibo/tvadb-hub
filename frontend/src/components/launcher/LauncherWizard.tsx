import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useLauncher } from '@/hooks/useLauncher'

// The Current+ wizard is a small, TV-friendly modal on the existing device surface.
// It has no sidebar entry: it is opened from the TV panel (and the devices header) and
// it refuses to apply anything until the read-only check, an explicit candidate test
// and an explicit confirmation agree on one target.
export function LauncherWizardTrigger({ className = '' }: { className?: string }) {
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className={`h-8 text-xs font-medium ${className}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
      >
        Custom launcher (Current+)
      </Button>
      {open && <LauncherWizard onClose={() => setOpen(false)} />}
    </>
  )
}

function LauncherWizard({ onClose }: { onClose: () => void }) {
  const launcher = useLauncher()
  const [candidate, setCandidate] = useState('')
  const closeRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    closeRef.current?.focus()
    void launcher.loadRecovery()
    void launcher.loadPreflight()
    // The wizard owns its own lifecycle: it runs one check when it opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (candidate && !(launcher.preflight?.homeCandidates ?? []).some((c) => c.component === candidate)) {
      setCandidate('')
    }
  }, [candidate, launcher.preflight])

  function handleKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape') {
      event.stopPropagation()
      onClose()
    }
  }

  const identity = launcher.preflight?.identity
  const current = launcher.preflight?.currentHome ?? ''
  const consent = launcher.consent
  const result = launcher.applyResult

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="launcher-wizard-title"
        className="w-full max-w-2xl"
        onClick={(event) => event.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <Card className="border border-border/60 bg-card shadow-[var(--shadow-card)]">
          <CardHeader className="flex flex-row items-start justify-between gap-3 px-4 pb-2 pt-4">
            <div>
              <CardTitle
                id="launcher-wizard-title"
                className="text-sm font-semibold tracking-tight"
              >
                Custom launcher (Current+)
              </CardTitle>
              <p className="mt-1 text-[11px] text-muted-foreground">
                Explicit steps, guarded changes, and recovery options.
              </p>
            </div>
            <Button ref={closeRef} type="button" variant="ghost" size="sm" onClick={onClose}>
              Close
            </Button>
          </CardHeader>

          <CardContent className="space-y-4 px-4 pb-4 pt-2 text-xs">
            <section aria-label="Target" className="space-y-1">
              <h3 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">
                Target
              </h3>
              {identity ? (
                <p className="font-mono">
                  {identity.serial}
                  {identity.model ? ` · ${identity.model}` : ''}
                  {identity.androidVersion ? ` · Android ${identity.androidVersion}` : ''}
                </p>
              ) : (
                <p className="text-muted-foreground">
                  {launcher.activeSerial
                    ? `Confirmed: ${launcher.activeSerial}`
                    : 'No confirmed device selected.'}
                </p>
              )}
              <p>
                Current HOME: <span className="font-mono">{current || 'unknown'}</span>
              </p>
            </section>

            <section aria-label="Candidate launchers" className="space-y-2">
              <h3 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/80">
                New launcher
              </h3>
              {(launcher.preflight?.homeCandidates ?? []).length === 0 && (
                <p className="text-muted-foreground">No HOME candidates were reported.</p>
              )}
              <div className="flex flex-wrap gap-2">
                {(launcher.preflight?.homeCandidates ?? []).map((entry) => (
                  <Button
                    key={entry.component}
                    type="button"
                    size="sm"
                    variant={candidate === entry.component ? 'default' : 'outline'}
                    className="h-8 text-xs"
                    onClick={() => setCandidate(entry.component)}
                  >
                    {entry.package}
                    {entry.installed ? '' : ' (not installed)'}
                    {entry.enabled ? '' : ' (disabled)'}
                    {entry.chooser ? ' (chooser)' : ''}
                  </Button>
                ))}
              </div>
              <div className="flex flex-wrap gap-2 pt-1">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-8 text-xs"
                  disabled={!candidate || launcher.busy !== null}
                  onClick={() => void launcher.testCandidate(candidate)}
                >
                  1. Open for inspection
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-8 text-xs"
                  disabled={!candidate || launcher.busy !== null}
                  onClick={() => {
                    // The hook stores a refusal in its error state, which the
                    // role="alert" line below already renders.
                    launcher.grantConsent(candidate)
                  }}
                >
                  2. Confirm this target
                </Button>
                <Button
                  type="button"
                  size="sm"
                  className="h-8 text-xs"
                  disabled={!launcher.canApply || launcher.busy !== null}
                  onClick={() => void launcher.apply()}
                >
                  3. Apply HOME change
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  className="h-8 text-xs"
                  disabled={!consent}
                  onClick={() => void launcher.cancel()}
                >
                  Cancel owned operation
                </Button>
              </div>
            </section>

            {launcher.blockedReason && !launcher.canApply && (
              <p role="status" className="text-amber-500">
                {launcher.blockedReason}
              </p>
            )}

            {launcher.testResult && (
              <p role="status">
                Candidate test: {launcher.testResult.component} —{' '}
                {launcher.testResult.note || launcher.testResult.detail || 'no detail'}
              </p>
            )}

            {consent && (
              <p role="status" className="font-mono text-[11px]">
                Confirmed for {consent.serial}: {consent.expectedComponent} →{' '}
                {consent.candidateComponent} (operation {consent.operationId})
              </p>
            )}

            {result && (
              <section aria-label="Result" className="space-y-1">
                <p role="status">
                  State: {result.state} · verified: {result.verified ? 'yes' : 'no'} · rolled
                  back: {result.rolledBack ? 'yes' : 'no'}
                </p>
                {result.currentComponent && (
                  <p className="font-mono">
                    Current HOME now: {result.currentComponent}
                  </p>
                )}
                {result.detail && <p className="text-muted-foreground">{result.detail}</p>}
                {!result.verified && (
                  <p className="text-amber-500">
                    Not verified: treat the device as unconfirmed and use the manual command
                    if the launcher did not change.
                  </p>
                )}
                {result.manualCommand && (
                  <p className="font-mono break-all text-[11px]">{result.manualCommand}</p>
                )}
              </section>
            )}

            {launcher.preflight?.probeDetail && (
              <p className="text-muted-foreground">{launcher.preflight.probeDetail}</p>
            )}
            {launcher.error && (
              <p role="alert" className="text-red-500">
                {launcher.error}
              </p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

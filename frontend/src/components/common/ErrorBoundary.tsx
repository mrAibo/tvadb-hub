import { ErrorBoundary } from 'react-error-boundary'
import { isRouteErrorResponse, useRouteError } from 'react-router-dom'

function errorMessage(error: unknown) {
  if (isRouteErrorResponse(error)) {
    return error.statusText || error.data?.message || `Route error ${error.status}`
  }
  return error instanceof Error ? error.message : 'Unknown error'
}

function ErrorScreen({
  title,
  error,
  onRetry,
}: {
  title: string
  error: unknown
  onRetry: () => void
}) {
  const message = errorMessage(error)

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <div className="w-full max-w-lg rounded-2xl border border-border/60 bg-card p-6 text-center shadow-[var(--shadow-card)]">
        <img src="/logo.png" alt="" className="mx-auto h-12 w-12 rounded-xl object-contain" />
        <h1 className="mt-4 text-lg font-semibold text-destructive">{title}</h1>
        <p className="mt-2 break-words text-sm leading-relaxed text-muted-foreground">
          {message}
        </p>
        <p className="mt-3 text-xs leading-relaxed text-muted-foreground">
          DroidSphere caught this error before the interface could continue safely.
        </p>
        <button
          type="button"
          onClick={onRetry}
          className="mt-5 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          Reload DroidSphere
        </button>
      </div>
    </div>
  )
}

function RouteErrorFallback({
  error,
  resetErrorBoundary,
}: {
  error: unknown
  resetErrorBoundary: () => void
}) {
  return (
    <ErrorScreen
      title="Something went wrong"
      error={error}
      onRetry={resetErrorBoundary}
    />
  )
}

function GlobalErrorFallback({
  error,
  resetErrorBoundary,
}: {
  error: unknown
  resetErrorBoundary: () => void
}) {
  return (
    <ErrorScreen
      title="Application Error"
      error={error}
      onRetry={resetErrorBoundary}
    />
  )
}

export function RouterErrorPage() {
  const error = useRouteError()
  return (
    <ErrorScreen
      title="Application Error"
      error={error}
      onRetry={() => window.location.reload()}
    />
  )
}

export function AppErrorBoundary({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary
      FallbackComponent={GlobalErrorFallback}
      onReset={() => window.location.reload()}
    >
      {children}
    </ErrorBoundary>
  )
}

export function RouteErrorBoundary({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary FallbackComponent={RouteErrorFallback}>
      {children}
    </ErrorBoundary>
  )
}


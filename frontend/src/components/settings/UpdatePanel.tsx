import { useState } from 'react'
import {
  IconRefresh as RefreshCw,
  IconExternalLink as ExternalLink,
  IconCircleCheck as CheckCircle2,
  IconDownload as Download,
} from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { checkForUpdates } from '@/services/updateService'
import type { UpdateInfo } from '@/lib/types'

export function UpdatePanel() {
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleCheck() {
    setLoading(true)
    setError(null)
    try {
      setInfo(await checkForUpdates())
    } catch (err) {
      setInfo(null)
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card className="rounded-2xl border border-border bg-card shadow-[var(--shadow-card)]">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center justify-between gap-3 text-sm font-semibold">
          <span>Application updates</span>
          {info && (
            <Badge variant={info.updateAvailable ? 'default' : 'secondary'}>
              {info.updateAvailable ? 'Update available' : 'Up to date'}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs leading-relaxed text-muted-foreground">
          Check the official TVADB Hub GitHub Releases feed. No update is installed automatically.
        </p>

        {info && (
          <div className="rounded-xl border border-border/50 bg-muted/20 p-3 text-xs">
            <div className="flex items-center gap-2">
              {info.updateAvailable ? (
                <Download className="h-4 w-4 text-primary" />
              ) : (
                <CheckCircle2 className="h-4 w-4 text-emerald-500" />
              )}
              <span className="font-medium">
                Current {info.currentVersion} · Latest {info.latestVersion}
              </span>
            </div>
            {info.releaseName && (
              <p className="mt-1 text-muted-foreground">{info.releaseName}</p>
            )}
          </div>
        )}

        {error && (
          <p className="rounded-lg border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {error}
          </p>
        )}

        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="outline"
            className="h-8 text-xs"
            onClick={() => void handleCheck()}
            disabled={loading}
          >
            <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            {loading ? 'Checking...' : 'Check for updates'}
          </Button>

          {info?.releaseUrl && info.updateAvailable && (
            <Button
              size="sm"
              className="h-8 text-xs"
              onClick={() => window.open(info.releaseUrl, '_blank', 'noopener,noreferrer')}
            >
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              Open release
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

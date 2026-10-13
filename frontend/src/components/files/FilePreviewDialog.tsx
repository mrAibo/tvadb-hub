import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { FilePreviewResult } from '@/lib/types'

interface FilePreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  filename: string
  location: string
  loading: boolean
  result: FilePreviewResult | null
  error: string | null
}

const MAX_RENDER_CHARS = 200_000

export function FilePreviewDialog({
  open, onOpenChange, filename, location, loading, result, error,
}: FilePreviewDialogProps) {
  const text = result?.text ?? ''
  const truncated = text.length > MAX_RENDER_CHARS
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] w-[min(95vw,900px)] max-w-[900px] flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle className="truncate">{filename}</DialogTitle>
          <DialogDescription className="truncate" title={location}>
            Read-only preview · {location} · 2 MiB file limit
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-28 min-w-0 flex-1 overflow-auto rounded-lg border border-border/50 bg-muted/20 p-3">
          {loading && <p role="status" className="text-xs text-muted-foreground">Loading preview…</p>}
          {!loading && error && <p role="alert" className="text-xs text-destructive">{error}</p>}
          {!loading && !error && result?.kind === 'image' && result.imageDataUrl && (
            <img src={result.imageDataUrl} alt={filename} className="mx-auto max-h-[65vh] max-w-full object-contain" />
          )}
          {!loading && !error && result?.kind === 'text' && (
            <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-relaxed">{text.slice(0, MAX_RENDER_CHARS)}</pre>
          )}
          {!loading && !error && result?.kind === 'text' && truncated && (
            <p className="mt-3 text-xs text-muted-foreground">Displaying first 200,000 characters only.</p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

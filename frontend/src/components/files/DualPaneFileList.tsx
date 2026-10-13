import {
  IconFile as FileIcon,
  IconFolder as Folder,
  IconLink as Link,
  IconEye as Eye,
} from '@tabler/icons-react'
import { Checkbox } from '@/components/ui/checkbox'
import { cn } from '@/lib/utils'
import type { FileEntry } from '@/lib/types'

interface DualPaneFileListProps {
  files: FileEntry[]
  selected: string[]
  loading?: boolean
  emptyLabel: string
  onToggle: (path: string) => void
  onOpenDirectory: (file: FileEntry) => void
  onPreview?: (file: FileEntry) => void
}

function iconFor(file: FileEntry) {
  if (file.type === 'directory') return Folder
  if (file.type === 'symlink') return Link
  return FileIcon
}

export function DualPaneFileList({
  files,
  selected,
  loading,
  emptyLabel,
  onToggle,
  onOpenDirectory,
  onPreview,
}: DualPaneFileListProps) {
  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center text-xs text-muted-foreground">
        Loading…
      </div>
    )
  }

  if (files.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-center text-xs text-muted-foreground">
        {emptyLabel}
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-auto perf-scroll">
      <div className="sticky top-0 z-10 grid h-8 grid-cols-[28px_minmax(0,1fr)_76px_116px] items-center border-b border-border/50 bg-card/95 px-2 text-[9px] font-bold uppercase tracking-wider text-muted-foreground backdrop-blur">
        <span />
        <span>Name</span>
        <span className="text-right">Size</span>
        <span className="text-right">Modified</span>
      </div>
      <div className="divide-y divide-border/35">
        {files.map((file) => {
          const Icon = iconFor(file)
          const isSelected = selected.includes(file.path)
          return (
            <div
              key={file.path}
              className={cn(
                'grid min-h-9 grid-cols-[28px_minmax(0,1fr)_76px_116px] items-center px-2 text-[10px] transition-colors',
                isSelected ? 'bg-primary/8' : 'hover:bg-muted/25',
              )}
            >
              <Checkbox
                checked={isSelected}
                onCheckedChange={() => onToggle(file.path)}
                aria-label={`Select ${file.name}`}
              />
              <div className="flex min-w-0 items-center gap-1">
                <button
                  type="button"
                  onClick={() => file.type === 'directory' && onOpenDirectory(file)}
                  className={cn(
                    'flex min-w-0 flex-1 items-center gap-2 text-left',
                    file.type === 'directory' ? 'cursor-pointer hover:text-primary' : 'cursor-default',
                  )}
                  title={file.path}
                >
                  <Icon className={cn('h-3.5 w-3.5 shrink-0', file.type === 'directory' ? 'text-primary' : 'text-muted-foreground')} />
                  <span className="truncate font-medium">{file.name}</span>
                </button>
                {onPreview && file.type === 'file' && /\.(txt|md|log|csv|json|xml|yaml|yml|ini|conf|cfg|properties|go|ts|tsx|js|jsx|py|sh|css|html|htm|png|jpe?g|gif|webp)$/i.test(file.name) && (
                  <button type="button" onClick={() => onPreview(file)} title={`Preview ${file.name}`} aria-label={`Preview ${file.name}`} className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-primary">
                    <Eye className="h-3.5 w-3.5" />
                  </button>
                )}
              </div>
              <span className="truncate text-right font-mono text-[9px] text-muted-foreground">
                {file.sizeHuman}
              </span>
              <span className="truncate text-right font-mono text-[9px] text-muted-foreground">
                {file.modifiedAt || '—'}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

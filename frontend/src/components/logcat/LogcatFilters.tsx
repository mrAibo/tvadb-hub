import { useState, type FormEvent } from 'react'
import {
  IconDeviceFloppy as Save,
  IconX as X,
} from '@tabler/icons-react'
import type { LogcatIssueFilter, LogcatLevel } from '@/lib/types'
import { useLogcatStore } from '@/stores/useLogcatStore'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

const LEVELS: { value: LogcatLevel; label: string }[] = [
  { value: 'V', label: 'Verbose' },
  { value: 'D', label: 'Debug' },
  { value: 'I', label: 'Info' },
  { value: 'W', label: 'Warn' },
  { value: 'E', label: 'Error' },
  { value: 'F', label: 'Fatal' },
]

const ISSUE_FILTERS: Array<{ value: LogcatIssueFilter; label: string }> = [
  { value: 'all', label: 'Any' },
  { value: 'crash', label: 'Crash' },
  { value: 'anr', label: 'ANR' },
]

const LOGCAT_PRESETS: Array<{
  id: string
  label: string
  levels: LogcatLevel[]
  tag: string
  text: string
  issue: LogcatIssueFilter
}> = [
  {
    id: 'crashes',
    label: 'Crashes',
    levels: ['E', 'F'],
    tag: '',
    text: '',
    issue: 'crash',
  },
  {
    id: 'anr',
    label: 'ANR',
    levels: ['W', 'E', 'F'],
    tag: '',
    text: '',
    issue: 'anr',
  },
  {
    id: 'tv-errors',
    label: 'TV Errors',
    levels: ['W', 'E', 'F'],
    tag: '',
    text: '',
    issue: 'all',
  },
  {
    id: 'media',
    label: 'MediaCodec',
    levels: ['D', 'I', 'W', 'E', 'F'],
    tag: 'MediaCodec',
    text: '',
    issue: 'all',
  },
  {
    id: 'launcher',
    label: 'Launcher',
    levels: ['D', 'I', 'W', 'E', 'F'],
    tag: 'ActivityTaskManager',
    text: '',
    issue: 'all',
  },
  {
    id: 'wifi',
    label: 'Wi-Fi',
    levels: ['D', 'I', 'W', 'E', 'F'],
    tag: 'Wifi',
    text: '',
    issue: 'all',
  },
]

const LEVEL_BADGE_COLORS: Record<LogcatLevel, string> = {
  V: 'border-muted-foreground/30 text-muted-foreground hover:bg-muted-foreground/10',
  D: 'border-[var(--logcat-debug)]/30 text-[var(--logcat-debug)] hover:bg-[var(--logcat-debug)]/10',
  I: 'border-[var(--logcat-info)]/30 text-[var(--logcat-info)] hover:bg-[var(--logcat-info)]/10',
  W: 'border-[var(--logcat-warn)]/30 text-[var(--logcat-warn)] hover:bg-[var(--logcat-warn)]/10',
  E: 'border-[var(--logcat-error)]/30 text-[var(--logcat-error)] hover:bg-[var(--logcat-error)]/10',
  F: 'border-[var(--logcat-fatal)]/40 text-[var(--logcat-fatal)] font-bold hover:bg-[var(--logcat-fatal)]/20',
}

const LEVEL_ACTIVE_BG: Record<LogcatLevel, string> = {
  V: 'bg-muted-foreground/20',
  D: 'bg-[var(--logcat-debug)]/20',
  I: 'bg-[var(--logcat-info)]/20',
  W: 'bg-[var(--logcat-warn)]/20',
  E: 'bg-[var(--logcat-error)]/20',
  F: 'bg-[var(--logcat-error)]/30',
}

export function LogcatFilters() {
  const filter = useLogcatStore((state) => state.filter)
  const savedFilters = useLogcatStore((state) => state.savedFilters)
  const setFilter = useLogcatStore((state) => state.setFilter)
  const saveCurrentFilter = useLogcatStore((state) => state.saveCurrentFilter)
  const applySavedFilter = useLogcatStore((state) => state.applySavedFilter)
  const deleteSavedFilter = useLogcatStore((state) => state.deleteSavedFilter)
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)
  const [saveName, setSaveName] = useState('')

  function toggleLevel(level: LogcatLevel) {
    const next = filter.levels.includes(level)
      ? filter.levels.filter((l) => l !== level)
      : [...filter.levels, level]

    if (next.length > 0) {
      setFilter({ levels: next })
    }
  }

  function resetFilters() {
    setFilter({
      levels: ['V', 'D', 'I', 'W', 'E', 'F'],
      tag: '',
      text: '',
      pid: '',
      process: '',
      issue: 'all',
    })
  }

  function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!saveName.trim()) {
      return
    }

    saveCurrentFilter(saveName)
    setSaveName('')
    setSaveDialogOpen(false)
  }

  return (
    <>
      <div className="flex flex-col gap-2 border-b border-border/40 bg-muted/20 px-4 py-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
            Quick filters
          </span>
          {LOGCAT_PRESETS.map((preset) => (
            <Badge
              key={preset.id}
              variant="outline"
              className={cn(
                'h-5 cursor-pointer px-1.5 text-[10px] text-muted-foreground hover:bg-primary/10 hover:text-primary',
                preset.issue === 'crash' && 'border-destructive/40 text-destructive',
                preset.issue === 'anr' && 'border-[var(--warning)]/40 text-[var(--warning)]',
              )}
              onClick={() =>
                setFilter({
                  levels: [...preset.levels],
                  tag: preset.tag,
                  text: preset.text,
                  pid: '',
                  process: '',
                  issue: preset.issue,
                })
              }
            >
              {preset.label}
            </Badge>
          ))}
          <Badge
            variant="outline"
            className="h-5 cursor-pointer px-1.5 text-[10px] text-muted-foreground hover:bg-muted"
            onClick={resetFilters}
          >
            All
          </Badge>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1.5">
            {LEVELS.map(({ value, label }) => {
              const isActive = filter.levels.includes(value)

              return (
                <Badge
                  key={value}
                  variant="outline"
                  className={cn(
                    'cursor-pointer text-[10px] h-5 px-1.5 gap-0.5 transition-colors',
                    LEVEL_BADGE_COLORS[value],
                    isActive && LEVEL_ACTIVE_BG[value],
                  )}
                  onClick={() => toggleLevel(value)}
                  title={`${label} (${value})`}
                >
                  {value}
                </Badge>
              )
            })}
          </div>

          <div className="h-4 w-px bg-border/40" />

          <div className="flex items-center gap-1">
            <span className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
              Issue
            </span>
            {ISSUE_FILTERS.map((issue) => (
              <Badge
                key={issue.value}
                variant="outline"
                className={cn(
                  'h-5 cursor-pointer px-1.5 text-[10px] text-muted-foreground',
                  filter.issue === issue.value && 'bg-primary/10 text-primary',
                  issue.value === 'crash' && filter.issue === 'crash' && 'border-destructive/50 text-destructive',
                  issue.value === 'anr' && filter.issue === 'anr' && 'border-[var(--warning)]/50 text-[var(--warning)]',
                )}
                onClick={() => setFilter({ issue: issue.value })}
              >
                {issue.label}
              </Badge>
            ))}
          </div>

          <div className="h-4 w-px bg-border/40" />

          <Input
            value={filter.tag}
            onChange={(e) => setFilter({ tag: e.target.value })}
            placeholder="Filter by tag..."
            className="h-6 w-36 text-xs rounded-lg"
          />

          <Input
            value={filter.text}
            onChange={(e) => setFilter({ text: e.target.value })}
            placeholder="Search messages..."
            className="h-6 w-48 text-xs rounded-lg"
          />

          <Input
            value={filter.pid}
            onChange={(e) => setFilter({ pid: e.target.value.replace(/\D/g, '') })}
            placeholder="PID..."
            inputMode="numeric"
            className="h-6 w-20 text-xs rounded-lg"
          />

          <Input
            value={filter.process}
            onChange={(e) => setFilter({ process: e.target.value })}
            placeholder="App / process..."
            className="h-6 w-40 text-xs rounded-lg"
          />

          <Button
            size="sm"
            variant="ghost"
            className="h-6 gap-1 px-2 text-[10px] text-muted-foreground"
            onClick={() => setSaveDialogOpen(true)}
          >
            <Save className="h-3 w-3" />
            Save filter
          </Button>
        </div>

        {savedFilters.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="mr-1 text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">
              Saved
            </span>
            {savedFilters.map((saved) => (
              <div
                key={saved.id}
                className="flex items-center overflow-hidden rounded-md border border-border/50 bg-background/60"
              >
                <button
                  type="button"
                  className="h-5 px-1.5 text-[10px] text-muted-foreground hover:bg-primary/10 hover:text-primary"
                  onClick={() => applySavedFilter(saved.id)}
                >
                  {saved.name}
                </button>
                <button
                  type="button"
                  className="flex h-5 w-5 items-center justify-center border-l border-border/40 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                  aria-label={`Delete saved filter ${saved.name}`}
                  onClick={() => deleteSavedFilter(saved.id)}
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog open={saveDialogOpen} onOpenChange={setSaveDialogOpen}>
        <DialogContent>
          <form className="grid gap-4" onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle>Save Logcat filter</DialogTitle>
              <DialogDescription>
                Save the current level, issue, tag, message, PID and process filters on this computer.
              </DialogDescription>
            </DialogHeader>
            <Input
              autoFocus
              value={saveName}
              onChange={(event) => setSaveName(event.target.value)}
              placeholder="Filter name"
              maxLength={40}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setSaveDialogOpen(false)}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={!saveName.trim()}>
                Save
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}

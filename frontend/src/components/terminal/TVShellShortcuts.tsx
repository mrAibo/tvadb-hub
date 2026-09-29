import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import { TV_SHELL_SHORTCUTS } from '@/lib/tvShellShortcuts'

interface TVShellShortcutsProps {
  disabled?: boolean
  onRun: (command: string) => void
}

export function TVShellShortcuts({
  disabled = false,
  onRun,
}: TVShellShortcutsProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        disabled={disabled}
        className="inline-flex h-7 items-center rounded-full border border-border bg-background px-3 text-xs font-semibold text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
      >
        TV shortcuts
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        {TV_SHELL_SHORTCUTS.map((shortcut) => (
          <DropdownMenuItem
            key={shortcut.id}
            onClick={() => onRun(shortcut.command)}
            className="flex cursor-pointer flex-col items-start gap-0.5 py-2"
          >
            <span className="text-xs font-medium">{shortcut.label}</span>
            <span className="text-[10px] text-muted-foreground">
              {shortcut.description}
            </span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

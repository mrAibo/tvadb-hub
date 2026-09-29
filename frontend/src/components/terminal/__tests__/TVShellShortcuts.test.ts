import { describe, expect, it } from 'vitest'
import { TV_SHELL_SHORTCUTS } from '@/lib/tvShellShortcuts'

describe('TV_SHELL_SHORTCUTS', () => {
  it('contains useful read-only Android TV diagnostics', () => {
    const byId = new Map(TV_SHELL_SHORTCUTS.map((shortcut) => [shortcut.id, shortcut]))

    expect(byId.get('device')?.command).toContain('ro.product.model')
    expect(byId.get('display')?.command).toContain('wm size')
    expect(byId.get('network')?.command).toContain('wlan0')
    expect(byId.get('storage')?.command).toContain('df -h')
    expect(byId.get('uptime')?.command).toBe('uptime')
  })

  it('does not include destructive shell commands', () => {
    const joined = TV_SHELL_SHORTCUTS.map((shortcut) => shortcut.command)
      .join('\n')
      .toLowerCase()

    expect(joined).not.toContain(' rm ')
    expect(joined).not.toContain('reboot')
    expect(joined).not.toContain('pm uninstall')
    expect(joined).not.toContain('settings put')
  })
})

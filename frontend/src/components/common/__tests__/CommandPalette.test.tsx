import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUIStore } from '@/stores/useUIStore'

const mocks = vi.hoisted(() => ({
  persistTheme: vi.fn().mockResolvedValue(undefined),
  getDevices: vi.fn().mockResolvedValue([]),
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({ setTheme: mocks.persistTheme }),
}))

vi.mock('@/services/deviceService', () => ({
  getDevices: mocks.getDevices,
}))

vi.mock('sonner', () => ({ toast: mocks.toast }))

import { CommandPalette } from '../CommandPalette'

function pressKey(init: KeyboardEventInit) {
  act(() => {
    document.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, ...init }))
  })
}

function renderPalette() {
  return render(
    <MemoryRouter>
      <CommandPalette />
    </MemoryRouter>,
  )
}

describe('CommandPalette keyboard and theme behaviour', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window.navigator, 'platform', { value: 'Win32', configurable: true })
    useUIStore.setState({ commandPaletteOpen: false, theme: 'dark' })
  })

  it('opens on the unshifted palette chord', () => {
    renderPalette()

    pressKey({ key: 'k', ctrlKey: true })

    expect(screen.getByPlaceholderText('Type a command or search...')).toBeInTheDocument()
  })

  it('ignores the shifted chord that belongs to the Logcat surface', () => {
    renderPalette()

    pressKey({ key: 'K', ctrlKey: true, shiftKey: true })

    expect(screen.queryByPlaceholderText('Type a command or search...')).not.toBeInTheDocument()
  })

  it('ignores the chord when an extra modifier is held', () => {
    renderPalette()

    pressKey({ key: 'k', ctrlKey: true, altKey: true })

    expect(screen.queryByPlaceholderText('Type a command or search...')).not.toBeInTheDocument()
  })

  it('toggles the effective theme through the persisted setter', () => {
    renderPalette()
    pressKey({ key: 'k', ctrlKey: true })

    fireEvent.click(screen.getByText('Switch to light mode'))

    expect(mocks.persistTheme).toHaveBeenCalledWith('light')
    expect(useUIStore.getState().theme).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(useUIStore.getState().commandPaletteOpen).toBe(false)
  })

  it('toggles back to dark from the light theme', () => {
    useUIStore.setState({ theme: 'light' })
    renderPalette()
    pressKey({ key: 'k', ctrlKey: true })

    fireEvent.click(screen.getByText('Switch to dark mode'))

    expect(mocks.persistTheme).toHaveBeenCalledWith('dark')
    expect(useUIStore.getState().theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('advertises the platform modifier in the footer hint', () => {
    renderPalette()
    pressKey({ key: 'k', ctrlKey: true })

    expect(screen.getByText('Ctrl+K')).toBeInTheDocument()
  })
})

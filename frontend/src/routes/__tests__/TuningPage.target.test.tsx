import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SafeTuningAnalysis } from '@/lib/types'
import TuningPage from '../TuningPage'

const state = vi.hoisted(() => ({ serial: 'A' }))
const api = vi.hoisted(() => ({
  analyzeSafeTuning: vi.fn(), listTuningSnapshots: vi.fn(), applySafeTuning: vi.fn(),
  restoreTuningSnapshot: vi.fn(), getSafeTuningFeedConfig: vi.fn(), getSafeTuningFeedStatus: vi.fn(),
  configureSafeTuningFeed: vi.fn(), refreshSafeTuningFeed: vi.fn(), rollbackSafeTuningFeed: vi.fn(),
}))
vi.mock('@/hooks/useDevices', () => ({ useDevices: () => ({ activeSerial: state.serial, deviceInfo: null }) }))
vi.mock('@/services/tuningService', () => api)

function analysis(serial: string): SafeTuningAnalysis {
  return {
    serial, model: `TV ${serial}`, manufacturer: 'Test', androidVersion: '14', isTV: true,
    selectedProfile: { id: `profile-${serial}`, name: `Profile ${serial}`, deviceFamily: 'Test', description: 'Test', sourceName: 'Test', sourceUrl: '', sourceLicense: 'MIT', matchScore: 1, recommended: true },
    availableProfiles: [], matches: [{ packageName: 'com.example.optional', label: 'Optional', category: 'Apps', reason: 'Optional', risk: 'safe', defaultSelected: true, isEnabled: true, isSystemApp: true, protected: false, actionable: true }],
    installedCount: 1, defaultSelected: ['com.example.optional'], protectedInstalled: [],
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  state.serial = 'A'
  api.listTuningSnapshots.mockResolvedValue([])
  api.getSafeTuningFeedConfig.mockResolvedValue({ url: '', publicKey: '' })
  api.getSafeTuningFeedStatus.mockResolvedValue({ source: 'builtin' })
  api.applySafeTuning.mockResolvedValue({ changed: ['com.example.optional'], failed: {}, snapshotId: 'test' })
})

describe('Safe Tuning target binding', () => {
  it('discards a late analysis after the selected device changes', async () => {
    let finishA!: (value: SafeTuningAnalysis) => void
    api.analyzeSafeTuning.mockImplementation((serial: string) => serial === 'A' ? new Promise<SafeTuningAnalysis>((resolve) => { finishA = resolve }) : Promise.resolve(analysis(serial)))
    const view = render(<TuningPage />)
    state.serial = 'B'
    view.rerender(<TuningPage />)
    await waitFor(() => expect(document.body.textContent).toContain('Profile B'))
    await act(async () => { finishA(analysis('A')) })
    expect(document.body.textContent).not.toContain('Profile A')
    expect(document.body.textContent).toContain('Profile B')
    expect(api.analyzeSafeTuning).toHaveBeenCalledWith('B', '')
  })

  it('sends the confirmed serial and identifies it in the confirmation', async () => {
    api.analyzeSafeTuning.mockResolvedValue(analysis('A'))
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<TuningPage />)
    const button = await screen.findByRole('button', { name: 'Apply 1 selected' })
    fireEvent.click(button)
    await waitFor(() => expect(api.applySafeTuning).toHaveBeenCalledWith(expect.objectContaining({ expectedSerial: 'A' })))
    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('TV A (A)'))
    confirm.mockRestore()
  })
})

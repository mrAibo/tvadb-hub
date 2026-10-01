import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useDeviceStore } from '@/stores/useDeviceStore'

const mocks = vi.hoisted(() => ({
  preflightLauncher: vi.fn(),
  testLauncherCandidate: vi.fn(),
  applyLauncher: vi.fn(),
  cancelLauncherOperation: vi.fn(),
  restoreLauncher: vi.fn(),
  readLauncherRecovery: vi.fn(),
}))

vi.mock('@/services/launcherService', () => mocks)

// Isolate the store's service boundary: the real deviceService re-exports
// @wailsio/runtime, whose import-time 50 ms poll is not owned by the jsdom
// environment (PHASE0_RUNTIME_DIAGNOSIS.md). The runtime mock is a fail-fast
// sentinel: it is only evaluated if an import still reaches the runtime.
vi.mock('@/services/deviceService', () => ({
  getDeviceInfo: vi.fn(),
  getDeviceMode: vi.fn(),
  setActiveSerial: vi.fn(),
}))
vi.mock('@wailsio/runtime', () => {
  throw new Error('Unexpected Wails runtime import in isolated unit test')
})

import { useLauncher } from '../useLauncher'
import { LauncherWizardTrigger } from '@/components/launcher/LauncherWizard'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function candidate(component: string, packageName: string) {
  return { component, package: packageName, installed: true, enabled: true, selected: false }
}

const stock = candidate('com.stock/.Home', 'com.stock')
const newLauncher = candidate('com.tcl.launcher/.Main', 'com.tcl.launcher')

function preflightPayload(overrides: Record<string, unknown> = {}) {
  return {
    identity: { serial: 'A', model: 'TV A' },
    userId: 0,
    capability: { status: 'supported', setHomeActivity: true, resolveActivity: true },
    supported: true,
    restorable: true,
    currentHome: stock.component,
    homeCandidates: [stock, newLauncher],
    recovery: [],
    manualCommand: '',
    probeDetail: '',
    ...overrides,
  }
}

describe('useLauncher ownership', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useDeviceStore.setState({ activeSerial: 'A' })
    mocks.preflightLauncher.mockResolvedValue(preflightPayload())
    mocks.testLauncherCandidate.mockResolvedValue({
      component: newLauncher.component,
      resolved: true,
      launched: true,
      note: 'Candidate opened for inspection',
    })
    mocks.applyLauncher.mockResolvedValue({
      operationId: 'op-1',
      serial: 'A',
      state: 'applied',
      originalComponent: stock.component,
      candidateComponent: newLauncher.component,
      currentComponent: newLauncher.component,
      verified: true,
      rolledBack: false,
    })
    mocks.cancelLauncherOperation.mockResolvedValue({
      operationId: 'op-1',
      serial: 'A',
      cancelled: true,
    })
    mocks.restoreLauncher.mockResolvedValue({
      recordId: 'r1',
      serial: 'A',
      state: 'restored',
      restoredComponent: stock.component,
      verified: true,
    })
    mocks.readLauncherRecovery.mockResolvedValue({ records: [], note: 'local' })
  })

  it('cannot apply while the device is unsupported or unknown', async () => {
    mocks.preflightLauncher.mockResolvedValue(
      preflightPayload({
        supported: false,
        restorable: false,
        reason: 'The supported HOME commands could not be probed: cmd package help probe failed',
        homeCandidates: [],
        currentHome: '',
      }),
    )
    const { result } = renderHook(() => useLauncher())

    await act(async () => {
      await result.current.loadPreflight()
    })

    expect(result.current.canApply).toBe(false)
    expect(result.current.blockedReason).toContain('could not be probed')

    let applied: unknown
    await act(async () => {
      applied = await result.current.apply()
    })
    expect(applied).toBeNull()
    expect(mocks.applyLauncher).not.toHaveBeenCalled()
  })

  it('refuses a candidate package with several HOME activities', async () => {
    mocks.preflightLauncher.mockResolvedValue(
      preflightPayload({
        homeCandidates: [
          stock,
          newLauncher,
          candidate('com.tcl.launcher/.Second', 'com.tcl.launcher'),
        ],
      }),
    )
    const { result } = renderHook(() => useLauncher())
    await act(async () => {
      await result.current.loadPreflight()
    })

    expect(result.current.ambiguousPackages).toEqual(['com.tcl.launcher (2 HOME activities)'])
    expect(result.current.canApply).toBe(false)

    let refusal: string | null = null
    act(() => {
      refusal = result.current.grantConsent(newLauncher.component)
    })
    expect(refusal).toContain('several HOME activities')
    expect(result.current.consent).toBeNull()
    // The refusal must also reach the hook error so the role="alert" line renders it.
    expect(result.current.error).toContain('several HOME activities')
  })

  it('requires an explicit candidate test and confirmation before applying', async () => {
    const { result } = renderHook(() => useLauncher())
    await act(async () => {
      await result.current.loadPreflight()
    })

    // Confirmation without the explicit candidate test is refused.
    let refusal: string | null = null
    act(() => {
      refusal = result.current.grantConsent(newLauncher.component)
    })
    expect(refusal).toContain('inspection')
    expect(result.current.consent).toBeNull()
    expect(result.current.error).toContain('inspection')

    await act(async () => {
      await result.current.testCandidate(newLauncher.component)
    })
    expect(mocks.testLauncherCandidate).toHaveBeenCalledWith('A', newLauncher.component)

    act(() => {
      refusal = result.current.grantConsent(newLauncher.component)
    })
    expect(refusal).toBeNull()
    expect(result.current.consent).not.toBeNull()
    expect(result.current.canApply).toBe(true)

    await act(async () => {
      await result.current.apply()
    })
    expect(mocks.applyLauncher).toHaveBeenCalledTimes(1)
    expect(mocks.applyLauncher).toHaveBeenCalledWith({
      operationId: result.current.consent?.operationId,
      expectedSerial: 'A',
      expectedComponent: stock.component,
      candidateComponent: newLauncher.component,
    })
  })

  it('is single-flight and drops a reply that arrives after a device switch', async () => {
    const gate = deferred<unknown>()
    mocks.applyLauncher.mockReturnValueOnce(gate.promise)
    const { result } = renderHook(() => useLauncher())
    await act(async () => {
      await result.current.loadPreflight()
    })
    await act(async () => {
      await result.current.testCandidate(newLauncher.component)
    })
    act(() => {
      result.current.grantConsent(newLauncher.component)
    })

    let first!: Promise<unknown>
    act(() => {
      first = result.current.apply()
    })
    // A second apply while the first is running must not start another operation.
    await act(async () => {
      await result.current.apply()
    })
    expect(mocks.applyLauncher).toHaveBeenCalledTimes(1)

    // The device changes while the operation is in flight: the reply is not applied
    // to the new selection and no verification is claimed for it.
    act(() => {
      useDeviceStore.setState({ activeSerial: 'B' })
    })
    await act(async () => {
      gate.resolve({
        operationId: 'op-1',
        serial: 'A',
        state: 'applied',
        verified: true,
        rolledBack: false,
      })
      await first
    })

    expect(result.current.applyResult).toBeNull()
    expect(result.current.consent).toBeNull()
    expect(result.current.error).toContain('device changed')
  })

  it('invalidates consent on a switch and sends the owned serial when cancelling', async () => {
    const { result } = renderHook(() => useLauncher())
    await act(async () => {
      await result.current.loadPreflight()
    })
    await act(async () => {
      await result.current.testCandidate(newLauncher.component)
    })
    act(() => {
      result.current.grantConsent(newLauncher.component)
    })
    const ownedId = result.current.consent?.operationId
    expect(ownedId).toBeTruthy()

    await act(async () => {
      await result.current.cancel()
    })
    // The owned operation id and the serial it was created for - never the live one.
    expect(mocks.cancelLauncherOperation).toHaveBeenCalledWith({
      operationId: ownedId,
      expectedSerial: 'A',
    })

    act(() => {
      useDeviceStore.setState({ activeSerial: 'B' })
    })
    expect(result.current.consent).toBeNull()
    expect(result.current.canApply).toBe(false)

    let applied: unknown
    await act(async () => {
      applied = await result.current.apply()
    })
    expect(applied).toBeNull()
    expect(mocks.applyLauncher).not.toHaveBeenCalled()
  })

  it('discards a preflight reply that arrives after an A-B-A switch', async () => {
    const gate = deferred<unknown>()
    mocks.preflightLauncher.mockReturnValueOnce(gate.promise)
    const { result } = renderHook(() => useLauncher())

    let pending!: Promise<unknown>
    act(() => {
      pending = result.current.loadPreflight()
    })
    act(() => {
      useDeviceStore.setState({ activeSerial: 'B' })
    })
    act(() => {
      useDeviceStore.setState({ activeSerial: 'A' })
    })
    await act(async () => {
      gate.resolve(preflightPayload())
      await pending
    })

    expect(result.current.preflight).toBeNull()
  })

  it('keeps recovery reachable offline and reports failures instead of an empty list', async () => {
    useDeviceStore.setState({ activeSerial: '' })
    mocks.readLauncherRecovery.mockResolvedValue({
      records: [
        {
          id: 'r1',
          serial: 'A',
          state: 'unknown',
          needsRecovery: true,
          blocksApply: true,
          manualCommand: 'adb -s A shell cmd package set-home-activity --user 0 com.stock/.Home',
        },
        { id: '', unreadable: true, file: 'op-broken.json' },
      ],
      note: 'local',
    })
    const { result } = renderHook(() => useLauncher())

    await act(async () => {
      await result.current.loadRecovery()
    })
    expect(mocks.readLauncherRecovery).toHaveBeenCalledWith('')
    expect(result.current.recovery).toHaveLength(2)
    expect(result.current.recovery[0].state).toBe('unknown')
    expect(result.current.recovery[1].unreadable).toBe(true)

    // Restoring a record that belongs to another device is refused before any call.
    act(() => {
      useDeviceStore.setState({ activeSerial: 'B' })
    })
    let restored: unknown
    await act(async () => {
      restored = await result.current.restore('r1')
    })
    expect(restored).toBeNull()
    expect(mocks.restoreLauncher).not.toHaveBeenCalled()

    // A read failure is surfaced, never rendered as "nothing to recover".
    mocks.readLauncherRecovery.mockRejectedValueOnce(new Error('launcher_record: path is not a directory'))
    await act(async () => {
      await result.current.loadRecovery()
    })
    expect(result.current.recoveryError).toContain('not a directory')
  })

  it('restores only on an explicit live match', async () => {
    mocks.readLauncherRecovery.mockResolvedValue({
      records: [{ id: 'r1', serial: 'A', state: 'pending', needsRecovery: true, blocksApply: true }],
      note: 'local',
    })
    const { result } = renderHook(() => useLauncher())
    await act(async () => {
      await result.current.loadRecovery()
    })

    await act(async () => {
      await result.current.restore('r1')
    })
    expect(mocks.restoreLauncher).toHaveBeenCalledWith({ recordId: 'r1', expectedSerial: 'A' })
    await waitFor(() => expect(result.current.restoreResult?.state).toBe('restored'))
  })
})

describe('LauncherWizard refusal rendering', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useDeviceStore.getState().reset()
    useDeviceStore.setState({ activeSerial: 'A' })
    mocks.readLauncherRecovery.mockResolvedValue({ records: [], note: 'local' })
  })

  it('renders the confirmation refusal in the alert line and keeps Apply disabled', async () => {
    mocks.preflightLauncher.mockResolvedValue(
      preflightPayload({
        homeCandidates: [
          stock,
          newLauncher,
          candidate('com.tcl.launcher/.Second', 'com.tcl.launcher'),
        ],
      }),
    )
    render(<LauncherWizardTrigger />)
    fireEvent.click(screen.getByText('Custom launcher (Current+)'))

    // Two candidates share the package, so pick the first button of that package.
    const candidateButtons = await screen.findAllByText('com.tcl.launcher')
    fireEvent.click(candidateButtons[0])
    expect((screen.getByText('3. Apply HOME change') as HTMLButtonElement).disabled).toBe(true)

    fireEvent.click(screen.getByText('2. Confirm this target'))
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/several HOME activities/),
    )
    expect((screen.getByText('3. Apply HOME change') as HTMLButtonElement).disabled).toBe(true)
  })

  it('renders the preflight reason and keeps Apply disabled when the device is unsupported', async () => {
    mocks.preflightLauncher.mockResolvedValue(
      preflightPayload({
        supported: false,
        restorable: false,
        currentHome: '',
        homeCandidates: [],
        reason: 'The supported HOME commands could not be probed: cmd package help probe failed',
      }),
    )
    render(<LauncherWizardTrigger />)
    fireEvent.click(screen.getByText('Custom launcher (Current+)'))

    await waitFor(() => expect(screen.getByText(/could not be probed/)).toBeInTheDocument())
    expect((screen.getByText('3. Apply HOME change') as HTMLButtonElement).disabled).toBe(true)
  })
})

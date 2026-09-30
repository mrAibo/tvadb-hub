import { describe, expect, it } from 'vitest'
import { summarizeTransferBatch } from '../transferResult'
import type { TransferBatchResult } from '../types'

const complete: TransferBatchResult = { operationId: 'op', serial: 'TV-A', items: [], completed: 2, failed: 0, cancelled: 0, skipped: 0 }

describe('transfer batch summary', () => {
  it('uses actual completed count and target', () => {
    const summary = summarizeTransferBatch(complete, 'push')
    expect(summary.complete).toBe(true)
    expect(summary.message).toContain('TV-A: 2 completed')
  })
  it('makes failures and per-file diagnostics explicit', () => {
    const summary = summarizeTransferBatch({ ...complete, completed: 1, failed: 1, items: [{ source: 'bad.bin', destination: '/sdcard/bad.bin', status: 'failed', message: 'permission denied' }] }, 'pull')
    expect(summary.complete).toBe(false)
    expect(summary.message).toContain('1 completed, 1 failed')
    expect(summary.message).toContain('bad.bin: permission denied')
  })
  it('retains completed and unattempted counts after cancellation', () => {
    const summary = summarizeTransferBatch({ ...complete, completed: 1, cancelled: 1, skipped: 4 }, 'push')
    expect(summary.complete).toBe(false)
    expect(summary.cancelled).toBe(true)
    expect(summary.message).toContain('1 completed, 0 failed, 1 cancelled, 4 not attempted')
  })
})

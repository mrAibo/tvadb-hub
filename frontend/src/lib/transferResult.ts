import type { TransferBatchResult } from './types'

export function summarizeTransferBatch(result: TransferBatchResult, direction: 'push' | 'pull') {
  const label = direction === 'push' ? 'Push' : 'Pull'
  const complete = result.failed === 0 && result.cancelled === 0 && result.skipped === 0
  const cancelled = result.cancelled > 0
  const summary = `${label}${cancelled ? ' batch cancelled' : ' batch'} on ${result.serial}: ${result.completed} completed, ${result.failed} failed, ${result.cancelled} cancelled, ${result.skipped} not attempted`
  const failures = result.items.filter(item => item.status === 'failed')
  const detail = failures.slice(0, 3).map(item => `${item.source}: ${item.message}`).join(' · ')
  return { complete, cancelled, message: detail ? `${summary}. ${detail}${failures.length > 3 ? ' · More details in the batch result' : ''}` : summary }
}

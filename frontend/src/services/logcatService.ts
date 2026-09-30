import type { LogcatEntry, LogcatStatusEvent } from '@/lib/types'
import { StartLogcat, StopLogcat, SaveLogcatToFile } from '../../bindings/ADBKit/internal/app/app'
import { Events } from '@wailsio/runtime'

export const LOGCAT_BATCH_EVENT = 'logcat_batch'
export const LOGCAT_STATUS_EVENT = 'logcat_status'

export async function startLogcat(
  serial?: string,
  levels?: string,
  tagFilter?: string,
): Promise<void> {
  return StartLogcat(serial ?? '', levels ?? '', tagFilter ?? '')
}

export async function stopLogcat(serial?: string): Promise<void> {
  return StopLogcat(serial ?? '')
}

export async function saveLogcatToFile(
  content: string,
  defaultFilename: string,
): Promise<void> {
  return SaveLogcatToFile(content, defaultFilename)
}

export function onLogcatBatch(
  callback: (entries: LogcatEntry[]) => void,
): () => void {
  return Events.On(LOGCAT_BATCH_EVENT, (event) => {
    if (Array.isArray(event.data)) {
      callback(event.data as LogcatEntry[])
    }
  })
}

export function onLogcatStatus(
  callback: (event: LogcatStatusEvent) => void,
): () => void {
  return Events.On(LOGCAT_STATUS_EVENT, (event) => {
    callback(event.data)
  })
}

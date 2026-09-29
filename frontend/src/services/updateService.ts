import { Call as WailsCall } from '@wailsio/runtime'
import type { UpdateInfo } from '@/lib/types'

export async function checkForUpdates(): Promise<UpdateInfo> {
  const result = await WailsCall.ByName('ADBKit/internal/app.App.CheckForUpdates')
  return result as UpdateInfo
}

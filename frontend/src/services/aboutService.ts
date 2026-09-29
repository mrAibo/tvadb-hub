import { Call as WailsCall } from '@wailsio/runtime'
import type { AppInfo } from '@/lib/types'

export async function getAppInfo(): Promise<AppInfo> {
  const result = await WailsCall.ByName('ADBKit/internal/app.App.GetAppInfo')
  return result as AppInfo
}

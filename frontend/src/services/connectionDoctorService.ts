import { Call as WailsCall } from '@wailsio/runtime'
import type { ConnectionDoctorReport } from '@/lib/types'

export async function getConnectionDoctorReport(): Promise<ConnectionDoctorReport> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.GetConnectionDoctorReport')
  return raw as ConnectionDoctorReport
}

export async function restartADBServer(): Promise<string> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.RestartADBServer')
  return String(raw ?? 'ADB server restarted.')
}

import type { DeviceInfo, WirelessDiagnosticsReport } from '@/lib/types'

export type TVValidationStatus = 'pass' | 'warning' | 'fail'

export interface TVValidationCheck {
  id: string
  label: string
  status: TVValidationStatus
  detail: string
}

export interface TVValidationReport {
  generatedAt: string
  serial: string
  model?: string
  manufacturer?: string
  androidVersion?: string
  connectionLabel?: string
  ipAddress?: string
  isTV: boolean
  healthy: boolean
  checks: TVValidationCheck[]
  wirelessDiagnostics: WirelessDiagnosticsReport
}

function isNetworkADBSerial(serial: string): boolean {
  const value = serial.trim()
  if (!value) return false
  if (value.startsWith('[')) {
    return /\]:\d+$/.test(value)
  }
  return /:\d+$/.test(value)
}

export function selectorFromDeviceInfo(info: DeviceInfo): string {
  if (info.ipAddress?.trim()) return info.ipAddress.trim()
  const serial = info.serial.trim()
  if (serial.startsWith('[')) {
    const closing = serial.indexOf(']')
    if (closing > 1) return serial.slice(1, closing)
  }
  const separator = serial.lastIndexOf(':')
  if (separator > 0 && /^\d+$/.test(serial.slice(separator + 1))) {
    return serial.slice(0, separator)
  }
  return serial
}

export function buildTVValidationReport(
  info: DeviceInfo,
  wirelessDiagnostics: WirelessDiagnosticsReport,
  generatedAt: string = new Date().toISOString(),
): TVValidationReport {
  const checks: TVValidationCheck[] = []

  checks.push({
    id: 'adb-ready',
    label: 'ADB connection',
    status: info.mode === 'adb' && info.state === 'device' ? 'pass' : 'fail',
    detail:
      info.mode === 'adb' && info.state === 'device'
        ? `${info.serial} is connected and ready.`
        : `Device state is ${info.mode}/${info.state}; a ready ADB connection is required.`,
  })

  checks.push({
    id: 'tv-classification',
    label: 'Android / Google TV classification',
    status: info.isTV ? 'pass' : 'fail',
    detail: info.isTV
      ? `${info.manufacturer || 'Unknown manufacturer'} ${info.model || info.device || info.serial} reports TV characteristics.`
      : `The connected device does not report the Android TV build characteristic (${info.characteristics || 'no characteristics returned'}).`,
  })

  checks.push({
    id: 'wireless-transport',
    label: 'Wireless ADB transport',
    status: isNetworkADBSerial(info.serial) ? 'pass' : 'warning',
    detail: isNetworkADBSerial(info.serial)
      ? `ADB serial ${info.serial} uses a network endpoint.`
      : `ADB serial ${info.serial} does not look like a network endpoint; USB may be active instead.`,
  })

  const mdnsPass = wirelessDiagnostics.services.some(
    (service) => service.kind === 'connect' && service.secure,
  )
  checks.push({
    id: 'secure-mdns',
    label: 'Secure mDNS endpoint',
    status: mdnsPass ? 'pass' : 'warning',
    detail: mdnsPass
      ? 'A secure _adb-tls-connect._tcp service is advertised.'
      : 'No secure _adb-tls-connect._tcp service is currently advertised.',
  })

  const adbStateCheck = wirelessDiagnostics.checks.find(
    (check) => check.id === 'adb_state',
  )
  checks.push({
    id: 'wireless-diagnostics',
    label: 'Wireless diagnostics',
    status: wirelessDiagnostics.healthy
      ? 'pass'
      : adbStateCheck?.status === 'fail'
        ? 'fail'
        : 'warning',
    detail: wirelessDiagnostics.healthy
      ? 'Wireless ADB diagnostics completed without a failed check.'
      : 'Wireless diagnostics reported at least one failed check; inspect the detailed Wireless ADB diagnostics before release.',
  })

  checks.push({
    id: 'device-metadata',
    label: 'TV metadata',
    status: info.model && info.androidVersion ? 'pass' : 'warning',
    detail:
      info.model && info.androidVersion
        ? `${info.model} · Android ${info.androidVersion}${info.securityPatch ? ` · patch ${info.securityPatch}` : ''}`
        : 'Model and Android version were not both available.',
  })

  return {
    generatedAt,
    serial: info.serial,
    model: info.model,
    manufacturer: info.manufacturer,
    androidVersion: info.androidVersion,
    connectionLabel: info.connectionLabel,
    ipAddress: info.ipAddress,
    isTV: Boolean(info.isTV),
    healthy: checks.every((check) => check.status !== 'fail'),
    checks,
    wirelessDiagnostics,
  }
}

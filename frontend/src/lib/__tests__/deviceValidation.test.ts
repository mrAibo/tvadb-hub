import { describe, expect, it } from 'vitest'
import {
  buildDeviceValidationReport,
  selectorFromDeviceInfo,
} from '../deviceValidation'
import type { DeviceInfo, WirelessDiagnosticsReport } from '@/lib/types'

const baseInfo: DeviceInfo = {
  serial: '192.168.178.99:38219',
  state: 'device',
  mode: 'adb',
  model: 'Pixel 9 Pro',
  manufacturer: 'Google',
  characteristics: 'default',
  isTV: false,
  androidVersion: '16',
  ipAddress: '192.168.178.99',
}

const diagnostics: WirelessDiagnosticsReport = {
  adbPath: 'adb',
  adbVersion: 'Version 37.0.1',
  selectedHost: '192.168.178.99',
  selectedEndpoint: '192.168.178.99:38219',
  healthy: true,
  services: [
    {
      instanceName: 'adb-device',
      serviceName: '_adb-tls-connect._tcp',
      kind: 'connect',
      address: '192.168.178.99:38219',
      host: '192.168.178.99',
      port: '38219',
      secure: true,
    },
  ],
  devices: [],
  checks: [
    {
      id: 'adb_state',
      label: 'ADB device state',
      status: 'pass',
      detail: 'connected',
    },
  ],
}

describe('physical Android device validation', () => {
  it('passes a connected general Android device', () => {
    const report = buildDeviceValidationReport(baseInfo, diagnostics, '2026-09-29T12:00:00Z')
    expect(report.healthy).toBe(true)
    expect(report.checks.every((check) => check.status !== 'fail')).toBe(true)
    expect(report.checks.find((check) => check.id === 'device-classification')?.status).toBe('pass')
  })

  it('also passes TV classification without making TV mandatory', () => {
    const report = buildDeviceValidationReport(
      { ...baseInfo, model: 'Google TV Streamer', isTV: true, characteristics: 'tv' },
      diagnostics,
    )
    expect(report.healthy).toBe(true)
    expect(report.isTV).toBe(true)
  })

  it('uses the IP address as the diagnostics selector', () => {
    expect(selectorFromDeviceInfo(baseInfo)).toBe('192.168.178.99')
  })
})

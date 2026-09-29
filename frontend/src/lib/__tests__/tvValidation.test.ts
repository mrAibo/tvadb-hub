import { describe, expect, it } from 'vitest'
import {
  buildTVValidationReport,
  selectorFromDeviceInfo,
} from '../tvValidation'
import type { DeviceInfo, WirelessDiagnosticsReport } from '@/lib/types'

const baseInfo: DeviceInfo = {
  serial: '192.168.178.99:38219',
  state: 'device',
  mode: 'adb',
  model: 'Google TV Streamer',
  manufacturer: 'Google',
  characteristics: 'tv',
  isTV: true,
  androidVersion: '14',
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
      instanceName: 'adb-tv',
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

describe('TV physical validation', () => {
  it('passes a connected TV over secure Wireless ADB', () => {
    const report = buildTVValidationReport(
      baseInfo,
      diagnostics,
      '2026-09-29T12:00:00Z',
    )

    expect(report.healthy).toBe(true)
    expect(report.checks.every((check) => check.status !== 'fail')).toBe(true)
    expect(report.checks.find((check) => check.id === 'secure-mdns')?.status).toBe('pass')
  })

  it('fails when the target is not classified as a TV', () => {
    const report = buildTVValidationReport(
      { ...baseInfo, isTV: false, characteristics: 'default' },
      diagnostics,
    )

    expect(report.healthy).toBe(false)
    expect(report.checks.find((check) => check.id === 'tv-classification')?.status).toBe('fail')
  })

  it('uses the IP address as the diagnostics selector', () => {
    expect(selectorFromDeviceInfo(baseInfo)).toBe('192.168.178.99')
    expect(
      selectorFromDeviceInfo({
        ...baseInfo,
        ipAddress: undefined,
        serial: '[fe80::1234]:39123',
      }),
    ).toBe('fe80::1234')
  })
})

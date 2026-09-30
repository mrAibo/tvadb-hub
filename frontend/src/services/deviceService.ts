import { Call as WailsCall } from '@wailsio/runtime'
import {
  GetDevices,
  GetActiveSerial,
  SetActiveSerial,
  GetDeviceInfo,
  GetDeviceMode,
  RebootDevice,
  ConnectWireless,
  DiscoverWirelessDevices,
  AutoConnectWireless,
  AutoReconnectRememberedWireless,
  PairAndConnectWireless,
  GetWirelessDiagnostics,
  EnableWirelessTCPIP,
  DisconnectWireless,
  GetWirelessHistory,
  GetRememberedWirelessDevices,
  ForgetRememberedWirelessDevice,
  SaveWirelessHistory,
  GetPerformanceSnapshot,
  GetDeviceNicknames,
  SetDeviceNickname,
  ClearDeviceNickname,
} from '../../bindings/ADBKit/internal/app/app'
import type {
  DeviceSummary,
  DeviceInfo,
  DeviceMode,
  PerformanceSnapshot,
  DeviceNicknames,
  WirelessHistoryEntry,
  RememberedWirelessDevice,
  DiscoveredWirelessDevice,
  WirelessConnectResult,
  WirelessPairAndConnectResult,
  WirelessDiagnosticsReport,
  WirelessReconnectReport,
  ScreenshotResult,
  TVRemoteKey,
  TVTextInputResult,
} from '@/lib/types'

export async function getDevices(): Promise<DeviceSummary[]> {
  const raw = await GetDevices()
  return raw as unknown as DeviceSummary[]
}

export async function getActiveSerial(): Promise<string> {
  return GetActiveSerial()
}

export async function setActiveSerial(serial: string): Promise<void> {
  await SetActiveSerial(serial)
}

export async function getDeviceInfo(serial?: string): Promise<DeviceInfo> {
  const raw = await GetDeviceInfo(serial ?? '')
  return raw as unknown as DeviceInfo
}

export async function getDeviceMode(serial?: string): Promise<DeviceMode> {
  const raw = await GetDeviceMode(serial ?? '')
  return raw as unknown as DeviceMode
}

export async function rebootDevice(
  serial?: string,
  mode: string = 'system',
): Promise<string> {
  return RebootDevice(serial ?? '', mode)
}

export async function captureScreenshot(
  localPath: string,
  serial?: string,
): Promise<ScreenshotResult> {
  const result = await WailsCall.ByName(
    'ADBKit/internal/app.App.CaptureScreenshot',
    serial ?? '',
    localPath,
  )
  return result as ScreenshotResult
}

export async function sendTVRemoteKey(
  key: TVRemoteKey,
  serial?: string,
): Promise<string> {
  const result = await WailsCall.ByName(
    'ADBKit/internal/app.App.SendTVRemoteKey',
    serial ?? '',
    key,
  )
  return String(result ?? '')
}

export async function sendTVText(
  text: string,
  serial?: string,
): Promise<TVTextInputResult> {
  const result = await WailsCall.ByName(
    'ADBKit/internal/app.App.SendTVText',
    serial ?? '',
    text,
  )
  return result as TVTextInputResult
}

export async function connectWireless(address: string): Promise<string> {
  return ConnectWireless(address)
}

export async function discoverWirelessDevices(): Promise<DiscoveredWirelessDevice[]> {
  const raw = await DiscoverWirelessDevices()
  return (raw as unknown as DiscoveredWirelessDevice[]) ?? []
}

export async function autoConnectWireless(selector: string): Promise<WirelessConnectResult> {
  const raw = await AutoConnectWireless(selector)
  return raw as unknown as WirelessConnectResult
}

export async function autoReconnectRememberedWireless(): Promise<WirelessReconnectReport> {
  const raw = await AutoReconnectRememberedWireless()
  return raw as unknown as WirelessReconnectReport
}

export async function pairAndConnectWireless(
  selector: string,
  code: string,
): Promise<WirelessPairAndConnectResult> {
  const raw = await PairAndConnectWireless(selector, code)
  return raw as unknown as WirelessPairAndConnectResult
}

export async function getWirelessDiagnostics(
  selector: string = '',
): Promise<WirelessDiagnosticsReport> {
  const raw = await GetWirelessDiagnostics(selector)
  return raw as WirelessDiagnosticsReport
}

export async function enableWirelessTCPIP(
  port: string,
  serial?: string,
): Promise<string> {
  return EnableWirelessTCPIP(port, serial ?? '')
}

export async function disconnectWireless(address: string): Promise<string> {
  return DisconnectWireless(address)
}

export async function getRememberedWirelessDevices(): Promise<RememberedWirelessDevice[]> {
  const raw = await GetRememberedWirelessDevices()
  return (raw as RememberedWirelessDevice[]) ?? []
}

export async function forgetRememberedWirelessDevice(key: string): Promise<void> {
  await ForgetRememberedWirelessDevice(key)
}

export async function getWirelessHistory(): Promise<WirelessHistoryEntry[]> {
  const raw = await GetWirelessHistory()
  return (raw as unknown as WirelessHistoryEntry[]) ?? []
}

export async function saveWirelessHistory(entries: WirelessHistoryEntry[]): Promise<void> {
  await SaveWirelessHistory(entries as never)
}

export async function getPerformanceSnapshot(
  serial?: string,
): Promise<PerformanceSnapshot> {
  const raw = await GetPerformanceSnapshot(serial ?? '')
  return raw as unknown as PerformanceSnapshot
}

export async function getDeviceNicknames(): Promise<DeviceNicknames> {
  const raw = await GetDeviceNicknames()
  return raw as unknown as DeviceNicknames
}

export async function setDeviceNickname(
  serial: string,
  nickname: string,
): Promise<void> {
  await SetDeviceNickname(serial, nickname)
}

export async function clearDeviceNickname(serial: string): Promise<void> {
  await ClearDeviceNickname(serial)
}

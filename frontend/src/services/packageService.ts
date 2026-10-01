import { Call as WailsCall } from '@wailsio/runtime'
import {
  ListPackagesForDevice,
  InstallPackageWithModeForDevice,
  InstallPackagesWithModeForDevice,
  UninstallPackageForDevice,
  UninstallMultiplePackagesForDevice,
  EnablePackageForDevice,
  EnableMultiplePackagesForDevice,
  DisablePackageForDevice,
  DisableMultiplePackagesForDevice,
  ClearPackageDataForDevice,
  PullPackageApkForDevice,
  LaunchPackageForDevice,
  ForceStopPackageForDevice,
  GetPackageDetailsForDevice,
  SelectApkFile,
} from '../../bindings/ADBKit/internal/app/app'
import type {
  PackageDetails,
  PackageFilter,
  PackageInfo,
  PackageInstallMode,
} from '@/lib/types'

// Every package operation is bound to a caller-confirmed serial. The frontend never
// falls back to the mutable global selection: an empty target is refused here, and
// the approved Go twins refuse it again before running any command.
function confirmedSerial(operation: string, serial: string): string {
  const trimmed = serial.trim()
  if (!trimmed) {
    throw new Error(`${operation}: no confirmed device target is selected`)
  }
  return trimmed
}

export async function listPackages(
  serial: string,
  filter: PackageFilter,
): Promise<PackageInfo[]> {
  const raw = await ListPackagesForDevice(confirmedSerial('list_packages', serial), filter)
  return raw as unknown as PackageInfo[]
}

export async function installPackage(
  serial: string,
  filePath: string,
  mode: PackageInstallMode = 'replace',
): Promise<string> {
  const result = await InstallPackageWithModeForDevice(
    confirmedSerial('install_package', serial),
    filePath,
    mode,
  )
  return String(result ?? '')
}

export async function installPackages(
  serial: string,
  filePaths: string[],
  mode: PackageInstallMode = 'replace',
): Promise<string> {
  const result = await InstallPackagesWithModeForDevice(
    confirmedSerial('install_multiple_packages', serial),
    filePaths,
    mode,
  )
  return String(result ?? '')
}

export async function uninstallPackage(serial: string, packageName: string): Promise<string> {
  return UninstallPackageForDevice(confirmedSerial('uninstall_package', serial), packageName)
}

export async function uninstallMultiplePackages(
  serial: string,
  packageNames: string[],
): Promise<string> {
  return UninstallMultiplePackagesForDevice(
    confirmedSerial('uninstall_packages', serial),
    packageNames,
  )
}

export async function enablePackage(serial: string, packageName: string): Promise<string> {
  return EnablePackageForDevice(confirmedSerial('enable_package', serial), packageName)
}

export async function enableMultiplePackages(
  serial: string,
  packageNames: string[],
): Promise<string> {
  return EnableMultiplePackagesForDevice(
    confirmedSerial('enable_packages', serial),
    packageNames,
  )
}

export async function disablePackage(serial: string, packageName: string): Promise<string> {
  return DisablePackageForDevice(confirmedSerial('disable_package', serial), packageName)
}

export async function disableMultiplePackages(
  serial: string,
  packageNames: string[],
): Promise<string> {
  return DisableMultiplePackagesForDevice(
    confirmedSerial('disable_packages', serial),
    packageNames,
  )
}

export async function clearPackageData(serial: string, packageName: string): Promise<string> {
  return ClearPackageDataForDevice(confirmedSerial('clear_package_data', serial), packageName)
}

export async function pullPackageApk(serial: string, packageName: string): Promise<string> {
  return PullPackageApkForDevice(confirmedSerial('pull_package_apk', serial), packageName)
}

export async function launchPackage(serial: string, packageName: string): Promise<string> {
  return LaunchPackageForDevice(confirmedSerial('launch_package', serial), packageName)
}

export async function forceStopPackage(serial: string, packageName: string): Promise<string> {
  return ForceStopPackageForDevice(confirmedSerial('force_stop_package', serial), packageName)
}

export async function getPackageDetails(
  serial: string,
  packageName: string,
): Promise<PackageDetails> {
  const raw = await GetPackageDetailsForDevice(
    confirmedSerial('get_package_details', serial),
    packageName,
  )
  return raw as unknown as PackageDetails
}

export async function selectApkFile(): Promise<string> {
  return SelectApkFile()
}

export async function selectApkFiles(): Promise<string[]> {
  const result = await WailsCall.ByName('ADBKit/internal/app.App.SelectApkFiles')
  return (result as string[] | null) ?? []
}

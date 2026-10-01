import { Call as WailsCall } from '@wailsio/runtime'
import {
  ListFilesForDevice,
  GetDirectorySizeForDevice,
  PullFileForDevice,
  PushFileForDevice,
  DeleteFileForDevice,
  DeleteMultipleFilesForDevice,
  CreateDirectoryForDevice,
  RenameFileForDevice,
  SelectFile,
  SelectSavePath,
  SelectDirectory,
  SelectMultipleFiles,
  CancelFileTransfer,
  GetStorageInfoForDevice,
  ListSdCardsForDevice,
  UnblockPathForDevice,
} from '../../bindings/ADBKit/internal/app/app'
import { Events } from '@wailsio/runtime'
import type { FileEntry, HostFileSystemInfo, StorageInfo, SdCard, TransferVerificationStatus, TransferBatchResult, UnblockResult } from '@/lib/types'

export const FILE_TRANSFER_PROGRESS_EVENT = 'file_transfer_progress'

export interface FileTransferProgress {
  operationId?: string
  serial?: string
  fileName: string
  direction: 'push' | 'pull'
  percent: number
  verification?: TransferVerificationStatus
  verificationDetail?: string
}

export function onFileTransferProgress(
  callback: (progress: FileTransferProgress) => void,
): () => void {
  return Events.On(FILE_TRANSFER_PROGRESS_EVENT, (event) => {
    callback(event.data)
  })
}

// Listing, sizing and every destructive file action are bound to a caller-confirmed
// serial. The frontend never falls back to the mutable global selection: an empty
// target is refused here, and the approved Go twins refuse it again before running
// any command. Transfers keep running on the root service (see the *Detailed and
// *ForDevice helpers below) so one owned transfer is never retargeted.
function confirmedSerial(operation: string, serial: string): string {
  const trimmed = serial.trim()
  if (!trimmed) {
    throw new Error(`${operation}: no confirmed device target is selected`)
  }
  return trimmed
}

export async function listFiles(
  serial: string,
  remotePath: string,
  showHidden: boolean,
): Promise<FileEntry[]> {
  const raw = await ListFilesForDevice(confirmedSerial('list_files', serial), remotePath, showHidden)
  return raw as unknown as FileEntry[]
}

export async function getDirectorySize(serial: string, remotePath: string): Promise<string> {
  return GetDirectorySizeForDevice(confirmedSerial('get_directory_size', serial), remotePath)
}

// Storage info of the confirmed device only. The legacy serial-less binding is no
// longer reachable from the frontend: an unanswered target is refused here.
export async function getStorageInfoForDevice(serial: string): Promise<StorageInfo> {
  const raw = await GetStorageInfoForDevice(confirmedSerial('get_storage_info', serial))
  return raw as unknown as StorageInfo
}

export async function selectFile(): Promise<string> {
  return SelectFile()
}

export async function selectSaveFile(defaultFilename: string): Promise<string> {
  return SelectSavePath(defaultFilename)
}

export async function selectMultipleFiles(): Promise<string[]> {
  const raw = await SelectMultipleFiles()
  return raw as unknown as string[]
}

export async function selectDirectory(): Promise<string> {
  return SelectDirectory()
}

export async function pullFile(
  serial: string,
  remotePath: string,
  localPath: string,
): Promise<string> {
  return PullFileForDevice(confirmedSerial('pull_file', serial), remotePath, localPath)
}

export async function pushFile(
  serial: string,
  localPath: string,
  remotePath: string,
): Promise<string> {
  return PushFileForDevice(confirmedSerial('push_file', serial), localPath, remotePath)
}

export async function pushMultipleFilesDetailed(serial: string, paths: string[], destination: string): Promise<TransferBatchResult> {
  return WailsCall.ByName('ADBKit/internal/app.App.PushMultipleFilesDetailed', confirmedSerial('push_multiple_files', serial), paths, destination) as Promise<TransferBatchResult>
}

export async function pullMultipleFilesDetailed(serial: string, paths: string[], destination: string): Promise<TransferBatchResult> {
  return WailsCall.ByName('ADBKit/internal/app.App.PullMultipleFilesDetailed', confirmedSerial('pull_multiple_files', serial), paths, destination) as Promise<TransferBatchResult>
}

export async function deleteFile(serial: string, remotePath: string): Promise<string> {
  return DeleteFileForDevice(confirmedSerial('delete_file', serial), remotePath)
}

export async function deleteMultipleFiles(serial: string, remotePaths: string[]): Promise<string> {
  return DeleteMultipleFilesForDevice(
    confirmedSerial('delete_multiple_files', serial),
    remotePaths,
  )
}

export async function createDirectory(serial: string, remotePath: string): Promise<string> {
  return CreateDirectoryForDevice(confirmedSerial('create_directory', serial), remotePath)
}

export async function renameFile(
  serial: string,
  oldRemotePath: string,
  newRemotePath: string,
): Promise<string> {
  return RenameFileForDevice(
    confirmedSerial('rename_file', serial),
    oldRemotePath,
    newRemotePath,
  )
}

// Cancelling is deliberately not device-bound: it addresses the single root-owned
// transfer by its operation id, so a device switch cannot cancel another target.
export function cancelFileTransfer(operationId?: string): void {
  if (operationId) void WailsCall.ByName('ADBKit/internal/app.App.CancelFileTransferFor', operationId)
  else CancelFileTransfer()
}

// listSdCardsForDevice reads the mounted volumes of the confirmed device only.
export async function listSdCardsForDevice(serial: string): Promise<SdCard[]> {
  const raw = await ListSdCardsForDevice(confirmedSerial('list_sd_cards', serial))
  return (raw as unknown as SdCard[] | null) ?? []
}

// unblockPathForDevice returns honest guidance for recovering access to a blocked
// path on the confirmed device. No fake bypass — it tells the user exactly what they
// need to do on that device.
export async function unblockPathForDevice(
  serial: string,
  remotePath: string,
): Promise<UnblockResult> {
  const raw = await UnblockPathForDevice(
    confirmedSerial('unblock_path', serial),
    remotePath,
  )
  return raw as UnblockResult
}


export async function getHostFileSystemInfo(): Promise<HostFileSystemInfo> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.GetHostFileSystemInfo')
  return raw as HostFileSystemInfo
}

export async function listLocalFiles(localPath: string, showHidden: boolean): Promise<FileEntry[]> {
  const raw = await WailsCall.ByName(
    'ADBKit/internal/app.App.ListLocalFiles',
    localPath,
    showHidden,
  )
  return (raw as FileEntry[] | null) ?? []
}

export async function getLocalParentPath(localPath: string): Promise<string> {
  const raw = await WailsCall.ByName('ADBKit/internal/app.App.GetLocalParentPath', localPath)
  return String(raw ?? localPath)
}

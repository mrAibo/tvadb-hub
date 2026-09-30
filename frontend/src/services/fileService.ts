import { Call as WailsCall } from '@wailsio/runtime'
import {
  ListFiles,
  GetDirectorySize,
  GetStorageInfo,
  PullFile,
  PullMultipleFiles,
  PushFile,
  PushMultipleFiles,
  DeleteFile,
  DeleteMultipleFiles,
  CreateDirectory,
  RenameFile,
  SelectFile,
  SelectSavePath,
  SelectDirectory,
  SelectMultipleFiles,
  CancelFileTransfer,
  ListSdCards,
  UnblockPath,
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

export async function listFiles(remotePath: string, showHidden: boolean): Promise<FileEntry[]> {
  const raw = await ListFiles(remotePath, showHidden)
  return raw as unknown as FileEntry[]
}

export async function getDirectorySize(remotePath: string): Promise<string> {
  return GetDirectorySize(remotePath)
}

export async function getStorageInfo(): Promise<StorageInfo> {
  const raw = await GetStorageInfo()
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

export async function pullFile(remotePath: string, localPath: string, expectedSerial?: string): Promise<string> {
  if (expectedSerial !== undefined) return WailsCall.ByName('ADBKit/internal/app.App.PullFileForDevice', expectedSerial, remotePath, localPath) as Promise<string>
  return PullFile(remotePath, localPath)
}

export async function pullMultipleFiles(remotePaths: string[], localDirectory: string): Promise<string> {
  return PullMultipleFiles(remotePaths, localDirectory)
}

export async function pushFile(localPath: string, remotePath: string, expectedSerial?: string): Promise<string> {
  if (expectedSerial !== undefined) return WailsCall.ByName('ADBKit/internal/app.App.PushFileForDevice', expectedSerial, localPath, remotePath) as Promise<string>
  return PushFile(localPath, remotePath)
}

export async function pushMultipleFiles(localPaths: string[], remoteDirectory: string): Promise<string> {
  return PushMultipleFiles(localPaths, remoteDirectory)
}

export async function pushMultipleFilesDetailed(serial: string, paths: string[], destination: string): Promise<TransferBatchResult> {
  return WailsCall.ByName('ADBKit/internal/app.App.PushMultipleFilesDetailed', serial, paths, destination) as Promise<TransferBatchResult>
}

export async function pullMultipleFilesDetailed(serial: string, paths: string[], destination: string): Promise<TransferBatchResult> {
  return WailsCall.ByName('ADBKit/internal/app.App.PullMultipleFilesDetailed', serial, paths, destination) as Promise<TransferBatchResult>
}

export async function deleteFile(remotePath: string): Promise<string> {
  return DeleteFile(remotePath)
}

export async function deleteMultipleFiles(remotePaths: string[]): Promise<string> {
  return DeleteMultipleFiles(remotePaths)
}

export async function createDirectory(remotePath: string): Promise<string> {
  return CreateDirectory(remotePath)
}

export async function renameFile(oldRemotePath: string, newRemotePath: string): Promise<string> {
  return RenameFile(oldRemotePath, newRemotePath)
}

export function cancelFileTransfer(operationId?: string): void {
  if (operationId) void WailsCall.ByName('ADBKit/internal/app.App.CancelFileTransferFor', operationId)
  else CancelFileTransfer()
}

// listSdCards calls adb shell sm list-volumes and returns mounted volumes.
export async function listSdCards(): Promise<SdCard[]> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const raw = await (ListSdCards as (...args: unknown[]) => Promise<unknown>)('')
  return raw as SdCard[]
}

// unblockPath returns honest guidance for recovering access to a blocked path.
// No fake bypass — tells the user exactly what they need to do on their device.
export async function unblockPath(remotePath: string): Promise<UnblockResult> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const raw = await (UnblockPath as (...args: unknown[]) => Promise<unknown>)(remotePath)
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

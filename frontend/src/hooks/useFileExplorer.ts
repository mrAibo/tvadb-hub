import { useCallback, useEffect, useMemo, useRef } from 'react'
import { toast } from 'sonner'
import {
  listFiles,
  getDirectorySize,
  pullFile,
  pullMultipleFilesDetailed,
  pushFile,
  pushMultipleFilesDetailed,
  deleteMultipleFiles,
  createDirectory,
  renameFile,
  selectFile,
  selectSaveFile,
  selectMultipleFiles,
  selectDirectory,
  onFileTransferProgress,
  cancelFileTransfer,
} from '@/services/fileService'
import { useDeviceStore } from '@/stores/useDeviceStore'
import { useFileExplorerStore } from '@/stores/useFileExplorerStore'
import { summarizeTransferBatch } from '@/lib/transferResult'
import type {
  DeviceSummary,
  FileEntry,
  FileSortField,
  FileSortDirection,
  TransferVerificationSummary,
} from '@/lib/types'


function getErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

function isCancelledMessage(message: string): boolean {
  const normalized = message.toLowerCase()
  return normalized.includes('cancelled') || normalized.includes('canceled')
}

function canStartTransfer(): boolean {
  if (useFileExplorerStore.getState().transferProgress?.active) {
    toast.error('Another file transfer is already active')
    return false
  }
  return true
}

function normalizePath(value: string): string {
  const trimmed = value.trim()
  if (trimmed === '' || trimmed === '/') return '/sdcard'
  return trimmed.replace(/\/+$/, '') || '/sdcard'
}

// The cache key is a collision-safe tuple of everything that identifies the
// response: the confirmed serial, the normalized path and the hidden flag. A
// response captured on one device can therefore never be served for another.
function cacheKey(serial: string, path: string, showHidden: boolean): string {
  return JSON.stringify([serial, normalizePath(path), showHidden])
}

function compareNames(a: FileEntry, b: FileEntry) {
  return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
}

function compareDates(a: FileEntry, b: FileEntry) {
  return a.modifiedAt.localeCompare(b.modifiedAt)
}

function parseHumanSize(value: string): number | null {
  const normalized = value.trim().toUpperCase()
  const match = normalized.match(/^(\d+(?:\.\d+)?)\s*([KMGT]?B?|)$/)
  if (!match) return null
  const amount = Number.parseFloat(match[1] ?? '0')
  const unit = match[2] || 'B'
  const multipliers: Record<string, number> = {
    B: 1, K: 1024, KB: 1024,
    M: 1024 ** 2, MB: 1024 ** 2,
    G: 1024 ** 3, GB: 1024 ** 3,
    T: 1024 ** 4, TB: 1024 ** 4,
  }
  return amount * (multipliers[unit] ?? 1)
}

function compareSizes(a: FileEntry, b: FileEntry) {
  const sizeA = parseHumanSize(a.sizeHuman) ?? a.size
  const sizeB = parseHumanSize(b.sizeHuman) ?? b.size
  return sizeA - sizeB
}

function sortFiles(files: FileEntry[], field: FileSortField, dir: FileSortDirection) {
  return [...files].sort((a, b) => {
    if (a.type === 'directory' && b.type !== 'directory') return -1
    if (a.type !== 'directory' && b.type === 'directory') return 1
    const cmp =
      field === 'size' ? compareSizes(a, b)
        : field === 'date' ? compareDates(a, b)
          : compareNames(a, b)
    return dir === 'desc' ? cmp * -1 : cmp
  })
}

function buildBreadcrumbs(currentPath: string) {
  const normalized = normalizePath(currentPath)
  if (normalized === '/') return [{ label: '/', path: '/' }]
  const segments = normalized.split('/').filter(Boolean)
  const crumbs = [{ label: '/', path: '/' }]
  let next = ''
  for (const seg of segments) {
    next += `/${seg}`
    crumbs.push({ label: seg, path: next })
  }
  return crumbs
}

function isReadyAdb(d: DeviceSummary): boolean {
  return d.mode === 'adb' && d.state === 'device'
}

export function useFileExplorer() {
  const { devices, activeSerial } = useDeviceStore()
  const store = useFileExplorerStore()
  const requestIdRef = useRef(0)

  const hasReadyAdb = useMemo(() => devices.some(isReadyAdb), [devices])
  const hasReadyActive = useMemo(
    () => devices.some((d) => d.serial === activeSerial && isReadyAdb(d)),
    [activeSerial, devices],
  )

  // Mutations re-read the live confirmed serial and refuse to act when the listing
  // was captured for a different device, so a stale row or dialog cannot reach
  // another target.
  const requireConfirmedSerial = useCallback((operation: string): string | null => {
    const live = useDeviceStore.getState().activeSerial
    if (!live) {
      toast.error(`${operation}: no device is selected`)
      return null
    }
    const listing = useFileExplorerStore.getState().listingSerial
    if (listing !== live) {
      toast.error(`${operation}: the device changed — refresh the file list and try again`)
      return null
    }
    return live
  }, [])

  useEffect(() => {
    const unsub = onFileTransferProgress((progress) => {
      const current = useFileExplorerStore.getState().transferProgress
      if (!current?.active) return
      if (progress.serial && progress.serial !== useDeviceStore.getState().activeSerial) return
      if (current.operationId && progress.operationId && current.operationId !== progress.operationId) return

      useFileExplorerStore.getState().setTransferProgress({
        operationId: progress.operationId ?? current.operationId,
        serial: progress.serial ?? current.serial,
        fileName: progress.fileName || current.fileName,
        direction: progress.direction,
        percent: progress.percent,
        active: true,
        verification: progress.verification ?? current.verification,
        verificationDetail: progress.verificationDetail ?? current.verificationDetail,
      })

      if (progress.verification && progress.verification !== 'verifying') {
        useFileExplorerStore.getState().setLastTransferVerification({
          fileName: progress.fileName || current.fileName,
          direction: progress.direction,
          status: progress.verification as TransferVerificationSummary['status'],
          detail: progress.verificationDetail,
        })
      }
    })
    return unsub
  }, [])


  const loadFiles = useCallback(
    async (nextPath: string, opts: { background?: boolean; force?: boolean } = {}) => {
      const serial = activeSerial
      if (!hasReadyAdb || !hasReadyActive || !serial) {
        store.clearMachineBoundState()
        store.setError('No ADB device connected')
        store.setLoading(false)
        store.setRefreshing(false)
        return
      }

      const normalized = normalizePath(nextPath)
      const key = cacheKey(serial, normalized, store.showHidden)
      const cached = useFileExplorerStore.getState().fileCache[key]
      const useCache = Boolean(cached) && !opts.force
      const reqId = requestIdRef.current + 1
      requestIdRef.current = reqId

      if (useCache && cached) {
        store.setFiles(cached.files)
        store.clearSelection()
        store.setLastUpdatedAt(cached.lastUpdatedAt)
        store.setRefreshing(true)
      } else if (opts.background) {
        store.setRefreshing(true)
      } else {
        store.setFiles([])
        store.setLoading(true)
      }

      store.setError(null)

      try {
        const nextFiles = await listFiles(serial, normalized, store.showHidden)
        if (requestIdRef.current !== reqId) return
        // A reply for another serial (including A-B-A) must never render.
        if (useDeviceStore.getState().activeSerial !== serial) return
        const now = Date.now()
        store.setFiles(nextFiles)
        store.setCachedFiles(key, nextFiles, now)
        store.setListingSerial(serial)
        store.clearSelection()
        store.setLastUpdatedAt(now)

      } catch (err) {
        if (requestIdRef.current !== reqId) return
        if (useDeviceStore.getState().activeSerial !== serial) return
        store.setError(getErrorMessage(err, 'Failed to load files'))
        store.setFiles([])
        store.clearSelection()
        store.setLastUpdatedAt(null)
      } finally {
        if (requestIdRef.current === reqId) {
          store.setLoading(false)
          store.setRefreshing(false)
        }
      }
    },
    [hasReadyAdb, hasReadyActive, activeSerial, store.showHidden],
  )

  // Switching the confirmed device drops only the machine-bound listing, cache,
  // dialogs and selection. An owned transfer (progress, verification evidence and
  // its operation id) is deliberately preserved, and nothing is cancelled here.
  useEffect(() => {
    store.clearMachineBoundState()
  }, [activeSerial])

  // One listing request per (serial, path, hidden) commit: the previous pair of
  // effects could fire two loads for the same path.
  useEffect(() => {
    void loadFiles(useFileExplorerStore.getState().currentPath)
  }, [activeSerial, store.currentPath, store.showHidden, loadFiles])

  const visibleFiles = useMemo(() => {
    const term = store.searchTerm.trim().toLowerCase()
    const filtered = store.files.filter((f) =>
      term === '' ? true : f.name.toLowerCase().includes(term),
    )
    return sortFiles(filtered, store.sortField, store.sortDirection)
  }, [store.files, store.searchTerm, store.sortField, store.sortDirection])

  const visiblePaths = useMemo(() => visibleFiles.map((f) => f.path), [visibleFiles])
  const breadcrumbs = useMemo(() => buildBreadcrumbs(store.currentPath), [store.currentPath])

  const folderCount = store.files.filter((f) => f.type === 'directory').length
  const fileCount = store.files.length - folderCount

  async function refreshFiles() {
    await loadFiles(store.currentPath, { background: true, force: true })
  }

  function navigateTo(nextPath: string) {
    const normalized = normalizePath(nextPath)
    if (normalized === store.currentPath) {
      void loadFiles(normalized, { background: true, force: true })
      return
    }
    store.setCurrentPath(normalized)
  }

  function navigateUp() {
    const normalized = normalizePath(store.currentPath)
    if (normalized === '/') return
    const parent = normalized.slice(0, normalized.lastIndexOf('/')) || '/'
    store.setCurrentPath(normalizePath(parent))
  }

  function openDirectory(file: FileEntry) {
    if (file.type !== 'directory') return
    store.setCurrentPath(normalizePath(file.path))
  }

  async function chooseLocalFile() {
    try { return await selectFile() } catch { return '' }
  }

  async function chooseLocalSaveFile(defaultFilename: string) {
    try { return await selectSaveFile(defaultFilename) } catch { return '' }
  }

  async function chooseMultipleLocalFiles() {
    try { return await selectMultipleFiles() } catch { return [] }
  }

  async function chooseLocalDirectory() {
    try { return await selectDirectory() } catch { return '' }
  }

  async function pullSingleFile(remotePath: string, localPath: string) {
    if (!localPath.trim()) { toast.error('Destination path is required'); return false }
    const targetSerial = useDeviceStore.getState().activeSerial
    if (!targetSerial) { toast.error('Select an ADB device before transferring'); return false }
    if (!canStartTransfer()) return false
    const name = remotePath.split('/').pop() ?? remotePath
    store.setBusyFilePath(remotePath)
    store.setError(null)
    store.setTransferProgress({ serial: targetSerial, fileName: name, direction: 'pull', percent: 0, active: true })
    try {
      const message = await pullFile(targetSerial, remotePath, localPath)
      toast.success(`${targetSerial}: ${message}`)
      return true
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to pull file')
      if (isCancelledMessage(msg)) {
        toast.info('Pull cancelled — partial data may remain on your computer')
      } else {
        if (useDeviceStore.getState().activeSerial === targetSerial) store.setError(msg)
        toast.error(msg)
      }
      return false
    } finally {
      store.setBusyFilePath(null)
      store.setTransferProgress(null)
    }
  }

  async function pushSingleFile(localPath: string, remotePath: string) {
    if (!localPath.trim()) { toast.error('Local file path is required'); return false }
    const targetSerial = useDeviceStore.getState().activeSerial
    if (!targetSerial) { toast.error('Select an ADB device before transferring'); return false }
    if (!canStartTransfer()) return false
    const name = localPath.split(/[/\\]/).pop() ?? localPath
    store.setBusyFilePath(remotePath)
    store.setError(null)
    store.setTransferProgress({ serial: targetSerial, fileName: name, direction: 'push', percent: 0, active: true })
    try {
      const message = await pushFile(targetSerial, localPath, remotePath)
      toast.success(`${targetSerial}: ${message}`)
      if (useDeviceStore.getState().activeSerial === targetSerial) await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to push file')
      if (isCancelledMessage(msg)) {
        toast.info('Push cancelled — partial data may remain on the device')
      } else {
        if (useDeviceStore.getState().activeSerial === targetSerial) store.setError(msg)
        toast.error(msg)
      }
      return false
    } finally {
      store.setBusyFilePath(null)
      store.setTransferProgress(null)
    }
  }

  async function removeFile(remotePath: string) {
    const name = remotePath.split('/').pop() ?? remotePath
    store.setBusyFilePath(remotePath)
    store.setError(null)
    const serial = requireConfirmedSerial('Delete file')
    if (!serial) return false
    try {
      await toast.promise(deleteMultipleFiles(serial, [remotePath]), {
        loading: `Deleting ${name}...`,
        success: (msg) => msg,
        error: (err) => getErrorMessage(err, 'Failed to delete file'),
      })
      await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      store.setError(getErrorMessage(err, 'Failed to delete file'))
      return false
    } finally {
      store.setBusyFilePath(null)
    }
  }

  async function createFolder(folderName: string) {
    const trimmed = folderName.trim()
    if (!trimmed) { toast.error('Folder name is required'); return false }
    store.setBusyFilePath(store.currentPath)
    store.setError(null)
    try {
      const serial = requireConfirmedSerial('Create folder')
      if (!serial) return false
      const msg = await createDirectory(serial, `${normalizePath(store.currentPath)}/${trimmed}`)
      toast.success(msg)
      await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to create folder')
      store.setError(msg)
      toast.error(msg)
      return false
    } finally {
      store.setBusyFilePath(null)
    }
  }

  async function renameExistingFile(oldPath: string, newPath: string) {
    const trimmed = newPath.trim()
    if (!trimmed) { toast.error('New file path is required'); return false }
    store.setBusyFilePath(oldPath)
    store.setError(null)
    try {
      const serial = requireConfirmedSerial('Rename file')
      if (!serial) return false
      const msg = await renameFile(serial, oldPath, trimmed)
      toast.success(msg)
      await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to rename file')
      store.setError(msg)
      toast.error(msg)
      return false
    } finally {
      store.setBusyFilePath(null)
    }
  }

  async function moveFile(sourcePath: string, targetDirectory: string) {
    const name = sourcePath.split('/').pop() ?? sourcePath
    const newPath = `${targetDirectory.replace(/\/$/, '')}/${name}`
    if (newPath === sourcePath) return false

    store.setBusyFilePath(sourcePath)
    store.setError(null)
    try {
      const serial = requireConfirmedSerial('Move file')
      if (!serial) return false
      await toast.promise(renameFile(serial, sourcePath, newPath), {
        loading: `Moving ${name}...`,
        success: (msg) => msg,
        error: (err) => getErrorMessage(err, 'Failed to move file'),
      })
      await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      store.setError(getErrorMessage(err, 'Failed to move file'))
      return false
    } finally {
      store.setBusyFilePath(null)
    }
  }

  async function pullSelectedFiles(localDir: string) {
    const trimmed = localDir.trim()
    if (store.selectedFiles.length === 0) { toast.error('No files selected'); return false }
    if (!trimmed) { toast.error('Destination directory is required'); return false }
    const targetSerial = useDeviceStore.getState().activeSerial
    if (!targetSerial) { toast.error('Select an ADB device before transferring'); return false }
    if (!canStartTransfer()) return false
    store.setBusyBatchAction('pull')
    store.setError(null)
    store.setTransferProgress({ serial: targetSerial, fileName: `${store.selectedFiles.length} file(s)`, direction: 'pull', percent: 0, active: true })
    try {
      const result = await pullMultipleFilesDetailed(targetSerial, [...store.selectedFiles], trimmed)
      store.setLastTransferBatch(result)
      const summary = summarizeTransferBatch(result, 'pull')
      if (summary.complete) toast.success(summary.message)
      else if (summary.cancelled) toast.info(`${summary.message}. Partial data may remain on your computer`)
      else toast.error(summary.message)
      if (useDeviceStore.getState().activeSerial === targetSerial) {
        store.setSelectedFiles(result.items.filter(item => item.status !== 'success').map(item => item.source))
        if (!summary.complete && !summary.cancelled) store.setError(summary.message)
      }
      return summary.complete
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to pull files')
      if (isCancelledMessage(msg)) {
        toast.info('Pull batch cancelled — partial data may remain on your computer')
      } else {
        if (useDeviceStore.getState().activeSerial === targetSerial) store.setError(msg)
        toast.error(msg)
      }
      return false
    } finally {
      store.setBusyBatchAction(null)
      store.setTransferProgress(null)
    }
  }

  async function deleteSelectedFiles() {
    if (store.selectedFiles.length === 0) { toast.error('No files selected'); return false }
    store.setBusyBatchAction('delete')
    store.setError(null)
    try {
      const serial = requireConfirmedSerial('Delete files')
      if (!serial) return false
      await toast.promise(deleteMultipleFiles(serial, store.selectedFiles), {
        loading: `Deleting ${store.selectedFiles.length} file(s)...`,
        success: (msg) => msg,
        error: (err) => getErrorMessage(err, 'Failed to delete files'),
      })
      store.clearSelection()
      await loadFiles(store.currentPath, { background: true, force: true })
      return true
    } catch (err) {
      store.setError(getErrorMessage(err, 'Failed to delete files'))
      return false
    } finally {
      store.setBusyBatchAction(null)
    }
  }

  async function pushMultipleToCurrentDir(localPaths: string[]) {
    if (localPaths.length === 0) return false
    const targetSerial = useDeviceStore.getState().activeSerial
    if (!targetSerial) { toast.error('Select an ADB device before transferring'); return false }
    if (!canStartTransfer()) return false
    const remoteDir = normalizePath(store.currentPath)
    store.setBusyBatchAction('push')
    store.setError(null)
    store.setTransferProgress({ serial: targetSerial, fileName: `${localPaths.length} file(s)`, direction: 'push', percent: 0, active: true })
    try {
      const result = await pushMultipleFilesDetailed(targetSerial, [...localPaths], remoteDir)
      store.setLastTransferBatch(result)
      const summary = summarizeTransferBatch(result, 'push')
      if (summary.complete) toast.success(summary.message)
      else if (summary.cancelled) toast.info(`${summary.message}. Partial data may remain on the device`)
      else toast.error(summary.message)
      if (useDeviceStore.getState().activeSerial === targetSerial) {
        await loadFiles(store.currentPath, { background: true, force: true })
        if (useDeviceStore.getState().activeSerial === targetSerial && !summary.complete && !summary.cancelled) store.setError(summary.message)
      }
      return summary.complete
    } catch (err) {
      const msg = getErrorMessage(err, 'Failed to push files')
      if (isCancelledMessage(msg)) {
        toast.info('Push batch cancelled — partial data may remain on the device')
      } else {
        if (useDeviceStore.getState().activeSerial === targetSerial) store.setError(msg)
        toast.error(msg)
      }
      return false
    } finally {
      store.setBusyBatchAction(null)
      store.setTransferProgress(null)
    }
  }

  function setSort(field: FileSortField) {
    if (store.sortField === field) {
      store.setSortDirection(store.sortDirection === 'asc' ? 'desc' : 'asc')
      return
    }
    store.setSortField(field)
    store.setSortDirection('asc')
  }

  function cancelTransfer() {
    cancelFileTransfer(useFileExplorerStore.getState().transferProgress?.operationId)
  }

  function dismissError() { store.setError(null) }

  function openPullDialog(file: FileEntry) {
    store.setDialogTargetFile(file)
    store.setIsPullDialogOpen(true)
  }
  function openPushDialog(file: FileEntry) {
    store.setDialogTargetFile(file)
    store.setIsPushDialogOpen(true)
  }
  function openRenameDialog(file: FileEntry) {
    store.setDialogTargetFile(file)
    store.setIsRenameDialogOpen(true)
  }
  function openDeleteDialog(file: FileEntry) {
    store.setDialogTargetFile(file)
    store.setIsDeleteDialogOpen(true)
  }
  function openNewFolderDialog() { store.setIsNewFolderDialogOpen(true) }
  function openPushFolderDialog() { store.setIsPushFolderDialogOpen(true) }
  function openBatchPullDialog() { store.setIsBatchPullDialogOpen(true) }
  function openBatchDeleteDialog() { store.setIsBatchDeleteDialogOpen(true) }
  function openMoveDialog(file: FileEntry) {
    store.setDialogTargetFile(file)
    store.setIsMoveDialogOpen(true)
  }

  async function getSizeForFile(file: FileEntry) {
    if (file.type !== 'directory') return
    const serial = requireConfirmedSerial('Directory size')
    if (!serial) return
    const key = cacheKey(serial, store.currentPath, store.showHidden)
    try {
      const size = await getDirectorySize(serial, file.path)
      if (useDeviceStore.getState().activeSerial === serial) store.updateFileSize(file.path, size, key)
    } catch {
      if (useDeviceStore.getState().activeSerial === serial) store.updateFileSize(file.path, '--', key)
    }
  }

  async function handlePullConfirm(localPath: string) {
    if (!store.dialogTargetFile) return
    const success = await pullSingleFile(store.dialogTargetFile.path, localPath)
    if (success) store.setIsPullDialogOpen(false)
  }
  async function handlePushConfirm(localPath: string) {
    if (!store.dialogTargetFile) return
    const success = await pushSingleFile(localPath, store.dialogTargetFile.path)
    if (success) store.setIsPushDialogOpen(false)
  }
  async function handlePushFolderConfirm(localPath: string) {
    const remoteDir = normalizePath(store.currentPath)
    const name = localPath.split(/[/\\]/).pop() ?? localPath
    const success = await pushSingleFile(localPath, `${remoteDir}/${name}`)
    if (success) store.setIsPushFolderDialogOpen(false)
  }
  async function handlePushFilesToCurrentDirectory() {
    const paths = await chooseMultipleLocalFiles()
    if (paths.length === 0) return
    await pushMultipleToCurrentDir(paths)
  }
  async function handleRenameConfirm(newName: string) {
    if (!store.dialogTargetFile) return
    const parent = store.dialogTargetFile.path.slice(0, store.dialogTargetFile.path.lastIndexOf('/'))
    const success = await renameExistingFile(store.dialogTargetFile.path, `${parent}/${newName}`)
    if (success) store.setIsRenameDialogOpen(false)
  }
  async function handleDeleteConfirm() {
    if (!store.dialogTargetFile) return
    const success = await removeFile(store.dialogTargetFile.path)
    if (success) store.setIsDeleteDialogOpen(false)
  }
  async function handleNewFolderConfirm(name: string) {
    const success = await createFolder(name)
    if (success) store.setIsNewFolderDialogOpen(false)
  }
  async function handleMoveConfirm(destinationDir: string) {
    if (!store.dialogTargetFile) return
    const success = await moveFile(store.dialogTargetFile.path, normalizePath(destinationDir))
    if (success) store.setIsMoveDialogOpen(false)
  }
  async function handleBatchPullConfirm(localDir: string) {
    const success = await pullSelectedFiles(localDir)
    if (success) store.setIsBatchPullDialogOpen(false)
  }
  async function handleBatchDeleteConfirm() {
    const success = await deleteSelectedFiles()
    if (success) store.setIsBatchDeleteDialogOpen(false)
  }

  return {
    currentPath: store.currentPath,
    breadcrumbs,
    files: store.files,
    visibleFiles,
    visiblePaths,
    searchTerm: store.searchTerm,
    showHidden: store.showHidden,
    sortField: store.sortField,
    sortDirection: store.sortDirection,
    selectedFiles: store.selectedFiles,
    loading: store.loading,
    refreshing: store.refreshing,
    busyFilePath: store.busyFilePath,
    busyBatchAction: store.busyBatchAction,
    error: store.error,
    lastUpdatedAt: store.lastUpdatedAt,
    transferProgress: store.transferProgress,
    lastTransferVerification: store.lastTransferVerification,
    lastTransferBatch: store.lastTransferBatch,
    totalItems: store.files.length,
    folderCount,
    fileCount,
    selectedCount: store.selectedFiles.length,

    setSearchTerm: store.setSearchTerm,
    setShowHidden: store.setShowHidden,
    setSort,
    setSortDirection: store.setSortDirection,
    dismissError,
    cancelTransfer,
    toggleFileSelection: store.toggleFileSelection,
    toggleVisibleSelection: () => store.toggleVisibleSelection(visiblePaths),
    clearSelection: store.clearSelection,
    resetFilters: store.resetFilters,
    refreshFiles,
    navigateTo,
    navigateUp,
    openDirectory,
    chooseLocalFile,
    chooseLocalSaveFile,
    chooseMultipleLocalFiles,
    chooseLocalDirectory,
    pullSingleFile,
    pushSingleFile,
    pushMultipleToCurrentDir,
    removeFile,
    createFolder,
    renameExistingFile,
    moveFile,
    pullSelectedFiles,
    deleteSelectedFiles,

    isPullDialogOpen: store.isPullDialogOpen,
    isPushDialogOpen: store.isPushDialogOpen,
    isPushFolderDialogOpen: store.isPushFolderDialogOpen,
    isRenameDialogOpen: store.isRenameDialogOpen,
    isDeleteDialogOpen: store.isDeleteDialogOpen,
    isNewFolderDialogOpen: store.isNewFolderDialogOpen,
    isMoveDialogOpen: store.isMoveDialogOpen,
    isBatchPullDialogOpen: store.isBatchPullDialogOpen,
    isBatchDeleteDialogOpen: store.isBatchDeleteDialogOpen,
    dialogTargetFile: store.dialogTargetFile,

    setIsPullDialogOpen: store.setIsPullDialogOpen,
    setIsPushDialogOpen: store.setIsPushDialogOpen,
    setIsPushFolderDialogOpen: store.setIsPushFolderDialogOpen,
    setIsRenameDialogOpen: store.setIsRenameDialogOpen,
    setIsDeleteDialogOpen: store.setIsDeleteDialogOpen,
    setIsNewFolderDialogOpen: store.setIsNewFolderDialogOpen,
    setIsMoveDialogOpen: store.setIsMoveDialogOpen,
    setIsBatchPullDialogOpen: store.setIsBatchPullDialogOpen,
    setIsBatchDeleteDialogOpen: store.setIsBatchDeleteDialogOpen,

    openPullDialog,
    openPushDialog,
    openPushFolderDialog,
    openRenameDialog,
    openDeleteDialog,
    openNewFolderDialog,
    openMoveDialog,
    openBatchPullDialog,
    openBatchDeleteDialog,
    getSizeForFile,
    handlePullConfirm,
    handlePushConfirm,
    handlePushFolderConfirm,
    handlePushFilesToCurrentDirectory,
    handleRenameConfirm,
    handleDeleteConfirm,
    handleNewFolderConfirm,
    handleMoveConfirm,
    handleBatchPullConfirm,
    handleBatchDeleteConfirm,
  }
}

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  IconArrowLeft as ArrowLeft,
  IconArrowRight as ArrowRight,
  IconArrowsExchange as ArrowsExchange,
  IconDeviceMobile as DeviceMobile,
  IconEye as Eye,
  IconEyeOff as EyeOff,
  IconFolderPlus as FolderPlus,
  IconHome as Home,
  IconPencil as Pencil,
  IconRefresh as RefreshCw,
  IconServer as Server,
  IconTrash as Trash2,
} from '@tabler/icons-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { StorageBar } from '@/components/files/StorageBar'
import { DualPaneFileList } from '@/components/files/DualPaneFileList'
import { FileActionDialogs } from '@/components/files/FileActionDialogs'
import { TransferProgressOverlay } from '@/components/files/TransferProgressOverlay'
import { UnblockPathDialog } from '@/components/files/UnblockPathDialog'
import { useDevices } from '@/hooks/useDevices'
import { useFileExplorer } from '@/hooks/useFileExplorer'
import { useFileExplorerStore } from '@/stores/useFileExplorerStore'
import {
  getHostFileSystemInfo,
  getLocalParentPath,
  getStorageInfo,
  listLocalFiles,
  pullMultipleFiles,
  pushMultipleFiles,
  unblockPath,
} from '@/services/fileService'
import type {
  FileEntry,
  HostFileSystemInfo,
  StorageInfo,
  UnblockResult,
} from '@/lib/types'
import { cn } from '@/lib/utils'

function filterLocal(files: FileEntry[], search: string) {
  const term = search.trim().toLowerCase()
  if (!term) return files
  return files.filter((file) => file.name.toLowerCase().includes(term))
}

function PaneHeader({
  title,
  subtitle,
  icon: Icon,
  selectedCount,
}: {
  title: string
  subtitle: string
  icon: typeof Server
  selectedCount: number
}) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-border/50 px-3 py-2.5">
      <div className="flex min-w-0 items-center gap-2">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Icon className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0">
          <div className="text-xs font-semibold">{title}</div>
          <div className="truncate text-[9px] text-muted-foreground">{subtitle}</div>
        </div>
      </div>
      {selectedCount > 0 && (
        <span className="rounded-full border border-primary/20 bg-primary/8 px-2 py-0.5 text-[9px] font-semibold text-primary">
          {selectedCount} selected
        </span>
      )}
    </div>
  )
}

export default function FilesPage() {
  const { activeSerial, deviceInfo } = useDevices()
  const fe = useFileExplorer()

  const [hostInfo, setHostInfo] = useState<HostFileSystemInfo | null>(null)
  const [localPath, setLocalPath] = useState('')
  const [localDraft, setLocalDraft] = useState('')
  const [localFiles, setLocalFiles] = useState<FileEntry[]>([])
  const [localSelected, setLocalSelected] = useState<string[]>([])
  const [localSearch, setLocalSearch] = useState('')
  const [localShowHidden, setLocalShowHidden] = useState(false)
  const [localLoading, setLocalLoading] = useState(true)
  const [localError, setLocalError] = useState<string | null>(null)

  const [remoteDraft, setRemoteDraft] = useState(fe.currentPath)
  const [storageInfo, setStorageInfo] = useState<StorageInfo | null>(null)
  const [transferBusy, setTransferBusy] = useState<'push' | 'pull' | null>(null)
  const [unblockResult, setUnblockResult] = useState<UnblockResult | null>(null)
  const [isUnblockDialogOpen, setIsUnblockDialogOpen] = useState(false)

  const loadLocal = useCallback(async (targetPath: string, showHidden = localShowHidden) => {
    setLocalLoading(true)
    setLocalError(null)
    try {
      const files = await listLocalFiles(targetPath, showHidden)
      const nextPath = targetPath.trim() || hostInfo?.home || ''
      setLocalPath(nextPath)
      setLocalDraft(nextPath)
      setLocalFiles(files)
      setLocalSelected([])
    } catch (error) {
      setLocalError(error instanceof Error ? error.message : String(error))
    } finally {
      setLocalLoading(false)
    }
  }, [hostInfo?.home, localShowHidden])

  useEffect(() => {
    let cancelled = false
    void getHostFileSystemInfo()
      .then((info) => {
        if (cancelled) return
        setHostInfo(info)
        setLocalPath(info.home)
        setLocalDraft(info.home)
        return listLocalFiles(info.home, localShowHidden)
      })
      .then((files) => {
        if (!cancelled && files) setLocalFiles(files)
      })
      .catch((error) => {
        if (!cancelled) setLocalError(error instanceof Error ? error.message : String(error))
      })
      .finally(() => {
        if (!cancelled) setLocalLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!localPath) return
    void loadLocal(localPath, localShowHidden)
  }, [localShowHidden])

  useEffect(() => {
    setRemoteDraft(fe.currentPath)
  }, [fe.currentPath])

  useEffect(() => {
    if (!activeSerial) {
      setStorageInfo(null)
      return
    }
    void getStorageInfo()
      .then(setStorageInfo)
      .catch(() => setStorageInfo(null))
  }, [activeSerial, fe.lastUpdatedAt])

  useEffect(() => {
    if (!fe.error) return
    const lower = fe.error.toLowerCase()
    if (
      !lower.includes('protected') &&
      !lower.includes('permission') &&
      !lower.includes('access denied') &&
      !lower.includes('scoped storage')
    ) {
      return
    }
    const pathMatch = fe.error.match(/\/[^\s]+/)
    const path = pathMatch ? pathMatch[0] : fe.currentPath
    void unblockPath(path)
      .then(setUnblockResult)
      .catch(() => setUnblockResult(null))
      .finally(() => setIsUnblockDialogOpen(true))
  }, [fe.error, fe.currentPath])

  const visibleLocalFiles = useMemo(
    () => filterLocal(localFiles, localSearch),
    [localFiles, localSearch],
  )

  const selectedRemoteEntry = useMemo(() => {
    if (fe.selectedFiles.length !== 1) return null
    return fe.files.find((file) => file.path === fe.selectedFiles[0]) ?? null
  }, [fe.files, fe.selectedFiles])

  function toggleLocal(path: string) {
    setLocalSelected((current) =>
      current.includes(path)
        ? current.filter((item) => item !== path)
        : [...current, path],
    )
  }

  function selectAllLocal() {
    const paths = visibleLocalFiles.map((file) => file.path)
    const allSelected = paths.length > 0 && paths.every((path) => localSelected.includes(path))
    setLocalSelected(allSelected ? [] : paths)
  }

  async function goLocalUp() {
    if (!localPath) return
    const parent = await getLocalParentPath(localPath)
    await loadLocal(parent)
  }

  async function pushToAndroid() {
    if (!activeSerial || localSelected.length === 0) return
    setTransferBusy('push')
    try {
      await toast.promise(pushMultipleFiles(localSelected, fe.currentPath), {
        loading: `Sending ${localSelected.length} item(s) to Android…`,
        success: (message) => message,
        error: (error) => error instanceof Error ? error.message : 'Transfer failed',
      })
      await fe.refreshFiles()
    } finally {
      setTransferBusy(null)
    }
  }

  async function pullToComputer() {
    if (fe.selectedFiles.length === 0 || !localPath) return
    setTransferBusy('pull')
    try {
      await toast.promise(pullMultipleFiles(fe.selectedFiles, localPath), {
        loading: `Receiving ${fe.selectedFiles.length} item(s) from Android…`,
        success: (message) => message,
        error: (error) => error instanceof Error ? error.message : 'Transfer failed',
      })
      await loadLocal(localPath)
    } finally {
      setTransferBusy(null)
    }
  }

  function openRemoteAction(action: 'rename' | 'move' | 'delete') {
    if (!selectedRemoteEntry) return
    if (action === 'rename') fe.openRenameDialog(selectedRemoteEntry)
    if (action === 'move') fe.openMoveDialog(selectedRemoteEntry)
    if (action === 'delete') fe.openDeleteDialog(selectedRemoteEntry)
  }

  if (!activeSerial) {
    return (
      <div className="flex h-full flex-col gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dual-pane File Manager</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Browse your computer and an Android device side by side.
          </p>
        </div>
        <Card className="flex flex-1 items-center justify-center rounded-2xl border-border/60 bg-card/60">
          <div className="max-w-md p-8 text-center">
            <ArrowsExchange className="mx-auto h-10 w-10 text-muted-foreground" />
            <h2 className="mt-4 font-semibold">Connect an Android device</h2>
            <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
              The local panel is ready, but Android browsing and transfer actions require an active ADB device.
            </p>
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 overflow-hidden">
      <div className="flex shrink-0 items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dual-pane File Manager</h1>
          <p className="mt-1 text-xs text-muted-foreground">
            PC ↔ {deviceInfo?.model || activeSerial}. Select files or folders, then transfer directly between panes.
          </p>
        </div>
        <div className="w-56 shrink-0">
          <StorageBar info={storageInfo} />
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_72px_minmax(0,1fr)] gap-3">
        <Card className="flex min-h-0 flex-col overflow-hidden rounded-2xl border-border/60 bg-card/65">
          <PaneHeader
            title="This computer"
            subtitle={hostInfo ? `${hostInfo.os} · local filesystem` : 'Local filesystem'}
            icon={Server}
            selectedCount={localSelected.length}
          />

          <div className="space-y-2 border-b border-border/50 p-2.5">
            <div className="flex items-center gap-1.5">
              <Button size="icon" variant="outline" className="h-7 w-7 shrink-0" onClick={() => void goLocalUp()} title="Parent folder">
                <ArrowLeft className="h-3.5 w-3.5" />
              </Button>
              <Button
                size="icon"
                variant="outline"
                className="h-7 w-7 shrink-0"
                onClick={() => hostInfo && void loadLocal(hostInfo.home)}
                title="Home"
              >
                <Home className="h-3.5 w-3.5" />
              </Button>
              <Input
                value={localDraft}
                onChange={(event) => setLocalDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void loadLocal(localDraft)
                }}
                className="h-7 min-w-0 flex-1 rounded-lg font-mono text-[10px]"
                aria-label="Local path"
              />
              <Button
                size="icon"
                variant="ghost"
                className="h-7 w-7 shrink-0"
                onClick={() => void loadLocal(localPath)}
                title="Refresh local"
              >
                <RefreshCw className={cn('h-3.5 w-3.5', localLoading && 'animate-spin')} />
              </Button>
            </div>

            <div className="flex items-center gap-1.5">
              <Input
                value={localSearch}
                onChange={(event) => setLocalSearch(event.target.value)}
                placeholder="Search local…"
                className="h-7 min-w-0 flex-1 rounded-lg text-[10px]"
              />
              <Button
                size="sm"
                variant="ghost"
                className="h-7 gap-1 px-2 text-[9px]"
                onClick={() => setLocalShowHidden((value) => !value)}
              >
                {localShowHidden ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                Hidden
              </Button>
              <Button size="sm" variant="ghost" className="h-7 px-2 text-[9px]" onClick={selectAllLocal}>
                Select all
              </Button>
            </div>

            {hostInfo && hostInfo.roots.length > 1 && (
              <div className="flex flex-wrap gap-1">
                {hostInfo.roots.map((root) => (
                  <button
                    key={root}
                    type="button"
                    onClick={() => void loadLocal(root)}
                    className="rounded-md border border-border/50 bg-muted/20 px-2 py-0.5 font-mono text-[9px] text-muted-foreground hover:text-foreground"
                  >
                    {root}
                  </button>
                ))}
              </div>
            )}

            {localError && (
              <div className="rounded-lg border border-destructive/20 bg-destructive/5 px-2.5 py-1.5 text-[9px] text-destructive">
                {localError}
              </div>
            )}
          </div>

          <DualPaneFileList
            files={visibleLocalFiles}
            selected={localSelected}
            loading={localLoading}
            emptyLabel={localSearch ? 'No matching local files.' : 'This local folder is empty.'}
            onToggle={toggleLocal}
            onOpenDirectory={(file) => void loadLocal(file.path)}
          />
        </Card>

        <div className="flex min-h-0 flex-col items-center justify-center gap-2">
          <Button
            size="icon"
            className="h-10 w-10 rounded-xl"
            disabled={localSelected.length === 0 || transferBusy !== null}
            onClick={() => void pushToAndroid()}
            title="Send selected PC items to Android"
          >
            <ArrowRight className="h-4 w-4" />
          </Button>
          <Button
            size="icon"
            variant="outline"
            className="h-10 w-10 rounded-xl"
            disabled={fe.selectedFiles.length === 0 || transferBusy !== null}
            onClick={() => void pullToComputer()}
            title="Copy selected Android items to PC"
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="mt-1 text-center text-[8px] leading-relaxed text-muted-foreground">
            PC<br />↔<br />Android
          </div>
        </div>

        <Card className="flex min-h-0 flex-col overflow-hidden rounded-2xl border-border/60 bg-card/65">
          <PaneHeader
            title={deviceInfo?.model || 'Android device'}
            subtitle={activeSerial}
            icon={DeviceMobile}
            selectedCount={fe.selectedCount}
          />

          <div className="space-y-2 border-b border-border/50 p-2.5">
            <div className="flex items-center gap-1.5">
              <Button
                size="icon"
                variant="outline"
                className="h-7 w-7 shrink-0"
                onClick={fe.navigateUp}
                title="Parent folder"
              >
                <ArrowLeft className="h-3.5 w-3.5" />
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-7 shrink-0 px-2 text-[9px]"
                onClick={() => fe.navigateTo('/sdcard')}
                title="Internal storage"
              >
                Internal
              </Button>
              <Input
                value={remoteDraft}
                onChange={(event) => setRemoteDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') fe.navigateTo(remoteDraft)
                }}
                className="h-7 min-w-0 flex-1 rounded-lg font-mono text-[10px]"
                aria-label="Android path"
              />
              <Button
                size="icon"
                variant="ghost"
                className="h-7 w-7 shrink-0"
                onClick={fe.refreshFiles}
                title="Refresh Android"
              >
                <RefreshCw className={cn('h-3.5 w-3.5', fe.refreshing && 'animate-spin')} />
              </Button>
            </div>

            <div className="flex items-center gap-1.5">
              <Input
                value={fe.searchTerm}
                onChange={(event) => fe.setSearchTerm(event.target.value)}
                placeholder="Search Android…"
                className="h-7 min-w-0 flex-1 rounded-lg text-[10px]"
              />
              <Button
                size="sm"
                variant="ghost"
                className="h-7 gap-1 px-2 text-[9px]"
                onClick={() => fe.setShowHidden(!fe.showHidden)}
              >
                {fe.showHidden ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                Hidden
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 px-2 text-[9px]"
                onClick={fe.toggleVisibleSelection}
              >
                Select all
              </Button>
            </div>

            <div className="flex min-h-7 items-center gap-1.5">
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 px-2 text-[9px]"
                onClick={fe.openNewFolderDialog}
              >
                <FolderPlus className="h-3 w-3" />
                New folder
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 px-2 text-[9px]"
                disabled={!selectedRemoteEntry}
                onClick={() => openRemoteAction('rename')}
              >
                <Pencil className="h-3 w-3" />
                Rename
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-7 px-2 text-[9px]"
                disabled={!selectedRemoteEntry}
                onClick={() => openRemoteAction('move')}
              >
                Move
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 px-2 text-[9px] text-destructive"
                disabled={fe.selectedCount === 0}
                onClick={() => {
                  if (fe.selectedCount === 1) openRemoteAction('delete')
                  else fe.openBatchDeleteDialog()
                }}
              >
                <Trash2 className="h-3 w-3" />
                Delete
              </Button>
              <span className="ml-auto text-[9px] text-muted-foreground">
                {fe.folderCount} folders · {fe.fileCount} files
              </span>
            </div>

            {fe.error && (
              <div className="rounded-lg border border-destructive/20 bg-destructive/5 px-2.5 py-1.5 text-[9px] text-destructive">
                {fe.error}
              </div>
            )}
          </div>

          <DualPaneFileList
            files={fe.visibleFiles}
            selected={fe.selectedFiles}
            loading={fe.loading}
            emptyLabel={fe.searchTerm ? 'No matching Android files.' : 'This Android folder is empty.'}
            onToggle={fe.toggleFileSelection}
            onOpenDirectory={fe.openDirectory}
          />
        </Card>
      </div>

      <div className="flex shrink-0 items-center justify-between rounded-xl border border-border/50 bg-muted/15 px-3 py-2 text-[9px] text-muted-foreground">
        <span>
          Transfers use the existing ADB push/pull engine, including folders, retries, progress and cancellation.
        </span>
        <span className="font-mono">
          {hostInfo?.os ?? 'host'} {hostInfo?.separator ?? ''} · Android {deviceInfo?.androidVersion || ''}
        </span>
      </div>

      <FileActionDialogs
        dialogTargetFile={fe.dialogTargetFile}
        selectedCount={fe.selectedCount}
        isPullDialogOpen={fe.isPullDialogOpen}
        isPushDialogOpen={fe.isPushDialogOpen}
        isPushFolderDialogOpen={fe.isPushFolderDialogOpen}
        isRenameDialogOpen={fe.isRenameDialogOpen}
        isDeleteDialogOpen={fe.isDeleteDialogOpen}
        isNewFolderDialogOpen={fe.isNewFolderDialogOpen}
        isMoveDialogOpen={fe.isMoveDialogOpen}
        isBatchPullDialogOpen={fe.isBatchPullDialogOpen}
        isBatchDeleteDialogOpen={fe.isBatchDeleteDialogOpen}
        setIsPullDialogOpen={fe.setIsPullDialogOpen}
        setIsPushDialogOpen={fe.setIsPushDialogOpen}
        setIsPushFolderDialogOpen={fe.setIsPushFolderDialogOpen}
        setIsRenameDialogOpen={fe.setIsRenameDialogOpen}
        setIsDeleteDialogOpen={fe.setIsDeleteDialogOpen}
        setIsNewFolderDialogOpen={fe.setIsNewFolderDialogOpen}
        setIsMoveDialogOpen={fe.setIsMoveDialogOpen}
        setIsBatchPullDialogOpen={fe.setIsBatchPullDialogOpen}
        setIsBatchDeleteDialogOpen={fe.setIsBatchDeleteDialogOpen}
        onPullConfirm={fe.handlePullConfirm}
        onPushConfirm={fe.handlePushConfirm}
        onPushFolderConfirm={fe.handlePushFolderConfirm}
        onRenameConfirm={fe.handleRenameConfirm}
        onDeleteConfirm={fe.handleDeleteConfirm}
        onNewFolderConfirm={fe.handleNewFolderConfirm}
        onMoveConfirm={fe.handleMoveConfirm}
        onBatchPullConfirm={fe.handleBatchPullConfirm}
        onBatchDeleteConfirm={fe.handleBatchDeleteConfirm}
        chooseLocalFile={fe.chooseLocalFile}
        chooseLocalSaveFile={fe.chooseLocalSaveFile}
        chooseLocalDirectory={fe.chooseLocalDirectory}
      />

      {fe.transferProgress?.active && (
        <TransferProgressOverlay
          fileName={fe.transferProgress.fileName}
          direction={fe.transferProgress.direction}
          percent={fe.transferProgress.percent}
          onCancel={fe.cancelTransfer}
        />
      )}

      <UnblockPathDialog
        result={unblockResult}
        open={isUnblockDialogOpen}
        onOpenChange={(open) => {
          setIsUnblockDialogOpen(open)
          if (!open) {
            setUnblockResult(null)
            useFileExplorerStore.getState().setError(null)
          }
        }}
        onRetry={() => {
          setIsUnblockDialogOpen(false)
          setUnblockResult(null)
          useFileExplorerStore.getState().setError(null)
          fe.refreshFiles()
        }}
      />
    </div>
  )
}

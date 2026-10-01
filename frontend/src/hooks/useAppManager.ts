import { useCallback, useEffect, useMemo, useRef } from 'react'
import { toast } from 'sonner'
import { useAppManagerStore } from '@/stores/useAppManagerStore'
import { useDeviceStore } from '@/stores/useDeviceStore'
import {
  listPackages,
  installPackage as svcInstallPackage,
  installPackages as svcInstallPackages,
  uninstallPackage as svcUninstallPackage,
  uninstallMultiplePackages as svcUninstallBatch,
  enablePackage as svcEnablePackage,
  enableMultiplePackages as svcEnableBatch,
  disablePackage as svcDisablePackage,
  disableMultiplePackages as svcDisableBatch,
  clearPackageData as svcClearData,
  pullPackageApk as svcPullApk,
  launchPackage as svcLaunch,
  forceStopPackage as svcForceStop,
  getPackageDetails as svcGetDetails,
  selectApkFile as svcSelectApk,
  selectApkFiles as svcSelectApks,
} from '@/services/packageService'
import type { PackageDetails, DeviceSummary, PackageInstallMode } from '@/lib/types'

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Operation failed'
}

function isReadyAdbDevice(d: DeviceSummary): boolean {
  return d.mode === 'adb' && d.state === 'device'
}

export function useAppManager() {
  const store = useAppManagerStore()
  const { devices, activeSerial } = useDeviceStore()
  const detailsCache = useAppManagerStore((s) => s.detailsCache)
  const updateDetails = useAppManagerStore((s) => s.updateDetails)
  const clearDetailsCache = useAppManagerStore((s) => s.clearDetailsCache)
  const pendingDetailsRef = useRef<Set<string>>(new Set())
  const queueRef = useRef<string[]>([])
  const isProcessingRef = useRef(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    clearDetailsCache()
  }, [activeSerial, clearDetailsCache])

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [])

  const hasReadyAdbDevice = useMemo(
    () => devices.some(isReadyAdbDevice),
    [devices],
  )

  const hasReadyActiveDevice = useMemo(
    () => devices.some((d) => d.serial === activeSerial && isReadyAdbDevice(d)),
    [activeSerial, devices],
  )

  // Every request carries the serial it was captured for plus a generation, so a
  // late reply can never render for another device.
  const loadGenerationRef = useRef(0)

  // Every mutation re-reads the live confirmed serial and refuses to act when the
  // listing in the store was captured for a different device, so a stale selection
  // or a stale detail sheet can never reach another target.
  const requireConfirmedSerial = useCallback((operation: string): string | null => {
    const live = useDeviceStore.getState().activeSerial
    if (!live) {
      toast.error(`${operation}: no device is selected`)
      return null
    }
    const snapshotSerial = useAppManagerStore.getState().packagesSerial
    if (snapshotSerial !== live) {
      toast.error(`${operation}: the device changed — refresh the package list and try again`)
      return null
    }
    return live
  }, [])

  const processQueue = useCallback(async () => {
    if (isProcessingRef.current || queueRef.current.length === 0) return
    isProcessingRef.current = true

    const BATCH_SIZE = 2
    while (queueRef.current.length > 0) {
      const batch = queueRef.current.splice(0, BATCH_SIZE)
      const toFetch = batch.filter(
        (name) => !detailsCache.has(name) && !pendingDetailsRef.current.has(name)
      )

      if (toFetch.length === 0) continue

      for (const name of toFetch) {
        pendingDetailsRef.current.add(name)
      }

      await Promise.allSettled(
        toFetch.map(async (name) => {
          try {
            const serial = useDeviceStore.getState().activeSerial
            if (!serial) return
            const details = await svcGetDetails(serial, name)
            if (details && useDeviceStore.getState().activeSerial === serial) {
              updateDetails(name, details)
            }
          } catch {
            // ignore
          } finally {
            pendingDetailsRef.current.delete(name)
          }
        })
      )

      await new Promise((resolve) => setTimeout(resolve, 50))
    }

    isProcessingRef.current = false
  }, [detailsCache, updateDetails])

  const loadDetails = useCallback((packageName: string) => {
    if (detailsCache.has(packageName) || pendingDetailsRef.current.has(packageName)) return

    if (!queueRef.current.includes(packageName)) {
      queueRef.current.push(packageName)
    }

    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }
    timerRef.current = setTimeout(() => {
      void processQueue()
    }, 200)
  }, [detailsCache, processQueue])

  const fetchPackages = useCallback(
    async (isRefresh = false) => {
      const serial = activeSerial
      if (!hasReadyAdbDevice || !hasReadyActiveDevice || !serial) {
        store.clearMachineBoundState()
        store.setError('No ADB device connected')
        store.setLoading(false)
        store.setRefreshing(false)
        return
      }

      const generation = ++loadGenerationRef.current

      if (isRefresh) {
        store.setRefreshing(true)
      } else {
        store.setLoading(true)
      }
      store.setError(null)

      try {
        const packages = await listPackages(serial, store.filter)
        // A reply for another serial (or a superseded request) must not render.
        if (generation !== loadGenerationRef.current) return
        if (useDeviceStore.getState().activeSerial !== serial) return
        store.setPackages(serial, packages)
        store.setLastUpdatedAt(Date.now())
      } catch (err) {
        if (generation === loadGenerationRef.current && useDeviceStore.getState().activeSerial === serial) {
          store.setError(getErrorMessage(err))
        }
      } finally {
        if (generation === loadGenerationRef.current) {
          store.setLoading(false)
          store.setRefreshing(false)
        }
      }
    },
    [store, activeSerial, hasReadyAdbDevice, hasReadyActiveDevice],
  )

  useEffect(() => {
    // A target change drops only the machine-bound listing/selection/details.
    store.clearMachineBoundState()
    void fetchPackages()
  }, [activeSerial, store.filter])

  const filteredPackages = useMemo(() => {
    let result = [...store.packages]

    if (store.statusFilter === 'enabled') {
      result = result.filter((pkg) => pkg.isEnabled)
    } else if (store.statusFilter === 'disabled') {
      result = result.filter((pkg) => !pkg.isEnabled)
    }

    if (store.searchTerm.trim()) {
      const term = store.searchTerm.toLowerCase()
      result = result.filter((pkg) =>
        pkg.packageName.toLowerCase().includes(term),
      )
    }

    switch (store.sortOrder) {
      case 'za':
        result.sort((a, b) => b.packageName.localeCompare(a.packageName))
        break
      case 'size-desc':
        result.sort((a, b) => {
          const sizeA = detailsCache.get(a.packageName)?.totalSizeBytes ?? -1
          const sizeB = detailsCache.get(b.packageName)?.totalSizeBytes ?? -1
          return sizeB - sizeA
        })
        break
      case 'size-asc':
        result.sort((a, b) => {
          const sizeA = detailsCache.get(a.packageName)?.totalSizeBytes ?? -1
          const sizeB = detailsCache.get(b.packageName)?.totalSizeBytes ?? -1
          return sizeA - sizeB
        })
        break
      default:
        result.sort((a, b) => a.packageName.localeCompare(b.packageName))
    }

    return result
  }, [store.packages, store.statusFilter, store.searchTerm, store.sortOrder, detailsCache])

  const isSizeSort = store.sortOrder === 'size-desc' || store.sortOrder === 'size-asc'

  useEffect(() => {
    if (!isSizeSort || filteredPackages.length === 0) return

    const uncached = filteredPackages
      .filter((pkg) => !detailsCache.has(pkg.packageName))
      .slice(0, 20)

    if (uncached.length === 0) return

    for (const pkg of uncached) {
      loadDetails(pkg.packageName)
    }
  }, [isSizeSort, filteredPackages, detailsCache, loadDetails])

  const installApk = useCallback(async () => {
    try {
      const filePath = await svcSelectApk()
      if (!filePath) return
      const serial = requireConfirmedSerial('Install APK')
      if (!serial) return

      store.setInstalling(true)
      const message = await svcInstallPackage(serial, filePath)
      toast.success(message)
      await fetchPackages(true)
    } catch (err) {
      toast.error(getErrorMessage(err))
    } finally {
      store.setInstalling(false)
    }
  }, [fetchPackages])

  const installApkFromPath = useCallback(
    async (
      filePath: string,
      mode: PackageInstallMode = 'replace',
    ): Promise<boolean> => {
      try {
        const serial = requireConfirmedSerial('Install package')
        if (!serial) return false
        store.setInstalling(true)
        const message = await svcInstallPackage(serial, filePath, mode)
        toast.success(message)
        await fetchPackages(true)
        return true
      } catch (err) {
        toast.error(getErrorMessage(err))
        return false
      } finally {
        store.setInstalling(false)
      }
    },
    [fetchPackages],
  )

  const installSplitApksFromPaths = useCallback(
    async (
      filePaths: string[],
      mode: PackageInstallMode = 'replace',
    ): Promise<boolean> => {
      try {
        const serial = requireConfirmedSerial('Install split APKs')
        if (!serial) return false
        store.setInstalling(true)
        const message = await svcInstallPackages(serial, filePaths, mode)
        toast.success(message)
        await fetchPackages(true)
        return true
      } catch (err) {
        toast.error(getErrorMessage(err))
        return false
      } finally {
        store.setInstalling(false)
      }
    },
    [fetchPackages],
  )

  const selectSplitApks = useCallback(async (): Promise<string[]> => {
    try {
      return await svcSelectApks()
    } catch (err) {
      toast.error(getErrorMessage(err))
      return []
    }
  }, [])

  const uninstallSingle = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Uninstall package')
        if (!serial) return
        const message = await svcUninstallPackage(serial, packageName)
        toast.success(message)
        // The package is gone: drop it from the selection instead of toggling it in,
        // which used to leave a phantom entry that enabled single-package actions.
        store.removePackageFromSelection(packageName)
        await fetchPackages(true)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [fetchPackages],
  )

  const uninstallBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Uninstall packages')
    if (!serial) return

    store.setBusyBatchAction('uninstall')
    try {
      const message = await svcUninstallBatch(serial, names)
      toast.success(message)
      store.clearSelection()
      await fetchPackages(true)
    } catch (err) {
      toast.error(getErrorMessage(err))
    } finally {
      store.setBusyBatchAction(null)
    }
  }, [fetchPackages])

  const enableSingle = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Enable package')
        if (!serial) return
        const message = await svcEnablePackage(serial, packageName)
        toast.success(message)
        await fetchPackages(true)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [fetchPackages],
  )

  const enableBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Enable packages')
    if (!serial) return

    store.setBusyBatchAction('enable')
    try {
      const message = await svcEnableBatch(serial, names)
      toast.success(message)
      store.clearSelection()
      await fetchPackages(true)
    } catch (err) {
      toast.error(getErrorMessage(err))
    } finally {
      store.setBusyBatchAction(null)
    }
  }, [fetchPackages])

  const disableSingle = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Disable package')
        if (!serial) return
        const message = await svcDisablePackage(serial, packageName)
        toast.success(message)
        await fetchPackages(true)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [fetchPackages],
  )

  const disableBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Disable packages')
    if (!serial) return

    store.setBusyBatchAction('disable')
    try {
      const message = await svcDisableBatch(serial, names)
      toast.success(message)
      store.clearSelection()
      await fetchPackages(true)
    } catch (err) {
      toast.error(getErrorMessage(err))
    } finally {
      store.setBusyBatchAction(null)
    }
  }, [fetchPackages])

  const forceStopBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Force-stop packages')
    if (!serial) return

    store.setBusyBatchAction('force-stop')
    let successCount = 0
    const failures: string[] = []
    for (const name of names) {
      if (useDeviceStore.getState().activeSerial !== serial) {
        failures.push(`${name}: the device changed — action skipped`)
        continue
      }
      try {
        await svcForceStop(serial, name)
        successCount++
      } catch (err) {
        failures.push(`${name}: ${getErrorMessage(err)}`)
      }
    }
    store.setBusyBatchAction(null)

    if (failures.length === 0) {
      toast.success(`Successfully stopped ${successCount} package(s)`)
    } else if (successCount === 0) {
      toast.error(`Failed to stop ${failures.length} package(s)`, {
        description: failures.slice(0, 3).join('\n'),
      })
    } else {
      toast.warning(`Stopped ${successCount}, failed ${failures.length}`, {
        description: failures.slice(0, 3).join('\n'),
      })
    }
    store.clearSelection()
  }, [])

  const clearDataBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Clear app data')
    if (!serial) return

    store.setBusyBatchAction('clear-data')
    let successCount = 0
    const failures: string[] = []
    for (const name of names) {
      if (useDeviceStore.getState().activeSerial !== serial) {
        failures.push(`${name}: the device changed — action skipped`)
        continue
      }
      try {
        await svcClearData(serial, name)
        successCount++
      } catch (err) {
        failures.push(`${name}: ${getErrorMessage(err)}`)
      }
    }
    store.setBusyBatchAction(null)

    if (failures.length === 0) {
      toast.success(`Successfully cleared data for ${successCount} package(s)`)
    } else if (successCount === 0) {
      toast.error(`Failed to clear data for ${failures.length} package(s)`, {
        description: failures.slice(0, 3).join('\n'),
      })
    } else {
      toast.warning(`Cleared ${successCount}, failed ${failures.length}`, {
        description: failures.slice(0, 3).join('\n'),
      })
    }
    store.clearSelection()
  }, [])

  const exportApkBatch = useCallback(async () => {
    const names = store.selectedPackages
    if (names.length === 0) return

    const serial = requireConfirmedSerial('Export APKs')
    if (!serial) return

    store.setBusyBatchAction('pull-apk')
    let successCount = 0
    const failures: string[] = []
    for (const name of names) {
      if (useDeviceStore.getState().activeSerial !== serial) {
        failures.push(`${name}: the device changed — action skipped`)
        continue
      }
      try {
        await svcPullApk(serial, name)
        successCount++
      } catch (err) {
        failures.push(`${name}: ${getErrorMessage(err)}`)
      }
    }
    store.setBusyBatchAction(null)

    if (failures.length === 0) {
      toast.success(`Successfully exported ${successCount} APK(s)`)
    } else if (successCount === 0) {
      toast.error(`Failed to export ${failures.length} APK(s)`, {
        description: failures.slice(0, 3).join('\n'),
      })
    } else {
      toast.warning(`Exported ${successCount}, failed ${failures.length}`, {
        description: failures.slice(0, 3).join('\n'),
      })
    }
    store.clearSelection()
  }, [])

  const clearData = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Clear app data')
        if (!serial) return
        const message = await svcClearData(serial, packageName)
        toast.success(message)
        void fetchPackages(true)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [fetchPackages],
  )

  const pullApk = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Export APK')
        if (!serial) return
        const message = await svcPullApk(serial, packageName)
        toast.success(message)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [],
  )

  const launch = useCallback(
    async (packageName: string) => {
      try {
        const serial = requireConfirmedSerial('Launch package')
        if (!serial) return
        const message = await svcLaunch(serial, packageName)
        toast.success(message)
      } catch (err) {
        toast.error(getErrorMessage(err))
      }
    },
    [],
  )

  const forceStop = useCallback(
    async (packageName: string) => {
      store.setBusyPackageName(packageName)
      try {
        const serial = requireConfirmedSerial('Force-stop package')
        if (!serial) return
        const message = await svcForceStop(serial, packageName)
        toast.success(message)
      } catch (err) {
        toast.error(getErrorMessage(err))
      } finally {
        store.setBusyPackageName(null)
      }
    },
    [],
  )


  const getDetails = useCallback(
    async (packageName: string): Promise<PackageDetails | null> => {
      try {
        const serial = requireConfirmedSerial('Package details')
        if (!serial) return null
        return await svcGetDetails(serial, packageName)
      } catch (err) {
        toast.error(getErrorMessage(err))
        return null
      }
    },
    [],
  )

  return {
    packages: filteredPackages,
    allPackages: store.packages,
    filter: store.filter,
    statusFilter: store.statusFilter,
    sortOrder: store.sortOrder,
    searchTerm: store.searchTerm,
    selectedPackages: store.selectedPackages,
    loading: store.loading,
    refreshing: store.refreshing,
    installing: store.installing,
    busyPackageName: store.busyPackageName,
    busyBatchAction: store.busyBatchAction,
    error: store.error,
    lastUpdatedAt: store.lastUpdatedAt,
    detailsCache,

    setFilter: store.setFilter,
    setStatusFilter: store.setStatusFilter,
    setSortOrder: store.setSortOrder,
    setSearchTerm: store.setSearchTerm,
    togglePackageSelection: store.togglePackageSelection,
    toggleVisibleSelection: store.toggleVisibleSelection,
    clearSelection: store.clearSelection,

    fetchPackages,
    installApk,
    installApkFromPath,
    installSplitApksFromPaths,
    selectSplitApks,
    uninstallSingle,
    uninstallBatch,
    enableSingle,
    enableBatch,
    disableSingle,
    disableBatch,
    forceStopBatch,
    clearDataBatch,
    exportApkBatch,
    clearData,
    pullApk,
    launch,
    forceStop,
    getDetails,
    loadDetails,
  }
}
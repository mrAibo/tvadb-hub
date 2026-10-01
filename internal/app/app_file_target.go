package app

import "ADBKit/internal/file"

// The *ForDevice twins below are additive: every legacy serial-less binding stays
// untouched. Each twin admits one caller-confirmed device and then uses the file
// service's smallest non-transfer view pinned to that device and its tool paths.
// Transfers and their cancellation keep running on the original service, which is
// why no transfer twin is added here: PushFileForDevice/PullFileForDevice and the
// detailed batch bindings already own that path.

func (a *App) ListFilesForDevice(expectedSerial string, remotePath string, showHidden bool) ([]file.Entry, error) {
	return auditAction(a, "list_files", func() ([]file.Entry, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return nil, err
		}
		return a.fileSvc.ForTarget(serial).ListFiles(a.ctx, remotePath, showHidden)
	})
}

func (a *App) GetDirectorySizeForDevice(expectedSerial string, remotePath string) (string, error) {
	return auditAction(a, "get_directory_size", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.fileSvc.ForTarget(serial).GetDirectorySize(a.ctx, remotePath)
	})
}

func (a *App) GetStorageInfoForDevice(expectedSerial string) (file.StorageInfo, error) {
	return auditAction(a, "get_storage_info", func() (file.StorageInfo, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return file.StorageInfo{}, err
		}
		return a.fileSvc.ForTarget(serial).GetStorageInfo(a.ctx)
	})
}

func (a *App) ListSdCardsForDevice(expectedSerial string) ([]file.SdCard, error) {
	return auditAction(a, "list_sd_cards", func() ([]file.SdCard, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return nil, err
		}
		return a.fileSvc.ForTarget(serial).ListSdCards(a.ctx)
	})
}

func (a *App) UnblockPathForDevice(expectedSerial string, remotePath string) (file.UnblockResult, error) {
	return auditAction(a, "unblock_path", func() (file.UnblockResult, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return file.UnblockResult{}, err
		}
		return a.fileSvc.ForTarget(serial).UnblockPath(a.ctx, remotePath)
	})
}

func (a *App) DeleteFileForDevice(expectedSerial string, remotePath string) (string, error) {
	return auditAction(a, "delete_file", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.fileSvc.ForTarget(serial).DeleteFile(a.ctx, remotePath)
	})
}

func (a *App) DeleteMultipleFilesForDevice(expectedSerial string, remotePaths []string) (string, error) {
	return auditAction(a, "delete_multiple_files", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.fileSvc.ForTarget(serial).DeleteMultipleFiles(a.ctx, remotePaths)
	})
}

func (a *App) CreateDirectoryForDevice(expectedSerial string, remotePath string) (string, error) {
	return auditAction(a, "create_directory", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.fileSvc.ForTarget(serial).CreateDirectory(a.ctx, remotePath)
	})
}

func (a *App) RenameFileForDevice(expectedSerial string, oldRemotePath string, newRemotePath string) (string, error) {
	return auditAction(a, "rename_file", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.fileSvc.ForTarget(serial).RenameFile(a.ctx, oldRemotePath, newRemotePath)
	})
}

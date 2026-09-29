package app

import (
	"ADBKit/internal/binary"
	"ADBKit/internal/core"
)

func (a *App) DownloadPlatformTools() error {
	return auditVoidAction(a, "download_platform_tools", func() error {
		if err := a.dlSvc.DownloadPlatformTools(a.ctx); err != nil {
			return err
		}

		adbPath, err := a.binSvc.GetManagedBinaryPath(binary.BinaryNameAdb)
		if err != nil {
			return err
		}
		fastbootPath, err := a.binSvc.GetManagedBinaryPath(binary.BinaryNameFastboot)
		if err != nil {
			return err
		}

		a.mu.Lock()
		defer a.mu.Unlock()

		oldAdbPath := a.cfg.AdbPath
		oldFastbootPath := a.cfg.FastbootPath
		oldSetupCompleted := a.cfg.SetupCompleted
		oldVersions := make(map[string]string, len(a.cfg.BinaryVersions))
		for key, value := range a.cfg.BinaryVersions {
			oldVersions[key] = value
		}

		if err := a.binSvc.SetCustomBinary(a.cfg, binary.BinaryNameAdb, adbPath); err != nil {
			return err
		}
		if err := a.binSvc.SetCustomBinary(a.cfg, binary.BinaryNameFastboot, fastbootPath); err != nil {
			a.cfg.AdbPath = oldAdbPath
			a.cfg.FastbootPath = oldFastbootPath
			a.cfg.SetupCompleted = oldSetupCompleted
			a.cfg.BinaryVersions = oldVersions
			return err
		}
		if oldSetupCompleted {
			if _, err := a.binSvc.CompleteSetup(a.cfg); err != nil {
				a.cfg.AdbPath = oldAdbPath
				a.cfg.FastbootPath = oldFastbootPath
				a.cfg.SetupCompleted = oldSetupCompleted
				a.cfg.BinaryVersions = oldVersions
				return err
			}
		}
		if err := saveManagedBinaryConfig(a); err != nil {
			a.cfg.AdbPath = oldAdbPath
			a.cfg.FastbootPath = oldFastbootPath
			a.cfg.SetupCompleted = oldSetupCompleted
			a.cfg.BinaryVersions = oldVersions
			return err
		}
		return nil
	})
}

func (a *App) DownloadScrcpy() error {
	return auditVoidAction(a, "download_scrcpy", func() error {
		if err := a.dlSvc.DownloadScrcpy(a.ctx); err != nil {
			return err
		}

		scrcpyPath, err := a.binSvc.GetManagedBinaryPath(binary.BinaryNameScrcpy)
		if err != nil {
			return err
		}

		a.mu.Lock()
		defer a.mu.Unlock()

		oldPath := a.cfg.ScrcpyPath
		oldSetupCompleted := a.cfg.SetupCompleted
		oldVersion := a.cfg.BinaryVersions[binary.BinaryNameScrcpy]
		hadVersion := false
		if a.cfg.BinaryVersions != nil {
			_, hadVersion = a.cfg.BinaryVersions[binary.BinaryNameScrcpy]
		}

		if err := a.binSvc.SetCustomBinary(a.cfg, binary.BinaryNameScrcpy, scrcpyPath); err != nil {
			return err
		}
		if oldSetupCompleted {
			if _, err := a.binSvc.CompleteSetup(a.cfg); err != nil {
				a.cfg.ScrcpyPath = oldPath
				a.cfg.SetupCompleted = oldSetupCompleted
				if hadVersion {
					a.cfg.BinaryVersions[binary.BinaryNameScrcpy] = oldVersion
				} else {
					delete(a.cfg.BinaryVersions, binary.BinaryNameScrcpy)
				}
				return err
			}
		}
		if err := saveManagedBinaryConfig(a); err != nil {
			a.cfg.ScrcpyPath = oldPath
			a.cfg.SetupCompleted = oldSetupCompleted
			if hadVersion {
				a.cfg.BinaryVersions[binary.BinaryNameScrcpy] = oldVersion
			} else {
				delete(a.cfg.BinaryVersions, binary.BinaryNameScrcpy)
			}
			return err
		}
		return nil
	})
}

func saveManagedBinaryConfig(a *App) error {
	return core.SaveConfig(a.dataDir, a.cfg)
}

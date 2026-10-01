package app

import packagemgr "ADBKit/internal/package_mgr"

// The *ForDevice twins below are additive: every legacy serial-less binding stays
// untouched, and each twin admits one caller-confirmed device and then runs the
// whole operation (batch loops included) on a service pinned to that device and
// its tool paths. The pinned service keeps the Apps user semantics: --user is
// emitted only when a user was explicitly requested.

func (a *App) ListPackagesForDevice(expectedSerial string, filterType string) ([]packagemgr.Info, error) {
	return auditAction(a, "list_packages", func() ([]packagemgr.Info, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return nil, err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).ListPackages(a.ctx, filterType)
	})
}

func (a *App) InstallPackageWithModeForDevice(expectedSerial string, filePath string, mode string) (string, error) {
	return auditAction(a, "install_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).InstallPackageWithMode(a.ctx, filePath, mode)
	})
}

func (a *App) InstallPackagesWithModeForDevice(expectedSerial string, filePaths []string, mode string) (string, error) {
	return auditAction(a, "install_multiple_packages", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).InstallPackagesWithMode(a.ctx, filePaths, mode)
	})
}

func (a *App) UninstallPackageForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "uninstall_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).UninstallPackage(a.ctx, packageName)
	})
}

func (a *App) UninstallMultiplePackagesForDevice(expectedSerial string, packageNames []string) (string, error) {
	return auditAction(a, "uninstall_packages", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).UninstallMultiplePackages(a.ctx, packageNames)
	})
}

func (a *App) EnablePackageForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "enable_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).EnablePackage(a.ctx, packageName)
	})
}

func (a *App) EnableMultiplePackagesForDevice(expectedSerial string, packageNames []string) (string, error) {
	return auditAction(a, "enable_packages", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).EnableMultiplePackages(a.ctx, packageNames)
	})
}

func (a *App) DisablePackageForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "disable_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).DisablePackage(a.ctx, packageName)
	})
}

func (a *App) DisableMultiplePackagesForDevice(expectedSerial string, packageNames []string) (string, error) {
	return auditAction(a, "disable_packages", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).DisableMultiplePackages(a.ctx, packageNames)
	})
}

func (a *App) ClearPackageDataForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "clear_package_data", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).ClearPackageData(a.ctx, packageName)
	})
}

func (a *App) PullPackageApkForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "pull_package_apk", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).PullPackageApk(a.ctx, packageName)
	})
}

func (a *App) LaunchPackageForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "launch_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).LaunchPackage(a.ctx, packageName)
	})
}

func (a *App) ForceStopPackageForDevice(expectedSerial string, packageName string) (string, error) {
	return auditAction(a, "force_stop_package", func() (string, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return "", err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).ForceStopPackage(a.ctx, packageName)
	})
}

func (a *App) GetPackageDetailsForDevice(expectedSerial string, packageName string) (packagemgr.Details, error) {
	return auditAction(a, "get_package_details", func() (packagemgr.Details, error) {
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return packagemgr.Details{}, err
		}
		return a.pkgSvc.ForTargetKeepUser(serial).GetPackageDetails(a.ctx, packageName)
	})
}

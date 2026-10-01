package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/file"
	packagemgr "ADBKit/internal/package_mgr"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// newTargetTestApp wires an App whose package and file services record every tool
// resolution, so a refused target can be proven to issue no command at all.
func newTargetTestApp(t *testing.T, activeSerial string, getBinPath func() core.BinaryPaths) *App {
	t.Helper()
	a, _ := newTestApp(t)
	a.ctx = context.Background()
	a.activeSerial = activeSerial
	a.pkgSvc = packagemgr.NewService(a.resolveActiveSerial, nil, getBinPath)
	a.fileSvc = file.NewService(a.ctx, a.resolveActiveSerial, getBinPath)
	return a
}

func isDeviceTargetError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "Confirmed device is required") || strings.Contains(text, "Confirmed device changed")
}

// TestDeviceTargetAdmissionRefusesBlankAndChangedSerialBeforeTools covers blank
// and stale confirmed serials across the Apps and Files mutations the frontend
// calls: every one must fail before the tool getter runs.
func TestDeviceTargetAdmissionRefusesBlankAndChangedSerialBeforeTools(t *testing.T) {
	resolutions := 0
	missing := filepath.Join(t.TempDir(), "missing-adb")
	a := newTargetTestApp(t, "A", func() core.BinaryPaths {
		resolutions++
		return core.BinaryPaths{Adb: missing, Fastboot: missing}
	})

	cases := []struct {
		name string
		call func() error
	}{
		{name: "list packages blank", call: func() error { _, err := a.ListPackagesForDevice("", "all"); return err }},
		{name: "list packages stale", call: func() error { _, err := a.ListPackagesForDevice("B", "all"); return err }},
		{name: "install stale", call: func() error {
			_, err := a.InstallPackageWithModeForDevice("B", "/tmp/app.apk", "replace")
			return err
		}},
		{name: "install split blank", call: func() error {
			_, err := a.InstallPackagesWithModeForDevice("", []string{"/tmp/split.apk"}, "replace")
			return err
		}},
		{name: "uninstall stale", call: func() error { _, err := a.UninstallPackageForDevice("B", "com.example.app"); return err }},
		{name: "uninstall batch blank", call: func() error {
			_, err := a.UninstallMultiplePackagesForDevice("", []string{"com.example.app"})
			return err
		}},
		{name: "disable batch stale", call: func() error {
			_, err := a.DisableMultiplePackagesForDevice("B", []string{"com.example.app"})
			return err
		}},
		{name: "enable stale", call: func() error { _, err := a.EnablePackageForDevice("B", "com.example.app"); return err }},
		{name: "clear data stale", call: func() error { _, err := a.ClearPackageDataForDevice("B", "com.example.app"); return err }},
		{name: "pull apk blank", call: func() error { _, err := a.PullPackageApkForDevice("", "com.example.app"); return err }},
		{name: "launch stale", call: func() error { _, err := a.LaunchPackageForDevice("B", "com.example.app"); return err }},
		{name: "force stop stale", call: func() error { _, err := a.ForceStopPackageForDevice("B", "com.example.app"); return err }},
		{name: "details stale", call: func() error { _, err := a.GetPackageDetailsForDevice("B", "com.example.app"); return err }},
		{name: "list files blank", call: func() error { _, err := a.ListFilesForDevice("", "/sdcard/", false); return err }},
		{name: "directory size stale", call: func() error { _, err := a.GetDirectorySizeForDevice("B", "/sdcard/"); return err }},
		{name: "storage blank", call: func() error { _, err := a.GetStorageInfoForDevice(" "); return err }},
		{name: "sd cards stale", call: func() error { _, err := a.ListSdCardsForDevice("B"); return err }},
		{name: "unblock stale", call: func() error { _, err := a.UnblockPathForDevice("B", "/sdcard/"); return err }},
		{name: "delete stale", call: func() error { _, err := a.DeleteFileForDevice("B", "/sdcard/one"); return err }},
		{name: "delete batch blank", call: func() error {
			_, err := a.DeleteMultipleFilesForDevice("", []string{"/sdcard/one"})
			return err
		}},
		{name: "mkdir stale", call: func() error { _, err := a.CreateDirectoryForDevice("B", "/sdcard/new"); return err }},
		{name: "rename stale", call: func() error { _, err := a.RenameFileForDevice("B", "/sdcard/one", "/sdcard/two"); return err }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !isDeviceTargetError(err) {
				t.Fatalf("expected a device-target refusal, got %v", err)
			}
			opErr, ok := err.(*core.OperationError)
			if !ok || opErr.Operation != "device_target" || opErr.Retryable {
				t.Fatalf("unexpected error shape: %#v", err)
			}
		})
	}

	if resolutions != 0 {
		t.Fatalf("refused targets resolved tools %d time(s)", resolutions)
	}
}

// TestDeviceTargetAdmissionBindsConfirmedSerial proves an admitted call reaches the
// pinned service and keeps the tool snapshot taken at admission: the getter
// returns a different tool on its next call, and the operation must still use the
// first one.
func TestDeviceTargetAdmissionBindsConfirmedSerial(t *testing.T) {
	resolutions := 0
	pinned := filepath.Join(t.TempDir(), "adb-pinned")
	switched := filepath.Join(t.TempDir(), "adb-switched")
	a := newTargetTestApp(t, "A", func() core.BinaryPaths {
		resolutions++
		if resolutions == 1 {
			return core.BinaryPaths{Adb: pinned, Fastboot: pinned}
		}
		return core.BinaryPaths{Adb: switched, Fastboot: switched}
	})

	cases := []struct {
		name string
		call func() (string, error)
	}{
		{name: "list packages", call: func() (string, error) {
			_, err := a.ListPackagesForDevice("A", "all")
			return "", err
		}},
		{name: "uninstall batch", call: func() (string, error) {
			return a.UninstallMultiplePackagesForDevice("A", []string{"com.example.app"})
		}},
		{name: "list files", call: func() (string, error) {
			_, err := a.ListFilesForDevice("A", "/sdcard/", false)
			return "", err
		}},
		{name: "delete files", call: func() (string, error) {
			return a.DeleteMultipleFilesForDevice("A", []string{"/sdcard/one"})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolutions = 0
			text, err := tc.call()
			if isDeviceTargetError(err) {
				t.Fatalf("an admitted target was refused: %v", err)
			}
			if err != nil {
				text += " " + err.Error()
			}
			if text == "" {
				t.Fatal("the operation reported neither a result nor an error")
			}
			if strings.Contains(text, filepath.Base(switched)) {
				t.Fatalf("the operation re-read the tool paths after admission: %s", text)
			}
			if !strings.Contains(text, filepath.Base(pinned)) {
				t.Fatalf("the operation did not use the pinned tool: %s", text)
			}
			if resolutions == 0 {
				t.Fatal("the operation never resolved the pinned tools")
			}
		})
	}
}

// TestDeviceTargetTwinsExposeConfirmedSerialFirst is the compile-time contract for
// the frontend: every twin takes the caller-confirmed serial first, and the legacy
// bindings (including the existing transfer twins) keep their signatures.
func TestDeviceTargetTwinsExposeConfirmedSerialFirst(t *testing.T) {
	a := &App{}

	var (
		_ func(string, string) ([]packagemgr.Info, error)  = a.ListPackagesForDevice
		_ func(string, string, string) (string, error)     = a.InstallPackageWithModeForDevice
		_ func(string, []string, string) (string, error)   = a.InstallPackagesWithModeForDevice
		_ func(string, string) (string, error)             = a.UninstallPackageForDevice
		_ func(string, []string) (string, error)           = a.UninstallMultiplePackagesForDevice
		_ func(string, string) (string, error)             = a.EnablePackageForDevice
		_ func(string, []string) (string, error)           = a.EnableMultiplePackagesForDevice
		_ func(string, string) (string, error)             = a.DisablePackageForDevice
		_ func(string, []string) (string, error)           = a.DisableMultiplePackagesForDevice
		_ func(string, string) (string, error)             = a.ClearPackageDataForDevice
		_ func(string, string) (string, error)             = a.PullPackageApkForDevice
		_ func(string, string) (string, error)             = a.LaunchPackageForDevice
		_ func(string, string) (string, error)             = a.ForceStopPackageForDevice
		_ func(string, string) (packagemgr.Details, error) = a.GetPackageDetailsForDevice
		_ func(string, string, bool) ([]file.Entry, error) = a.ListFilesForDevice
		_ func(string, string) (string, error)             = a.GetDirectorySizeForDevice
		_ func(string) (file.StorageInfo, error)           = a.GetStorageInfoForDevice
		_ func(string) ([]file.SdCard, error)              = a.ListSdCardsForDevice
		_ func(string, string) (file.UnblockResult, error) = a.UnblockPathForDevice
		_ func(string, string) (string, error)             = a.DeleteFileForDevice
		_ func(string, []string) (string, error)           = a.DeleteMultipleFilesForDevice
		_ func(string, string) (string, error)             = a.CreateDirectoryForDevice
		_ func(string, string, string) (string, error)     = a.RenameFileForDevice
	)

	// Legacy serial-less bindings and the existing transfer twins stay available.
	var (
		_ func(string) ([]packagemgr.Info, error)                          = a.ListPackages
		_ func(string, string) (string, error)                             = a.InstallPackageWithMode
		_ func(string) (string, error)                                     = a.UninstallPackage
		_ func([]string) (string, error)                                   = a.UninstallMultiplePackages
		_ func(string, bool) ([]file.Entry, error)                         = a.ListFiles
		_ func([]string) (string, error)                                   = a.DeleteMultipleFiles
		_ func(string, string, string) (string, error)                     = a.PushFileForDevice
		_ func(string, string, string) (string, error)                     = a.PullFileForDevice
		_ func(string, []string, string) (file.TransferBatchResult, error) = a.PushMultipleFilesDetailed
	)
}

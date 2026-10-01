package packagemgr

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	filterUser   Filter = "user"
	filterSystem Filter = "system"
	filterAll    Filter = "all"

	installPackageTimeout = 15 * time.Minute
	pullPackageTimeout    = 10 * time.Minute
	packageDetailsTimeout = 10 * time.Second
)

type Service struct {
	resolveActiveSerial func(context.Context) (string, error)
	selectSaveFile      func(string) (string, error)
	getBinPath          func() core.BinaryPaths
	userID              *int
	runCommand          func(context.Context, core.ExecRequest) (*core.ExecResult, error)
}

// ForTarget captures device, Android user and tool paths for an operation.
// The returned service never consults the mutable global device selection.
func (s *Service) ForTarget(serial string, userID int) *Service {
	paths := s.getBinPath()
	return &Service{
		resolveActiveSerial: func(context.Context) (string, error) { return serial, nil },
		selectSaveFile:      s.selectSaveFile,
		getBinPath:          func() core.BinaryPaths { return paths },
		userID:              &userID,
		runCommand:          s.runCommand,
	}
}

// ForTargetKeepUser pins the same device and tool paths as ForTarget without
// forcing an Android user scope: --user is emitted only when a user was
// explicitly requested, which is what the Apps surface expects. A blank serial is
// refused by the pinned resolver, so no command runs for an unconfirmed target.
func (s *Service) ForTargetKeepUser(serial string) *Service {
	paths := s.getBinPath()
	trimmed := strings.TrimSpace(serial)
	return &Service{
		resolveActiveSerial: func(context.Context) (string, error) {
			if trimmed == "" {
				return "", core.NewOperationError("device_target", "Confirmed device is required", "select and confirm an ADB device", false)
			}
			return trimmed, nil
		},
		selectSaveFile: s.selectSaveFile,
		getBinPath:     func() core.BinaryPaths { return paths },
		runCommand:     s.runCommand,
	}
}

func (s *Service) execute(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
	if s.runCommand != nil {
		return s.runCommand(ctx, req)
	}
	return core.RunCommand(ctx, req)
}

func NewService(
	resolveActiveSerial func(context.Context) (string, error),
	selectSaveFile func(string) (string, error),
	getBinPath func() core.BinaryPaths,
) *Service {
	return &Service{
		resolveActiveSerial: resolveActiveSerial,
		selectSaveFile:      selectSaveFile,
		getBinPath:          getBinPath,
	}
}

func (s *Service) requireActiveSerial(ctx context.Context) (string, error) {
	if s.resolveActiveSerial == nil {
		return "", core.NewOperationError("resolve_active_serial", "Device selection is unavailable", "active serial resolver is not configured", false)
	}

	serial, err := s.resolveActiveSerial(ctx)
	if err != nil {
		return "", core.NewOperationError("resolve_active_serial", "No active device is available", err.Error(), true)
	}

	return strings.TrimSpace(serial), nil
}

func (s *Service) InstallPackage(ctx context.Context, filePath string) (string, error) {
	// Preserve the original ADBKit behaviour for callers that do not expose
	// an install mode: replace/update the existing package while keeping data.
	return s.InstallPackageWithMode(ctx, filePath, string(InstallModeReplace))
}

func (s *Service) LaunchPackage(ctx context.Context, packageName string) (string, error) {
	trimmedName, err := validatePackageName(packageName)
	if err != nil {
		return "", err
	}

	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return "", err
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args: []string{
			"-s", serial, "shell", "monkey",
			"-p", trimmedName,
			"-c", "android.intent.category.LAUNCHER",
			"1",
		},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return "", core.NewOperationError("launch_package", "Failed to launch package", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("launch_package", "Failed to launch package", strings.TrimSpace(result.Stderr), true)
	}

	return fallbackMessage(result.Stdout, fmt.Sprintf("Launched %s", trimmedName)), nil
}

func (s *Service) ForceStopPackage(ctx context.Context, packageName string) (string, error) {
	trimmedName, err := validatePackageName(packageName)
	if err != nil {
		return "", err
	}

	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return "", err
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"-s", serial, "shell", "am", "force-stop", trimmedName},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return "", core.NewOperationError("force_stop_package", "Failed to stop package", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("force_stop_package", "Failed to stop package", strings.TrimSpace(result.Stderr), true)
	}

	return fallbackMessage(result.Stdout, fmt.Sprintf("Stopped %s", trimmedName)), nil
}

func (s *Service) ClearPackageData(ctx context.Context, packageName string) (string, error) {
	trimmedName, err := validatePackageName(packageName)
	if err != nil {
		return "", err
	}

	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return "", err
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"-s", serial, "shell", "pm", "clear", trimmedName},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return "", core.NewOperationError("clear_package_data", "Failed to clear package data", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("clear_package_data", "Failed to clear package data", strings.TrimSpace(result.Stderr), true)
	}

	return fallbackMessage(result.Stdout, fmt.Sprintf("Cleared data for %s", trimmedName)), nil
}

package packagemgr

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type InstallMode string

const (
	InstallModeInstall   InstallMode = "install"
	InstallModeReplace   InstallMode = "replace"
	InstallModeDowngrade InstallMode = "downgrade"
)

func (s *Service) InstallPackageWithMode(ctx context.Context, filePath string, mode string) (string, error) {
	trimmedPath := strings.TrimSpace(filePath)
	if err := core.ValidateAPKFile(trimmedPath); err != nil {
		return "", err
	}

	installMode, err := parseInstallMode(mode)
	if err != nil {
		return "", err
	}

	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return "", err
	}

	installCtx, cancel := context.WithTimeout(ctx, installPackageTimeout)
	defer cancel()

	result, err := core.RunCommand(installCtx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    buildInstallArgs(serial, trimmedPath, installMode),
		Timeout: installPackageTimeout,
	})
	if err != nil {
		return "", core.NewOperationError(
			"install_package",
			fmt.Sprintf("Failed to install APK (%s mode)", installMode),
			err.Error(),
			true,
		)
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return "", core.NewOperationError(
			"install_package",
			fmt.Sprintf("Failed to install APK (%s mode)", installMode),
			detail,
			true,
		)
	}

	message := extractFirstLine(result.Stdout)
	if message == "" {
		message = fmt.Sprintf("Installed %s using %s mode", filepath.Base(trimmedPath), installMode)
	}
	return message, nil
}

func (s *Service) InstallPackagesWithMode(ctx context.Context, filePaths []string, mode string) (string, error) {
	if len(filePaths) < 2 {
		return "", core.NewOperationError(
			"install_multiple_packages",
			"Split APK installation requires at least two APK files",
			"select the base APK and all required split APK files",
			false,
		)
	}

	trimmedPaths := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		trimmed := strings.TrimSpace(filePath)
		if err := core.ValidateAPKFile(trimmed); err != nil {
			return "", err
		}
		trimmedPaths = append(trimmedPaths, trimmed)
	}

	installMode, err := parseInstallMode(mode)
	if err != nil {
		return "", err
	}

	serial, err := s.requireActiveSerial(ctx)
	if err != nil {
		return "", err
	}

	installCtx, cancel := context.WithTimeout(ctx, installPackageTimeout)
	defer cancel()

	result, err := core.RunCommand(installCtx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    buildInstallMultipleArgs(serial, trimmedPaths, installMode),
		Timeout: installPackageTimeout,
	})
	if err != nil {
		return "", core.NewOperationError(
			"install_multiple_packages",
			fmt.Sprintf("Failed to install split APKs (%s mode)", installMode),
			err.Error(),
			true,
		)
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return "", core.NewOperationError(
			"install_multiple_packages",
			fmt.Sprintf("Failed to install split APKs (%s mode)", installMode),
			detail,
			true,
		)
	}

	message := extractFirstLine(result.Stdout)
	if message == "" {
		message = fmt.Sprintf("Installed %d APK files using %s mode", len(trimmedPaths), installMode)
	}
	return message, nil
}

func buildInstallMultipleArgs(serial string, filePaths []string, mode InstallMode) []string {
	args := []string{"-s", serial, "install-multiple"}
	switch mode {
	case InstallModeReplace:
		args = append(args, "-r")
	case InstallModeDowngrade:
		args = append(args, "-r", "-d")
	case InstallModeInstall:
		// No flags: fail if the package is already installed.
	}
	return append(args, filePaths...)
}

func parseInstallMode(mode string) (InstallMode, error) {
	normalized := InstallMode(strings.ToLower(strings.TrimSpace(mode)))
	switch normalized {
	case InstallModeInstall, InstallModeReplace, InstallModeDowngrade:
		return normalized, nil
	case "":
		return InstallModeReplace, nil
	default:
		return "", core.NewOperationError(
			"install_package",
			"Unsupported APK install mode",
			fmt.Sprintf("mode %q is not supported", mode),
			false,
		)
	}
}

func buildInstallArgs(serial string, filePath string, mode InstallMode) []string {
	args := []string{"-s", serial, "install"}
	switch mode {
	case InstallModeReplace:
		args = append(args, "-r")
	case InstallModeDowngrade:
		args = append(args, "-r", "-d")
	case InstallModeInstall:
		// No flags: fail if the package already exists.
	}
	return append(args, filePath)
}

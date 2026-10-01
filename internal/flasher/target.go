package flasher

import (
	"ADBKit/internal/core"
	"context"
	"path/filepath"
	"strings"
)

// Target is one immutable fastboot operation target: the caller-confirmed serial
// plus the fastboot executable captured once for the whole operation. Every command
// of that operation goes through this value, so a later configuration change cannot
// swap the tool, and no global service field, mutex or device selection is read or
// written while the operation runs.
//
// It carries only strings on purpose: it is a bound value view, never a copy of a
// service that owns a mutex.
type Target struct {
	fastbootPath string
	serial       string
}

// ForTarget binds a confirmed fastboot serial and the fastboot executable resolved
// at this moment.
//
// The serial must be explicit: fastboot devices are chosen independently from the
// ADB active device, so this constructor deliberately has no fallback to any global
// selection — a blank serial is refused before the tool is resolved.
func (s *FastbootService) ForTarget(confirmedSerial string) (Target, error) {
	serial := strings.TrimSpace(confirmedSerial)
	if serial == "" {
		return Target{}, core.NewOperationError("confirmed_fastboot_target", "confirmed fastboot serial is required", "", false)
	}
	path, err := s.resolveBinaryPath(core.BinaryNameFastboot)
	if err != nil {
		return Target{}, err
	}
	return Target{fastbootPath: path, serial: serial}, nil
}

// Serial reports the serial captured by ForTarget.
func (t Target) Serial() string {
	return t.serial
}

// ToolPath reports the fastboot executable captured by ForTarget.
func (t Target) ToolPath() string {
	return t.fastbootPath
}

// FlashPartition flashes one partition on the captured target. Validation, timeout,
// error and success-message semantics are identical to the legacy method; only the
// serial and the tool path come from the bound target instead of being resolved (or
// defaulted) again.
func (t Target) FlashPartition(ctx context.Context, partition string, filePath string) (string, error) {
	if t.serial == "" || t.fastbootPath == "" {
		return "", core.NewOperationError("flash_partition", "fastboot target is not bound", "", false)
	}
	trimmedPartition := strings.ToLower(strings.TrimSpace(partition))
	if err := core.ValidateFlashPartition(trimmedPartition); err != nil {
		return "", err
	}
	trimmedFilePath := strings.TrimSpace(filePath)
	if err := core.ValidateFlashFile(trimmedFilePath); err != nil {
		return "", err
	}
	flashCtx, cancel := context.WithTimeout(ctx, FlashTimeout)
	defer cancel()
	result, err := core.RunCommand(flashCtx, core.ExecRequest{
		Command: t.fastbootPath,
		Args:    []string{"-s", t.serial, "flash", trimmedPartition, trimmedFilePath},
	})
	if err != nil {
		return "", core.NewOperationError("flash_partition", "failed to flash partition", extractErrorDetail(result, err), true)
	}
	fallback := "Flashed " + trimmedPartition + " from " + filepath.Base(trimmedFilePath)
	return successMessage(result.Stdout, fallback), nil
}

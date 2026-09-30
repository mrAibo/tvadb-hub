package device

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"strings"
)

func (s *Service) RebootDevice(ctx context.Context, serial string, mode string) (string, error) {
	if serial == "" {
		return "", core.NewOperationError("reboot_device", "device serial is required", "serial must not be empty", false)
	}

	mode = strings.TrimSpace(mode)
	// Reject an unknown target before any device command runs. The previous
	// implementation appended the caller's string verbatim, so a typo reached
	// adb/fastboot as a reboot target.
	if !knownRebootMode(mode) {
		return "", core.NewOperationError("reboot_device", "unsupported reboot mode", mode, false)
	}

	connectionMode, err := s.DetectDeviceMode(ctx, serial)
	if err != nil {
		return "", err
	}

	args, err := rebootArgs(connectionMode, serial, mode)
	if err != nil {
		return "", err
	}

	command := s.getBinPath().Adb
	if connectionMode == ModeFastboot {
		command = s.getBinPath().Fastboot
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: command,
		Args:    args,
		Timeout: 10e9,
	})
	if err != nil {
		return "", core.NewOperationError("reboot_device", "failed to reboot device", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("reboot_device", "reboot command failed", strings.TrimSpace(result.Stderr), true)
	}
	return fmt.Sprintf("Reboot command sent to %s (%s)", serial, mode), nil
}

// knownRebootMode is the set of reboot targets DroidSphere accepts at all. The
// dashboard offers system/recovery/bootloader; the remaining values are the
// documented adb targets kept for parity with adb's own reboot argument.
func knownRebootMode(mode string) bool {
	switch mode {
	case "", "system", "bootloader", "recovery", "sideload", "fastboot", "fastbootd":
		return true
	default:
		return false
	}
}

func adbRebootMode(mode string) bool {
	switch mode {
	case "system", "bootloader", "recovery", "sideload", "fastboot":
		return true
	default:
		return false
	}
}

func fastbootRebootMode(mode string) bool {
	switch mode {
	case "bootloader", "recovery", "fastbootd":
		return true
	default:
		return false
	}
}

// rebootArgs returns the exact argument vector for a confirmed target. The serial
// is always pinned with -s, including the fastboot branch, so a second connected
// device can never be selected implicitly. Modes that are valid for adb but not
// for fastboot (or the other way round) are refused before the binary runs.
func rebootArgs(connectionMode Mode, serial string, mode string) ([]string, error) {
	if strings.TrimSpace(serial) == "" {
		return nil, core.NewOperationError("reboot_device", "device serial is required", "serial must not be empty", false)
	}

	switch connectionMode {
	case ModeADB:
		if mode != "" && !adbRebootMode(mode) {
			return nil, unsupportedRebootMode(mode, connectionMode)
		}
		args := []string{"-s", serial, "reboot"}
		if mode != "" {
			args = append(args, mode)
		}
		return args, nil

	case ModeFastboot:
		// fastboot does not accept "system" as a reboot target;
		// bare "fastboot reboot" already boots to system.
		if mode != "" && mode != "system" && !fastbootRebootMode(mode) {
			return nil, unsupportedRebootMode(mode, connectionMode)
		}
		args := []string{"-s", serial, "reboot"}
		if mode != "" && mode != "system" {
			args = append(args, mode)
		}
		return args, nil

	default:
		return nil, core.NewOperationError("reboot_device", "no connected device detected in adb or fastboot mode", fmt.Sprintf("serial '%s' mode is %s", serial, connectionMode), true)
	}
}

func unsupportedRebootMode(mode string, connectionMode Mode) error {
	return core.NewOperationError("reboot_device", "reboot mode is not supported for this device mode", fmt.Sprintf("mode '%s' is not an %s reboot target", mode, connectionMode), false)
}

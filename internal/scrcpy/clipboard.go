package scrcpy

import (
	"ADBKit/internal/core"
	"strings"
	"time"
)

func (s *Service) PushClipboard(serial, text string) error {
	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return core.NewOperationError(
			"push_scrcpy_clipboard",
			"Device serial is required",
			"serial must not be empty",
			false,
		)
	}
	if text == "" {
		return core.NewOperationError(
			"push_scrcpy_clipboard",
			"Clipboard text is required",
			"text must not be empty",
			false,
		)
	}

	adbPath, err := s.resolveADBPath()
	if err != nil {
		return err
	}

	if strings.ContainsRune(text, '\x00') {
		return core.NewOperationError(
			"push_scrcpy_clipboard",
			"Clipboard text is invalid",
			"text contains a null byte",
			false,
		)
	}

	result, runErr := core.RunCommand(s.ctx, core.ExecRequest{
		Command: adbPath,
		Args: []string{
			"-s", trimmedSerial, "shell",
			buildClipboardSetCommand(text),
		},
		Timeout: 5 * time.Second,
	})
	if runErr != nil || result == nil || result.ExitCode != 0 {
		detail := ""
		if result != nil {
			detail = strings.TrimSpace(result.Stderr)
			if detail == "" {
				detail = strings.TrimSpace(result.Stdout)
			}
		}
		if detail == "" && runErr != nil {
			detail = runErr.Error()
		}
		return core.NewOperationError(
			"push_scrcpy_clipboard",
			"Failed to push clipboard to device",
			detail,
			true,
		)
	}

	s.logAudit("push_scrcpy_clipboard", trimmedSerial, true, "")
	return nil
}

func (s *Service) GetClipboard(serial string) (string, error) {
	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return "", core.NewOperationError(
			"get_scrcpy_clipboard",
			"Device serial is required",
			"serial must not be empty",
			false,
		)
	}

	adbPath, err := s.resolveADBPath()
	if err != nil {
		return "", err
	}

	result, runErr := core.RunCommand(s.ctx, core.ExecRequest{
		Command: adbPath,
		Args:    []string{"-s", trimmedSerial, "shell", "cmd clipboard get"},
		Timeout: 5 * time.Second,
	})
	if runErr != nil || result == nil || result.ExitCode != 0 {
		detail := ""
		if result != nil {
			detail = strings.TrimSpace(result.Stderr)
			if detail == "" {
				detail = strings.TrimSpace(result.Stdout)
			}
		}
		if detail == "" && runErr != nil {
			detail = runErr.Error()
		}
		return "", core.NewOperationError(
			"get_scrcpy_clipboard",
			"Failed to read clipboard from device",
			detail,
			true,
		)
	}
	s.logAudit("get_scrcpy_clipboard", trimmedSerial, true, "")
	return strings.TrimRight(result.Stdout, "\r\n"), nil
}

func buildClipboardSetCommand(text string) string {
	return "cmd clipboard set " + core.QuoteShellArg(text)
}

package scrcpy

import (
	"ADBKit/internal/core"
	"strings"
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

	if err := core.SetAndroidClipboard(s.ctx, adbPath, trimmedSerial, text, nil); err != nil {
		return core.NewOperationError("push_scrcpy_clipboard", "Android clipboard shell access is unavailable", err.Error(), true)
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

	text, err := core.ReadAndroidClipboard(s.ctx, adbPath, trimmedSerial, nil)
	if err != nil {
		return "", core.NewOperationError("get_scrcpy_clipboard", "Android clipboard shell access is unavailable", err.Error(), true)
	}

	s.logAudit("get_scrcpy_clipboard", trimmedSerial, true, "")
	return text, nil
}

func buildClipboardSetCommand(text string) string {
	return "cmd clipboard set " + core.QuoteShellArg(text)
}

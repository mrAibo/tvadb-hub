package device

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	TVTextMethodScrcpyClipboard = "scrcpy-clipboard"
	TVTextMethodAndroidClipboard = "android-clipboard"
	TVTextMethodInputText        = "input-text"

	maxTVTextInputBytes = 8 * 1024
)

type TVTextInputResult struct {
	Method string `json:"method"`
	Detail string `json:"detail"`
}

type tvTextCommandRunner func(context.Context, core.ExecRequest) (*core.ExecResult, error)

// SendTVText sends text to the focused field on an ADB-connected Android
// target. Clipboard+PASTE is preferred because it preserves Unicode. Android's
// input text command is used only as a deliberately limited printable-ASCII
// fallback.
func (s *Service) SendTVText(ctx context.Context, serial string, text string) (TVTextInputResult, error) {
	return sendTVText(ctx, s.getBinPath().Adb, serial, text, core.RunCommand)
}

// ValidateTVTextInput applies the same limits to every TV text path, including
// the active-scrcpy fast path in the app facade.
func ValidateTVTextInput(text string) error {
	if text == "" {
		return core.NewOperationError(
			"send_tv_text",
			"text is required",
			"text must not be empty",
			false,
		)
	}
	if len([]byte(text)) > maxTVTextInputBytes {
		return core.NewOperationError(
			"send_tv_text",
			"text is too long",
			fmt.Sprintf("maximum input size is %d bytes", maxTVTextInputBytes),
			false,
		)
	}
	if strings.ContainsRune(text, '\x00') {
		return core.NewOperationError(
			"send_tv_text",
			"text contains an unsupported null byte",
			"null bytes cannot be sent through adb shell",
			false,
		)
	}
	return nil
}

func sendTVText(
	ctx context.Context,
	adbPath string,
	serial string,
	text string,
	run tvTextCommandRunner,
) (TVTextInputResult, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return TVTextInputResult{}, core.NewOperationError(
			"send_tv_text",
			"device serial is required",
			"serial must not be empty",
			false,
		)
	}
	if err := ValidateTVTextInput(text); err != nil {
		return TVTextInputResult{}, err
	}

	clipboardDetail := ""
	setResult, setErr := run(ctx, core.ExecRequest{
		Command: adbPath,
		Args: []string{
			"-s", serial, "shell",
			"cmd clipboard set " + core.QuoteShellArg(text),
		},
		Timeout: 5 * time.Second,
	})
	if commandSucceeded(setResult, setErr) {
		pasteResult, pasteErr := run(ctx, core.ExecRequest{
			Command: adbPath,
			Args:    []string{"-s", serial, "shell", "input keyevent KEYCODE_PASTE"},
			Timeout: 5 * time.Second,
		})
		if commandSucceeded(pasteResult, pasteErr) {
			return TVTextInputResult{
				Method: TVTextMethodAndroidClipboard,
				Detail: "Pasted through the Android clipboard",
			}, nil
		}
		clipboardDetail = commandFailureDetail(pasteResult, pasteErr)
	} else {
		clipboardDetail = commandFailureDetail(setResult, setErr)
	}

	if !isSimpleASCIIInput(text) {
		return TVTextInputResult{}, core.NewOperationError(
			"send_tv_text",
			"clipboard paste is unavailable for this text",
			"Android clipboard/paste failed and input text fallback is limited to printable ASCII without percent signs: "+clipboardDetail,
			true,
		)
	}

	inputCommand := "input text " + core.QuoteShellArg(encodeADBInputText(text))
	inputResult, inputErr := run(ctx, core.ExecRequest{
		Command: adbPath,
		Args:    []string{"-s", serial, "shell", inputCommand},
		Timeout: 5 * time.Second,
	})
	if !commandSucceeded(inputResult, inputErr) {
		return TVTextInputResult{}, core.NewOperationError(
			"send_tv_text",
			"failed to send TV text",
			fmt.Sprintf("clipboard path: %s; input text fallback: %s", clipboardDetail, commandFailureDetail(inputResult, inputErr)),
			true,
		)
	}

	return TVTextInputResult{
		Method: TVTextMethodInputText,
		Detail: "Android clipboard was unavailable; used printable-ASCII input text fallback",
	}, nil
}

func commandSucceeded(result *core.ExecResult, err error) bool {
	return err == nil && result != nil && result.ExitCode == 0
}

func commandFailureDetail(result *core.ExecResult, err error) string {
	if result != nil {
		if detail := strings.TrimSpace(result.Stderr); detail != "" {
			return detail
		}
		if detail := strings.TrimSpace(result.Stdout); detail != "" {
			return detail
		}
		if result.ExitCode != 0 {
			return fmt.Sprintf("exit code %d", result.ExitCode)
		}
	}
	if err != nil {
		return err.Error()
	}
	return "command unavailable"
}

func isSimpleASCIIInput(text string) bool {
	for _, r := range text {
		if r < 0x20 || r > 0x7e || r == '%' {
			return false
		}
	}
	return true
}

func encodeADBInputText(text string) string {
	return strings.ReplaceAll(text, " ", "%s")
}

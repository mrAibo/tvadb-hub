package device

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"strings"
	"time"
)

var tvRemoteKeycodes = map[string]string{
	"home":        "KEYCODE_HOME",
	"back":        "KEYCODE_BACK",
	"up":          "KEYCODE_DPAD_UP",
	"down":        "KEYCODE_DPAD_DOWN",
	"left":        "KEYCODE_DPAD_LEFT",
	"right":       "KEYCODE_DPAD_RIGHT",
	"select":      "KEYCODE_DPAD_CENTER",
	"ok":          "KEYCODE_DPAD_CENTER",
	"enter":       "KEYCODE_DPAD_CENTER",
	"menu":        "KEYCODE_MENU",
	"play_pause":  "KEYCODE_MEDIA_PLAY_PAUSE",
	"playpause":   "KEYCODE_MEDIA_PLAY_PAUSE",
	"volume_up":   "KEYCODE_VOLUME_UP",
	"volume_down": "KEYCODE_VOLUME_DOWN",
	"mute":        "KEYCODE_VOLUME_MUTE",
	"power":       "KEYCODE_POWER",
	"wake":        "KEYCODE_WAKEUP",
	"sleep":       "KEYCODE_SLEEP",
	"paste":       "KEYCODE_PASTE",
}

// SendTVRemoteKey sends one whitelisted Android key event to a connected ADB
// target. Keeping the public input as a semantic key instead of arbitrary shell
// text makes the API safe for a one-click TV remote UI.
func (s *Service) SendTVRemoteKey(ctx context.Context, serial string, key string) (string, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return "", core.NewOperationError(
			"send_tv_remote_key",
			"device serial is required",
			"serial must not be empty",
			false,
		)
	}

	keycode, normalized, ok := resolveTVRemoteKey(key)
	if !ok {
		return "", core.NewOperationError(
			"send_tv_remote_key",
			"unsupported TV remote key",
			fmt.Sprintf("key %q is not supported", strings.TrimSpace(key)),
			false,
		)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"-s", serial, "shell", "input", "keyevent", keycode},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return "", core.NewOperationError(
			"send_tv_remote_key",
			"failed to send TV remote key",
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
			"send_tv_remote_key",
			"TV remote command failed",
			detail,
			true,
		)
	}

	return fmt.Sprintf("Sent %s to %s", normalized, serial), nil
}

func resolveTVRemoteKey(key string) (keycode string, normalized string, ok bool) {
	normalized = strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	keycode, ok = tvRemoteKeycodes[normalized]
	return keycode, normalized, ok
}

package app

import (
	"ADBKit/internal/core"
	"fmt"
	"strings"
)

// expectedActive admits one caller-confirmed device for an operation. A blank
// serial and a serial that no longer matches the live selection are both refused
// before any command runs: the caller must re-confirm the device instead of
// mutating whatever happens to be selected now.
//
// This is an admission check, not a lock. It cannot stop a concurrent device
// switch between two calls, so every call re-checks and each operation captures
// its target and tools once admitted; a later switch never retargets it.
func (a *App) expectedActive(expectedSerial string) (string, error) {
	serial := strings.TrimSpace(expectedSerial)
	if serial == "" {
		return "", core.NewOperationError("device_target", "Confirmed device is required", "select and confirm an ADB device", false)
	}

	active, err := a.resolveActiveSerial(a.ctx)
	if err != nil {
		return "", err
	}
	if active = strings.TrimSpace(active); active != serial {
		return "", core.NewOperationError("device_target", "Confirmed device changed", fmt.Sprintf("expected %s, active %s", serial, active), false)
	}
	return serial, nil
}

package device

import (
	"ADBKit/internal/core"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRebootArgsPinConfirmedSerial is the regression guard for the fastboot
// branch that used to run a bare "fastboot reboot": every argument vector must
// pin the confirmed serial with -s, including the fastboot path.
func TestRebootArgsPinConfirmedSerial(t *testing.T) {
	cases := []struct {
		name           string
		connectionMode Mode
		mode           string
		want           []string
	}{
		{name: "adb default", connectionMode: ModeADB, want: []string{"-s", "SERIAL-1", "reboot"}},
		{name: "adb system", connectionMode: ModeADB, mode: "system", want: []string{"-s", "SERIAL-1", "reboot", "system"}},
		{name: "adb recovery", connectionMode: ModeADB, mode: "recovery", want: []string{"-s", "SERIAL-1", "reboot", "recovery"}},
		{name: "adb bootloader", connectionMode: ModeADB, mode: "bootloader", want: []string{"-s", "SERIAL-1", "reboot", "bootloader"}},
		{name: "fastboot default", connectionMode: ModeFastboot, want: []string{"-s", "SERIAL-1", "reboot"}},
		{name: "fastboot system keeps bare reboot", connectionMode: ModeFastboot, mode: "system", want: []string{"-s", "SERIAL-1", "reboot"}},
		{name: "fastboot recovery", connectionMode: ModeFastboot, mode: "recovery", want: []string{"-s", "SERIAL-1", "reboot", "recovery"}},
		{name: "fastboot bootloader", connectionMode: ModeFastboot, mode: "bootloader", want: []string{"-s", "SERIAL-1", "reboot", "bootloader"}},
		{name: "fastboot fastboot target", connectionMode: ModeFastboot, mode: "fastboot", want: []string{"-s", "SERIAL-1", "reboot", "fastboot"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rebootArgs(tc.connectionMode, "SERIAL-1", tc.mode)
			if err != nil {
				t.Fatalf("rebootArgs(%s, %q) = %v", tc.connectionMode, tc.mode, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("args = %#v, want %#v", got, tc.want)
			}
			if len(got) < 2 || got[0] != "-s" || got[1] != "SERIAL-1" {
				t.Fatalf("serial is not pinned: %#v", got)
			}
		})
	}
}

// TestRebootArgsRejectModesUnsupportedForConnectionMode keeps the documented
// per-mode target sets: a target valid for one tool is refused before the other
// tool runs, instead of being forwarded. "fastbootd" names the daemon/mode, not a
// target, so it is refused for both tools and the native "fastboot" target is the
// documented way to enter fastbootd.
func TestRebootArgsRejectModesUnsupportedForConnectionMode(t *testing.T) {
	cases := []struct {
		name           string
		connectionMode Mode
		mode           string
	}{
		{name: "sideload is not a fastboot target", connectionMode: ModeFastboot, mode: "sideload"},
		{name: "fastbootd is a daemon name, not a fastboot target", connectionMode: ModeFastboot, mode: "fastbootd"},
		{name: "fastbootd is not an adb target", connectionMode: ModeADB, mode: "fastbootd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := rebootArgs(tc.connectionMode, "SERIAL-1", tc.mode)
			if err == nil || args != nil {
				t.Fatalf("rebootArgs(%s, %q) = %#v, %v; want a refusal", tc.connectionMode, tc.mode, args, err)
			}
			opErr, ok := err.(*core.OperationError)
			if !ok || opErr.Operation != "reboot_device" || !strings.Contains(opErr.Message, "not supported") {
				t.Fatalf("unexpected error shape: %v", err)
			}
		})
	}
}

func TestRebootArgsRejectEmptySerialAndUnknownConnectionMode(t *testing.T) {
	if _, err := rebootArgs(ModeADB, "   ", ""); err == nil || !strings.Contains(err.Error(), "device serial is required") {
		t.Fatalf("empty serial accepted: %v", err)
	}
	if _, err := rebootArgs(ModeUnknown, "SERIAL-1", ""); err == nil || !strings.Contains(err.Error(), "no connected device detected") {
		t.Fatalf("unknown connection mode accepted: %v", err)
	}
}

// TestRebootDeviceRejectsUnknownModeBeforeAnyDeviceCommand proves the ordering:
// an unsupported mode must be refused even when device detection cannot succeed,
// so no adb/fastboot command was attempted for it. "fastbootd" is included because
// it is a daemon name that must never be forwarded to a binary.
func TestRebootDeviceRejectsUnknownModeBeforeAnyDeviceCommand(t *testing.T) {
	svc := rebootTestService(t)

	for _, mode := range []string{"wipe-everything", "fastbootd"} {
		t.Run("mode="+mode, func(t *testing.T) {
			_, err := svc.RebootDevice(context.Background(), "SERIAL-1", mode)
			if err == nil {
				t.Fatal("expected the unknown reboot mode to be rejected")
			}
			opErr, ok := err.(*core.OperationError)
			if !ok || opErr.Operation != "reboot_device" || opErr.Message != "unsupported reboot mode" || opErr.Detail != mode {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestRebootDeviceKeepsLegacyFlowForKnownModes guards the opposite direction: a
// dashboard mode must still reach device detection instead of being refused by
// the new mode guard.
func TestRebootDeviceKeepsLegacyFlowForKnownModes(t *testing.T) {
	svc := rebootTestService(t)

	for _, mode := range []string{"", "system", "recovery", "bootloader"} {
		t.Run("mode="+mode, func(t *testing.T) {
			_, err := svc.RebootDevice(context.Background(), "SERIAL-1", mode)
			if err == nil {
				t.Fatal("expected device detection to fail for a non-existent adb")
			}
			if strings.Contains(err.Error(), "unsupported reboot mode") ||
				strings.Contains(err.Error(), "not supported for this device mode") {
				t.Fatalf("known mode %q was rejected before detection: %v", mode, err)
			}
		})
	}
}

// rebootTestService points both tools at paths that cannot be executed, so any
// attempted device command fails deterministically instead of touching hardware.
func rebootTestService(t *testing.T) *Service {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "missing-adb")
	return &Service{
		getBinPath: func() core.BinaryPaths {
			return core.BinaryPaths{Adb: missing, Fastboot: missing + "-fastboot"}
		},
	}
}

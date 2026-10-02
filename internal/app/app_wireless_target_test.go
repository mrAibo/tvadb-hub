package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"path/filepath"
	"testing"
)

// TestEnableWirelessTCPIPRequiresExplicitConfirmedSerial is the regression guard for
// the implicit activeSerial fallback: a blank or stale caller-confirmed serial must
// be refused before any tool is resolved or any command is built.
func TestEnableWirelessTCPIPRequiresExplicitConfirmedSerial(t *testing.T) {
	resolutions := 0
	missing := filepath.Join(t.TempDir(), "missing-adb")
	a, _ := newTestApp(t)
	a.ctx = context.Background()
	a.activeSerial = "A"
	a.devSvc = device.NewService(a.dataDir, func() core.BinaryPaths {
		resolutions++
		return core.BinaryPaths{Adb: missing, Fastboot: missing}
	})
	a.wireSvc = device.NewWirelessService(a.dataDir, func() core.BinaryPaths {
		resolutions++
		return core.BinaryPaths{Adb: missing}
	})

	for _, serial := range []string{"", "   ", "B"} {
		_, err := a.EnableWirelessTCPIP("5555", serial)
		if !isDeviceTargetError(err) {
			t.Fatalf("EnableWirelessTCPIP(serial=%q) error = %v, want a device-target refusal", serial, err)
		}
		opErr, ok := err.(*core.OperationError)
		if !ok || opErr.Operation != "device_target" || opErr.Retryable {
			t.Fatalf("unexpected error shape for serial %q: %#v", serial, err)
		}
	}

	if resolutions != 0 {
		t.Fatalf("refused wireless targets resolved tools %d time(s)", resolutions)
	}
}

// TestEnableWirelessTCPIPProvesTargetReadyBeforeRunningWirelessCommand pins the
// order: when the live device list cannot prove the confirmed device is a ready ADB
// device, the wireless command must never run.
func TestEnableWirelessTCPIPProvesTargetReadyBeforeRunningWirelessCommand(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-adb")
	a, _ := newTestApp(t)
	a.ctx = context.Background()
	a.activeSerial = "A"
	a.devSvc = device.NewService(a.dataDir, func() core.BinaryPaths {
		return core.BinaryPaths{Adb: missing, Fastboot: missing}
	})
	a.wireSvc = device.NewWirelessService(a.dataDir, func() core.BinaryPaths {
		t.Fatal("the wireless command ran before the confirmed target was proven ready")
		return core.BinaryPaths{}
	})

	if _, err := a.EnableWirelessTCPIP("5555", "A"); err == nil {
		t.Fatal("expected an unverifiable target to be refused")
	}
}

func TestConfirmedReadyADBTarget(t *testing.T) {
	devices := []device.Summary{
		{Serial: "READY", State: device.StateReady, Mode: device.ModeADB},
		{Serial: "UNAUTH", State: device.StateUnauthorized, Mode: device.ModeADB},
		{Serial: "OFFLINE", State: device.StateOffline, Mode: device.ModeADB},
		{Serial: "FASTBOOT", State: device.StateFastboot, Mode: device.ModeFastboot},
	}

	cases := []struct {
		name    string
		serial  string
		wantErr bool
	}{
		{name: "ready adb device", serial: "READY"},
		{name: "unauthorized device", serial: "UNAUTH", wantErr: true},
		{name: "offline device", serial: "OFFLINE", wantErr: true},
		{name: "fastboot-only device", serial: "FASTBOOT", wantErr: true},
		{name: "device not connected", serial: "MISSING", wantErr: true},
		{name: "blank serial", serial: "", wantErr: true},
		{name: "whitespace serial", serial: "   ", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := confirmedReadyADBTarget(tc.serial, devices)
			if (err != nil) != tc.wantErr {
				t.Fatalf("confirmedReadyADBTarget(%q) error = %v, wantErr %v", tc.serial, err, tc.wantErr)
			}
			if err == nil {
				return
			}
			opErr, ok := err.(*core.OperationError)
			if !ok || opErr.Operation != "enable_wireless_tcpip" {
				t.Fatalf("unexpected error shape: %#v", err)
			}
		})
	}
}

// TestEnableWirelessTCPIPBindingContract keeps the facade signature the frontend
// already calls: port first, caller-confirmed serial second.
func TestEnableWirelessTCPIPBindingContract(t *testing.T) {
	a := &App{}
	var _ func(string, string) (string, error) = a.EnableWirelessTCPIP
}

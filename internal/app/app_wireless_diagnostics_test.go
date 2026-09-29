package app

import (
	"ADBKit/internal/device"
	"testing"
)

func TestAdbVersionDiagnostic(t *testing.T) {
	cases := []struct {
		name    string
		version string
		status  WirelessDiagnosticStatus
	}{
		{"modern", "Android Debug Bridge version 1.0.41 (Version 37.0.1-13742732)", WirelessDiagnosticPass},
		{"older-but-supported", "Android Debug Bridge version 1.0.41 (Version 31.0.3-7562133)", WirelessDiagnosticWarning},
		{"too-old", "Android Debug Bridge version 1.0.41 (Version 29.0.6-6198805)", WirelessDiagnosticFail},
		{"unknown", "unparseable", WirelessDiagnosticWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := adbVersionDiagnostic(tc.version)
			if got.Status != tc.status {
				t.Fatalf("status = %q, want %q", got.Status, tc.status)
			}
		})
	}
}

func TestResolveDiagnosticDevice(t *testing.T) {
	devices := []device.DiscoveredWirelessDevice{
		{
			DiscoveryKey:    "adb-tv",
			Host:            "192.168.178.99",
			InstanceNames:   []string{"adb-tv"},
			ConnectAddress:  "192.168.178.99:38219",
			PreferredAddress:"192.168.178.99:38219",
		},
		{
			DiscoveryKey:    "adb-phone",
			Host:            "192.168.178.37",
			InstanceNames:   []string{"adb-phone"},
			LegacyAddress:   "192.168.178.37:5555",
			PreferredAddress:"192.168.178.37:5555",
		},
	}

	if _, err := resolveDiagnosticDevice(devices, ""); err == nil {
		t.Fatal("expected selector to be required for multiple hosts")
	}

	got, err := resolveDiagnosticDevice(devices, "192.168.178.99")
	if err != nil {
		t.Fatalf("resolve by host failed: %v", err)
	}
	if got.ConnectAddress != "192.168.178.99:38219" {
		t.Fatalf("unexpected resolved device: %#v", got)
	}

	got, err = resolveDiagnosticDevice(devices, "adb-phone")
	if err != nil {
		t.Fatalf("resolve by instance failed: %v", err)
	}
	if got.Host != "192.168.178.37" {
		t.Fatalf("unexpected instance resolution: %#v", got)
	}
}

func TestAdbStateForHost(t *testing.T) {
	devices := []device.Summary{
		{Serial: "192.168.178.99:38219", State: device.StateReady, Mode: device.ModeADB},
		{Serial: "ABC123", State: device.StateReady, Mode: device.ModeADB},
	}

	state, serial := adbStateForHost(devices, "192.168.178.99")
	if state != device.StateReady || serial != "192.168.178.99:38219" {
		t.Fatalf("unexpected state resolution: %q %q", state, serial)
	}
}

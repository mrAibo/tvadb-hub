package app

import (
	"ADBKit/internal/device"
	"testing"
)

func TestNetworkADBHost(t *testing.T) {
	cases := []struct {
		serial string
		host   string
		ok     bool
	}{
		{"192.168.1.50:37121", "192.168.1.50", true},
		{"[fe80::1234]:37121", "fe80::1234", true},
		{"ABC123", "", false},
		{"emulator-5554", "", false},
		{"192.168.1.50:notaport", "", false},
	}
	for _, tc := range cases {
		host, ok := networkADBHost(tc.serial)
		if host != tc.host || ok != tc.ok {
			t.Fatalf("networkADBHost(%q) = %q,%v want %q,%v", tc.serial, host, ok, tc.host, tc.ok)
		}
	}
}

func TestSelectDoctorTarget(t *testing.T) {
	devices := []device.Summary{
		{Serial: "ABC", State: device.StateReady, Mode: device.ModeADB},
		{Serial: "192.168.1.5:37121", State: device.StateReady, Mode: device.ModeADB},
	}
	got, ok := selectDoctorTarget(devices, "ABC")
	if !ok || got.Serial != "ABC" {
		t.Fatalf("active target resolution failed: %#v %v", got, ok)
	}
	if _, ok := selectDoctorTarget(devices, ""); ok {
		t.Fatal("multiple devices without active serial must be ambiguous")
	}
}

func TestConnectionDoctorReportCountsFailures(t *testing.T) {
	report := ConnectionDoctorReport{Healthy: true}
	report.add(ConnectionDoctorCheck{Status: ConnectionDoctorPass})
	report.add(ConnectionDoctorCheck{Status: ConnectionDoctorWarning})
	report.add(ConnectionDoctorCheck{Status: ConnectionDoctorFail})
	report.add(ConnectionDoctorCheck{Status: ConnectionDoctorInfo})

	if report.Healthy {
		t.Fatal("report with a failure must not be healthy")
	}
	if report.PassCount != 1 || report.WarningCount != 1 || report.FailCount != 1 || report.InfoCount != 1 {
		t.Fatalf("unexpected counts: %#v", report)
	}
}

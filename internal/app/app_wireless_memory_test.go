package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"testing"
)

func TestResolveRememberedWirelessService_PrefersInstanceIdentity(t *testing.T) {
	entry := core.RememberedWirelessDevice{
		Key:          "adb-tv-stable",
		InstanceName: "adb-tv-stable",
		Host:         "192.168.1.20",
		AutoConnect:  true,
	}
	services := []device.MDNSService{
		{
			InstanceName: "adb-tv-stable",
			Kind:         device.MDNSServiceConnect,
			Address:      "192.168.1.77:41000",
			Host:         "192.168.1.77",
			Port:         "41000",
			Secure:       true,
		},
		{
			InstanceName: "other",
			Kind:         device.MDNSServiceConnect,
			Address:      "192.168.1.20:42000",
			Host:         "192.168.1.20",
			Port:         "42000",
			Secure:       true,
		},
	}

	got, ok := resolveRememberedWirelessService(services, entry)
	if !ok {
		t.Fatal("expected remembered service to resolve")
	}
	if got.Host != "192.168.1.77" {
		t.Fatalf("expected instance match to survive IP change, got %#v", got)
	}
}

func TestResolveRememberedWirelessService_PrefersTLSHostFallback(t *testing.T) {
	entry := core.RememberedWirelessDevice{
		Key:         "old-instance",
		Host:        "192.168.1.20",
		AutoConnect: true,
	}
	services := []device.MDNSService{
		{
			InstanceName: "legacy",
			Kind:         device.MDNSServiceLegacy,
			Address:      "192.168.1.20:5555",
			Host:         "192.168.1.20",
			Port:         "5555",
		},
		{
			InstanceName: "new-instance",
			Kind:         device.MDNSServiceConnect,
			Address:      "192.168.1.20:43000",
			Host:         "192.168.1.20",
			Port:         "43000",
			Secure:       true,
		},
	}

	got, ok := resolveRememberedWirelessService(services, entry)
	if !ok {
		t.Fatal("expected host fallback")
	}
	if got.Kind != device.MDNSServiceConnect {
		t.Fatalf("expected TLS connect service, got %#v", got)
	}
}

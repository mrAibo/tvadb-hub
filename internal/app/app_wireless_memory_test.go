package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"testing"
	"time"
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


func TestFindRememberedWirelessMatch_UsesHardwareSerialAcrossNetworkChanges(t *testing.T) {
	entries := []core.RememberedWirelessDevice{
		{
			Key:            "serial:TVSERIAL123",
			InstanceName:   "old-mdns-name",
			Host:           "192.168.1.20",
			HardwareSerial: "TVSERIAL123",
			AutoConnect:    true,
		},
	}
	service := device.MDNSService{
		InstanceName: "new-mdns-name",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.77:43000",
		Host:         "192.168.1.77",
		Port:         "43000",
		Secure:       true,
	}
	info := &device.Info{HardwareSerial: "TVSERIAL123", Model: "Living Room TV"}

	if got := findRememberedWirelessMatch(entries, service, info); got != 0 {
		t.Fatalf("findRememberedWirelessMatch() = %d, want 0", got)
	}
}

func TestBuildRememberedWirelessEntry_EnrichesIdentity(t *testing.T) {
	service := device.MDNSService{
		InstanceName: "adb-tv",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.178.99:38219",
		Host:         "192.168.178.99",
		Port:         "38219",
		Secure:       true,
	}
	info := &device.Info{
		HardwareSerial: "TVSERIAL123",
		Model:          "Google TV Streamer",
		Manufacturer:   "Google",
		AndroidVersion: "14",
	}
	seenAt := time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC)

	got := buildRememberedWirelessEntry(nil, service, info, seenAt)
	if got.Key != "serial:TVSERIAL123" {
		t.Fatalf("unexpected stable key: %q", got.Key)
	}
	if got.Name != "Google TV Streamer" {
		t.Fatalf("unexpected generated name: %q", got.Name)
	}
	if got.LastAddress != "192.168.178.99:38219" {
		t.Fatalf("unexpected last address: %q", got.LastAddress)
	}
	if got.Manufacturer != "Google" || got.AndroidVersion != "14" {
		t.Fatalf("identity enrichment missing: %#v", got)
	}
	if got.LastSeenAt != "2026-09-29T01:02:03Z" {
		t.Fatalf("unexpected last-seen timestamp: %q", got.LastSeenAt)
	}
}

func TestBuildRememberedWirelessEntry_PreservesCustomName(t *testing.T) {
	existing := &core.RememberedWirelessDevice{
		Key:            "serial:TVSERIAL123",
		Host:           "192.168.1.20",
		Name:           "Living room",
		HardwareSerial: "TVSERIAL123",
	}
	service := device.MDNSService{
		InstanceName: "adb-tv",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.77:43000",
		Host:         "192.168.1.77",
		Port:         "43000",
		Secure:       true,
	}
	info := &device.Info{HardwareSerial: "TVSERIAL123", Model: "Google TV Streamer"}

	got := buildRememberedWirelessEntry(existing, service, info, time.Unix(0, 0).UTC())
	if got.Name != "Living room" {
		t.Fatalf("custom name was overwritten: %q", got.Name)
	}
	if got.Host != "192.168.1.77" {
		t.Fatalf("host was not refreshed: %q", got.Host)
	}
}

package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"path/filepath"
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

	got, err := findRememberedWirelessMatch(entries, service, info)
	if err != nil {
		t.Fatalf("findRememberedWirelessMatch() error = %v", err)
	}
	if got != 0 {
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

func TestFindRememberedWirelessMatch_RefusesHostOnlyOverwriteOfConflictingSerial(t *testing.T) {
	entries := []core.RememberedWirelessDevice{
		{Key: "serial:TV-A", Host: "192.168.1.20", HardwareSerial: "TV-A", AutoConnect: true},
	}
	service := device.MDNSService{
		InstanceName: "adb-other",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}
	info := &device.Info{HardwareSerial: "TV-B"}

	got, err := findRememberedWirelessMatch(entries, service, info)
	if err != nil {
		t.Fatalf("a conflicting serial must not be ambiguous, it must not match: %v", err)
	}
	if got != -1 {
		t.Fatalf("host-only match overwrote the durable identity at index %d", got)
	}
}

func TestFindRememberedWirelessMatch_UnknownSerialMustNotOverwriteDurableIdentity(t *testing.T) {
	entries := []core.RememberedWirelessDevice{
		{Key: "serial:TV-A", Host: "192.168.1.20", HardwareSerial: "TV-A", AutoConnect: true},
	}
	service := device.MDNSService{
		InstanceName: "adb-other",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}

	got, err := findRememberedWirelessMatch(entries, service, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != -1 {
		t.Fatalf("an unidentified observation overwrote a durable identity at index %d", got)
	}
}

func TestFindRememberedWirelessMatch_ErrorsOnAmbiguousHost(t *testing.T) {
	entries := []core.RememberedWirelessDevice{
		{Key: "one", Host: "192.168.1.20"},
		{Key: "two", Host: "192.168.1.20"},
	}
	service := device.MDNSService{
		InstanceName: "adb-new",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}

	got, err := findRememberedWirelessMatch(entries, service, nil)
	if err == nil {
		t.Fatal("expected two remembered devices on one host to be refused")
	}
	if got != -1 {
		t.Fatalf("an ambiguous host chose index %d", got)
	}
}

func TestResolveRememberedWirelessService_RefusesAmbiguousHost(t *testing.T) {
	entry := core.RememberedWirelessDevice{Key: "old", Host: "192.168.1.20", AutoConnect: true}
	services := []device.MDNSService{
		{InstanceName: "adb-a", Kind: device.MDNSServiceConnect, Address: "192.168.1.20:43000", Host: "192.168.1.20", Port: "43000", Secure: true},
		{InstanceName: "adb-b", Kind: device.MDNSServiceConnect, Address: "192.168.1.20:43001", Host: "192.168.1.20", Port: "43001", Secure: true},
	}

	if _, ok := resolveRememberedWirelessService(services, entry); ok {
		t.Fatal("expected two connect endpoints on one host to be refused instead of picking the first")
	}
}

func TestResolveRememberedWirelessService_AllowsDifferingInstanceNames(t *testing.T) {
	entry := core.RememberedWirelessDevice{
		Key:          "adb-tv-pair",
		InstanceName: "adb-tv-pair",
		Host:         "192.168.1.50",
		AutoConnect:  true,
	}
	services := []device.MDNSService{
		{InstanceName: "adb-tv-connect", Kind: device.MDNSServiceConnect, Address: "192.168.1.50:43000", Host: "192.168.1.50", Port: "43000", Secure: true},
	}

	got, ok := resolveRememberedWirelessService(services, entry)
	if !ok {
		t.Fatal("expected the unique same-host connect endpoint to resolve")
	}
	if got.Port != "43000" {
		t.Fatalf("unexpected resolution: %#v", got)
	}
}

// newRememberTestApp wires only the wireless-memory path: the device service points
// at a nonexistent tool, so identity enrichment is unavailable and the matcher has
// to behave when the hardware serial is unknown.
func newRememberTestApp(t *testing.T, remembered ...core.RememberedWirelessDevice) *App {
	t.Helper()
	a, _ := newTestApp(t)
	a.ctx = context.Background()
	missing := filepath.Join(t.TempDir(), "missing-adb")
	a.devSvc = device.NewService(a.dataDir, func() core.BinaryPaths {
		return core.BinaryPaths{Adb: missing, Fastboot: missing}
	})
	a.cfg.RememberedWireless = remembered
	return a
}

func TestRememberWirelessServiceRefusesAmbiguousHostWithoutWriting(t *testing.T) {
	a := newRememberTestApp(t,
		core.RememberedWirelessDevice{Key: "one", Host: "192.168.1.20"},
		core.RememberedWirelessDevice{Key: "two", Host: "192.168.1.20"},
	)
	service := device.MDNSService{
		InstanceName: "adb-new",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}

	if err := a.rememberWirelessService(service); err == nil {
		t.Fatal("expected an ambiguous host to be refused")
	}
	if len(a.cfg.RememberedWireless) != 2 {
		t.Fatalf("ambiguous host mutated remembered devices: %#v", a.cfg.RememberedWireless)
	}
}

func TestRememberWirelessServiceDoesNotOverwriteDurableIdentity(t *testing.T) {
	a := newRememberTestApp(t, core.RememberedWirelessDevice{
		Key:            "serial:TV-A",
		Host:           "192.168.1.20",
		HardwareSerial: "TV-A",
		Name:           "Living room",
		AutoConnect:    true,
	})
	service := device.MDNSService{
		InstanceName: "adb-unknown",
		Kind:         device.MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}

	if err := a.rememberWirelessService(service); err != nil {
		t.Fatalf("remembering an unidentified observation failed: %v", err)
	}
	if len(a.cfg.RememberedWireless) != 2 {
		t.Fatalf("expected a new entry instead of an overwrite: %#v", a.cfg.RememberedWireless)
	}
	durable := a.cfg.RememberedWireless[0]
	if durable.HardwareSerial != "TV-A" || durable.Name != "Living room" || durable.Host != "192.168.1.20" {
		t.Fatalf("durable identity was overwritten: %#v", durable)
	}
}

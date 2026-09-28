package device

import "testing"

func TestGroupMDNSServices_RealWorldDiscovery(t *testing.T) {
	services := parseMDNSServices(`List of discovered mdns services
adb-126451105550676B0000B7EDF89-nFDIoP  _adb-tls-connect._tcp   192.168.178.99:38219
adb-G0911X03030200MX    _adb._tcp       192.168.178.37:5555
`)

	devices := groupMDNSServices(services)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d: %#v", len(devices), devices)
	}

	var tv *DiscoveredWirelessDevice
	for i := range devices {
		if devices[i].Host == "192.168.178.99" {
			tv = &devices[i]
			break
		}
	}
	if tv == nil {
		t.Fatal("expected TV host to be grouped")
	}
	if tv.ConnectAddress != "192.168.178.99:38219" {
		t.Fatalf("unexpected connect address: %q", tv.ConnectAddress)
	}
	if tv.PreferredAddress != tv.ConnectAddress {
		t.Fatalf("expected TLS connect endpoint to be preferred: %#v", tv)
	}
	if !tv.SecureConnect {
		t.Fatal("expected secure connect flag")
	}
}

func TestGroupMDNSServices_CombinesPairingConnectAndLegacyByHost(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-tv", Kind: MDNSServicePairing, Address: "192.168.1.9:42001", Host: "192.168.1.9", Port: "42001", Secure: true},
		{InstanceName: "adb-tv", Kind: MDNSServiceConnect, Address: "192.168.1.9:43001", Host: "192.168.1.9", Port: "43001", Secure: true},
		{InstanceName: "legacy-tv", Kind: MDNSServiceLegacy, Address: "192.168.1.9:5555", Host: "192.168.1.9", Port: "5555"},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if got.PairingAddress != "192.168.1.9:42001" {
		t.Fatalf("unexpected pairing address: %q", got.PairingAddress)
	}
	if got.ConnectAddress != "192.168.1.9:43001" {
		t.Fatalf("unexpected connect address: %q", got.ConnectAddress)
	}
	if got.LegacyAddress != "192.168.1.9:5555" {
		t.Fatalf("unexpected legacy address: %q", got.LegacyAddress)
	}
	if got.PreferredAddress != got.ConnectAddress {
		t.Fatalf("TLS connect should win over legacy: %#v", got)
	}
	if len(got.InstanceNames) != 2 {
		t.Fatalf("expected unique sorted instance names, got %#v", got.InstanceNames)
	}
}

func TestGroupMDNSServices_PairingOnlyStillAppears(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-new-tv", Kind: MDNSServicePairing, Address: "192.168.1.30:40000", Host: "192.168.1.30", Port: "40000", Secure: true},
	}
	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one pairing-only device, got %#v", devices)
	}
	if devices[0].PreferredAddress != "192.168.1.30:40000" {
		t.Fatalf("pairing endpoint should be exposed when it is the only endpoint: %#v", devices[0])
	}
}

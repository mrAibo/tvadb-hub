package device

import (
	"encoding/json"
	"testing"
)

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
	if got.PairingPort != 42001 {
		t.Fatalf("expected derived pairing port 42001, got %d", got.PairingPort)
	}
	if got.ConnectPort != 43001 {
		t.Fatalf("expected derived connect port 43001, got %d", got.ConnectPort)
	}
	if !got.NeedsPairing {
		t.Fatal("expected pairing to be reported as offered")
	}
}

func TestGroupMDNSServices_DerivedFieldsPerServiceType(t *testing.T) {
	cases := []struct {
		name         string
		services     []MDNSService
		wantPairing  int
		wantConnect  int
		wantPairingQ bool
	}{
		{
			name: "pairing only offers pairing and has no connect port",
			services: []MDNSService{
				{InstanceName: "adb-new-tv", Kind: MDNSServicePairing, Address: "192.168.1.30:40000", Host: "192.168.1.30", Port: "40000", Secure: true},
			},
			wantPairing:  40000,
			wantConnect:  0,
			wantPairingQ: true,
		},
		{
			name: "legacy only is a connect endpoint without pairing",
			services: []MDNSService{
				{InstanceName: "adb-legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.31:5555", Host: "192.168.1.31", Port: "5555"},
			},
			wantPairing:  0,
			wantConnect:  5555,
			wantPairingQ: false,
		},
		{
			name: "connect only is paired already",
			services: []MDNSService{
				{InstanceName: "adb-tv", Kind: MDNSServiceConnect, Address: "192.168.1.32:42000", Host: "192.168.1.32", Port: "42000", Secure: true},
			},
			wantPairing:  0,
			wantConnect:  42000,
			wantPairingQ: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			devices := groupMDNSServices(tc.services)
			if len(devices) != 1 {
				t.Fatalf("expected one device, got %#v", devices)
			}
			got := devices[0]
			if got.PairingPort != tc.wantPairing {
				t.Fatalf("PairingPort = %d, want %d", got.PairingPort, tc.wantPairing)
			}
			if got.ConnectPort != tc.wantConnect {
				t.Fatalf("ConnectPort = %d, want %d", got.ConnectPort, tc.wantConnect)
			}
			if got.NeedsPairing != tc.wantPairingQ {
				t.Fatalf("NeedsPairing = %v, want %v", got.NeedsPairing, tc.wantPairingQ)
			}
		})
	}
}

func TestDiscoveredWirelessDeviceJSONContract(t *testing.T) {
	payload, err := json.Marshal(DiscoveredWirelessDevice{
		Host:         "192.168.1.20",
		PairingPort:  37121,
		ConnectPort:  5555,
		NeedsPairing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pairingPort", "connectPort", "needsPairing"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("expected JSON key %q in %s", key, payload)
		}
	}

	// Zero ports must stay absent so older consumers keep seeing the same shape.
	empty, err := json.Marshal(DiscoveredWirelessDevice{Host: "192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	var emptyDecoded map[string]any
	if err := json.Unmarshal(empty, &emptyDecoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pairingPort", "connectPort"} {
		if _, ok := emptyDecoded[key]; ok {
			t.Fatalf("expected omitted empty field %q in %s", key, empty)
		}
	}
	if _, ok := emptyDecoded["needsPairing"]; !ok {
		t.Fatalf("expected needsPairing to always be present in %s", empty)
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

func TestGroupMDNSServices_SuppressesAmbiguousTLSNeverDowngrades(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-a", Kind: MDNSServiceConnect, Address: "192.168.1.40:43000", Host: "192.168.1.40", Port: "43000", Secure: true},
		{InstanceName: "tv-b", Kind: MDNSServiceConnect, Address: "192.168.1.40:43001", Host: "192.168.1.40", Port: "43001", Secure: true},
		{InstanceName: "tv-legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.40:5555", Host: "192.168.1.40", Port: "5555"},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if got.ConnectAddress != "" {
		t.Fatalf("two distinct TLS ports must not be exposed as one unique connect address: %#v", got)
	}
	if got.PreferredAddress != "" {
		t.Fatalf("TLS ambiguity must never downgrade to another endpoint: %#v", got)
	}
	if got.ConnectPort != 0 {
		t.Fatalf("ambiguous TLS must not fall back to the legacy port: %d", got.ConnectPort)
	}
	if got.LegacyAddress != "192.168.1.40:5555" {
		t.Fatalf("the unique legacy advertisement must stay visible as raw diagnostics: %#v", got)
	}
	if !got.SecureConnect {
		t.Fatal("the TLS connect advertisement must stay reported")
	}
}

func TestGroupMDNSServices_KeepsDuplicateEndpointAndSuppressesAmbiguousPairing(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-wlan0", Kind: MDNSServiceConnect, Address: "192.168.1.50:43000", Host: "192.168.1.50", Port: "43000", Secure: true},
		{InstanceName: "tv-eth0", Kind: MDNSServiceConnect, Address: "192.168.1.50:43000", Host: "192.168.1.50", Port: "43000", Secure: true},
		{InstanceName: "pair-a", Kind: MDNSServicePairing, Address: "192.168.1.50:42000", Host: "192.168.1.50", Port: "42000", Secure: true},
		{InstanceName: "pair-b", Kind: MDNSServicePairing, Address: "192.168.1.50:42001", Host: "192.168.1.50", Port: "42001", Secure: true},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if got.ConnectAddress != "192.168.1.50:43000" {
		t.Fatalf("duplicate advertisements of one endpoint must stay available: %#v", got)
	}
	if got.PreferredAddress != got.ConnectAddress {
		t.Fatalf("the unique TLS connect endpoint must be preferred: %#v", got)
	}
	if got.ConnectPort != 43000 {
		t.Fatalf("unexpected connect port: %d", got.ConnectPort)
	}
	if got.PairingAddress != "" || got.PairingPort != 0 || got.NeedsPairing {
		t.Fatalf("two distinct pairing ports must be suppressed, not guessed: %#v", got)
	}
	if len(got.InstanceNames) != 4 {
		t.Fatalf("raw instance names must be preserved: %#v", got.InstanceNames)
	}
}

func TestGroupMDNSServices_ConnectInstanceNamesFollowChosenKindOnly(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-pair", Kind: MDNSServicePairing, Address: "192.168.1.60:42000", Host: "192.168.1.60", Port: "42000", Secure: true},
		{InstanceName: "adb-connect", Kind: MDNSServiceConnect, Address: "192.168.1.60:43000", Host: "192.168.1.60", Port: "43000", Secure: true},
		{InstanceName: "adb-legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.60:5555", Host: "192.168.1.60", Port: "5555"},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if len(got.ConnectInstanceNames) != 1 || got.ConnectInstanceNames[0] != "adb-connect" {
		t.Fatalf("only the chosen TLS endpoint names may be derived, got %#v", got.ConnectInstanceNames)
	}
	if len(got.InstanceNames) != 3 {
		t.Fatalf("diagnostic InstanceNames must stay complete: %#v", got.InstanceNames)
	}
}

func TestGroupMDNSServices_ConnectInstanceNamesDedupeAndSort(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-b", Kind: MDNSServiceConnect, Address: "192.168.1.61:43000", Host: "192.168.1.61", Port: "43000", Secure: true},
		{InstanceName: "adb-a", Kind: MDNSServiceConnect, Address: "192.168.1.61:43000", Host: "192.168.1.61", Port: "43000", Secure: true},
		{InstanceName: "ADB-B", Kind: MDNSServiceConnect, Address: "192.168.1.61:43000", Host: "192.168.1.61", Port: "43000", Secure: true},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0].ConnectInstanceNames
	if len(got) != 2 || got[0] != "adb-a" || got[1] != "adb-b" {
		t.Fatalf("expected deduplicated sorted endpoint aliases, got %#v", got)
	}
}

func TestGroupMDNSServices_LegacyEndpointNamesOnlyWhenLegacyChosen(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.62:5555", Host: "192.168.1.62", Port: "5555"},
		{InstanceName: "adb-pair", Kind: MDNSServicePairing, Address: "192.168.1.62:42000", Host: "192.168.1.62", Port: "42000", Secure: true},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if got.ConnectAddress != "" {
		t.Fatalf("no TLS connect endpoint is advertised: %#v", got)
	}
	if len(got.ConnectInstanceNames) != 1 || got.ConnectInstanceNames[0] != "adb-legacy" {
		t.Fatalf("the unique legacy endpoint may name the legacy transport, got %#v", got.ConnectInstanceNames)
	}
}

func TestGroupMDNSServices_AmbiguousTLSHasNoConnectInstanceNames(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-a", Kind: MDNSServiceConnect, Address: "192.168.1.63:43000", Host: "192.168.1.63", Port: "43000", Secure: true},
		{InstanceName: "tv-b", Kind: MDNSServiceConnect, Address: "192.168.1.63:43001", Host: "192.168.1.63", Port: "43001", Secure: true},
		{InstanceName: "adb-legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.63:5555", Host: "192.168.1.63", Port: "5555"},
	}

	devices := groupMDNSServices(services)
	if len(devices) != 1 {
		t.Fatalf("expected one grouped host, got %#v", devices)
	}
	got := devices[0]
	if got.ConnectAddress != "" || got.LegacyAddress == "" {
		t.Fatalf("ambiguous TLS must stay ambiguous with legacy visible: %#v", got)
	}
	if len(got.ConnectInstanceNames) != 0 {
		t.Fatalf("ambiguous TLS must derive no names at all, got %#v", got.ConnectInstanceNames)
	}
}

func TestDiscoveredWirelessDeviceConnectInstanceNamesJSON(t *testing.T) {
	payload, err := json.Marshal(DiscoveredWirelessDevice{
		Host:                 "192.168.1.20",
		ConnectInstanceNames: []string{"adb-connect"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	names, ok := decoded["connectInstanceNames"].([]any)
	if !ok || len(names) != 1 || names[0] != "adb-connect" {
		t.Fatalf("expected connectInstanceNames in %s", payload)
	}

	// No derived names must stay absent so older consumers keep the same shape.
	empty, err := json.Marshal(DiscoveredWirelessDevice{Host: "192.168.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	var emptyDecoded map[string]any
	if err := json.Unmarshal(empty, &emptyDecoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := emptyDecoded["connectInstanceNames"]; ok {
		t.Fatalf("expected omitted connectInstanceNames in %s", empty)
	}
}

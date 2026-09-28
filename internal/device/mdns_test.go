package device

import (
	"strings"
	"testing"
)

func TestParseMDNSServices_RealWorldTVOutput(t *testing.T) {
	input := `List of discovered mdns services
adb-126451105550676B0000B7EDF89-nFDIoP  _adb-tls-connect._tcp   192.168.178.99:38219
adb-G0911X03030200MX    _adb._tcp       192.168.178.37:5555
`

	got := parseMDNSServices(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 services, got %d: %#v", len(got), got)
	}

	var tv *MDNSService
	for i := range got {
		if got[i].Host == "192.168.178.99" {
			tv = &got[i]
			break
		}
	}
	if tv == nil {
		t.Fatal("expected Google TV service to be parsed")
	}
	if tv.Kind != MDNSServiceConnect {
		t.Fatalf("expected connect service, got %q", tv.Kind)
	}
	if tv.Address != "192.168.178.99:38219" {
		t.Fatalf("unexpected TV address: %q", tv.Address)
	}
	if tv.Port != "38219" {
		t.Fatalf("unexpected TV port: %q", tv.Port)
	}
	if !tv.Secure {
		t.Fatal("expected TLS connect service to be secure")
	}
}

func TestParseMDNSServices_ParsesPairingAndIPv6(t *testing.T) {
	input := `List of discovered mdns services
adb-tv-1 _adb-tls-pairing._tcp [fe80::1234]:42629
adb-tv-1 _adb-tls-connect._tcp [fe80::1234]:38219
`

	got := parseMDNSServices(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 services, got %d", len(got))
	}
	if got[0].Host != "fe80::1234" || got[1].Host != "fe80::1234" {
		t.Fatalf("expected IPv6 host to be normalized, got %#v", got)
	}

	var pairingFound bool
	for _, service := range got {
		if service.Kind == MDNSServicePairing {
			pairingFound = true
			if service.Port != "42629" {
				t.Fatalf("unexpected pairing port: %s", service.Port)
			}
		}
	}
	if !pairingFound {
		t.Fatal("pairing service not found")
	}
}

func TestParseMDNSServices_IgnoresNoiseAndMalformedEndpoints(t *testing.T) {
	input := `List of discovered mdns services
random-service _http._tcp 192.168.1.20:80
broken _adb-tls-connect._tcp missing-port
badport _adb._tcp 192.168.1.5:70000
valid _adb-tls-connect._tcp 192.168.1.10:40123
`

	got := parseMDNSServices(input)
	if len(got) != 1 {
		t.Fatalf("expected exactly one valid ADB service, got %#v", got)
	}
	if got[0].InstanceName != "valid" {
		t.Fatalf("unexpected service parsed: %#v", got[0])
	}
}

func TestResolveConnectService_PrefersTLSOnSameHost(t *testing.T) {
	services := []MDNSService{
		{
			InstanceName: "legacy",
			ServiceName:  mdnsLegacyName,
			Kind:         MDNSServiceLegacy,
			Address:      "192.168.1.20:5555",
			Host:         "192.168.1.20",
			Port:         "5555",
		},
		{
			InstanceName: "modern",
			ServiceName:  mdnsConnectName,
			Kind:         MDNSServiceConnect,
			Address:      "192.168.1.20:41001",
			Host:         "192.168.1.20",
			Port:         "41001",
			Secure:       true,
		},
	}

	got, err := resolveConnectService(services, "")
	if err != nil {
		t.Fatalf("resolveConnectService returned error: %v", err)
	}
	if got.Kind != MDNSServiceConnect || got.Port != "41001" {
		t.Fatalf("expected secure TLS connect endpoint, got %#v", got)
	}
}

func TestResolveConnectService_RequiresSelectionForMultipleHosts(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv", Kind: MDNSServiceConnect, Address: "192.168.1.20:41001", Host: "192.168.1.20", Port: "41001"},
		{InstanceName: "phone", Kind: MDNSServiceConnect, Address: "192.168.1.21:41002", Host: "192.168.1.21", Port: "41002"},
	}

	_, err := resolveConnectService(services, "")
	if err == nil {
		t.Fatal("expected ambiguous discovery error")
	}
	if !strings.Contains(err.Error(), "multiple wireless ADB devices") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveConnectService_SelectsByHostOrInstance(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "adb-tv", Kind: MDNSServiceConnect, Address: "192.168.1.20:41001", Host: "192.168.1.20", Port: "41001"},
		{InstanceName: "adb-phone", Kind: MDNSServiceConnect, Address: "192.168.1.21:41002", Host: "192.168.1.21", Port: "41002"},
	}

	byHost, err := resolveConnectService(services, "192.168.1.20")
	if err != nil {
		t.Fatalf("select by host failed: %v", err)
	}
	if byHost.InstanceName != "adb-tv" {
		t.Fatalf("unexpected host selection: %#v", byHost)
	}

	byInstance, err := resolveConnectService(services, "adb-phone")
	if err != nil {
		t.Fatalf("select by instance failed: %v", err)
	}
	if byInstance.Host != "192.168.1.21" {
		t.Fatalf("unexpected instance selection: %#v", byInstance)
	}
}

func TestResolvePairingService_AllowsCodeOnlyWhenUnambiguous(t *testing.T) {
	services := []MDNSService{
		{
			InstanceName: "adb-tv",
			ServiceName:  mdnsPairingName,
			Kind:         MDNSServicePairing,
			Address:      "192.168.178.99:42629",
			Host:         "192.168.178.99",
			Port:         "42629",
			Secure:       true,
		},
	}

	got, err := resolvePairingService(services, "")
	if err != nil {
		t.Fatalf("resolvePairingService returned error: %v", err)
	}
	if got.Address != "192.168.178.99:42629" {
		t.Fatalf("unexpected pairing endpoint: %#v", got)
	}
}

func TestResolveConnectForPairing_PrefersInstanceThenFallsBackToHost(t *testing.T) {
	pairing := MDNSService{
		InstanceName: "adb-tv-pair",
		Kind:         MDNSServicePairing,
		Address:      "192.168.1.50:42000",
		Host:         "192.168.1.50",
		Port:         "42000",
	}
	services := []MDNSService{
		{InstanceName: "different-connect-name", Kind: MDNSServiceConnect, Address: "192.168.1.50:43000", Host: "192.168.1.50", Port: "43000", Secure: true},
		{InstanceName: "other-device", Kind: MDNSServiceConnect, Address: "192.168.1.60:44000", Host: "192.168.1.60", Port: "44000", Secure: true},
	}

	got, err := resolveConnectForPairing(services, pairing)
	if err != nil {
		t.Fatalf("resolveConnectForPairing returned error: %v", err)
	}
	if got.Address != "192.168.1.50:43000" {
		t.Fatalf("expected host fallback to resolve TV connect endpoint, got %#v", got)
	}
}

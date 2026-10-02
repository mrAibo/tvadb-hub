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

func TestParseMDNSServices_ToleratesCanonicalTrailingDotAndSpacedInstanceName(t *testing.T) {
	input := `List of discovered mdns services
adb-tv-1	_adb-tls-connect._tcp.	192.168.1.20:37121
_Legacy TV	_adb._tcp	192.168.1.21:5555
`

	got := parseMDNSServices(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 services, got %d: %#v", len(got), got)
	}

	var connect, legacy *MDNSService
	for i := range got {
		switch got[i].Kind {
		case MDNSServiceConnect:
			connect = &got[i]
		case MDNSServiceLegacy:
			legacy = &got[i]
		}
	}
	if connect == nil || !connect.Secure || connect.Address != "192.168.1.20:37121" {
		t.Fatalf("trailing-dot connect service was not parsed: %#v", got)
	}
	if legacy == nil || legacy.InstanceName != "_Legacy TV" {
		t.Fatalf("spaced instance name was not preserved verbatim: %#v", got)
	}
}

func TestParseMDNSServices_NeverFabricatesEndpointsFromOtherColumns(t *testing.T) {
	// Only a real host:port column may become an endpoint; TXT-like tokens must be
	// ignored rather than guessed into a host.
	input := `List of discovered mdns services
adb-tv _adb-tls-connect._tcp txtver=42 192.168.1.20:37121
adb-bad _adb-tls-connect._tcp model=TV-1
`

	got := parseMDNSServices(input)
	if len(got) != 1 {
		t.Fatalf("expected only the real endpoint, got %#v", got)
	}
	if got[0].Host != "192.168.1.20" || got[0].Port != "37121" {
		t.Fatalf("unexpected endpoint: %#v", got[0])
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

func TestResolveConnectForPairing_RefusesAmbiguousHost(t *testing.T) {
	pairing := MDNSService{
		InstanceName: "adb-tv-pair",
		Kind:         MDNSServicePairing,
		Address:      "192.168.1.50:42000",
		Host:         "192.168.1.50",
		Port:         "42000",
	}
	services := []MDNSService{
		{InstanceName: "adb-tv-a", Kind: MDNSServiceConnect, Address: "192.168.1.50:43000", Host: "192.168.1.50", Port: "43000", Secure: true},
		{InstanceName: "adb-tv-b", Kind: MDNSServiceConnect, Address: "192.168.1.50:43001", Host: "192.168.1.50", Port: "43001", Secure: true},
	}

	if _, err := resolveConnectForPairing(services, pairing); err == nil {
		t.Fatal("expected two connect ports on one host to be refused instead of picking the first")
	}
}

func TestPickConnectEndpoint(t *testing.T) {
	connect := MDNSService{InstanceName: "connect", Kind: MDNSServiceConnect, Address: "192.168.1.5:43000", Host: "192.168.1.5", Port: "43000"}
	legacy := MDNSService{InstanceName: "legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.5:5555", Host: "192.168.1.5", Port: "5555"}

	if got, ok := PickConnectEndpoint([]MDNSService{legacy, connect}); !ok || got.Kind != MDNSServiceConnect {
		t.Fatalf("expected the TLS connect endpoint to win: %#v %v", got, ok)
	}
	if got, ok := PickConnectEndpoint([]MDNSService{legacy}); !ok || got.Kind != MDNSServiceLegacy {
		t.Fatalf("expected a single legacy endpoint to be accepted: %#v %v", got, ok)
	}
	// A multihomed device advertises the same endpoint once per interface. That is
	// one endpoint, not an ambiguity: refusing it would hide the TV forever.
	if got, ok := PickConnectEndpoint([]MDNSService{connect, connect}); !ok || got.Address != connect.Address {
		t.Fatalf("expected a repeated connect endpoint to stay resolvable: %#v %v", got, ok)
	}
	// The same endpoint printed with different host casing and instance names is
	// still the same endpoint.
	upper := connect
	upper.InstanceName = "connect-on-eth1"
	upper.Host = "TV.LOCAL"
	upper.Address = "TV.LOCAL:43000"
	lower := connect
	lower.InstanceName = "connect-on-wlan0"
	lower.Host = "tv.local"
	lower.Address = "tv.local:43000"
	if got, ok := PickConnectEndpoint([]MDNSService{upper, lower}); !ok || got.Port != "43000" {
		t.Fatalf("expected casing/instance duplicates to dedupe: %#v %v", got, ok)
	}
	// Two different connect ports on one host are still genuinely ambiguous.
	secondPort := connect
	secondPort.InstanceName = "connect-other-port"
	secondPort.Port = "43001"
	secondPort.Address = "192.168.1.5:43001"
	if _, ok := PickConnectEndpoint([]MDNSService{connect, secondPort}); ok {
		t.Fatal("expected two distinct connect ports on one host to be ambiguous")
	}
	if _, ok := PickConnectEndpoint([]MDNSService{connect, secondPort, legacy}); ok {
		t.Fatal("ambiguous TLS endpoints must not silently downgrade to legacy")
	}
	// Duplicates never override the TLS-over-legacy preference.
	if got, ok := PickConnectEndpoint([]MDNSService{legacy, legacy, connect, connect}); !ok || got.Kind != MDNSServiceConnect {
		t.Fatalf("expected the duplicate TLS endpoint to win over legacy: %#v %v", got, ok)
	}
	if _, ok := PickConnectEndpoint(nil); ok {
		t.Fatal("expected no endpoint for an empty candidate set")
	}
}

func TestResolveConnectService_RefusesTwoDistinctTLSPortsOnOneHost(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-a", Kind: MDNSServiceConnect, Address: "192.168.1.5:43000", Host: "192.168.1.5", Port: "43000", Secure: true},
		{InstanceName: "tv-b", Kind: MDNSServiceConnect, Address: "192.168.1.5:43001", Host: "192.168.1.5", Port: "43001", Secure: true},
	}

	// A host selector must not fabricate one unique endpoint out of two ports.
	if _, err := resolveConnectService(services, "192.168.1.5"); err == nil {
		t.Fatal("expected a host selector to refuse two distinct TLS ports on one host")
	}
	// A blank selector carries the same obligation.
	if _, err := resolveConnectService(services, ""); err == nil {
		t.Fatal("expected a blank selector to refuse two distinct TLS ports on one host")
	}
}

func TestResolveConnectService_AmbiguousTLSNeverFallsBackToLegacy(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-a", Kind: MDNSServiceConnect, Address: "192.168.1.5:43000", Host: "192.168.1.5", Port: "43000", Secure: true},
		{InstanceName: "tv-b", Kind: MDNSServiceConnect, Address: "192.168.1.5:43001", Host: "192.168.1.5", Port: "43001", Secure: true},
		{InstanceName: "legacy", Kind: MDNSServiceLegacy, Address: "192.168.1.5:5555", Host: "192.168.1.5", Port: "5555"},
	}

	if _, err := resolveConnectService(services, ""); err == nil {
		t.Fatal("expected ambiguous TLS to be refused instead of falling back to legacy")
	}
	if _, err := resolveConnectService(services, "192.168.1.5"); err == nil {
		t.Fatal("expected ambiguous TLS on a host selector to be refused instead of falling back to legacy")
	}
	// An explicit endpoint names one endpoint and stays selectable.
	got, err := resolveConnectService(services, "192.168.1.5:43001")
	if err != nil {
		t.Fatalf("an explicit endpoint must still resolve: %v", err)
	}
	if got.Port != "43001" {
		t.Fatalf("unexpected explicit endpoint: %#v", got)
	}
}

func TestResolveConnectService_AcceptsDuplicateAdvertisementsOfOneEndpoint(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "tv-wlan0", Kind: MDNSServiceConnect, Address: "192.168.1.5:43000", Host: "192.168.1.5", Port: "43000", Secure: true},
		{InstanceName: "tv-eth0", Kind: MDNSServiceConnect, Address: "192.168.1.5:43000", Host: "192.168.1.5", Port: "43000", Secure: true},
	}

	for _, selector := range []string{"", "192.168.1.5", "tv-eth0", "192.168.1.5:43000"} {
		got, err := resolveConnectService(services, selector)
		if err != nil {
			t.Fatalf("duplicate advertisements of one endpoint must stay resolvable for %q: %v", selector, err)
		}
		if got.Port != "43000" {
			t.Fatalf("unexpected endpoint for %q: %#v", selector, got)
		}
	}
}

func TestResolvePairingService_RefusesTwoDistinctPairingPorts(t *testing.T) {
	services := []MDNSService{
		{InstanceName: "pair-a", Kind: MDNSServicePairing, Address: "192.168.1.5:42000", Host: "192.168.1.5", Port: "42000", Secure: true},
		{InstanceName: "pair-b", Kind: MDNSServicePairing, Address: "192.168.1.5:42001", Host: "192.168.1.5", Port: "42001", Secure: true},
	}

	if _, err := resolvePairingService(services, ""); err == nil {
		t.Fatal("expected a blank selector to refuse two distinct pairing ports on one host")
	}
	if _, err := resolvePairingService(services, "192.168.1.5"); err == nil {
		t.Fatal("expected a host selector to refuse two distinct pairing ports on one host")
	}
	got, err := resolvePairingService(services, "192.168.1.5:42001")
	if err != nil {
		t.Fatalf("an explicit pairing endpoint must still resolve: %v", err)
	}
	if got.Port != "42001" {
		t.Fatalf("unexpected pairing endpoint: %#v", got)
	}
	// Duplicate advertisements of the same pairing endpoint are not ambiguity.
	duplicates := []MDNSService{services[0], services[0]}
	got, err = resolvePairingService(duplicates, "")
	if err != nil {
		t.Fatalf("duplicate pairing advertisements of one port must stay resolvable: %v", err)
	}
	if got.Port != "42000" {
		t.Fatalf("unexpected duplicate pairing endpoint: %#v", got)
	}
}

func FuzzParseMDNSServices(f *testing.F) {
	f.Add("adb-device _adb-tls-connect._tcp 192.168.1.20:37121\n")
	f.Add("adb-device _adb-tls-pairing._tcp [fe80::1]:42629\n")
	f.Add("malformed input\x00with noise\n")

	f.Fuzz(func(t *testing.T, input string) {
		services := parseMDNSServices(input)
		for _, service := range services {
			if service.Host == "" || service.Port == "" {
				t.Fatalf("parser returned incomplete service: %#v", service)
			}
		}
	})
}

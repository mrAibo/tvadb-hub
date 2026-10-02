package device

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MDNSServiceKind string

const (
	MDNSServicePairing MDNSServiceKind = "pairing"
	MDNSServiceConnect MDNSServiceKind = "connect"
	MDNSServiceLegacy  MDNSServiceKind = "legacy"

	mdnsPairingName = "_adb-tls-pairing._tcp"
	mdnsConnectName = "_adb-tls-connect._tcp"
	mdnsLegacyName  = "_adb._tcp"
)

// MDNSService is one ADB service advertised on the local network.
//
// Modern Android wireless debugging advertises two TLS services:
//   - _adb-tls-pairing._tcp while the pairing-code screen is open
//   - _adb-tls-connect._tcp for normal authenticated ADB connections
//
// Older devices can advertise the legacy _adb._tcp service instead.
type MDNSService struct {
	InstanceName string          `json:"instanceName"`
	ServiceName  string          `json:"serviceName"`
	Kind         MDNSServiceKind `json:"kind"`
	Address      string          `json:"address"`
	Host         string          `json:"host"`
	Port         string          `json:"port"`
	Secure       bool            `json:"secure"`
}

type WirelessConnectResult struct {
	Service MDNSService `json:"service"`
	Message string      `json:"message"`
}

type WirelessPairResult struct {
	Service MDNSService `json:"service"`
	Message string      `json:"message"`
}

type WirelessPairAndConnectResult struct {
	PairingService MDNSService `json:"pairingService"`
	ConnectService MDNSService `json:"connectService"`
	PairMessage    string      `json:"pairMessage"`
	ConnectMessage string      `json:"connectMessage"`
}

// Discover asks the bundled/configured adb executable for all mDNS services.
// It deliberately relies on adb's own mDNS implementation instead of adding a
// second platform-specific discovery stack.
func (s *WirelessService) Discover(ctx context.Context) ([]MDNSService, error) {
	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"mdns", "services"},
		Timeout: 6 * time.Second,
	})
	if err != nil {
		detail := err.Error()
		if result != nil && strings.TrimSpace(result.Stderr) != "" {
			detail = strings.TrimSpace(result.Stderr)
		}
		return nil, core.NewOperationError(
			"discover_wireless",
			"failed to discover wireless ADB services",
			detail,
			true,
		)
	}
	if result.ExitCode != 0 {
		return nil, core.NewOperationError(
			"discover_wireless",
			"wireless ADB discovery failed",
			strings.TrimSpace(result.Stderr),
			true,
		)
	}

	return parseMDNSServices(result.Stdout), nil
}

const (
	mdnsReadyAttempts = 3
	mdnsReadyDelay    = 400 * time.Millisecond
)

// DiscoverReady is Discover with a bounded readiness retry: the first
// `adb mdns services` call can legitimately return an empty list before the ADB
// mDNS daemon has published its records (for example right after the Android 12
// pairing screen opens). A real discovery error is returned immediately instead of
// being retried or flattened into an empty result.
func (s *WirelessService) DiscoverReady(ctx context.Context) ([]MDNSService, error) {
	return discoverWithReadinessRetry(ctx, s.Discover)
}

func discoverWithReadinessRetry(ctx context.Context, discover func(context.Context) ([]MDNSService, error)) ([]MDNSService, error) {
	for attempt := 1; ; attempt++ {
		services, err := discover(ctx)
		if err != nil {
			return nil, err
		}
		if len(services) > 0 || attempt >= mdnsReadyAttempts {
			return services, nil
		}

		select {
		case <-ctx.Done():
			return nil, core.NewOperationError(
				"discover_wireless",
				"wireless ADB discovery was cancelled",
				ctx.Err().Error(),
				true,
			)
		case <-time.After(mdnsReadyDelay):
		}
	}
}

// AutoConnect discovers the current dynamic ADB endpoint and connects to it.
// selector may be an mDNS instance name, a host/IP, or an exact host:port.
// When selector is empty, auto-connect is allowed only when discovery can
// unambiguously identify a single device host.
func (s *WirelessService) AutoConnect(ctx context.Context, selector string) (WirelessConnectResult, error) {
	services, err := s.Discover(ctx)
	if err != nil {
		return WirelessConnectResult{}, err
	}

	service, err := resolveConnectService(services, selector)
	if err != nil {
		return WirelessConnectResult{}, core.NewOperationError(
			"auto_connect_wireless",
			"could not resolve a wireless ADB endpoint",
			err.Error(),
			true,
		)
	}

	message, err := s.Connect(ctx, service.Address)
	if err != nil {
		return WirelessConnectResult{Service: service}, err
	}

	return WirelessConnectResult{
		Service: service,
		Message: message,
	}, nil
}

// PairDiscovered resolves the currently advertised pairing endpoint and pairs
// against it. With exactly one pairing service visible, selector can be empty,
// so a GUI only needs to ask the user for the six-digit pairing code.
func (s *WirelessService) PairDiscovered(ctx context.Context, selector string, code string) (WirelessPairResult, error) {
	services, err := s.Discover(ctx)
	if err != nil {
		return WirelessPairResult{}, err
	}

	service, err := resolvePairingService(services, selector)
	if err != nil {
		return WirelessPairResult{}, core.NewOperationError(
			"pair_discovered_wireless",
			"could not resolve a wireless ADB pairing endpoint",
			err.Error(),
			true,
		)
	}

	message, err := s.Pair(ctx, service.Address, code)
	if err != nil {
		return WirelessPairResult{Service: service}, err
	}

	return WirelessPairResult{
		Service: service,
		Message: message,
	}, nil
}

// PairAndConnect resolves the temporary pairing port, pairs the device, waits
// for its normal TLS connect advertisement, then connects automatically.
// This is the backend primitive for the TV wizard where the user should only
// have to enter the six-digit code shown on the TV.
func (s *WirelessService) PairAndConnect(ctx context.Context, selector string, code string) (WirelessPairAndConnectResult, error) {
	services, err := s.Discover(ctx)
	if err != nil {
		return WirelessPairAndConnectResult{}, err
	}

	pairingService, err := resolvePairingService(services, selector)
	if err != nil {
		return WirelessPairAndConnectResult{}, core.NewOperationError(
			"pair_and_connect_wireless",
			"could not resolve a wireless ADB pairing endpoint",
			err.Error(),
			true,
		)
	}

	pairMessage, err := s.Pair(ctx, pairingService.Address, code)
	if err != nil {
		return WirelessPairAndConnectResult{PairingService: pairingService}, err
	}

	result := WirelessPairAndConnectResult{
		PairingService: pairingService,
		PairMessage:    pairMessage,
	}

	deadline := time.Now().Add(12 * time.Second)
	lastDetail := "connect service did not appear"
	for {
		services, discoverErr := s.Discover(ctx)
		if discoverErr == nil {
			connectService, resolveErr := resolveConnectForPairing(services, pairingService)
			if resolveErr == nil {
				result.ConnectService = connectService
				connectMessage, connectErr := s.Connect(ctx, connectService.Address)
				if connectErr == nil {
					result.ConnectMessage = connectMessage
					return result, nil
				}
				lastDetail = connectErr.Error()
			} else {
				lastDetail = resolveErr.Error()
			}
		} else {
			lastDetail = discoverErr.Error()
		}

		if time.Now().After(deadline) {
			break
		}

		select {
		case <-ctx.Done():
			return result, core.NewOperationError(
				"pair_and_connect_wireless",
				"wireless ADB connection was cancelled",
				ctx.Err().Error(),
				true,
			)
		case <-time.After(500 * time.Millisecond):
		}
	}

	return result, core.NewOperationError(
		"pair_and_connect_wireless",
		"paired successfully but could not connect to the device",
		lastDetail,
		true,
	)
}

func parseMDNSServices(output string) []MDNSService {
	services := make([]MDNSService, 0)

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "list of discovered mdns services") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Canonical adb output is "<instance> <service type> <host:port>". Locate
		// the service type instead of trusting a fixed column so an instance name
		// that contains spaces is still handled, and never invent a name or an
		// endpoint that adb did not print.
		serviceIndex := -1
		var kind MDNSServiceKind
		var secure bool
		for i := 1; i < len(fields); i++ {
			if classified, isSecure, ok := classifyMDNSService(fields[i]); ok {
				serviceIndex = i
				kind = classified
				secure = isSecure
				break
			}
		}
		if serviceIndex < 1 {
			continue
		}

		address := ""
		host, port := "", ""
		for i := serviceIndex + 1; i < len(fields); i++ {
			if parsedHost, parsedPort, ok := splitMDNSEndpoint(fields[i]); ok {
				address = fields[i]
				host = parsedHost
				port = parsedPort
				break
			}
		}
		if address == "" {
			continue
		}

		services = append(services, MDNSService{
			InstanceName: strings.Join(fields[:serviceIndex], " "),
			ServiceName:  fields[serviceIndex],
			Kind:         kind,
			Address:      address,
			Host:         host,
			Port:         port,
			Secure:       secure,
		})
	}

	sort.SliceStable(services, func(i, j int) bool {
		if services[i].Host != services[j].Host {
			return services[i].Host < services[j].Host
		}
		if services[i].Kind != services[j].Kind {
			return mdnsKindPriority(services[i].Kind) < mdnsKindPriority(services[j].Kind)
		}
		if services[i].InstanceName != services[j].InstanceName {
			return services[i].InstanceName < services[j].InstanceName
		}
		return services[i].Port < services[j].Port
	})

	return services
}

func classifyMDNSService(serviceName string) (MDNSServiceKind, bool, bool) {
	// Service types are case-insensitive and some adb/platform-tools builds print
	// them with the canonical trailing root dot ("_adb-tls-connect._tcp.").
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(serviceName)), ".")
	switch normalized {
	case mdnsPairingName:
		return MDNSServicePairing, true, true
	case mdnsConnectName:
		return MDNSServiceConnect, true, true
	case mdnsLegacyName:
		return MDNSServiceLegacy, false, true
	default:
		return "", false, false
	}
}

func splitMDNSEndpoint(address string) (string, string, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", "", false
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// adb normally brackets IPv6 literals. Keep a conservative fallback for
		// implementations that print a bare host followed by the final :port.
		idx := strings.LastIndex(address, ":")
		if idx <= 0 || idx == len(address)-1 {
			return "", "", false
		}
		host = strings.Trim(address[:idx], "[]")
		port = address[idx+1:]
	}

	if host == "" || !validTCPPort(port) {
		return "", "", false
	}

	host = strings.Trim(host, "[]")
	if !validWirelessHost(host) {
		return "", "", false
	}

	return host, port, true
}

func validTCPPort(port string) bool {
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
}

func resolveConnectService(services []MDNSService, selector string) (MDNSService, error) {
	candidates := make([]MDNSService, 0)
	for _, service := range services {
		if service.Kind == MDNSServiceConnect || service.Kind == MDNSServiceLegacy {
			candidates = append(candidates, service)
		}
	}
	return resolveMDNSService(candidates, selector, true)
}

func resolvePairingService(services []MDNSService, selector string) (MDNSService, error) {
	candidates := make([]MDNSService, 0)
	for _, service := range services {
		if service.Kind == MDNSServicePairing {
			candidates = append(candidates, service)
		}
	}
	return resolveMDNSService(candidates, selector, false)
}

func resolveConnectForPairing(services []MDNSService, pairing MDNSService) (MDNSService, error) {
	// The connect advertisement of the same device can use a different mDNS
	// instance name than the pairing advertisement, so an instance-name match is
	// never required. Prefer the pairing host, then fall back to the instance name,
	// and only accept a candidate when it is unambiguous.
	if pairing.Host != "" {
		byHost := filterMDNSServices(services, func(service MDNSService) bool {
			return strings.EqualFold(service.Host, pairing.Host)
		})
		if service, ok := PickConnectEndpoint(byHost); ok {
			return service, nil
		}
	}
	if pairing.InstanceName != "" {
		byInstance := filterMDNSServices(services, func(service MDNSService) bool {
			return strings.EqualFold(service.InstanceName, pairing.InstanceName)
		})
		if service, ok := PickConnectEndpoint(byInstance); ok {
			return service, nil
		}
	}
	return MDNSService{}, fmt.Errorf("paired device has no unambiguous connect advertisement yet")
}

// PickConnectEndpoint returns the secure TLS connect endpoint when exactly one is
// advertised, otherwise a single legacy endpoint.
//
// Ambiguity is counted over distinct endpoints, not over advertisement records: a
// multihomed device (several interfaces) can advertise the same canonical
// host:port more than once, in mixed case, and that is still one endpoint, so it
// must not hide the device forever. Two different connect ports on one host remain
// ambiguous: choosing the first could target another device, so the caller must
// rescan instead of connecting arbitrarily. TLS connect is preferred over the
// legacy _adb._tcp endpoint.
func PickConnectEndpoint(candidates []MDNSService) (MDNSService, bool) {
	for _, service := range candidates {
		if service.Kind == MDNSServiceConnect {
			// Ambiguous TLS advertisements must not downgrade to a legacy endpoint.
			return selectConnectEndpoint(candidates, MDNSServiceConnect)
		}
	}
	return selectConnectEndpoint(candidates, MDNSServiceLegacy)
}

// selectConnectEndpoint resolves exactly one distinct endpoint of one kind,
// ignoring duplicate advertisements of that same endpoint.
func selectConnectEndpoint(candidates []MDNSService, kind MDNSServiceKind) (MDNSService, bool) {
	distinct := 0
	chosen := -1
	seen := make(map[string]struct{}, len(candidates))
	for i := range candidates {
		if candidates[i].Kind != kind {
			continue
		}
		key := canonicalMDNSEndpoint(candidates[i])
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		distinct++
		if chosen < 0 {
			chosen = i
		}
	}

	if distinct != 1 || chosen < 0 {
		return MDNSService{}, false
	}
	return candidates[chosen], true
}

// canonicalMDNSEndpoint is the dedupe identity of one advertised endpoint:
// host and port, case-insensitive, independent of instance name, service kind and
// advertisement count. Without a parsed host and port the address is the identity.
func canonicalMDNSEndpoint(service MDNSService) string {
	host := strings.ToLower(strings.Trim(strings.TrimSpace(service.Host), "[]"))
	port := strings.TrimSpace(service.Port)
	if host == "" || port == "" {
		return strings.ToLower(strings.TrimSpace(service.Address))
	}
	return net.JoinHostPort(host, port)
}

func filterMDNSServices(services []MDNSService, keep func(MDNSService) bool) []MDNSService {
	filtered := make([]MDNSService, 0, len(services))
	for _, service := range services {
		if keep(service) {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func resolveMDNSService(candidates []MDNSService, selector string, preferSecureConnect bool) (MDNSService, error) {
	if len(candidates) == 0 {
		return MDNSService{}, fmt.Errorf("no matching ADB mDNS services were discovered")
	}

	selector = strings.TrimSpace(selector)
	if selector != "" {
		filtered := make([]MDNSService, 0)
		for _, service := range candidates {
			if matchesMDNSSelector(service, selector) {
				filtered = append(filtered, service)
			}
		}
		if len(filtered) == 0 {
			return MDNSService{}, fmt.Errorf("no discovered ADB service matches %q", selector)
		}
		candidates = filtered
	}

	// A selector can name an exact address, an exact instance name, or a whole
	// host. Only the last case can legitimately match several endpoints of one
	// device (TLS plus legacy), so a selector that spans hosts is refused.
	hosts := uniqueServiceHosts(candidates)
	if len(hosts) > 1 {
		if selector != "" {
			return MDNSService{}, fmt.Errorf("selector %q matches multiple device hosts", selector)
		}
		return MDNSService{}, fmt.Errorf(
			"multiple wireless ADB devices were discovered (%s); choose a device first",
			strings.Join(hosts, ", "),
		)
	}

	if !preferSecureConnect {
		// Pairing: every matching advertisement must still describe exactly one
		// distinct pairing endpoint. Duplicate records of that same endpoint are
		// allowed; two distinct pairing ports are never resolved by picking one.
		if service, ok := selectConnectEndpoint(candidates, MDNSServicePairing); ok {
			return service, nil
		}
		return MDNSService{}, fmt.Errorf(
			"multiple distinct pairing endpoints were advertised for %s; pair with the exact host:port shown on the device",
			hosts[0],
		)
	}

	// Connect: the TLS endpoint wins, but only when it is unique. An ambiguous
	// TLS advertisement is refused instead of silently falling back to the legacy
	// _adb._tcp endpoint or to the first record adb happened to print.
	if service, ok := PickConnectEndpoint(candidates); ok {
		return service, nil
	}
	return MDNSService{}, fmt.Errorf(
		"multiple distinct wireless ADB endpoints were advertised for %s; connect with the exact host:port shown on the device",
		hosts[0],
	)
}

func matchesMDNSSelector(service MDNSService, selector string) bool {
	normalized := strings.Trim(selector, "[]")
	return strings.EqualFold(service.InstanceName, selector) ||
		strings.EqualFold(service.Address, selector) ||
		strings.EqualFold(service.Host, normalized)
}

func uniqueServiceHosts(services []MDNSService) []string {
	seen := make(map[string]struct{})
	hosts := make([]string, 0)
	for _, service := range services {
		key := strings.ToLower(service.Host)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		hosts = append(hosts, service.Host)
	}
	sort.Strings(hosts)
	return hosts
}

func mdnsKindPriority(kind MDNSServiceKind) int {
	switch kind {
	case MDNSServiceConnect:
		return 0
	case MDNSServicePairing:
		return 1
	case MDNSServiceLegacy:
		return 2
	default:
		return 3
	}
}

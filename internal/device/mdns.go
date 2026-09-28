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

		instanceName := fields[0]
		serviceName := fields[1]
		address := fields[len(fields)-1]

		kind, secure, ok := classifyMDNSService(serviceName)
		if !ok {
			continue
		}

		host, port, ok := splitMDNSEndpoint(address)
		if !ok {
			continue
		}

		services = append(services, MDNSService{
			InstanceName: instanceName,
			ServiceName:  serviceName,
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
	switch strings.TrimSpace(serviceName) {
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

	return strings.Trim(host, "[]"), port, true
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

	if preferSecureConnect {
		sort.SliceStable(candidates, func(i, j int) bool {
			return mdnsKindPriority(candidates[i].Kind) < mdnsKindPriority(candidates[j].Kind)
		})
	}

	if selector != "" {
		// A selector can legitimately match both modern TLS and legacy ADB on
		// the same host. In that case the secure endpoint wins.
		hosts := uniqueServiceHosts(candidates)
		if len(hosts) == 1 {
			return candidates[0], nil
		}
		return MDNSService{}, fmt.Errorf("selector %q matches multiple device hosts", selector)
	}

	hosts := uniqueServiceHosts(candidates)
	if len(hosts) > 1 {
		return MDNSService{}, fmt.Errorf(
			"multiple wireless ADB devices were discovered (%s); choose a device first",
			strings.Join(hosts, ", "),
		)
	}

	return candidates[0], nil
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

package device

import (
	"context"
	"sort"
	"strconv"
	"strings"
)

// DiscoveredWirelessDevice groups the raw ADB mDNS records currently advertised
// by one network host. It is intentionally a discovery/session model, not a
// persistent device identity: DHCP addresses and mDNS instance names can change.
//
// Once the device is connected, TVADB Hub will enrich this with the ADB serial
// and Android properties for a more stable remembered-device identity.
type DiscoveredWirelessDevice struct {
	DiscoveryKey     string   `json:"discoveryKey"`
	Host             string   `json:"host"`
	InstanceNames    []string `json:"instanceNames"`
	PairingAddress   string   `json:"pairingAddress,omitempty"`
	ConnectAddress   string   `json:"connectAddress,omitempty"`
	LegacyAddress    string   `json:"legacyAddress,omitempty"`
	PreferredAddress string   `json:"preferredAddress,omitempty"`
	SecureConnect    bool     `json:"secureConnect"`

	// ConnectInstanceNames lists only the mDNS instance names that advertise the
	// chosen connect endpoint of the chosen kind: the unique TLS connect endpoint
	// when one is advertised, otherwise the unique legacy endpoint. It is the
	// evidence that may tie an authorized transport to this endpoint. InstanceNames
	// stays the unfiltered diagnostic view and must never be used that way.
	ConnectInstanceNames []string `json:"connectInstanceNames,omitempty"`

	// Derived convenience fields for the UI. PairingPort is the advertised
	// temporary pairing port. NeedsPairing means pairing is currently OFFERED
	// (a pairing endpoint exists); it is not proof that this host is not already
	// paired. ConnectPort is the port of the preferred connect endpoint.
	PairingPort  int  `json:"pairingPort,omitempty"`
	ConnectPort  int  `json:"connectPort,omitempty"`
	NeedsPairing bool `json:"needsPairing"`
}

// DiscoverDevices returns a UI-friendly view of the raw mDNS advertisements.
// It briefly retries an initially empty answer so a freshly opened pairing screen
// is not reported as "no devices" before adb's mDNS daemon has published it.
func (s *WirelessService) DiscoverDevices(ctx context.Context) ([]DiscoveredWirelessDevice, error) {
	services, err := s.DiscoverReady(ctx)
	if err != nil {
		return nil, err
	}
	return groupMDNSServices(services), nil
}

// GroupMDNSServices exposes the discovery grouping to the app layer so the
// diagnostics view and DiscoverDevices cannot drift apart.
func GroupMDNSServices(services []MDNSService) []DiscoveredWirelessDevice {
	return groupMDNSServices(services)
}

func endpointPort(address string) int {
	_, port, ok := splitMDNSEndpoint(address)
	if !ok {
		return 0
	}
	value, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return value
}

func groupMDNSServices(services []MDNSService) []DiscoveredWirelessDevice {
	// Raw candidates are accumulated per host and per service kind. The old
	// first-wins grouping exposed one arbitrary endpoint per kind, which let two
	// distinct TLS ports on one host look like a single "unique" connect target.
	type accumulator struct {
		host      string
		instances map[string]struct{}
		pairing   []MDNSService
		connect   []MDNSService
		legacy    []MDNSService
	}

	// connectInstanceNames narrows the grouped instance names down to those
	// advertising exactly the chosen endpoint of the chosen kind. Duplicate
	// advertisements of one endpoint (several interfaces, mixed case) still count
	// once, and the result is sorted so the derived field is stable.
	connectInstanceNames := func(candidates []MDNSService, chosen MDNSService) []string {
		want := canonicalMDNSEndpoint(chosen)
		seen := make(map[string]struct{}, len(candidates))
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.Kind != chosen.Kind || canonicalMDNSEndpoint(candidate) != want {
				continue
			}
			name := strings.TrimSpace(candidate.InstanceName)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	byHost := make(map[string]*accumulator)
	for _, service := range services {
		host := strings.TrimSpace(service.Host)
		if host == "" {
			continue
		}
		key := strings.ToLower(host)
		group, ok := byHost[key]
		if !ok {
			group = &accumulator{host: host, instances: make(map[string]struct{})}
			byHost[key] = group
		}

		if service.InstanceName != "" {
			group.instances[service.InstanceName] = struct{}{}
		}

		switch service.Kind {
		case MDNSServicePairing:
			group.pairing = append(group.pairing, service)
		case MDNSServiceConnect:
			group.connect = append(group.connect, service)
		case MDNSServiceLegacy:
			group.legacy = append(group.legacy, service)
		}
	}

	devices := make([]DiscoveredWirelessDevice, 0, len(byHost))
	for _, group := range byHost {
		instances := make([]string, 0, len(group.instances))
		for instance := range group.instances {
			instances = append(instances, instance)
		}
		sort.Strings(instances)

		device := DiscoveredWirelessDevice{
			Host:          group.host,
			InstanceNames: instances,
			// A TLS connect service is advertised, even when its endpoints are
			// too ambiguous to expose one of them.
			SecureConnect: len(group.connect) > 0,
		}

		// Only a unique endpoint per service kind is exposed. Several distinct
		// endpoints of one kind are genuine ambiguity, so the address stays empty
		// and the UI falls back to the manual IP/port path instead of receiving a
		// fabricated unique endpoint chosen by arrival order. Duplicate records of
		// the same endpoint (one per interface, different instance names) still
		// count once and stay available.
		if service, ok := selectConnectEndpoint(group.connect, MDNSServiceConnect); ok {
			device.ConnectAddress = service.Address
			device.ConnectInstanceNames = connectInstanceNames(group.connect, service)
		}
		if service, ok := selectConnectEndpoint(group.legacy, MDNSServiceLegacy); ok {
			device.LegacyAddress = service.Address
			// The legacy endpoint only names the authorized transport when it is
			// the chosen connect target: any TLS connect advertisement, unique or
			// ambiguous, outranks it and must not leak legacy aliases.
			if len(group.connect) == 0 {
				device.ConnectInstanceNames = connectInstanceNames(group.legacy, service)
			}
		}
		if service, ok := selectConnectEndpoint(group.pairing, MDNSServicePairing); ok {
			device.PairingAddress = service.Address
		}

		switch {
		case device.ConnectAddress != "":
			device.PreferredAddress = device.ConnectAddress
		case len(group.connect) > 0:
			// TLS connect advertisements exist but are ambiguous. Never downgrade
			// to the legacy _adb._tcp endpoint or to the pairing offer.
		case device.LegacyAddress != "":
			device.PreferredAddress = device.LegacyAddress
		case device.PairingAddress != "":
			device.PreferredAddress = device.PairingAddress
		}

		device.PairingPort = endpointPort(device.PairingAddress)
		device.ConnectPort = endpointPort(device.ConnectAddress)
		if device.ConnectPort == 0 && len(group.connect) == 0 {
			device.ConnectPort = endpointPort(device.LegacyAddress)
		}
		device.NeedsPairing = device.PairingAddress != ""

		if len(instances) > 0 {
			device.DiscoveryKey = instances[0]
		} else {
			device.DiscoveryKey = device.Host
		}

		devices = append(devices, device)
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Host != devices[j].Host {
			return devices[i].Host < devices[j].Host
		}
		return devices[i].DiscoveryKey < devices[j].DiscoveryKey
	})

	return devices
}

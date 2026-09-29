package device

import (
	"context"
	"sort"
	"strings"
)

// DiscoveredWirelessDevice groups the raw ADB mDNS records currently advertised
// by one network host. It is intentionally a discovery/session model, not a
// persistent device identity: DHCP addresses and mDNS instance names can change.
//
// Once the device is connected, TVADB Hub will enrich this with the ADB serial
// and Android properties for a more stable remembered-device identity.
type DiscoveredWirelessDevice struct {
	DiscoveryKey   string   `json:"discoveryKey"`
	Host           string   `json:"host"`
	InstanceNames  []string `json:"instanceNames"`
	PairingAddress string   `json:"pairingAddress,omitempty"`
	ConnectAddress string   `json:"connectAddress,omitempty"`
	LegacyAddress  string   `json:"legacyAddress,omitempty"`
	PreferredAddress string `json:"preferredAddress,omitempty"`
	SecureConnect  bool     `json:"secureConnect"`
}

// DiscoverDevices returns a UI-friendly view of the raw mDNS advertisements.
func (s *WirelessService) DiscoverDevices(ctx context.Context) ([]DiscoveredWirelessDevice, error) {
	services, err := s.Discover(ctx)
	if err != nil {
		return nil, err
	}
	return groupMDNSServices(services), nil
}

func groupMDNSServices(services []MDNSService) []DiscoveredWirelessDevice {
	type accumulator struct {
		device    DiscoveredWirelessDevice
		instances map[string]struct{}
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
			group = &accumulator{
				device: DiscoveredWirelessDevice{
					Host: host,
				},
				instances: make(map[string]struct{}),
			}
			byHost[key] = group
		}

		if service.InstanceName != "" {
			group.instances[service.InstanceName] = struct{}{}
		}

		switch service.Kind {
		case MDNSServicePairing:
			if group.device.PairingAddress == "" {
				group.device.PairingAddress = service.Address
			}
		case MDNSServiceConnect:
			if group.device.ConnectAddress == "" {
				group.device.ConnectAddress = service.Address
			}
			group.device.SecureConnect = true
		case MDNSServiceLegacy:
			if group.device.LegacyAddress == "" {
				group.device.LegacyAddress = service.Address
			}
		}
	}

	devices := make([]DiscoveredWirelessDevice, 0, len(byHost))
	for _, group := range byHost {
		instances := make([]string, 0, len(group.instances))
		for instance := range group.instances {
			instances = append(instances, instance)
		}
		sort.Strings(instances)
		group.device.InstanceNames = instances

		switch {
		case group.device.ConnectAddress != "":
			group.device.PreferredAddress = group.device.ConnectAddress
		case group.device.LegacyAddress != "":
			group.device.PreferredAddress = group.device.LegacyAddress
		case group.device.PairingAddress != "":
			group.device.PreferredAddress = group.device.PairingAddress
		}

		if len(instances) > 0 {
			group.device.DiscoveryKey = instances[0]
		} else {
			group.device.DiscoveryKey = group.device.Host
		}

		devices = append(devices, group.device)
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Host != devices[j].Host {
			return devices[i].Host < devices[j].Host
		}
		return devices[i].DiscoveryKey < devices[j].DiscoveryKey
	})

	return devices
}

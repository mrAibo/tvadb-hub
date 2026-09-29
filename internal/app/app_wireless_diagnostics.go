package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

type WirelessDiagnosticStatus string

const (
	WirelessDiagnosticPass    WirelessDiagnosticStatus = "pass"
	WirelessDiagnosticWarning WirelessDiagnosticStatus = "warning"
	WirelessDiagnosticFail    WirelessDiagnosticStatus = "fail"
	WirelessDiagnosticInfo    WirelessDiagnosticStatus = "info"
)

type WirelessDiagnosticCheck struct {
	ID             string                   `json:"id"`
	Label          string                   `json:"label"`
	Status         WirelessDiagnosticStatus `json:"status"`
	Detail         string                   `json:"detail"`
	Recommendation string                   `json:"recommendation,omitempty"`
}

type WirelessDiagnosticsReport struct {
	ADBPath       string                            `json:"adbPath,omitempty"`
	ADBVersion    string                            `json:"adbVersion,omitempty"`
	Services      []device.MDNSService              `json:"services"`
	Devices       []device.DiscoveredWirelessDevice `json:"devices"`
	SelectedHost  string                            `json:"selectedHost,omitempty"`
	SelectedEndpoint string                         `json:"selectedEndpoint,omitempty"`
	Checks        []WirelessDiagnosticCheck         `json:"checks"`
	Healthy       bool                              `json:"healthy"`
}

// GetWirelessDiagnostics performs non-destructive checks for the TV Wireless
// ADB workflow. selector may be a discovered host/IP, mDNS instance or endpoint.
// An empty selector is accepted when exactly one device host is discovered.
func (a *App) GetWirelessDiagnostics(selector string) WirelessDiagnosticsReport {
	report := WirelessDiagnosticsReport{
		Services: []device.MDNSService{},
		Devices:  []device.DiscoveredWirelessDevice{},
		Checks:   []WirelessDiagnosticCheck{},
		Healthy:  true,
	}

	a.mu.Lock()
	binaryStatus := a.binSvc.GetBinaryStatus(a.cfg)
	a.mu.Unlock()

	if binaryStatus == nil || binaryStatus.Adb == nil || binaryStatus.Adb.Status != core.BinaryReady {
		detail := "ADB executable is not available"
		if binaryStatus != nil && binaryStatus.Adb != nil && strings.TrimSpace(binaryStatus.Adb.Reason) != "" {
			detail = binaryStatus.Adb.Reason
		}
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "adb",
			Label:          "ADB executable",
			Status:         WirelessDiagnosticFail,
			Detail:         detail,
			Recommendation: "Install or select current Android SDK Platform Tools.",
		})
		return report
	}

	report.ADBPath = binaryStatus.Adb.Path
	report.ADBVersion = binaryStatus.Adb.Version
	report.addCheck(WirelessDiagnosticCheck{
		ID:     "adb",
		Label:  "ADB executable",
		Status: WirelessDiagnosticPass,
		Detail: fmt.Sprintf("%s — %s", binaryStatus.Adb.Path, binaryStatus.Adb.Version),
	})
	report.addCheck(adbVersionDiagnostic(binaryStatus.Adb.Version))

	services, err := a.wireSvc.Discover(a.ctx)
	if err != nil {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "mdns",
			Label:          "Wireless ADB discovery",
			Status:         WirelessDiagnosticFail,
			Detail:         err.Error(),
			Recommendation: "Enable Wireless debugging on the TV and verify that both devices are on the same local network.",
		})
		return report
	}
	report.Services = services
	report.Devices = groupDiagnosticDevices(services)

	if len(services) == 0 {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "mdns",
			Label:          "Wireless ADB discovery",
			Status:         WirelessDiagnosticWarning,
			Detail:         "No ADB mDNS services were discovered.",
			Recommendation: "Enable Wireless debugging on the TV. For first-time setup, keep the pairing-code screen open while scanning.",
		})
		return report
	}

	report.addCheck(WirelessDiagnosticCheck{
		ID:     "mdns",
		Label:  "Wireless ADB discovery",
		Status: WirelessDiagnosticPass,
		Detail: fmt.Sprintf("Discovered %d ADB mDNS service(s) across %d host(s).", len(services), len(report.Devices)),
	})

	selected, resolveErr := resolveDiagnosticDevice(report.Devices, selector)
	if resolveErr != nil {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "target",
			Label:          "Target TV",
			Status:         WirelessDiagnosticWarning,
			Detail:         resolveErr.Error(),
			Recommendation: "Select the TV you want to diagnose.",
		})
		return report
	}

	report.SelectedHost = selected.Host
	report.SelectedEndpoint = selected.PreferredAddress
	report.addCheck(WirelessDiagnosticCheck{
		ID:     "target",
		Label:  "Target TV",
		Status: WirelessDiagnosticPass,
		Detail: selected.Host,
	})

	if selected.ConnectAddress != "" {
		report.addCheck(WirelessDiagnosticCheck{
			ID:     "connect_endpoint",
			Label:  "ADB connect endpoint",
			Status: WirelessDiagnosticPass,
			Detail: selected.ConnectAddress + " (TLS)",
		})
	} else if selected.LegacyAddress != "" {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "connect_endpoint",
			Label:          "ADB connect endpoint",
			Status:         WirelessDiagnosticWarning,
			Detail:         selected.LegacyAddress + " (legacy TCP/IP ADB)",
			Recommendation: "Prefer Android Wireless debugging/TLS when the TV supports it.",
		})
	} else if selected.PairingAddress != "" {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "connect_endpoint",
			Label:          "ADB connect endpoint",
			Status:         WirelessDiagnosticInfo,
			Detail:         "Only a pairing endpoint is currently advertised.",
			Recommendation: "Pair with the six-digit code; the normal connect endpoint should appear immediately afterwards.",
		})
	} else {
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "connect_endpoint",
			Label:          "ADB connect endpoint",
			Status:         WirelessDiagnosticFail,
			Detail:         "The selected host has no usable ADB endpoint.",
			Recommendation: "Toggle Wireless debugging off and on, then scan again.",
		})
	}

	if selected.PreferredAddress != "" {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		err := probeTCPEndpoint(ctx, selected.PreferredAddress)
		cancel()
		if err != nil {
			report.addCheck(WirelessDiagnosticCheck{
				ID:             "tcp",
				Label:          "TCP endpoint",
				Status:         WirelessDiagnosticFail,
				Detail:         fmt.Sprintf("%s is not reachable: %v", selected.PreferredAddress, err),
				Recommendation: "Keep the pairing screen open when testing a pairing port. Otherwise check client isolation, guest Wi-Fi, firewall rules, or rescan for a changed dynamic port.",
			})
		} else {
			report.addCheck(WirelessDiagnosticCheck{
				ID:     "tcp",
				Label:  "TCP endpoint",
				Status: WirelessDiagnosticPass,
				Detail: selected.PreferredAddress + " accepts TCP connections.",
			})
		}
	}

	adbDevices, listErr := a.devSvc.ListDevices(a.ctx)
	if listErr != nil {
		report.addCheck(WirelessDiagnosticCheck{
			ID:     "adb_state",
			Label:  "ADB device state",
			Status: WirelessDiagnosticWarning,
			Detail: listErr.Error(),
		})
		return report
	}

	state, serial := adbStateForHost(adbDevices, selected.Host)
	switch state {
	case device.StateReady:
		report.addCheck(WirelessDiagnosticCheck{
			ID:     "adb_state",
			Label:  "ADB device state",
			Status: WirelessDiagnosticPass,
			Detail: serial + " is connected and ready.",
		})
	case device.StateUnauthorized:
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "adb_state",
			Label:          "ADB device state",
			Status:         WirelessDiagnosticFail,
			Detail:         serial + " is unauthorized.",
			Recommendation: "Approve the debugging authorization on the TV or pair the device again.",
		})
	case device.StateOffline:
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "adb_state",
			Label:          "ADB device state",
			Status:         WirelessDiagnosticWarning,
			Detail:         serial + " is offline.",
			Recommendation: "Rescan mDNS and reconnect using the current dynamic connect port.",
		})
	default:
		report.addCheck(WirelessDiagnosticCheck{
			ID:             "adb_state",
			Label:          "ADB device state",
			Status:         WirelessDiagnosticInfo,
			Detail:         "The TV is discoverable but is not currently listed by adb devices.",
			Recommendation: "Use Connect, or Pair & connect for a new TV.",
		})
	}

	return report
}

func (r *WirelessDiagnosticsReport) addCheck(check WirelessDiagnosticCheck) {
	r.Checks = append(r.Checks, check)
	if check.Status == WirelessDiagnosticFail {
		r.Healthy = false
	}
}

func adbVersionDiagnostic(version string) WirelessDiagnosticCheck {
	check := WirelessDiagnosticCheck{
		ID:     "adb_version",
		Label:  "Platform Tools version",
		Status: WirelessDiagnosticPass,
		Detail: version,
	}

	major, ok := parseAdbMajorVersion(version)
	if !ok {
		check.Status = WirelessDiagnosticWarning
		check.Recommendation = "Use a recent Android SDK Platform Tools release if Wireless debugging behaves unexpectedly."
		return check
	}

	if major < 30 {
		check.Status = WirelessDiagnosticFail
		check.Recommendation = "Update Android SDK Platform Tools; adb pair requires Platform Tools 30 or newer."
		return check
	}
	if major < 35 {
		check.Status = WirelessDiagnosticWarning
		check.Recommendation = "Wireless pairing is supported, but this Platform Tools release is old. Update it before troubleshooting mDNS or TLS pairing problems."
	}
	return check
}

func groupDiagnosticDevices(services []device.MDNSService) []device.DiscoveredWirelessDevice {
	// Reuse the public service grouping through a local pure equivalent because
	// the device package intentionally keeps its grouping helper unexported.
	type acc struct {
		device    device.DiscoveredWirelessDevice
		instances map[string]struct{}
	}
	groups := map[string]*acc{}
	for _, service := range services {
		key := strings.ToLower(service.Host)
		group := groups[key]
		if group == nil {
			group = &acc{
				device: device.DiscoveredWirelessDevice{Host: service.Host},
				instances: map[string]struct{}{},
			}
			groups[key] = group
		}
		if service.InstanceName != "" {
			group.instances[service.InstanceName] = struct{}{}
		}
		switch service.Kind {
		case device.MDNSServicePairing:
			if group.device.PairingAddress == "" {
				group.device.PairingAddress = service.Address
			}
		case device.MDNSServiceConnect:
			if group.device.ConnectAddress == "" {
				group.device.ConnectAddress = service.Address
			}
			group.device.SecureConnect = true
		case device.MDNSServiceLegacy:
			if group.device.LegacyAddress == "" {
				group.device.LegacyAddress = service.Address
			}
		}
	}
	result := make([]device.DiscoveredWirelessDevice, 0, len(groups))
	for _, group := range groups {
		for instance := range group.instances {
			group.device.InstanceNames = append(group.device.InstanceNames, instance)
		}
		sort.Strings(group.device.InstanceNames)
		switch {
		case group.device.ConnectAddress != "":
			group.device.PreferredAddress = group.device.ConnectAddress
		case group.device.LegacyAddress != "":
			group.device.PreferredAddress = group.device.LegacyAddress
		case group.device.PairingAddress != "":
			group.device.PreferredAddress = group.device.PairingAddress
		}
		if len(group.device.InstanceNames) > 0 {
			group.device.DiscoveryKey = group.device.InstanceNames[0]
		} else {
			group.device.DiscoveryKey = group.device.Host
		}
		result = append(result, group.device)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Host < result[j].Host })
	return result
}

func resolveDiagnosticDevice(devices []device.DiscoveredWirelessDevice, selector string) (device.DiscoveredWirelessDevice, error) {
	if len(devices) == 0 {
		return device.DiscoveredWirelessDevice{}, fmt.Errorf("no wireless ADB devices were discovered")
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(devices) == 1 {
			return devices[0], nil
		}
		return device.DiscoveredWirelessDevice{}, fmt.Errorf("multiple wireless ADB device hosts were discovered")
	}
	normalized := strings.Trim(selector, "[]")
	for _, candidate := range devices {
		if strings.EqualFold(candidate.Host, normalized) ||
			strings.EqualFold(candidate.PreferredAddress, selector) ||
			strings.EqualFold(candidate.ConnectAddress, selector) ||
			strings.EqualFold(candidate.PairingAddress, selector) ||
			strings.EqualFold(candidate.LegacyAddress, selector) {
			return candidate, nil
		}
		for _, instance := range candidate.InstanceNames {
			if strings.EqualFold(instance, selector) {
				return candidate, nil
			}
		}
	}
	return device.DiscoveredWirelessDevice{}, fmt.Errorf("no discovered wireless ADB device matches %q", selector)
}

func probeTCPEndpoint(ctx context.Context, address string) error {
	dialer := net.Dialer{Timeout: 1500 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func adbStateForHost(devices []device.Summary, host string) (device.State, string) {
	prefix := host + ":"
	for _, candidate := range devices {
		if candidate.Mode != device.ModeADB {
			continue
		}
		if candidate.Serial == host || strings.HasPrefix(candidate.Serial, prefix) {
			return candidate.State, candidate.Serial
		}
	}
	return device.StateUnknown, ""
}

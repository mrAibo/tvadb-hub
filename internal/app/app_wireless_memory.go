package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"fmt"
	"strings"
	"time"
)

type WirelessReconnectReport struct {
	Attempted        int               `json:"attempted"`
	Connected        []string          `json:"connected"`
	AlreadyConnected []string          `json:"alreadyConnected"`
	Unavailable      []string          `json:"unavailable"`
	Failed           map[string]string `json:"failed"`
}

func (a *App) GetRememberedWirelessDevices() []core.RememberedWirelessDevice {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil || a.cfg.RememberedWireless == nil {
		return []core.RememberedWirelessDevice{}
	}
	out := make([]core.RememberedWirelessDevice, len(a.cfg.RememberedWireless))
	copy(out, a.cfg.RememberedWireless)
	return out
}

func (a *App) ForgetRememberedWirelessDevice(key string) error {
	return auditVoidAction(a, "forget_remembered_wireless_device", func() error {
		key = strings.TrimSpace(key)
		if key == "" {
			return core.NewOperationError(
				"forget_remembered_wireless_device",
				"remembered wireless device key is required",
				"",
				false,
			)
		}

		a.mu.Lock()
		defer a.mu.Unlock()
		if a.cfg == nil {
			return core.NewOperationError(
				"forget_remembered_wireless_device",
				"app config is not available",
				"",
				false,
			)
		}

		next := make([]core.RememberedWirelessDevice, 0, len(a.cfg.RememberedWireless))
		found := false
		for _, entry := range a.cfg.RememberedWireless {
			if strings.EqualFold(entry.Key, key) {
				found = true
				continue
			}
			next = append(next, entry)
		}
		if !found {
			return nil
		}
		a.cfg.RememberedWireless = next
		return core.SaveConfig(a.dataDir, a.cfg)
	})
}

// AutoReconnectRememberedWireless discovers the current dynamic ADB endpoints
// once and reconnects remembered TVs that are not already ready in adb devices.
func (a *App) AutoReconnectRememberedWireless() (WirelessReconnectReport, error) {
	return auditAction(a, "auto_reconnect_remembered_wireless", func() (WirelessReconnectReport, error) {
		report := WirelessReconnectReport{
			Connected:        []string{},
			AlreadyConnected: []string{},
			Unavailable:      []string{},
			Failed:           map[string]string{},
		}

		remembered := a.GetRememberedWirelessDevices()
		for _, entry := range remembered {
			if entry.AutoConnect {
				report.Attempted++
			}
		}
		if report.Attempted == 0 {
			return report, nil
		}

		services, err := a.wireSvc.DiscoverReady(a.ctx)
		if err != nil {
			return report, err
		}

		adbDevices, err := a.devSvc.ListDevices(a.ctx)
		if err != nil {
			return report, err
		}

		for _, entry := range remembered {
			if !entry.AutoConnect {
				continue
			}

			service, ok := resolveRememberedWirelessService(services, entry)
			if !ok {
				report.Unavailable = append(report.Unavailable, entry.Key)
				continue
			}

			state, _ := adbStateForHost(adbDevices, service.Host)
			if state == device.StateReady {
				report.AlreadyConnected = append(report.AlreadyConnected, entry.Key)
				if err := a.rememberWirelessService(service); err != nil {
					report.Failed[entry.Key] = err.Error()
				}
				continue
			}

			if _, err := a.wireSvc.Connect(a.ctx, service.Address); err != nil {
				report.Failed[entry.Key] = err.Error()
				continue
			}
			report.Connected = append(report.Connected, entry.Key)
			if err := a.rememberWirelessService(service); err != nil {
				report.Failed[entry.Key] = err.Error()
			}
		}

		return report, nil
	})
}

func (a *App) rememberWirelessService(service device.MDNSService) error {
	if service.Host == "" || service.Address == "" {
		return core.NewOperationError(
			"remember_wireless_device",
			"wireless service is incomplete",
			"host and address are required",
			false,
		)
	}

	// Enrichment is best-effort. Remembering the dynamic mDNS endpoint must
	// still succeed when the device is connected but a property query fails.
	var info *device.Info
	if candidate, err := a.devSvc.GetDeviceInfo(a.ctx, service.Address); err == nil {
		info = candidate
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return core.NewOperationError(
			"remember_wireless_device",
			"app config is not available",
			"",
			false,
		)
	}
	if a.cfg.RememberedWireless == nil {
		a.cfg.RememberedWireless = []core.RememberedWirelessDevice{}
	}

	match, matchErr := findRememberedWirelessMatch(a.cfg.RememberedWireless, service, info)
	if matchErr != nil {
		return core.NewOperationError(
			"remember_wireless_device",
			"could not identify the remembered wireless device",
			matchErr.Error(),
			false,
		)
	}
	var existing *core.RememberedWirelessDevice
	if match >= 0 {
		snapshot := a.cfg.RememberedWireless[match]
		existing = &snapshot
	}

	entry := buildRememberedWirelessEntry(existing, service, info, time.Now().UTC())
	if match >= 0 {
		a.cfg.RememberedWireless[match] = entry
	} else {
		a.cfg.RememberedWireless = append(a.cfg.RememberedWireless, entry)
	}

	return core.SaveConfig(a.dataDir, a.cfg)
}

// findRememberedWirelessMatch returns the index of the one remembered device a
// freshly connected service belongs to. It never guesses: a host-only match is
// accepted only when exactly one remembered entry can be that device and the new
// observation does not contradict a durable hardware-serial identity.
func findRememberedWirelessMatch(
	entries []core.RememberedWirelessDevice,
	service device.MDNSService,
	info *device.Info,
) (int, error) {
	hardwareSerial := ""
	if info != nil {
		hardwareSerial = strings.TrimSpace(info.HardwareSerial)
	}

	if hardwareSerial != "" {
		matches := make([]int, 0, 1)
		for i, entry := range entries {
			if entry.HardwareSerial != "" && strings.EqualFold(entry.HardwareSerial, hardwareSerial) {
				matches = append(matches, i)
			}
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			// Fall through to weaker identities.
		default:
			return -1, fmt.Errorf("multiple remembered wireless devices share hardware serial %s", hardwareSerial)
		}
	}

	// Pairing and connect advertisements may carry different instance names, so an
	// instance name is used only when it actually matches.
	if instance := strings.TrimSpace(service.InstanceName); instance != "" {
		matches := make([]int, 0, 1)
		for i, entry := range entries {
			if entry.InstanceName == "" || !strings.EqualFold(entry.InstanceName, instance) {
				continue
			}
			if conflictsWithSerial(entry, hardwareSerial) {
				continue
			}
			matches = append(matches, i)
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			// Fall through to the host.
		default:
			return -1, fmt.Errorf("multiple remembered wireless devices share the mDNS instance name %s", instance)
		}
	}

	host := strings.TrimSpace(service.Host)
	if host == "" {
		return -1, nil
	}

	candidates := make([]int, 0, 1)
	for i, entry := range entries {
		if !strings.EqualFold(strings.TrimSpace(entry.Host), host) {
			continue
		}
		if conflictsWithSerial(entry, hardwareSerial) {
			continue
		}
		// An observation without a hardware serial must not overwrite a durable
		// serial identity that may belong to a different device on this address.
		if hardwareSerial == "" && strings.TrimSpace(entry.HardwareSerial) != "" {
			continue
		}
		candidates = append(candidates, i)
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return -1, nil
	default:
		return -1, fmt.Errorf("multiple remembered wireless devices were seen at %s; choose one before reconnecting", host)
	}
}

// conflictsWithSerial reports whether a remembered entry's durable hardware serial
// contradicts the serial observed for the current service.
func conflictsWithSerial(entry core.RememberedWirelessDevice, hardwareSerial string) bool {
	if hardwareSerial == "" {
		return false
	}
	entrySerial := strings.TrimSpace(entry.HardwareSerial)
	return entrySerial != "" && !strings.EqualFold(entrySerial, hardwareSerial)
}

func buildRememberedWirelessEntry(
	existing *core.RememberedWirelessDevice,
	service device.MDNSService,
	info *device.Info,
	seenAt time.Time,
) core.RememberedWirelessDevice {
	entry := core.RememberedWirelessDevice{}
	oldHost := ""
	if existing != nil {
		entry = *existing
		oldHost = existing.Host
	}

	hardwareSerial := strings.TrimSpace(entry.HardwareSerial)
	if info != nil && strings.TrimSpace(info.HardwareSerial) != "" {
		hardwareSerial = strings.TrimSpace(info.HardwareSerial)
	}

	if entry.Key == "" {
		switch {
		case hardwareSerial != "":
			entry.Key = "serial:" + hardwareSerial
		case strings.TrimSpace(service.InstanceName) != "":
			entry.Key = strings.TrimSpace(service.InstanceName)
		default:
			entry.Key = service.Host
		}
	}

	customName := entry.Name != "" && entry.Name != oldHost

	entry.InstanceName = service.InstanceName
	entry.Host = service.Host
	entry.LastAddress = service.Address
	entry.HardwareSerial = hardwareSerial
	entry.AutoConnect = true
	entry.LastSeenAt = seenAt.UTC().Format(time.RFC3339)

	if info != nil {
		if strings.TrimSpace(info.Model) != "" {
			entry.Model = strings.TrimSpace(info.Model)
		}
		if strings.TrimSpace(info.Manufacturer) != "" {
			entry.Manufacturer = strings.TrimSpace(info.Manufacturer)
		}
		if strings.TrimSpace(info.AndroidVersion) != "" {
			entry.AndroidVersion = strings.TrimSpace(info.AndroidVersion)
		}
	}

	if !customName {
		switch {
		case entry.Model != "":
			entry.Name = entry.Model
		case entry.Name != "":
			// Preserve the previous generated name if enrichment is unavailable.
		default:
			entry.Name = service.Host
		}
	}

	return entry
}

// resolveRememberedWirelessService maps one remembered TV to its current connect
// endpoint. Pairing and connect advertisements may carry different instance names,
// so an instance match is a preference, not a requirement. A host-only fallback is
// accepted only when exactly one connect (or legacy) endpoint is advertised for
// that host: two different endpoints must never be resolved by picking the first.
func resolveRememberedWirelessService(
	services []device.MDNSService,
	entry core.RememberedWirelessDevice,
) (device.MDNSService, bool) {
	if instance := strings.TrimSpace(entry.InstanceName); instance != "" {
		byInstance := make([]device.MDNSService, 0, 1)
		for _, service := range services {
			if strings.EqualFold(service.InstanceName, instance) {
				byInstance = append(byInstance, service)
			}
		}
		if service, ok := device.PickConnectEndpoint(byInstance); ok {
			return service, true
		}
	}

	host := strings.TrimSpace(entry.Host)
	if host == "" {
		return device.MDNSService{}, false
	}

	byHost := make([]device.MDNSService, 0, 2)
	for _, service := range services {
		if strings.EqualFold(service.Host, host) {
			byHost = append(byHost, service)
		}
	}
	return device.PickConnectEndpoint(byHost)
}

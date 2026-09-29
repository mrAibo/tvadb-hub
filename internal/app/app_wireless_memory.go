package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
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

		services, err := a.wireSvc.Discover(a.ctx)
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

	match := findRememberedWirelessMatch(a.cfg.RememberedWireless, service, info)
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

func findRememberedWirelessMatch(
	entries []core.RememberedWirelessDevice,
	service device.MDNSService,
	info *device.Info,
) int {
	hardwareSerial := ""
	if info != nil {
		hardwareSerial = strings.TrimSpace(info.HardwareSerial)
	}

	if hardwareSerial != "" {
		for i, entry := range entries {
			if entry.HardwareSerial != "" &&
				strings.EqualFold(entry.HardwareSerial, hardwareSerial) {
				return i
			}
		}
	}

	for i, entry := range entries {
		sameInstance := service.InstanceName != "" &&
			entry.InstanceName != "" &&
			strings.EqualFold(entry.InstanceName, service.InstanceName)
		if sameInstance {
			return i
		}
	}

	for i, entry := range entries {
		if entry.Host != "" && strings.EqualFold(entry.Host, service.Host) {
			return i
		}
	}

	return -1
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

func resolveRememberedWirelessService(
	services []device.MDNSService,
	entry core.RememberedWirelessDevice,
) (device.MDNSService, bool) {
	var hostFallback *device.MDNSService
	for i := range services {
		service := services[i]
		if service.Kind != device.MDNSServiceConnect && service.Kind != device.MDNSServiceLegacy {
			continue
		}

		if entry.InstanceName != "" &&
			strings.EqualFold(service.InstanceName, entry.InstanceName) {
			return service, true
		}
		if entry.Host != "" && strings.EqualFold(service.Host, entry.Host) {
			candidate := service
			if hostFallback == nil ||
				(hostFallback.Kind == device.MDNSServiceLegacy &&
				 candidate.Kind == device.MDNSServiceConnect) {
				hostFallback = &candidate
			}
		}
	}
	if hostFallback != nil {
		return *hostFallback, true
	}
	return device.MDNSService{}, false
}

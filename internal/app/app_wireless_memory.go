package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"fmt"
	"strings"
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

	key := strings.TrimSpace(service.InstanceName)
	if key == "" {
		key = service.Host
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

	match := -1
	for i, entry := range a.cfg.RememberedWireless {
		sameInstance := service.InstanceName != "" &&
			entry.InstanceName != "" &&
			strings.EqualFold(entry.InstanceName, service.InstanceName)
		sameHost := entry.Host != "" && strings.EqualFold(entry.Host, service.Host)
		if sameInstance || sameHost {
			match = i
			break
		}
	}

	entry := core.RememberedWirelessDevice{
		Key:          key,
		InstanceName: service.InstanceName,
		Host:         service.Host,
		LastAddress:  service.Address,
		Name:         service.Host,
		AutoConnect:  true,
	}
	if match >= 0 {
		existing := a.cfg.RememberedWireless[match]
		if existing.Name != "" {
			entry.Name = existing.Name
		}
		entry.AutoConnect = existing.AutoConnect || !existing.AutoConnect
		a.cfg.RememberedWireless[match] = entry
	} else {
		a.cfg.RememberedWireless = append(a.cfg.RememberedWireless, entry)
	}

	return core.SaveConfig(a.dataDir, a.cfg)
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

func rememberedWirelessLabel(entry core.RememberedWirelessDevice) string {
	if entry.Name != "" {
		return entry.Name
	}
	if entry.Host != "" {
		return entry.Host
	}
	return fmt.Sprintf("wireless device %s", entry.Key)
}

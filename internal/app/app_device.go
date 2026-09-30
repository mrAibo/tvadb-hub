package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"fmt"
	"net"
	"strings"
)

func (a *App) GetDevices() ([]device.Summary, error) {
	return a.devSvc.ListDevices(a.ctx)
}

func (a *App) GetActiveSerial() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeSerial
}

func (a *App) SetActiveSerial(serial string) error {
	return auditVoidAction(a, "set_active_serial", func() error {
		devices, err := a.devSvc.ListDevices(a.ctx)
		if err != nil {
			return err
		}

		for _, d := range devices {
			if d.Serial == serial {
				a.mu.Lock()
				a.activeSerial = serial
				a.mu.Unlock()
				return nil
			}
		}

		return core.NewOperationError("set_active_serial", "device not found", fmt.Sprintf("serial '%s' is not connected", serial), true)
	})
}

func (a *App) GetDeviceInfo(serial string) (*device.Info, error) {
	resolved := serial
	if resolved == "" {
		a.mu.Lock()
		resolved = a.activeSerial
		a.mu.Unlock()
	}
	return a.devSvc.GetDeviceInfo(a.ctx, resolved)
}

func (a *App) GetDeviceMode(serial string) (device.Mode, error) {
	resolved := serial
	if resolved == "" {
		a.mu.Lock()
		resolved = a.activeSerial
		a.mu.Unlock()
	}
	return a.devSvc.DetectDeviceMode(a.ctx, resolved)
}

func (a *App) SendTVRemoteKey(serial string, key string) (string, error) {
	return auditAction(a, "send_tv_remote_key", func() (string, error) {
		resolved := strings.TrimSpace(serial)
		if resolved == "" {
			var err error
			resolved, err = a.resolveActiveSerial(a.ctx)
			if err != nil {
				return "", err
			}
		}
		return a.devSvc.SendTVRemoteKey(a.ctx, resolved, key)
	})
}

func (a *App) SendTVText(serial string, text string) (device.TVTextInputResult, error) {
	return auditAction(a, "send_tv_text", func() (device.TVTextInputResult, error) {
		resolved := strings.TrimSpace(serial)
		if resolved == "" {
			var err error
			resolved, err = a.resolveActiveSerial(a.ctx)
			if err != nil {
				return device.TVTextInputResult{}, err
			}
		}

		if err := device.ValidateTVTextInput(text); err != nil {
			return device.TVTextInputResult{}, err
		}

		// An active scrcpy session gets first use of DroidSphere's existing
		// clipboard bridge, followed by Android's PASTE key event. If that
		// path is unavailable, the device service applies its clipboard then
		// printable-ASCII input-text fallback policy.
		if a.scrSvc != nil {
			if session := a.scrSvc.GetActiveSession(); session != nil && session.Serial == resolved {
				if err := a.scrSvc.PushClipboard(resolved, text); err == nil {
					if _, err := a.devSvc.SendTVRemoteKey(a.ctx, resolved, "paste"); err == nil {
						return device.TVTextInputResult{
							Method: device.TVTextMethodScrcpyClipboard,
							Detail: "Pasted through the active scrcpy clipboard path",
						}, nil
					}
				}
			}
		}

		return a.devSvc.SendTVText(a.ctx, resolved, text)
	})
}

func (a *App) CaptureScreenshot(serial string, localPath string) (device.ScreenshotResult, error) {
	return auditAction(a, "capture_screenshot", func() (device.ScreenshotResult, error) {
		resolved := strings.TrimSpace(serial)
		if resolved == "" {
			var err error
			resolved, err = a.resolveActiveSerial(a.ctx)
			if err != nil {
				return device.ScreenshotResult{}, err
			}
		}
		return a.devSvc.CaptureScreenshot(a.ctx, resolved, localPath)
	})
}

func (a *App) RebootDevice(serial string, mode string) (string, error) {
	return auditAction(a, "reboot_device", func() (string, error) {
		resolved := serial
		if resolved == "" {
			a.mu.Lock()
			resolved = a.activeSerial
			a.mu.Unlock()
		}
		return a.devSvc.RebootDevice(a.ctx, resolved, mode)
	})
}

func (a *App) DiscoverWireless() ([]device.MDNSService, error) {
	return a.wireSvc.Discover(a.ctx)
}

func (a *App) DiscoverWirelessDevices() ([]device.DiscoveredWirelessDevice, error) {
	return a.wireSvc.DiscoverDevices(a.ctx)
}

func (a *App) AutoConnectWireless(selector string) (device.WirelessConnectResult, error) {
	return auditAction(a, "auto_connect_wireless", func() (device.WirelessConnectResult, error) {
		result, err := a.wireSvc.AutoConnect(a.ctx, selector)
		if err != nil {
			return result, err
		}
		if err := a.rememberWirelessService(result.Service); err != nil {
			return result, err
		}
		return result, nil
	})
}

func (a *App) PairDiscoveredWireless(selector string, code string) (device.WirelessPairResult, error) {
	return auditAction(a, "pair_discovered_wireless", func() (device.WirelessPairResult, error) {
		return a.wireSvc.PairDiscovered(a.ctx, selector, code)
	})
}

func (a *App) PairAndConnectWireless(selector string, code string) (device.WirelessPairAndConnectResult, error) {
	return auditAction(a, "pair_and_connect_wireless", func() (device.WirelessPairAndConnectResult, error) {
		result, err := a.wireSvc.PairAndConnect(a.ctx, selector, code)
		if err != nil {
			return result, err
		}
		if err := a.rememberWirelessService(result.ConnectService); err != nil {
			return result, err
		}
		return result, nil
	})
}

func (a *App) ConnectWireless(address string) (string, error) {
	return auditAction(a, "connect_wireless", func() (string, error) {
		return a.wireSvc.Connect(a.ctx, address)
	})
}

func (a *App) EnableWirelessTCPIP(port string, serial string) (string, error) {
	return auditAction(a, "enable_wireless_tcpip", func() (string, error) {
		resolved := serial
		if resolved == "" {
			a.mu.Lock()
			resolved = a.activeSerial
			a.mu.Unlock()
		}
		return a.wireSvc.EnableTCPIP(a.ctx, resolved, port)
	})
}

func (a *App) DisconnectWireless(address string) (string, error) {
	return auditAction(a, "disconnect_wireless", func() (string, error) {
		return a.wireSvc.Disconnect(a.ctx, address)
	})
}

func (a *App) PairWireless(address string, code string) (string, error) {
	return auditAction(a, "pair_wireless", func() (string, error) {
		return a.wireSvc.Pair(a.ctx, address, code)
	})
}

func (a *App) GetWirelessHistory() []core.WirelessHistoryEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return []core.WirelessHistoryEntry{}
	}
	return cloneWirelessHistory(a.cfg.WirelessHistory)
}

func (a *App) SaveWirelessHistory(entries []core.WirelessHistoryEntry) error {
	return auditVoidAction(a, "save_wireless_history", func() error {
		normalized := make([]core.WirelessHistoryEntry, 0, len(entries))
		seen := make(map[string]struct{}, len(entries))
		for _, entry := range entries {
			address := strings.TrimSpace(entry.Address)
			if _, _, err := net.SplitHostPort(address); err != nil {
				return core.NewOperationError("save_wireless_history", "wireless address is invalid", address, false)
			}
			key := strings.ToLower(address)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				name = address
			}
			normalized = append(normalized, core.WirelessHistoryEntry{Address: address, Name: name})
		}

		a.mu.Lock()
		defer a.mu.Unlock()
		if a.cfg == nil {
			return core.NewOperationError("save_wireless_history", "app config is not available", "", false)
		}
		a.cfg.WirelessHistory = normalized
		return core.SaveConfig(a.dataDir, a.cfg)
	})
}

func cloneWirelessHistory(input []core.WirelessHistoryEntry) []core.WirelessHistoryEntry {
	if input == nil {
		return []core.WirelessHistoryEntry{}
	}
	out := make([]core.WirelessHistoryEntry, len(input))
	copy(out, input)
	return out
}

func (a *App) GetPerformanceSnapshot(serial string) (device.PerformanceSnapshot, error) {
	resolved := serial
	if resolved == "" {
		a.mu.Lock()
		resolved = a.activeSerial
		a.mu.Unlock()
	}
	return a.monSvc.GetSnapshot(a.ctx, resolved)
}

func (a *App) GetDeviceNicknames() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return map[string]string{}
	}
	return cloneStringMap(a.cfg.DeviceNicknames)
}

func (a *App) SetDeviceNickname(serial string, nickname string) error {
	return auditVoidAction(a, "set_device_nickname", func() error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.cfg == nil {
			return core.NewOperationError("set_device_nickname", "app config is not available", "", false)
		}
		if a.cfg.DeviceNicknames == nil {
			a.cfg.DeviceNicknames = map[string]string{}
		}
		a.cfg.DeviceNicknames[serial] = nickname
		return core.SaveConfig(a.dataDir, a.cfg)
	})
}

func (a *App) ClearDeviceNickname(serial string) error {
	return auditVoidAction(a, "clear_device_nickname", func() error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.cfg == nil {
			return core.NewOperationError("clear_device_nickname", "app config is not available", "", false)
		}
		delete(a.cfg.DeviceNicknames, serial)
		return core.SaveConfig(a.dataDir, a.cfg)
	})
}

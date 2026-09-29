package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

type ConnectionDoctorStatus string

const (
	ConnectionDoctorPass    ConnectionDoctorStatus = "pass"
	ConnectionDoctorWarning ConnectionDoctorStatus = "warning"
	ConnectionDoctorFail    ConnectionDoctorStatus = "fail"
	ConnectionDoctorInfo    ConnectionDoctorStatus = "info"
)

type ConnectionDoctorCheck struct {
	ID             string                 `json:"id"`
	Group          string                 `json:"group"`
	Label          string                 `json:"label"`
	Status         ConnectionDoctorStatus `json:"status"`
	Detail         string                 `json:"detail"`
	Recommendation string                 `json:"recommendation,omitempty"`
	Action         string                 `json:"action,omitempty"`
}

type ConnectionDoctorReport struct {
	GeneratedAt  string                  `json:"generatedAt"`
	OS           string                  `json:"os"`
	Arch         string                  `json:"arch"`
	Healthy      bool                    `json:"healthy"`
	PassCount    int                     `json:"passCount"`
	WarningCount int                     `json:"warningCount"`
	FailCount    int                     `json:"failCount"`
	InfoCount    int                     `json:"infoCount"`
	TargetSerial string                  `json:"targetSerial,omitempty"`
	TargetModel  string                  `json:"targetModel,omitempty"`
	TargetMode   string                  `json:"targetMode,omitempty"`
	TargetState  string                  `json:"targetState,omitempty"`
	Transport    string                  `json:"transport,omitempty"`
	Checks       []ConnectionDoctorCheck `json:"checks"`
}

func (r *ConnectionDoctorReport) add(check ConnectionDoctorCheck) {
	r.Checks = append(r.Checks, check)
	switch check.Status {
	case ConnectionDoctorPass:
		r.PassCount++
	case ConnectionDoctorWarning:
		r.WarningCount++
	case ConnectionDoctorFail:
		r.FailCount++
		r.Healthy = false
	case ConnectionDoctorInfo:
		r.InfoCount++
	}
}

func (a *App) GetConnectionDoctorReport() (ConnectionDoctorReport, error) {
	return auditAction(a, "connection_doctor", func() (ConnectionDoctorReport, error) {
		report := ConnectionDoctorReport{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
			Healthy:     true,
			Checks:      []ConnectionDoctorCheck{},
		}

		a.mu.Lock()
		status := a.binSvc.GetBinaryStatus(a.cfg)
		activeSerial := a.activeSerial
		setupCompleted := a.cfg != nil && a.cfg.SetupCompleted
		a.mu.Unlock()

		report.add(ConnectionDoctorCheck{
			ID: "host", Group: "Host", Label: "Host environment", Status: ConnectionDoctorPass,
			Detail: fmt.Sprintf("%s/%s · data directory %s", runtime.GOOS, runtime.GOARCH, a.dataDir),
		})
		if setupCompleted {
			report.add(ConnectionDoctorCheck{
				ID: "setup", Group: "Host", Label: "DroidSphere setup", Status: ConnectionDoctorPass,
				Detail: "Initial Android tool setup is complete.",
			})
		} else {
			report.add(ConnectionDoctorCheck{
				ID: "setup", Group: "Host", Label: "DroidSphere setup", Status: ConnectionDoctorWarning,
				Detail: "Initial Android tool setup is not marked complete.",
				Recommendation: "Open Settings → Binaries and complete tool setup.",
			})
		}

		if status == nil || status.Adb == nil || status.Adb.Status != core.BinaryReady {
			detail := "ADB executable is not available."
			if status != nil && status.Adb != nil && strings.TrimSpace(status.Adb.Reason) != "" {
				detail = status.Adb.Reason
			}
			report.add(ConnectionDoctorCheck{
				ID: "adb-binary", Group: "Toolchain", Label: "ADB", Status: ConnectionDoctorFail,
				Detail: detail,
				Recommendation: "Install or select current Android SDK Platform Tools in Settings → Binaries.",
			})
			return report, nil
		}

		report.add(ConnectionDoctorCheck{
			ID: "adb-binary", Group: "Toolchain", Label: "ADB", Status: ConnectionDoctorPass,
			Detail: fmt.Sprintf("%s · %s · %s", status.Adb.Version, status.Adb.Source, status.Adb.Path),
		})
		versionCheck := adbVersionDiagnostic(status.Adb.Version)
		report.add(ConnectionDoctorCheck{
			ID: "adb-version", Group: "Toolchain", Label: versionCheck.Label,
			Status: doctorStatus(versionCheck.Status), Detail: versionCheck.Detail,
			Recommendation: versionCheck.Recommendation,
		})

		if len(status.AdbCandidates) > 1 {
			paths := make([]string, 0, len(status.AdbCandidates))
			for _, candidate := range status.AdbCandidates {
				paths = append(paths, candidate.Path)
			}
			report.add(ConnectionDoctorCheck{
				ID: "adb-candidates", Group: "Toolchain", Label: "Multiple ADB installations",
				Status: ConnectionDoctorWarning,
				Detail: fmt.Sprintf("%d valid ADB executables were found: %s", len(paths), strings.Join(paths, " · ")),
				Recommendation: "Keep DroidSphere pinned to one known Platform Tools installation. Multiple adb clients can make daemon/version troubleshooting confusing.",
			})
		} else {
			report.add(ConnectionDoctorCheck{
				ID: "adb-candidates", Group: "Toolchain", Label: "ADB installation selection",
				Status: ConnectionDoctorPass, Detail: "One resolved ADB installation is in use.",
			})
		}

		addOptionalBinaryCheck(&report, "fastboot", "Fastboot", status.Fastboot)
		addOptionalBinaryCheck(&report, "scrcpy", "scrcpy", status.Scrcpy)

		daemonResult, daemonErr := core.RunCommand(a.ctx, core.ExecRequest{
			Command: status.Adb.Path,
			Args:    []string{"start-server"},
			Timeout: 8 * time.Second,
		})
		if daemonErr != nil || daemonResult.ExitCode != 0 {
			detail := strings.TrimSpace(daemonResult.Stderr)
			if detail == "" && daemonErr != nil {
				detail = daemonErr.Error()
			}
			if detail == "" {
				detail = "adb start-server returned a non-zero result."
			}
			report.add(ConnectionDoctorCheck{
				ID: "adb-daemon", Group: "ADB server", Label: "ADB server",
				Status: ConnectionDoctorFail, Detail: detail,
				Recommendation: "Restart the ADB server. If the problem returns, close other ADB tools and make sure they use the same Platform Tools installation.",
				Action: "restart-adb",
			})
		} else {
			report.add(ConnectionDoctorCheck{
				ID: "adb-daemon", Group: "ADB server", Label: "ADB server",
				Status: ConnectionDoctorPass, Detail: "ADB daemon responded successfully.",
			})
		}

		devices, err := a.devSvc.ListDevices(a.ctx)
		if err != nil {
			report.add(ConnectionDoctorCheck{
				ID: "device-inventory", Group: "Device", Label: "Device discovery",
				Status: ConnectionDoctorFail, Detail: err.Error(),
				Recommendation: "Restart the ADB server, reconnect USB/Wireless debugging, and verify the device appears in adb devices.",
				Action: "restart-adb",
			})
			return report, nil
		}
		if len(devices) == 0 {
			report.add(ConnectionDoctorCheck{
				ID: "device-inventory", Group: "Device", Label: "Device discovery",
				Status: ConnectionDoctorFail, Detail: "No ADB or Fastboot device is currently detected.",
				Recommendation: "For USB: enable USB debugging, use a data-capable cable and accept the RSA prompt. For Wireless debugging: pair/connect the device from Devices.",
			})
			return report, nil
		}

		report.add(ConnectionDoctorCheck{
			ID: "device-inventory", Group: "Device", Label: "Device discovery",
			Status: ConnectionDoctorPass, Detail: fmt.Sprintf("%d Android/Fastboot target(s) detected.", len(devices)),
		})

		target, ok := selectDoctorTarget(devices, activeSerial)
		if !ok {
			report.add(ConnectionDoctorCheck{
				ID: "target", Group: "Device", Label: "Active target",
				Status: ConnectionDoctorFail,
				Detail: "Multiple devices are connected and no unambiguous active target could be selected.",
				Recommendation: "Choose the device in the Current+ device selector, then run Connection Doctor again.",
			})
			return report, nil
		}

		report.TargetSerial = target.Serial
		report.TargetMode = string(target.Mode)
		report.TargetState = string(target.State)
		if len(devices) > 1 {
			report.add(ConnectionDoctorCheck{
				ID: "target", Group: "Device", Label: "Active target",
				Status: ConnectionDoctorInfo,
				Detail: fmt.Sprintf("%s is pinned as the diagnostic target while %d targets are connected.", target.Serial, len(devices)),
			})
		} else {
			report.add(ConnectionDoctorCheck{
				ID: "target", Group: "Device", Label: "Active target",
				Status: ConnectionDoctorPass, Detail: target.Serial,
			})
		}

		addDeviceStateCheck(&report, target)
		if target.Mode != device.ModeADB || target.State != device.StateReady {
			return report, nil
		}

		info, infoErr := a.devSvc.GetDeviceInfo(a.ctx, target.Serial)
		if infoErr == nil && info != nil {
			report.TargetModel = firstNonEmpty(info.Model, info.Product, info.Device)
			report.add(ConnectionDoctorCheck{
				ID: "metadata", Group: "Device", Label: "Android metadata",
				Status: ConnectionDoctorPass,
				Detail: fmt.Sprintf("%s · Android %s · SDK %s", firstNonEmpty(info.Model, target.Serial), firstNonEmpty(info.AndroidVersion, "?"), firstNonEmpty(info.SDKVersion, "?")),
			})
		} else if infoErr != nil {
			report.add(ConnectionDoctorCheck{
				ID: "metadata", Group: "Device", Label: "Android metadata",
				Status: ConnectionDoctorWarning, Detail: infoErr.Error(),
				Recommendation: "The ADB connection works, but device properties could not be read. Reconnect the device and run the doctor again.",
			})
		}

		host, wireless := networkADBHost(target.Serial)
		if !wireless {
			report.Transport = "usb"
			report.add(ConnectionDoctorCheck{
				ID: "transport", Group: "Transport", Label: "Active transport",
				Status: ConnectionDoctorPass,
				Detail: "USB/local ADB transport. Wireless-only checks are not required.",
			})
			return report, nil
		}

		report.Transport = "wireless"
		report.add(ConnectionDoctorCheck{
			ID: "transport", Group: "Transport", Label: "Active transport",
			Status: ConnectionDoctorPass, Detail: "Wireless ADB · " + target.Serial,
		})

		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		tcpErr := probeTCPEndpoint(ctx, target.Serial)
		cancel()
		if tcpErr != nil {
			report.add(ConnectionDoctorCheck{
				ID: "wireless-tcp", Group: "Wireless ADB", Label: "Active TCP endpoint",
				Status: ConnectionDoctorWarning,
				Detail: fmt.Sprintf("%s did not accept a fresh TCP probe: %v", target.Serial, tcpErr),
				Recommendation: "The existing ADB session may still be alive. If reconnect fails, check Wi-Fi client isolation/firewall and rescan for a changed dynamic port.",
			})
		} else {
			report.add(ConnectionDoctorCheck{
				ID: "wireless-tcp", Group: "Wireless ADB", Label: "Active TCP endpoint",
				Status: ConnectionDoctorPass, Detail: target.Serial + " accepts TCP connections.",
			})
		}

		wirelessReport := a.GetWirelessDiagnostics(host)
		for _, check := range wirelessReport.Checks {
			if check.ID == "adb" || check.ID == "adb_version" || check.ID == "tcp" || check.ID == "adb_state" {
				continue
			}
			status := doctorStatus(check.Status)
			if check.ID == "mdns" && status == ConnectionDoctorFail {
				status = ConnectionDoctorWarning
			}
			report.add(ConnectionDoctorCheck{
				ID: "wireless-" + check.ID, Group: "Wireless ADB", Label: check.Label,
				Status: status, Detail: check.Detail, Recommendation: doctorRecommendation(check),
			})
		}

		return report, nil
	})
}

func (a *App) RestartADBServer() (string, error) {
	return auditAction(a, "restart_adb_server", func() (string, error) {
		a.mu.Lock()
		status := a.binSvc.GetBinaryStatus(a.cfg)
		a.mu.Unlock()
		if status == nil || status.Adb == nil || status.Adb.Status != core.BinaryReady {
			return "", core.NewOperationError("restart_adb_server", "ADB executable is not available", "", false)
		}
		kill, err := core.RunCommand(a.ctx, core.ExecRequest{
			Command: status.Adb.Path,
			Args:    []string{"kill-server"},
			Timeout: 8 * time.Second,
		})
		if err != nil && kill.ExitCode != 0 {
			return "", core.NewOperationError("restart_adb_server", "failed to stop ADB server", err.Error(), true)
		}
		start, err := core.RunCommand(a.ctx, core.ExecRequest{
			Command: status.Adb.Path,
			Args:    []string{"start-server"},
			Timeout: 10 * time.Second,
		})
		if err != nil || start.ExitCode != 0 {
			detail := strings.TrimSpace(start.Stderr)
			if detail == "" && err != nil {
				detail = err.Error()
			}
			return "", core.NewOperationError("restart_adb_server", "failed to start ADB server", detail, true)
		}
		return "ADB server restarted successfully.", nil
	})
}

func addOptionalBinaryCheck(report *ConnectionDoctorReport, id, label string, info *core.BinaryInfo) {
	if info == nil || info.Status != core.BinaryReady {
		detail := label + " is not ready."
		if info != nil && strings.TrimSpace(info.Reason) != "" {
			detail = info.Reason
		}
		report.add(ConnectionDoctorCheck{
			ID: id, Group: "Toolchain", Label: label, Status: ConnectionDoctorWarning,
			Detail: detail,
			Recommendation: fmt.Sprintf("Install or select %s in Settings → Binaries to enable all related features.", label),
		})
		return
	}
	report.add(ConnectionDoctorCheck{
		ID: id, Group: "Toolchain", Label: label, Status: ConnectionDoctorPass,
		Detail: fmt.Sprintf("%s · %s · %s", info.Version, info.Source, info.Path),
	})
}

func selectDoctorTarget(devices []device.Summary, activeSerial string) (device.Summary, bool) {
	activeSerial = strings.TrimSpace(activeSerial)
	if activeSerial != "" {
		for _, candidate := range devices {
			if candidate.Serial == activeSerial {
				return candidate, true
			}
		}
	}
	if len(devices) == 1 {
		return devices[0], true
	}
	return device.Summary{}, false
}

func addDeviceStateCheck(report *ConnectionDoctorReport, target device.Summary) {
	switch {
	case target.Mode == device.ModeFastboot:
		report.add(ConnectionDoctorCheck{
			ID: "device-state", Group: "Device", Label: "Device state", Status: ConnectionDoctorInfo,
			Detail: target.Serial + " is in Fastboot mode. ADB-only diagnostics are intentionally skipped.",
			Recommendation: "Reboot to Android if you need ADB, apps, files, Logcat or scrcpy.",
		})
	case target.State == device.StateReady:
		report.add(ConnectionDoctorCheck{
			ID: "device-state", Group: "Device", Label: "ADB authorization",
			Status: ConnectionDoctorPass, Detail: target.Serial + " is connected and authorized.",
		})
	case target.State == device.StateUnauthorized:
		report.add(ConnectionDoctorCheck{
			ID: "device-state", Group: "Device", Label: "ADB authorization",
			Status: ConnectionDoctorFail, Detail: target.Serial + " is unauthorized.",
			Recommendation: "Unlock the Android device and accept the RSA debugging prompt. If no prompt appears, revoke USB debugging authorizations and reconnect.",
		})
	case target.State == device.StateOffline:
		report.add(ConnectionDoctorCheck{
			ID: "device-state", Group: "Device", Label: "ADB device state",
			Status: ConnectionDoctorFail, Detail: target.Serial + " is offline.",
			Recommendation: "Restart the ADB server and reconnect the USB cable or Wireless ADB endpoint.",
			Action: "restart-adb",
		})
	default:
		report.add(ConnectionDoctorCheck{
			ID: "device-state", Group: "Device", Label: "ADB device state",
			Status: ConnectionDoctorWarning, Detail: fmt.Sprintf("%s reports state %s.", target.Serial, target.State),
			Recommendation: "Return the device to normal Android/ADB mode, then run Connection Doctor again.",
		})
	}
}

func networkADBHost(serial string) (string, bool) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return "", false
	}
	if strings.HasPrefix(serial, "[") {
		if end := strings.Index(serial, "]"); end > 1 && end+1 < len(serial) && serial[end+1] == ':' {
			return serial[1:end], true
		}
		return "", false
	}
	idx := strings.LastIndex(serial, ":")
	if idx <= 0 || idx == len(serial)-1 {
		return "", false
	}
	for _, r := range serial[idx+1:] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return serial[:idx], true
}

func doctorStatus(status WirelessDiagnosticStatus) ConnectionDoctorStatus {
	switch status {
	case WirelessDiagnosticPass:
		return ConnectionDoctorPass
	case WirelessDiagnosticWarning:
		return ConnectionDoctorWarning
	case WirelessDiagnosticFail:
		return ConnectionDoctorFail
	default:
		return ConnectionDoctorInfo
	}
}

func doctorRecommendation(check WirelessDiagnosticCheck) string {
	value := strings.ReplaceAll(check.Recommendation, "TV", "Android device")
	value = strings.ReplaceAll(value, "television", "Android device")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

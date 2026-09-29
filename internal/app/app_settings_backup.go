package app

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"ADBKit/internal/core"
)

const settingsBackupFormat = "droidsphere-settings"
const legacySettingsBackupFormat = "tvadb-hub-settings"
const settingsBackupVersion = 1

type settingsBackupEnvelope struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	AppVersion  string          `json:"appVersion"`
	ExportedAt  string          `json:"exportedAt"`
	WindowState string          `json:"windowState"`
	Config      json.RawMessage `json:"config"`
}

func (a *App) ExportSettings(path string) error {
	return auditVoidAction(a, "export_settings", func() error {
		target := strings.TrimSpace(path)
		if target == "" {
			return core.NewOperationError("export_settings", "export path is required", "", false)
		}

		a.mu.Lock()
		configJSON, err := json.Marshal(a.cfg)
		windowState := core.LoadWindowState(a.dataDir)
		a.mu.Unlock()
		if err != nil {
			return core.NewOperationError("export_settings", "failed to encode settings", err.Error(), true)
		}

		envelope := settingsBackupEnvelope{
			Format:      settingsBackupFormat,
			Version:     settingsBackupVersion,
			AppVersion:  core.Version,
			ExportedAt:  time.Now().UTC().Format(time.RFC3339),
			WindowState: windowState,
			Config:      configJSON,
		}
		data, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return core.NewOperationError("export_settings", "failed to create settings backup", err.Error(), true)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return core.NewOperationError("export_settings", "failed to write settings backup", err.Error(), true)
		}
		return nil
	})
}

func (a *App) ImportSettings(path string) (core.AppConfigSnapshot, error) {
	return auditAction(a, "import_settings", func() (core.AppConfigSnapshot, error) {
		target := strings.TrimSpace(path)
		if target == "" {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "import path is required", "", false)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "failed to read settings backup", err.Error(), true)
		}

		var envelope settingsBackupEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "settings backup is not valid JSON", err.Error(), false)
		}
		if envelope.Format != settingsBackupFormat && envelope.Format != legacySettingsBackupFormat {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "unsupported settings backup format", envelope.Format, false)
		}
		if envelope.Version != settingsBackupVersion {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "unsupported settings backup version", "", false)
		}

		next := core.DefaultConfig()
		if err := json.Unmarshal(envelope.Config, next); err != nil {
			return core.AppConfigSnapshot{}, core.NewOperationError("import_settings", "settings payload is invalid", err.Error(), false)
		}
		normalizeImportedConfig(next)

		a.mu.Lock()
		if a.cfg == nil {
			a.cfg = core.DefaultConfig()
		}
		// Mutate the existing config object instead of replacing its pointer:
		// several services hold closures that resolve paths through this object.
		*a.cfg = *next
		if err := core.SaveConfig(a.dataDir, a.cfg); err != nil {
			a.mu.Unlock()
			return core.AppConfigSnapshot{}, err
		}
		snapshot := a.snapshotConfigLocked()
		a.mu.Unlock()

		if envelope.WindowState != "" {
			if err := core.SaveWindowState(a.dataDir, core.NormalizeWindowState(envelope.WindowState)); err != nil {
				return core.AppConfigSnapshot{}, err
			}
		}
		return snapshot, nil
	})
}

func normalizeImportedConfig(cfg *core.AppConfig) {
	if cfg.Theme != core.ThemeDark && cfg.Theme != core.ThemeLight {
		cfg.Theme = core.ThemeDark
	}
	if cfg.BinaryVersions == nil {
		cfg.BinaryVersions = map[string]string{}
	}
	if cfg.DeviceNicknames == nil {
		cfg.DeviceNicknames = map[string]string{}
	}
	if cfg.WirelessHistory == nil {
		cfg.WirelessHistory = []core.WirelessHistoryEntry{}
	}
	if cfg.RememberedWireless == nil {
		cfg.RememberedWireless = []core.RememberedWirelessDevice{}
	}
	if cfg.ScrcpyPresets == nil {
		cfg.ScrcpyPresets = []core.ScrcpyPreset{}
	}
	if cfg.LogcatBufferLimit <= 0 {
		cfg.LogcatBufferLimit = core.DefaultLogcatBufferLimit
	}
	if cfg.DefaultTerminalMode == "" {
		cfg.DefaultTerminalMode = core.DefaultTerminalMode
	}
	if cfg.DeviceRefreshSeconds <= 0 {
		cfg.DeviceRefreshSeconds = core.DefaultDeviceRefreshSeconds
	}
}

package app

import (
	"testing"

	"ADBKit/internal/core"
)

func TestNormalizeImportedConfig(t *testing.T) {
	cfg := &core.AppConfig{
		Theme:                "broken",
		LogcatBufferLimit:    0,
		DefaultTerminalMode:  "",
		DeviceRefreshSeconds: 0,
	}
	normalizeImportedConfig(cfg)

	if cfg.Theme != core.ThemeDark {
		t.Fatalf("theme=%q", cfg.Theme)
	}
	if cfg.LogcatBufferLimit != core.DefaultLogcatBufferLimit {
		t.Fatalf("logcat buffer=%d", cfg.LogcatBufferLimit)
	}
	if cfg.DefaultTerminalMode != core.DefaultTerminalMode {
		t.Fatalf("terminal mode=%q", cfg.DefaultTerminalMode)
	}
	if cfg.DeviceRefreshSeconds != core.DefaultDeviceRefreshSeconds {
		t.Fatalf("refresh seconds=%d", cfg.DeviceRefreshSeconds)
	}
	if cfg.BinaryVersions == nil || cfg.DeviceNicknames == nil || cfg.RememberedWireless == nil {
		t.Fatal("maps/slices must be normalized")
	}
}


func TestSettingsBackupFormatsKeepLegacyCompatibility(t *testing.T) {
	if settingsBackupFormat != "droidsphere-settings" {
		t.Fatalf("new backup format=%q", settingsBackupFormat)
	}
	if legacySettingsBackupFormat != "tvadb-hub-settings" {
		t.Fatalf("legacy backup format=%q", legacySettingsBackupFormat)
	}
}

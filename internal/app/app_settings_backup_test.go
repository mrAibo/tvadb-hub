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
		FileTransferCompression: "invalid",
		ScrcpyOptions: core.ScrcpyOptions{
			AudioSource: "invalid",
			AudioOnly:   true,
			NoAudio:     true,
		},
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
	if cfg.FileTransferCompression != core.DefaultFileTransferCompression {
		t.Fatalf("file transfer compression=%q", cfg.FileTransferCompression)
	}
	if cfg.ScrcpyOptions.AudioSource != core.ScrcpyAudioSourceOutput || cfg.ScrcpyOptions.AudioOnly {
		t.Fatalf("scrcpy audio options were not normalized: %#v", cfg.ScrcpyOptions)
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

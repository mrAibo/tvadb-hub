package app

import (
	"runtime"
	"testing"

	"ADBKit/internal/core"
)

func TestGetAppInfo(t *testing.T) {
	info := NewApp().GetAppInfo()

	if info.Name != "DroidSphere" {
		t.Fatalf("unexpected name: %q", info.Name)
	}
	if info.Version != core.Version {
		t.Fatalf("version mismatch: got %q want %q", info.Version, core.Version)
	}
	if info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		t.Fatalf("unexpected platform: %s/%s", info.OS, info.Arch)
	}
	if info.Repository != "https://github.com/mrAibo/tvadb-hub" {
		t.Fatalf("unexpected repository: %q", info.Repository)
	}
	if info.License != "MIT" {
		t.Fatalf("unexpected license: %q", info.License)
	}
}

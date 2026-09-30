package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCurrentConfigAndNicknamesDoNotExposeMutableAliases(t *testing.T) {
	a, _ := newTestApp(t)
	a.cfg.DeviceNicknames["A"] = "TV"
	a.cfg.BinaryVersions["adb"] = "version"
	a.cfg.WirelessHistory = []core.WirelessHistoryEntry{{Name: "TV"}}
	a.cfg.RememberedWireless = []core.RememberedWirelessDevice{{Name: "TV"}}
	a.cfg.ScrcpyPresets = []core.ScrcpyPreset{{Name: "TV"}}
	snapshot := a.currentConfig()
	snapshot.DeviceNicknames["A"] = "changed"
	snapshot.BinaryVersions["adb"] = "changed"
	snapshot.WirelessHistory[0].Name = "changed"
	snapshot.RememberedWireless[0].Name = "changed"
	snapshot.ScrcpyPresets[0].Name = "changed"
	names := a.GetDeviceNicknames()
	names["A"] = "changed"
	if a.cfg.DeviceNicknames["A"] != "TV" || a.cfg.BinaryVersions["adb"] != "version" || a.cfg.WirelessHistory[0].Name != "TV" || a.cfg.RememberedWireless[0].Name != "TV" || a.cfg.ScrcpyPresets[0].Name != "TV" {
		t.Fatal("config snapshot aliases application state")
	}
}

func TestConcurrentConfigReadsAndWrites(t *testing.T) {
	a, _ := newTestApp(t)
	getPaths := core.GetBinaryPathsFrom(a.currentConfig)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 80; i++ {
			a.mu.Lock()
			value := fmt.Sprint(i)
			a.cfg.AdbPath = value
			a.cfg.FastbootPath = value
			a.cfg.BinaryVersions["adb"] = value
			a.mu.Unlock()
			if err := a.SetDeviceNickname("A", value); err != nil {
				t.Errorf("nickname: %v", err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 160 {
			paths := getPaths()
			if paths.Adb != core.BinaryNameAdb && paths.Adb != paths.Fastboot {
				t.Errorf("incoherent tool paths: %+v", paths)
			}
			snapshot := a.currentConfig()
			snapshot.DeviceNicknames["local"] = "isolated"
			_ = a.GetDeviceNicknames()
			_ = a.GetAppConfig()
			_ = a.isAuditEnabled()
		}
	}()
	wg.Wait()
	if _, ok := a.GetDeviceNicknames()["local"]; ok {
		t.Fatal("caller mutated live nickname map")
	}
}

func TestSetActiveSerialDoesNotHoldConfigLockDuringDiscovery(t *testing.T) {
	a, _ := newTestApp(t)
	a.cfg.AdbPath = "droidsphere-test-missing-adb"
	a.cfg.FastbootPath = "droidsphere-test-missing-fastboot"
	a.ctx = context.Background()
	a.devSvc = device.NewService(a.dataDir, core.GetBinaryPathsFrom(a.currentConfig))
	done := make(chan error, 1)
	go func() { done <- a.SetActiveSerial("A") }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected absent device error")
		}
	case <-time.After(time.Second):
		t.Fatal("device selection deadlocked against synchronized config resolver")
	}
}

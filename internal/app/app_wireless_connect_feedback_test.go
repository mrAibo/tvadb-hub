package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAutoConnectWirelessKeepsConnectedResultWhenMemoryIsAmbiguous is the
// regression guard for a persistence failure reported as a CLI failure: adb has
// already connected, so the dynamic endpoint memory write must never turn the
// result into an error, must never overwrite an ambiguous durable identity, and
// the non-fatal warning must still reach the caller through the existing message
// field (no new API, no pairing PIN in the message).
func TestAutoConnectWirelessKeepsConnectedResultWhenMemoryIsAmbiguous(t *testing.T) {
	a, dataDir := newTestApp(t)
	a.ctx = context.Background()

	address := "192.168.1.20:37121"
	adb := newFakeAdbConnectTool(t, "connected to "+address)

	a.devSvc = device.NewService(dataDir, fakeBinaryPaths(adb))
	a.wireSvc = device.NewWirelessService(dataDir, fakeBinaryPaths(adb))
	a.cfg.RememberedWireless = []core.RememberedWirelessDevice{
		{Key: "one", Host: "192.168.1.20"},
		{Key: "two", Host: "192.168.1.20"},
	}

	result, err := a.AutoConnectWireless("192.168.1.20")
	if err != nil {
		t.Fatalf("a successful adb connect must not fail because memory was ambiguous: %v", err)
	}
	if result.Service.Address != address {
		t.Fatalf("unexpected connected endpoint: %#v", result.Service)
	}
	if !strings.Contains(result.Message, "connected to "+address) {
		t.Fatalf("the adb connect message was lost: %q", result.Message)
	}
	if !strings.Contains(result.Message, "not remembered") {
		t.Fatalf("the persistence warning was swallowed: %q", result.Message)
	}
	// The pairing-code-shaped check: nothing on this path carries a raw PIN.
	if strings.Contains(result.Message, "042915") {
		t.Fatalf("unexpected raw pairing code in message: %q", result.Message)
	}

	if len(a.cfg.RememberedWireless) != 2 {
		t.Fatalf("ambiguous memory overwrote the remembered list: %#v", a.cfg.RememberedWireless)
	}
	if a.cfg.RememberedWireless[0].Key != "one" || a.cfg.RememberedWireless[1].Key != "two" {
		t.Fatalf("ambiguous memory mutated durable identities: %#v", a.cfg.RememberedWireless)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "config.json")); statErr == nil {
		t.Fatal("an ambiguous memory write must not persist any config")
	}
}

// TestAutoConnectWirelessReportsUnavailableMemoryAsWarningOnly separates the two
// failure modes: a broken config store is a warning on a connected result, not a
// pretend CLI failure.
func TestAutoConnectWirelessReportsUnavailableMemoryAsWarningOnly(t *testing.T) {
	a, dataDir := newTestApp(t)
	a.ctx = context.Background()

	address := "192.168.1.20:37121"
	adb := newFakeAdbConnectTool(t, "connected to "+address)

	a.devSvc = device.NewService(dataDir, fakeBinaryPaths(adb))
	a.wireSvc = device.NewWirelessService(dataDir, fakeBinaryPaths(adb))
	a.cfg = nil

	result, err := a.AutoConnectWireless("192.168.1.20")
	if err != nil {
		t.Fatalf("a connected device must survive an unavailable config store: %v", err)
	}
	if !strings.Contains(result.Message, "not remembered") {
		t.Fatalf("the unavailable-memory warning was swallowed: %q", result.Message)
	}
}

func fakeBinaryPaths(adb string) func() core.BinaryPaths {
	return func() core.BinaryPaths {
		return core.BinaryPaths{Adb: adb, Fastboot: adb}
	}
}

// newFakeAdbConnectTool writes a local stand-in for adb that answers exactly the
// three calls this path makes: mDNS discovery, the connect itself, and the device
// list. No real device, server or network is involved, and nothing is dialled.
func newFakeAdbConnectTool(t *testing.T, connectMessage string) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "fake-adb.cmd")
		script := strings.Join([]string{
			"@echo off",
			"if \"%~1\"==\"mdns\" (",
			"  echo List of discovered mdns services",
			"  echo adb-tv _adb-tls-connect._tcp 192.168.1.20:37121",
			"  goto :eof",
			")",
			"if \"%~1\"==\"connect\" (",
			"  echo " + connectMessage,
			"  goto :eof",
			")",
			"if \"%~1\"==\"devices\" (",
			"  echo List of devices attached",
			"  goto :eof",
			")",
			"goto :eof",
			"",
		}, "\r\n")
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
			t.Fatalf("could not write the adb stand-in: %v", err)
		}
		return path
	}

	path := filepath.Join(dir, "fake-adb")
	script := strings.Join([]string{
		"#!/bin/sh",
		"case \"$1\" in",
		"  mdns)",
		"    echo \"List of discovered mdns services\"",
		"    echo \"adb-tv _adb-tls-connect._tcp 192.168.1.20:37121\"",
		"    exit 0",
		"    ;;",
		"  connect)",
		"    echo \"" + connectMessage + "\"",
		"    exit 0",
		"    ;;",
		"  devices)",
		"    echo \"List of devices attached\"",
		"    exit 0",
		"    ;;",
		"esac",
		"exit 0",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("could not write the adb stand-in: %v", err)
	}
	return path
}

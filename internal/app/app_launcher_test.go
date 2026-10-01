package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"ADBKit/internal/file"
	"ADBKit/internal/launcher"
	packagemgr "ADBKit/internal/package_mgr"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newLauncherTestApp wires an App whose launcher service resolves a tool path that
// cannot exist, so a refused admission is proven to send no command and an admitted
// call cannot reach a real adb either.
func newLauncherTestApp(t *testing.T, activeSerial string) (*App, *int) {
	t.Helper()
	a, dataDir := newTestApp(t)
	a.ctx = context.Background()
	a.activeSerial = activeSerial
	resolutions := 0
	getBinPath := func() core.BinaryPaths {
		resolutions++
		return core.BinaryPaths{Adb: filepath.Join(t.TempDir(), "missing-adb")}
	}
	a.lauSvc = launcher.NewService(dataDir, getBinPath)
	a.devSvc = device.NewService(dataDir, getBinPath)
	return a, &resolutions
}

func writeLauncherRecord(t *testing.T, dataDir, id, state string) {
	t.Helper()
	dir := filepath.Join(dataDir, "launcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	record := `{
  "schemaVersion": 1,
  "id": "` + id + `",
  "serial": "A",
  "userId": 0,
  "action": "set-home",
  "state": "` + state + `",
  "createdAt": "2026-10-01T00:00:00Z",
  "updatedAt": "2026-10-01T00:00:01Z",
  "originalComponent": "com.stock.launcher/com.stock.launcher.HomeActivity",
  "candidateComponent": "com.custom.launcher/com.custom.launcher.MainActivity"
}
`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLauncherAdmissionRefusesBlankAndStaleTargetBeforeAnyCommand covers the four
// admitted facades: a blank or stale serial must be refused before the launcher
// service is touched at all.
func TestLauncherAdmissionRefusesBlankAndStaleTargetBeforeAnyCommand(t *testing.T) {
	a, resolutions := newLauncherTestApp(t, "A")

	cases := []struct {
		name string
		call func() error
	}{
		{name: "preflight blank", call: func() error { _, err := a.PreflightLauncher(""); return err }},
		{name: "preflight stale", call: func() error { _, err := a.PreflightLauncher("B"); return err }},
		{name: "candidate test blank", call: func() error { _, err := a.TestLauncherCandidate("", "com.custom.launcher/.Main"); return err }},
		{name: "candidate test stale", call: func() error { _, err := a.TestLauncherCandidate("B", "com.custom.launcher/.Main"); return err }},
		{name: "apply blank", call: func() error {
			_, err := a.ApplyLauncher(launcher.ApplyRequest{OperationID: "op-blank0001", ExpectedComponent: "com.stock.launcher/.Home", CandidateComponent: "com.custom.launcher/.Main"})
			return err
		}},
		{name: "apply stale", call: func() error {
			_, err := a.ApplyLauncher(launcher.ApplyRequest{OperationID: "op-stale0001", ExpectedSerial: "B", ExpectedComponent: "com.stock.launcher/.Home", CandidateComponent: "com.custom.launcher/.Main"})
			return err
		}},
		{name: "restore blank", call: func() error {
			_, err := a.RestoreLauncher(launcher.RestoreRequest{RecordID: "op-record01"})
			return err
		}},
		{name: "restore stale", call: func() error {
			_, err := a.RestoreLauncher(launcher.RestoreRequest{ExpectedSerial: "B", RecordID: "op-record01"})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !isDeviceTargetError(err) {
				t.Fatalf("expected a device-target refusal, got %v", err)
			}
			opErr, ok := err.(*core.OperationError)
			if !ok || opErr.Operation != "device_target" || opErr.Retryable {
				t.Fatalf("unexpected error shape: %#v", err)
			}
		})
	}

	if *resolutions != 0 {
		t.Fatalf("a refused launcher call resolved tools %d time(s)", *resolutions)
	}
}

// TestLauncherAdmissionLetsAdmittedCallsReachTheService is the opposite direction:
// a matching serial must pass admission and reach the launcher service (which then
// fails on the deliberately missing tool, never on admission).
func TestLauncherAdmissionLetsAdmittedCallsReachTheService(t *testing.T) {
	a, _ := newLauncherTestApp(t, "A")

	if _, err := a.ApplyLauncher(launcher.ApplyRequest{
		OperationID:        "op-admitted1",
		ExpectedSerial:     "A",
		ExpectedComponent:  "com.stock.launcher/.Home",
		CandidateComponent: "com.custom.launcher/.Main",
	}); err == nil || isDeviceTargetError(err) {
		t.Fatalf("an admitted apply must reach the service: %v", err)
	}
	if _, err := a.PreflightLauncher("A"); err == nil || isDeviceTargetError(err) {
		t.Fatalf("an admitted preflight must reach the device reads: %v", err)
	}
}

// TestLauncherCancelIgnoresTheGlobalSelection proves the contract clause: a cancel
// matches the owned operation, never the current device selection, so it must not be
// blocked by a changed selection - and it must still reject another serial.
func TestLauncherCancelIgnoresTheGlobalSelection(t *testing.T) {
	a, resolutions := newLauncherTestApp(t, "B")

	// No operation is owned yet: the cancel must fail as "not owned" rather than as
	// an admission error, even though the active selection is a different serial.
	_, err := a.CancelLauncherOperation(launcher.CancelRequest{OperationID: "op-cancelxyz", ExpectedSerial: "A"})
	if err == nil || isDeviceTargetError(err) {
		t.Fatalf("cancel must be reported as unowned, not as a target refusal: %v", err)
	}
	if !strings.Contains(err.Error(), "No owned launcher operation") {
		t.Fatalf("unexpected cancel error: %v", err)
	}
	if *resolutions != 0 {
		t.Fatal("cancel must not resolve or run anything")
	}
}

// TestLauncherRecoveryIsAvailableOffline covers the offline recovery surface: it
// needs no device, no admission and no command.
func TestLauncherRecoveryIsAvailableOffline(t *testing.T) {
	a, _ := newLauncherTestApp(t, "A")
	writeLauncherRecord(t, a.dataDir, "op-offline001", "applied")
	writeLauncherRecord(t, a.dataDir, "op-pending001", "pending")

	recovery, err := a.ReadLauncherRecovery("A")
	if err != nil {
		t.Fatalf("ReadLauncherRecovery: %v", err)
	}
	if len(recovery.Records) != 2 {
		t.Fatalf("recovery = %+v", recovery.Records)
	}
	byID := map[string]launcher.RecordSummary{}
	for _, entry := range recovery.Records {
		byID[entry.ID] = entry
	}
	applied, ok := byID["op-offline001"]
	if !ok {
		t.Fatalf("missing applied record: %+v", recovery.Records)
	}
	// An applied record is restorable but must not block the next change; a pending
	// record blocks it.
	if !applied.NeedsRecovery || applied.BlocksApply {
		t.Fatalf("applied record flags are not truthful: %+v", applied)
	}
	pending, ok := byID["op-pending001"]
	if !ok || !pending.NeedsRecovery || !pending.BlocksApply {
		t.Fatalf("pending record flags are not truthful: %+v", pending)
	}
	if !strings.Contains(applied.ManualCommand, "'A'") || !strings.Contains(applied.ManualCommand, "com.stock.launcher") {
		t.Fatalf("manual command must be serial-pinned and name the original: %q", applied.ManualCommand)
	}

	// Even a blank serial (all records) stays local and read-only.
	if all, err := a.ReadLauncherRecovery(""); err != nil || len(all.Records) != 2 {
		t.Fatalf("unfiltered recovery = %+v, %v", all, err)
	}
	if other, err := a.ReadLauncherRecovery("OTHER"); err != nil || len(other.Records) != 0 {
		t.Fatalf("filtered recovery = %+v, %v", other, err)
	}
	// The listing is explicitly read-only; the launcher package tests additionally
	// prove that no device command is sent for it.
	if !strings.Contains(recovery.Note, "read-only") {
		t.Fatalf("recovery note = %q", recovery.Note)
	}
}

// TestLauncherFacadeSignatureContract is the compile-time contract for the
// frontend bindings.
func TestLauncherFacadeSignatureContract(t *testing.T) {
	a := &App{}
	var (
		_ func(string) (launcher.Preflight, error)                      = a.PreflightLauncher
		_ func(string, string) (launcher.CandidateTestResult, error)    = a.TestLauncherCandidate
		_ func(launcher.ApplyRequest) (launcher.ApplyResult, error)     = a.ApplyLauncher
		_ func(launcher.RestoreRequest) (launcher.RestoreResult, error) = a.RestoreLauncher
		_ func(launcher.CancelRequest) (launcher.CancelResult, error)   = a.CancelLauncherOperation
		_ func(string) (launcher.Recovery, error)                       = a.ReadLauncherRecovery
	)
	// The existing Apps/Files API and the device service remain untouched.
	var (
		_ func(string, string) ([]packagemgr.Info, error)  = a.ListPackagesForDevice
		_ func(string, string, bool) ([]file.Entry, error) = a.ListFilesForDevice
		_ func() ([]device.Summary, error)                 = a.GetDevices
	)
}

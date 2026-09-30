package tuning

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func journalTestService(t *testing.T, states map[string]PackageState) (*Service, *int) {
	t.Helper()
	mutations := new(int)
	s := NewService(t.TempDir(), func(context.Context) (string, error) { return "A", nil }, func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb"} }, nil)
	s.runCommand = func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if req.Args[1] != "A" {
			t.Fatalf("unexpected target %+v", req)
		}
		args := strings.Join(req.Args, " ")
		pkg := req.Args[len(req.Args)-1]
		current := states[pkg]
		if strings.Contains(args, "dumpsys package") {
			return &core.ExecResult{Stdout: fmt.Sprintf("  User 0: installed=%t hidden=false suspended=false stopped=false enabled=%d instant=false\n", current.Installed, current.Enabled)}, nil
		}
		*mutations++
		if strings.Contains(args, "install-existing") {
			current.Installed = true
		} else if strings.Contains(args, "uninstall") {
			current.Installed = false
		} else {
			for i, command := range []string{"default-state", "enable", "disable", "disable-user", "disable-until-used"} {
				if strings.Contains(args, "pm "+command+" ") {
					current.Enabled = i
					break
				}
			}
		}
		states[pkg] = current
		return &core.ExecResult{}, nil
	}
	return s, mutations
}

func journalTestSnapshot(items ...SnapshotItem) Snapshot {
	return Snapshot{Version: 2, ID: "journal-test", Serial: "A", UserID: 0, Items: items}
}
func journalTestItem(pkg string, state PackageState, action ActionMode) SnapshotItem {
	return SnapshotItem{PackageName: pkg, Before: &state, Action: action, IsSystemApp: true, State: JournalPlanned}
}
func emptyApplyResult() ApplyResult {
	return ApplyResult{Changed: []string{}, Skipped: []string{}, Failed: map[string]string{}}
}

func TestInterruptedResultWriteLeavesRecoverableIntentAndStopsBatch(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 0}
	states := map[string]PackageState{"com.example.one": original, "com.example.two": original}
	s, mutations := journalTestService(t, states)
	snapshot := journalTestSnapshot(journalTestItem("com.example.one", original, ActionDisable), journalTestItem("com.example.two", original, ActionDisable))
	if err := s.saveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	s.writeSnapshot = func(value Snapshot) error {
		if value.Items[0].State == JournalApplied {
			return errors.New("simulated interrupted result write")
		}
		return s.saveSnapshot(value)
	}
	result, err := s.applyJournal(context.Background(), &snapshot, emptyApplyResult())
	if err == nil || len(result.Changed) != 1 || *mutations != 1 {
		t.Fatalf("result=%+v mutations=%d err=%v", result, *mutations, err)
	}
	saved, err := s.loadSnapshot("A", snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Items[0].State != JournalPending || saved.Items[1].State != JournalPlanned {
		t.Fatalf("intent not preserved: %+v", saved)
	}
	// Simulate restart with only the durable journal and observed device state.
	s.writeSnapshot = nil
	restored, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
	if err != nil || len(restored.Restored) != 1 || len(restored.Failed) != 0 || states["com.example.one"] != original {
		t.Fatalf("recovery=%+v err=%v states=%+v", restored, err, states)
	}
	before := *mutations
	again, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
	if err != nil || len(again.Restored) != 0 || *mutations != before {
		t.Fatalf("recovery not idempotent: %+v %v", again, err)
	}
}

func TestIntentWriteFailurePreventsMutation(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 1}
	s, mutations := journalTestService(t, map[string]PackageState{"com.example.app": original})
	snapshot := journalTestSnapshot(journalTestItem("com.example.app", original, ActionDisable))
	s.writeSnapshot = func(Snapshot) error { return errors.New("disk full") }
	if _, err := s.applyJournal(context.Background(), &snapshot, emptyApplyResult()); err == nil {
		t.Fatal("expected journal failure")
	}
	if *mutations != 0 {
		t.Fatalf("mutated without durable intent: %d", *mutations)
	}
}

func TestRestorePreservesAllExactEnabledStates(t *testing.T) {
	for enabled := 0; enabled <= 4; enabled++ {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			original := PackageState{Installed: true, Enabled: enabled}
			states := map[string]PackageState{"com.example.app": {Installed: false, Enabled: enabled}}
			s, _ := journalTestService(t, states)
			item := journalTestItem("com.example.app", original, ActionUninstallUser)
			item.State = JournalPending
			snapshot := journalTestSnapshot(item)
			if err := s.saveSnapshot(snapshot); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
			if err != nil || len(result.Failed) > 0 || states["com.example.app"] != original {
				t.Fatalf("restore=%+v state=%+v err=%v", result, states, err)
			}
		})
	}
}

func TestRestoreRejectsConflictingDeviceState(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 0}
	s, mutations := journalTestService(t, map[string]PackageState{"com.example.app": {Installed: true, Enabled: 4}})
	item := journalTestItem("com.example.app", original, ActionDisable)
	item.State = JournalUnknown
	snapshot := journalTestSnapshot(item)
	if err := s.saveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
	if err != nil || len(result.Failed) != 1 || *mutations != 0 {
		t.Fatalf("conflicting state overwritten: %+v %v mutations=%d", result, err, *mutations)
	}
}

func TestLegacyUnrecordedChangeCanBeRecovered(t *testing.T) {
	states := map[string]PackageState{"com.example.app": {Installed: true, Enabled: 3}}
	s, _ := journalTestService(t, states)
	snapshot := Snapshot{ID: "legacy", Serial: "A", Items: []SnapshotItem{{PackageName: "com.example.app", Action: ActionDisable, WasEnabled: true, Applied: false}}}
	if err := s.saveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
	if err != nil || len(result.Restored) != 1 || states["com.example.app"].Enabled != 1 {
		t.Fatalf("legacy recovery=%+v %v", result, err)
	}
}

func TestParsePackageStateFailsClosedOnUnknownOrAmbiguousDump(t *testing.T) {
	for _, text := range []string{"", "User 10: installed=true enabled=0", "User 0: installed=true enabled=9", "User 0: installed=true enabled=0\nUser 0: installed=true enabled=1"} {
		if _, err := parsePackageState(text); err == nil {
			t.Fatalf("accepted ambiguous state %q", text)
		}
	}
	state, err := parsePackageState("User 0: installed=true hidden=false enabled=0\nUser 10: installed=false enabled=3")
	if err != nil || state != (PackageState{Installed: true, Enabled: 0}) {
		t.Fatalf("parse=%+v %v", state, err)
	}
}

func TestUninstallMayResetEnabledStateBeforeExactRestore(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 2}
	states := map[string]PackageState{"com.example.app": {Installed: false, Enabled: 0}}
	s, _ := journalTestService(t, states)
	item := journalTestItem("com.example.app", original, ActionUninstallUser)
	item.State = JournalUnknown
	snapshot := journalTestSnapshot(item)
	if err := s.saveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
	if err != nil || len(result.Failed) > 0 || states["com.example.app"] != original {
		t.Fatalf("restore=%+v %v state=%+v", result, err, states)
	}
}

func TestOperationCapturesToolPathOnce(t *testing.T) {
	path := "adb-A"
	s, _ := journalTestService(t, map[string]PackageState{})
	s.getBinPath = func() core.BinaryPaths { return core.BinaryPaths{Adb: path} }
	bound := s.bound()
	path = "adb-B"
	bound.runCommand = func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if req.Command != "adb-A" {
			t.Fatalf("operation changed tool: %+v", req)
		}
		return &core.ExecResult{Stdout: "User 0: installed=true enabled=0"}, nil
	}
	for range 2 {
		if _, err := bound.readPackageState(context.Background(), "A", "com.example.app"); err != nil {
			t.Fatal(err)
		}
	}
}

package tuning

import (
	"ADBKit/internal/core"
	"context"
	"strings"
	"testing"
)

func TestJournalCannotDisableUnknownOEMRecoveryPackage(t *testing.T) {
	for _, pkg := range []string{"com.oem.home", "com.oem.keyboard"} {
		original := PackageState{Installed: true, Enabled: 0}
		s, mutations := journalTestService(t, map[string]PackageState{pkg: original})
		snapshot := journalTestSnapshot(journalTestItem(pkg, original, ActionDisable))
		if _, err := s.applyJournal(context.Background(), &snapshot, emptyApplyResult()); err == nil || *mutations != 0 {
			t.Fatalf("dynamic recovery package changed: %s, mutations %d, err %v", pkg, *mutations, err)
		}
	}
}

func TestJournalRechecksRecoveryForEveryItem(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 0}
	states := map[string]PackageState{"com.example.one": original, "com.example.two": original}
	s, mutations := journalTestService(t, states)
	base := s.runCommand
	s.runCommand = func(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if *mutations > 0 && strings.Contains(strings.Join(req.Args, " "), "cmd package") {
			return &core.ExecResult{Stdout: "com.example.two/.Home"}, nil
		}
		return base(ctx, req)
	}
	snapshot := journalTestSnapshot(journalTestItem("com.example.one", original, ActionDisable), journalTestItem("com.example.two", original, ActionDisable))
	result, err := s.applyJournal(context.Background(), &snapshot, emptyApplyResult())
	if err == nil || *mutations != 1 || len(result.Changed) != 1 || states["com.example.two"] != original {
		t.Fatalf("new HOME was disabled: result=%+v mutations=%d err=%v", result, *mutations, err)
	}
}

func TestUnknownRecoveryCapabilityPreventsMutation(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 0}
	s, mutations := journalTestService(t, map[string]PackageState{"com.example.app": original})
	base := s.runCommand
	s.runCommand = func(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if strings.Contains(strings.Join(req.Args, " "), "cmd package") {
			return &core.ExecResult{Stdout: "Unknown command"}, nil
		}
		return base(ctx, req)
	}
	snapshot := journalTestSnapshot(journalTestItem("com.example.app", original, ActionDisable))
	if _, err := s.applyJournal(context.Background(), &snapshot, emptyApplyResult()); err == nil || *mutations != 0 {
		t.Fatalf("unsupported exit-0 capability authorized mutation: %d %v", *mutations, err)
	}
}

func TestOldSnapshotCannotDisableCurrentOEMRecoveryPackage(t *testing.T) {
	for _, pkg := range []string{"com.oem.home", "com.oem.keyboard"} {
		original := PackageState{Installed: true, Enabled: 3}
		item := journalTestItem(pkg, original, ActionUninstallUser)
		item.State = JournalRestorePending
		s, mutations := journalTestService(t, map[string]PackageState{pkg: {Installed: true, Enabled: 0}})
		snapshot := journalTestSnapshot(item)
		if err := s.saveSnapshot(snapshot); err != nil {
			t.Fatal(err)
		}
		result, err := s.RestoreForSerial(context.Background(), "A", snapshot.ID)
		if err != nil || len(result.Failed) != 1 || *mutations != 0 || !strings.Contains(result.Failed[pkg], "HOME/IME") {
			t.Fatalf("snapshot bypassed live recovery floor: %+v %d %v", result, *mutations, err)
		}
	}
}

func TestEnablingRecoveryWorksWhenHomeCapabilityIsBroken(t *testing.T) {
	original := PackageState{Installed: true, Enabled: 0}
	item := journalTestItem("com.oem.home", original, ActionDisable)
	item.State = JournalUnknown
	states := map[string]PackageState{"com.oem.home": {Installed: true, Enabled: 3}}
	s, mutations := journalTestService(t, states)
	base := s.runCommand
	s.runCommand = func(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if strings.Contains(strings.Join(req.Args, " "), "cmd package") {
			t.Fatal("enabling recovery must not require a working HOME")
		}
		return base(ctx, req)
	}
	snapshot := journalTestSnapshot(item)
	result, err := s.restoreJournal(context.Background(), &snapshot)
	if err != nil || len(result.Restored) != 1 || *mutations != 1 || states["com.oem.home"] != original {
		t.Fatalf("recovery blocked: %+v %d %v", result, *mutations, err)
	}
}

package tuning

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestApplyRequiresConfirmedTargetBeforeQueries(t *testing.T) {
	svc := &Service{}
	for _, expected := range []string{"", "other-device"} {
		if _, err := svc.Apply(context.Background(), device.Info{Serial: "A"}, ApplyRequest{ExpectedSerial: expected, Mode: ActionDisable}); err == nil {
			t.Fatalf("expected target %q to be rejected", expected)
		}
	}
}

func TestRestoreDoesNotFollowSelectionChanges(t *testing.T) {
	selected := "A"
	svc := NewService(t.TempDir(), func(context.Context) (string, error) { return selected, nil }, func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb"} }, nil)
	snapshot := Snapshot{ID: "test", Serial: "A", Items: []SnapshotItem{
		{PackageName: "com.example.one", Action: ActionDisable, WasEnabled: true, Applied: true},
		{PackageName: "com.example.two", Action: ActionUninstallUser, WasEnabled: false, Applied: true},
	}}
	if err := svc.saveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	calls := 0
	states := map[string]PackageState{"com.example.one": {Installed: true, Enabled: 3}, "com.example.two": {Installed: true, Enabled: 3}}
	svc.runCommand = func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		selected = "B"
		calls++
		if req.Args[1] != "A" {
			t.Fatalf("restore switched target: %+v", req)
		}
		pkg := req.Args[len(req.Args)-1]
		args := strings.Join(req.Args, " ")
		if strings.Contains(args, "cmd package query-activities") || strings.Contains(args, "cmd package resolve-activity") {
			return &core.ExecResult{Stdout: "com.oem.home/.Home\n"}, nil
		}
		if strings.Contains(args, "settings --user 0 get secure") {
			return &core.ExecResult{Stdout: "com.oem.keyboard/.IME\n"}, nil
		}
		if strings.Contains(strings.Join(req.Args, " "), "dumpsys package") {
			state := states[pkg]
			return &core.ExecResult{Stdout: fmt.Sprintf("User 0: installed=%t enabled=%d", state.Installed, state.Enabled)}, nil
		}
		if strings.Contains(strings.Join(req.Args, " "), "pm enable") {
			states[pkg] = PackageState{Installed: true, Enabled: 1}
		}
		return &core.ExecResult{}, nil
	}
	result, err := svc.Restore(context.Background(), "test")
	if err != nil || len(result.Restored) != 2 || calls < 3 {
		t.Fatalf("restore=%+v calls=%d err=%v", result, calls, err)
	}
}

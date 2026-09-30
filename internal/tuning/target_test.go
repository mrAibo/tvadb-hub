package tuning

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
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
	svc.runCommand = func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		selected = "B"
		calls++
		if req.Args[1] != "A" {
			t.Fatalf("restore switched target: %+v", req)
		}
		return &core.ExecResult{}, nil
	}
	result, err := svc.Restore(context.Background(), "test")
	if err != nil || len(result.Restored) != 2 || calls != 3 {
		t.Fatalf("restore=%+v calls=%d err=%v", result, calls, err)
	}
}

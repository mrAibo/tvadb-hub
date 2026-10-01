package file

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fakeADBLogEnv = "DROIDSPHERE_FAKE_ADB_LOG"

// TestMain lets this test binary act as a fake adb executable: it appends its own
// path and argv to the log named by the environment and then fails, so both the
// caller's error and the log carry the exact command that was attempted. A normal
// test run never sets the variable, so the helper stays a no-op.
func TestMain(m *testing.M) {
	if logPath := os.Getenv(fakeADBLogEnv); logPath != "" {
		record := fakeADBRecord(os.Args[1:])
		if logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, _ = fmt.Fprintln(logFile, record)
			_ = logFile.Close()
		}
		fmt.Fprintln(os.Stderr, record)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func fakeADBRecord(args []string) string {
	executable, _ := os.Executable()
	return executable + " " + strings.Join(args, " ")
}

// writeFakeADB copies this test binary to an executable file so it can stand in
// for adb without a device, a compiler or a shell wrapper.
// writeFakeADB copies this test binary to an executable file so it can stand in
// for adb without a device, a compiler or a shell wrapper. Windows needs an
// executable extension, so the platform suffix is added there.
func writeFakeADB(t *testing.T, base string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	name := base
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func readFakeADBLog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	records := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			records = append(records, trimmed)
		}
	}
	return records
}

// newRootService builds a root service whose resolver records any global lookup,
// so a bound view that escapes its pinned target is visible in the test.
func newRootService(t *testing.T, pinnedPath string) (*Service, *bool, *core.BinaryPaths) {
	t.Helper()
	globalConsulted := false
	paths := &core.BinaryPaths{Adb: pinnedPath}
	root := &Service{
		resolveActiveSerial: func(context.Context) (string, error) {
			globalConsulted = true
			return "B", nil
		},
		getBinPath: func() core.BinaryPaths { return *paths },
	}
	return root, &globalConsulted, paths
}

func TestForTargetPinsDeviceAndToolsAcrossMutationBatch(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adb.log")
	t.Setenv(fakeADBLogEnv, logPath)
	pinned := writeFakeADB(t, "adb-pinned")
	switched := writeFakeADB(t, "adb-switched")

	root, globalConsulted, paths := newRootService(t, pinned)
	bound := root.ForTarget("A")
	*paths = core.BinaryPaths{Adb: switched} // tools changed after admission

	if _, err := bound.DeleteMultipleFiles(context.Background(), []string{"/sdcard/one", "/sdcard/two"}); err != nil {
		t.Fatalf("DeleteMultipleFiles: %v", err)
	}

	records := readFakeADBLog(t, logPath)
	if len(records) != 2 {
		t.Fatalf("expected one command per file, got %d: %v", len(records), records)
	}
	for _, record := range records {
		if !strings.HasPrefix(record, pinned+" ") {
			t.Fatalf("tool paths were not pinned: %s", record)
		}
		if !strings.Contains(record, "-s A ") {
			t.Fatalf("mutation moved to another device: %s", record)
		}
		if !strings.Contains(record, "rm -rf") {
			t.Fatalf("command vector changed: %s", record)
		}
	}
	if *globalConsulted {
		t.Fatal("the bound view consulted the mutable global device selection")
	}
}

func TestForTargetPinsQueriesAndStorage(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adb.log")
	t.Setenv(fakeADBLogEnv, logPath)
	pinned := writeFakeADB(t, "adb-pinned")

	root, globalConsulted, _ := newRootService(t, pinned)
	bound := root.ForTarget("A")

	_, _ = bound.ListFiles(context.Background(), "/sdcard/", false)
	_, _ = bound.GetDirectorySize(context.Background(), "/sdcard/")
	_, _ = bound.GetStorageInfo(context.Background())
	_, _ = bound.ListSdCards(context.Background())

	records := readFakeADBLog(t, logPath)
	if len(records) != 4 {
		t.Fatalf("expected one command per query, got %d: %v", len(records), records)
	}
	joined := strings.Join(records, "\n")
	for _, record := range records {
		if !strings.HasPrefix(record, pinned+" ") {
			t.Fatalf("tool paths were not pinned: %s", record)
		}
		if !strings.Contains(record, "-s A ") {
			t.Fatalf("query moved to another device: %s", record)
		}
	}
	for _, want := range []string{"ls -lAL", "du -sh", "df -k /sdcard", "sm list-volumes"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("command vector %q is missing:\n%s", want, joined)
		}
	}
	if *globalConsulted {
		t.Fatal("the bound view consulted the mutable global device selection")
	}
}

func TestForTargetRefusesBlankSerialBeforeCommand(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adb.log")
	t.Setenv(fakeADBLogEnv, logPath)
	pinned := writeFakeADB(t, "adb-pinned")

	root, _, _ := newRootService(t, pinned)
	bound := root.ForTarget("   ")

	if _, err := bound.ListFiles(context.Background(), "/sdcard/", false); err == nil || !strings.Contains(err.Error(), "Confirmed device is required") {
		t.Fatalf("blank target was not refused for a query: %v", err)
	}
	if _, err := bound.DeleteFile(context.Background(), "/sdcard/one"); err == nil || !strings.Contains(err.Error(), "Confirmed device is required") {
		t.Fatalf("blank target was not refused for a mutation: %v", err)
	}
	if records := readFakeADBLog(t, logPath); len(records) != 0 {
		t.Fatalf("a blank target issued commands: %v", records)
	}
}

// TestForTargetLeavesTransferOwnershipOnRootService pins the corrected R5 shape:
// the bound view is a distinct service that inherits no transfer or cancellation
// state, and the root keeps owning its in-flight operation.
func TestForTargetLeavesTransferOwnershipOnRootService(t *testing.T) {
	root := &Service{
		resolveActiveSerial: func(context.Context) (string, error) { return "A", nil },
		getBinPath:          func() core.BinaryPaths { return core.BinaryPaths{Adb: filepath.Join(t.TempDir(), "missing-adb")} },
		compressionCache:    map[string]adbCompressionCapabilities{},
	}
	cancelled := false
	root.mu.Lock()
	root.activeOperationID = "op-owned"
	root.nextOperationID = 7
	root.cancelFunc = func() { cancelled = true }
	root.mu.Unlock()

	bound := root.ForTarget("A")
	if bound.svc == root {
		t.Fatal("the bound view must not be the root service")
	}
	if bound.svc.cancelFunc != nil || bound.svc.activeOperationID != "" || bound.svc.nextOperationID != 0 {
		t.Fatal("the bound view inherited transfer or cancellation state")
	}

	if _, err := bound.ListFiles(context.Background(), "/sdcard/", false); err == nil {
		t.Fatal("expected the missing adb path to fail the query")
	}

	root.mu.Lock()
	activeID, nextID, cancelFunc := root.activeOperationID, root.nextOperationID, root.cancelFunc
	root.mu.Unlock()
	if activeID != "op-owned" || nextID != 7 || cancelFunc == nil || cancelled {
		t.Fatalf("bound operations disturbed the root transfer state: %q %d %v %v", activeID, nextID, cancelFunc != nil, cancelled)
	}

	root.CancelTransferFor("op-owned")
	if !cancelled {
		t.Fatal("root cancellation no longer reaches the owned operation")
	}
}

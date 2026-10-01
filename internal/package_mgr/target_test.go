package packagemgr

import (
	"ADBKit/internal/core"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestListPackagesPinsDeviceAcrossConcurrentQueries(t *testing.T) {
	calls := 0
	var mu sync.Mutex
	var requests []core.ExecRequest
	svc := &Service{
		resolveActiveSerial: func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "A", nil
			}
			return "B", nil
		},
		getBinPath: func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb-A"} },
		runCommand: func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()
			return &core.ExecResult{Stdout: "package:com.example.app\n"}, nil
		},
	}
	if _, err := svc.ListPackages(context.Background(), "all"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(requests) != 4 {
		t.Fatalf("resolver calls=%d requests=%d", calls, len(requests))
	}
	for _, req := range requests {
		if req.Args[1] != "A" {
			t.Fatalf("query moved to another device: %+v", req)
		}
	}
}

func TestForTargetPinsUserAndBinary(t *testing.T) {
	path := "adb-A"
	svc := &Service{
		getBinPath: func() core.BinaryPaths { return core.BinaryPaths{Adb: path} },
		runCommand: func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			if req.Command != "adb-A" || req.Args[1] != "A" || !strings.Contains(strings.Join(req.Args, " "), "--user 0") {
				t.Errorf("unbound query: %+v", req)
			}
			return &core.ExecResult{}, nil
		},
	}
	bound := svc.ForTarget("A", 0)
	path = "adb-B"
	if _, err := bound.ListPackages(context.Background(), "all"); err != nil {
		t.Fatal(err)
	}
}

// TestForTargetKeepUserPinsQueriesWithoutUserScope is the Apps twin of the tuning
// pin: the device and the tool paths are captured for the whole query loop, and no
// Android user scope is added because Apps never requested one.
func TestForTargetKeepUserPinsQueriesWithoutUserScope(t *testing.T) {
	path := "adb-A"
	var mu sync.Mutex
	var requests []core.ExecRequest
	svc := &Service{
		resolveActiveSerial: func(context.Context) (string, error) { return "B", nil },
		getBinPath:          func() core.BinaryPaths { return core.BinaryPaths{Adb: path} },
		runCommand: func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()
			return &core.ExecResult{Stdout: "package:com.example.app\n"}, nil
		},
	}

	bound := svc.ForTargetKeepUser("A")
	if bound.userID != nil {
		t.Fatalf("Apps target must keep the default nil user, got %v", *bound.userID)
	}
	path = "adb-B" // the root configuration changed after admission

	if _, err := bound.ListPackages(context.Background(), "all"); err != nil {
		t.Fatal(err)
	}
	if len(requests) == 0 {
		t.Fatal("no commands were issued")
	}
	for _, req := range requests {
		joined := strings.Join(req.Args, " ")
		if req.Command != "adb-A" {
			t.Fatalf("tool paths were not pinned: %+v", req)
		}
		if !strings.Contains(joined, "-s A") {
			t.Fatalf("query moved to another device: %+v", req)
		}
		if strings.Contains(joined, "--user") {
			t.Fatalf("Apps query gained a user scope: %+v", req)
		}
	}

	requests = nil
	userScoped := svc.ForTarget("A", 0)
	if userScoped.userID == nil || *userScoped.userID != 0 {
		t.Fatal("ForTarget(serial, 0) must keep forcing user 0")
	}
	if _, err := userScoped.ListPackages(context.Background(), "all"); err != nil {
		t.Fatal(err)
	}
	if len(requests) == 0 {
		t.Fatal("no commands were issued for the user-scoped target")
	}
	for _, req := range requests {
		if !strings.Contains(strings.Join(req.Args, " "), "--user 0") {
			t.Fatalf("user-scoped query lost --user 0: %+v", req)
		}
	}
}

func TestForTargetKeepUserRefusesBlankSerialBeforeCommand(t *testing.T) {
	commands := 0
	svc := &Service{
		resolveActiveSerial: func(context.Context) (string, error) { return "A", nil },
		getBinPath:          func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb-A"} },
		runCommand: func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
			commands++
			return &core.ExecResult{}, nil
		},
	}

	bound := svc.ForTargetKeepUser("   ")
	if _, err := bound.ListPackages(context.Background(), "all"); err == nil || !strings.Contains(err.Error(), "Confirmed device is required") {
		t.Fatalf("blank target was not refused: %v", err)
	}
	if commands != 0 {
		t.Fatalf("a blank target issued %d command(s)", commands)
	}
}

// TestForTargetKeepUserPinsWholeBatch proves a batch keeps the captured device and
// tools: every package runs through the pinned service even after the root paths
// changed, and the mutable global selection is never consulted.
func TestForTargetKeepUserPinsWholeBatch(t *testing.T) {
	pinned := filepath.Join(t.TempDir(), "adb-pinned")
	switched := filepath.Join(t.TempDir(), "adb-switched")
	paths := core.BinaryPaths{Adb: pinned}
	globalConsulted := false
	svc := &Service{
		resolveActiveSerial: func(context.Context) (string, error) {
			globalConsulted = true
			return "B", nil
		},
		getBinPath: func() core.BinaryPaths { return paths },
	}

	bound := svc.ForTargetKeepUser("A")
	paths = core.BinaryPaths{Adb: switched}

	message, err := bound.UninstallMultiplePackages(context.Background(), []string{"com.example.one", "com.example.two"})
	if err != nil {
		t.Fatalf("batch returned an unexpected error: %v", err)
	}
	if globalConsulted {
		t.Fatal("the bound batch consulted the mutable global device selection")
	}
	// The exec error quotes the path (strconv.Quote escapes the separators), so the
	// distinct base names are what identifies the tool that ran.
	if strings.Contains(message, filepath.Base(switched)) {
		t.Fatalf("the batch escaped its pinned tools: %s", message)
	}
	if got := strings.Count(message, filepath.Base(pinned)); got != 2 {
		t.Fatalf("expected both packages to run with the pinned tool, saw %d in %s", got, message)
	}
}

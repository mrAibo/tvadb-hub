package packagemgr

import (
	"ADBKit/internal/core"
	"context"
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

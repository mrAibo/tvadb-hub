package shell

import (
	"ADBKit/internal/binary"
	"ADBKit/internal/core"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const fakeADBEnv = "DROIDSPHERE_FAKE_ADB"

// TestMain lets this test binary act as a fake adb executable when the
// environment asks for it. binary.Service resolves the configured adb path by
// running it with "version", so a copy of this test binary can answer that call
// deterministically, without a shipped fixture and without a device.
func TestMain(m *testing.M) {
	if os.Getenv(fakeADBEnv) == "1" {
		fmt.Println("Android Debug Bridge version 1.0.41")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type promptContextKey struct{}

// TestPromptCodenameUsesCallerContextAndFiniteTimeout is the regression guard for
// the prompt probe that used to run with context.Background() and no timeout.
func TestPromptCodenameUsesCallerContextAndFiniteTimeout(t *testing.T) {
	type probeCall struct {
		ctx context.Context
		req core.ExecRequest
	}
	var got probeCall

	ctx := context.WithValue(context.Background(), promptContextKey{}, "caller")
	codename, err := promptCodename(ctx, "adb-path", "SERIAL-1", func(runCtx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		got.ctx, got.req = runCtx, req
		return &core.ExecResult{Stdout: "shibuya\n"}, nil
	})
	if err != nil {
		t.Fatalf("promptCodename: %v", err)
	}
	if codename != "shibuya" {
		t.Fatalf("codename = %q, want shibuya", codename)
	}
	if got.ctx != ctx || got.ctx.Value(promptContextKey{}) != "caller" {
		t.Fatal("the caller context was not propagated to the probe")
	}
	if got.req.Timeout != promptProbeTimeout || got.req.Timeout <= 0 {
		t.Fatalf("probe timeout = %v, want the finite %v", got.req.Timeout, promptProbeTimeout)
	}
	if got.req.Command != "adb-path" {
		t.Fatalf("probe command = %q, want the resolved adb path", got.req.Command)
	}
	wantArgs := []string{"-s", "SERIAL-1", "shell", "getprop", "ro.product.device"}
	if !reflect.DeepEqual(got.req.Args, wantArgs) {
		t.Fatalf("probe args = %#v, want %#v", got.req.Args, wantArgs)
	}
}

func TestPromptCodenameHonoursCancellationAndErrors(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := promptCodename(cancelled, "adb", "SERIAL-1", func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		// A probe that reports success anyway must not hide caller cancellation.
		return &core.ExecResult{Stdout: "shibuya"}, nil
	}); err == nil {
		t.Fatal("caller cancellation was ignored")
	}

	runErr := errors.New("adb failed")
	cases := []struct {
		name string
		run  func(context.Context, core.ExecRequest) (*core.ExecResult, error)
	}{
		{
			name: "runner error",
			run:  func(context.Context, core.ExecRequest) (*core.ExecResult, error) { return nil, runErr },
		},
		{
			name: "nil result",
			run:  func(context.Context, core.ExecRequest) (*core.ExecResult, error) { return nil, nil },
		},
		{
			name: "blank output",
			run: func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
				return &core.ExecResult{Stdout: " \n"}, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := promptCodename(context.Background(), "adb", "SERIAL-1", tc.run); err == nil {
				t.Fatal("expected the probe to report an error")
			}
		})
	}
}

// TestResolveCodenameUsesInjectedProbeAndFallsBack covers the wiring: the probe
// receives the resolved adb path, and cancellation or probe errors still fall back
// to the serial so the terminal prompt is never blocked.
func TestResolveCodenameUsesInjectedProbeAndFallsBack(t *testing.T) {
	fakeADB := writeFakeADBBinary(t)
	config := &core.AppConfig{AdbPath: fakeADB}

	var captured core.ExecRequest
	svc := &TerminalService{
		binaryService: binary.NewService(t.TempDir()),
		getConfig:     func() *core.AppConfig { return config },
		runProbe: func(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
			captured = req
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &core.ExecResult{Stdout: "shibuya\n"}, nil
		},
	}

	if got := svc.resolveCodename(context.Background(), "SERIAL-1"); got != "shibuya" {
		t.Fatalf("codename = %q, want shibuya", got)
	}
	if captured.Command != fakeADB {
		t.Fatalf("probe command = %q, want the resolved adb %q", captured.Command, fakeADB)
	}
	if captured.Timeout != promptProbeTimeout {
		t.Fatalf("probe timeout = %v, want %v", captured.Timeout, promptProbeTimeout)
	}
	if got := svc.buildPrompt(context.Background(), "SERIAL-1", ModeShell); got != "shibuya:/ $ " {
		t.Fatalf("shell prompt = %q", got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := svc.resolveCodename(cancelled, "SERIAL-1"); got != "SERIAL-1" {
		t.Fatalf("cancelled probe did not fall back to the serial: %q", got)
	}
	if got := svc.resolveCodename(context.Background(), ""); got != "device" {
		t.Fatalf("empty serial = %q, want device", got)
	}
	if got := svc.buildPrompt(context.Background(), "SERIAL-1", ModeADBHost); got != "$ " {
		t.Fatalf("adb host prompt = %q", got)
	}
	if got := svc.buildPrompt(context.Background(), "SERIAL-1", ModeFastboot); got != "fastboot> " {
		t.Fatalf("fastboot prompt = %q", got)
	}
}

// writeFakeADBBinary copies this test binary to a file named like the platform adb
// executable and switches on the helper mode through the environment, so binary
// discovery resolves it without a device, a compiler or a shell wrapper.
func writeFakeADBBinary(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), core.BinaryExecutableName(core.BinaryNameAdb))
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeADBEnv, "1")
	return target
}

package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/flasher"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const fakeFastbootEnv = "DROIDSPHERE_FAKE_FASTBOOT"

// TestMain lets this test binary act as a deterministic fastboot stand-in when the
// environment asks for it: binary.Service resolves the configured fastboot path by
// running it (version probe), so a copy of this test binary can answer that probe
// and record the real flash argv without a shipped fixture and without a device.
func TestMain(m *testing.M) {
	if os.Getenv(fakeFastbootEnv) == "1" {
		os.Exit(runFakeFastboot())
	}
	os.Exit(m.Run())
}

func runFakeFastboot() int {
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "--version" || arg == "version" {
			fmt.Println("fastboot version 35.0.2-android-tools")
			return 0
		}
	}
	if self, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(self), "argv.log")
		if file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(file, strings.Join(args, " "))
			file.Close()
		}
	}
	fmt.Println("Sending 'boot' (1 KB) OKAY [ 0.100s]")
	return 0
}

// installFakeFastboot copies the test binary under the canonical fastboot name into
// a private directory, so binary.Service resolves it as a ready tool. Each copy logs
// its own argv into its own directory, which makes tool identity observable.
func installFakeFastboot(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, core.BinaryExecutableName(core.BinaryNameFastboot))
	if err := os.WriteFile(exe, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

// toolArgv returns the flash argv recorded by one copied tool.
func toolArgv(t *testing.T, exe string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "argv.log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func writeFlashImage(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// toolScript answers the configuration lookup with a different fastboot path after
// the first resolution and counts how often the configuration was consulted.
type toolScript struct {
	mu    sync.Mutex
	calls int
	paths []string
}

func (s *toolScript) getConfig() *core.AppConfig {
	s.mu.Lock()
	s.calls++
	index := len(s.paths) - 1
	if s.calls-1 < len(s.paths) {
		index = s.calls - 1
	}
	path := s.paths[index]
	s.mu.Unlock()
	return &core.AppConfig{FastbootPath: path}
}

func (s *toolScript) resolutions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// newConfirmedFlashTestApp wires the flasher services the same way app.go does, with
// a deliberately different ADB selection so the confirmed path cannot silently reuse it.
func newConfirmedFlashTestApp(t *testing.T, script *toolScript) *App {
	t.Helper()
	a, _ := newTestApp(t)
	a.ctx = context.Background()
	a.activeSerial = "A"
	a.fbSvc = flasher.NewFastbootService(a.binSvc, script.getConfig, nil)
	a.fpSvc = flasher.NewPlanService(a.fbSvc)
	return a
}

// TestConfirmedFlashRejectsBlankSerialBeforeAnyToolWork covers the fail-closed
// boundary: a blank (or whitespace-only) confirmed serial must be refused before the
// configuration is consulted, before the fastboot tool is resolved and before any
// command is built.
func TestConfirmedFlashRejectsBlankSerialBeforeAnyToolWork(t *testing.T) {
	t.Setenv(fakeFastbootEnv, "1")
	tool := installFakeFastboot(t, "tool-a")
	script := &toolScript{paths: []string{tool}}
	a := newConfirmedFlashTestApp(t, script)

	folder := t.TempDir()
	image := writeFlashImage(t, folder, "boot.img")
	plan := flasher.Plan{Steps: []flasher.Step{{Partition: "boot", ImageFile: image}}}

	cases := []struct {
		name string
		call func() error
	}{
		{"partition empty", func() error { _, err := a.FlashPartitionForDevice("", "boot", image); return err }},
		{"partition whitespace", func() error { _, err := a.FlashPartitionForDevice("   ", "boot", image); return err }},
		{"batch empty", func() error { _, err := a.FlashRomFolderForDevice("", folder, plan); return err }},
		{"batch whitespace", func() error { _, err := a.FlashRomFolderForDevice("\t ", folder, plan); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("a blank confirmed serial must be refused")
			}
			opErr, ok := err.(*core.OperationError)
			if !ok {
				t.Fatalf("expected a structured error, got %T", err)
			}
			if opErr.Retryable {
				t.Fatalf("the refusal must not be retryable: %#v", err)
			}
			if !strings.Contains(err.Error(), "Confirmed fastboot device is required") {
				t.Fatalf("unexpected refusal: %v", err)
			}
		})
	}
	if got := script.resolutions(); got != 0 {
		t.Fatalf("a blank serial resolved the tool %d time(s)", got)
	}
	if argv := toolArgv(t, tool); len(argv) != 0 {
		t.Fatalf("a blank serial ran a command: %v", argv)
	}
}

// TestConfirmedFlashUsesConfirmedSerialNotADBSelection proves both directions: the
// new RPC flattens the explicit serial (never the ADB selection, even when that
// selection is empty), while the legacy RPC keeps its documented fallback and
// re-resolves the tool.
func TestConfirmedFlashUsesConfirmedSerialNotADBSelection(t *testing.T) {
	t.Setenv(fakeFastbootEnv, "1")
	toolA := installFakeFastboot(t, "tool-a")
	toolB := installFakeFastboot(t, "tool-b")
	script := &toolScript{paths: []string{toolA, toolB}}
	a := newConfirmedFlashTestApp(t, script)
	image := writeFlashImage(t, t.TempDir(), "boot.img")

	if _, err := a.FlashPartitionForDevice(" F1 ", "BOOT", image); err != nil {
		t.Fatalf("confirmed flash: %v", err)
	}
	confirmed := toolArgv(t, toolA)
	if len(confirmed) != 1 {
		t.Fatalf("want exactly one captured-tool command, got %v", confirmed)
	}
	if !strings.Contains(confirmed[0], "-s F1 flash boot "+image) {
		t.Fatalf("confirmed flash argv = %q", confirmed[0])
	}
	if strings.Contains(confirmed[0], "-s A ") {
		t.Fatalf("the confirmed flash used the ADB selection: %q", confirmed[0])
	}
	if got := script.resolutions(); got != 1 {
		t.Fatalf("the confirmed flash resolved the tool %d time(s), want one capture", got)
	}

	// The ADB selection is irrelevant: with no selection at all the confirmed path
	// still uses the caller-confirmed serial. A new operation captures the tool again
	// (the configuration now hands out tool-b), so that line lands in tool-b's log.
	a.activeSerial = ""
	if _, err := a.FlashPartitionForDevice("F2", "boot", image); err != nil {
		t.Fatalf("confirmed flash without an ADB selection: %v", err)
	}
	if got := toolArgv(t, toolA); len(got) != 1 {
		t.Fatalf("the second operation must not reuse another operation's capture: %v", got)
	}
	second := toolArgv(t, toolB)
	if len(second) != 1 || !strings.Contains(second[0], "-s F2 flash boot ") {
		t.Fatalf("confirmed flash argv = %v", second)
	}

	// Legacy RPC: unchanged, still defaults to the ADB selection and re-resolves.
	a.activeSerial = "A"
	if _, err := a.FlashPartition("", "boot", image); err != nil {
		t.Fatalf("legacy flash: %v", err)
	}
	legacy := toolArgv(t, toolB)
	if len(legacy) != 2 || !strings.Contains(legacy[1], "-s A flash boot ") {
		t.Fatalf("legacy flash argv = %v (want the ADB selection on the re-resolved tool)", legacy)
	}
	if got := toolArgv(t, toolA); len(got) != 1 {
		t.Fatalf("the legacy call must not be served by the first captured tool: %v", got)
	}
}

// TestConfirmedBatchPinsSerialAndToolAcrossSteps is the batch proof: every step of a
// two-step flash carries the confirmed serial and the same captured tool, even
// though the configuration would hand out a different fastboot path from the second
// resolution onwards.
func TestConfirmedBatchPinsSerialAndToolAcrossSteps(t *testing.T) {
	t.Setenv(fakeFastbootEnv, "1")
	toolA := installFakeFastboot(t, "tool-a")
	toolB := installFakeFastboot(t, "tool-b")
	script := &toolScript{paths: []string{toolA, toolB}}
	a := newConfirmedFlashTestApp(t, script)

	folder := t.TempDir()
	writeFlashImage(t, folder, "boot.img")
	writeFlashImage(t, folder, "vbmeta.img")
	plan, err := a.fpSvc.ScanRomFolder(folder)
	if err != nil {
		t.Fatalf("ScanRomFolder: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("plan = %+v, want two steps", plan.Steps)
	}

	if _, err := a.FlashRomFolderForDevice(" F1 ", folder, *plan); err != nil {
		t.Fatalf("confirmed batch: %v", err)
	}

	argv := toolArgv(t, toolA)
	if len(argv) != 2 {
		t.Fatalf("want two commands from the captured tool, got %v", argv)
	}
	for _, command := range argv {
		if !strings.HasPrefix(command, "-s F1 flash ") {
			t.Fatalf("step is not pinned to the confirmed serial: %q", command)
		}
	}
	if !strings.Contains(argv[0], "flash boot ") || !strings.Contains(argv[1], "flash vbmeta ") {
		t.Fatalf("unexpected step order or partitions: %v", argv)
	}
	if switched := toolArgv(t, toolB); len(switched) != 0 {
		t.Fatalf("the batch switched tools mid-operation: %v", switched)
	}
	if got := script.resolutions(); got != 1 {
		t.Fatalf("the batch resolved the tool %d time(s), want exactly one capture", got)
	}
}

// TestConfirmedBatchPreservesPlanValidation keeps the existing validation semantics:
// an image outside the selected folder, a missing image and an empty plan are all
// refused before any flash command runs.
func TestConfirmedBatchPreservesPlanValidation(t *testing.T) {
	t.Setenv(fakeFastbootEnv, "1")
	tool := installFakeFastboot(t, "tool-a")
	script := &toolScript{paths: []string{tool}}
	a := newConfirmedFlashTestApp(t, script)

	folder := t.TempDir()
	writeFlashImage(t, folder, "boot.img")
	outside := writeFlashImage(t, t.TempDir(), "boot.img")

	cases := []struct {
		name string
		plan flasher.Plan
		want string
	}{
		{
			name: "image outside the selected folder",
			plan: flasher.Plan{Steps: []flasher.Step{{Partition: "boot", ImageFile: outside}}},
			want: "outside the selected folder",
		},
		{
			name: "missing image",
			plan: flasher.Plan{Steps: []flasher.Step{{Partition: "boot", ImageFile: filepath.Join(folder, "missing.img")}}},
			want: "missing image",
		},
		{
			name: "empty plan",
			plan: flasher.Plan{},
			want: "flash plan is empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := script.resolutions()
			_, err := a.FlashRomFolderForDevice("F1", folder, tc.plan)
			if err == nil {
				t.Fatal("an invalid plan must be refused")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if script.resolutions() != before {
				t.Fatal("an invalid plan must be refused before the tool is resolved")
			}
		})
	}
	if argv := toolArgv(t, tool); len(argv) != 0 {
		t.Fatalf("an invalid plan ran a command: %v", argv)
	}
}

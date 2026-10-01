package launcher

import (
	"ADBKit/internal/core"
	"context"
	"strings"
	"testing"
)

func TestProbeCapabilityReportsSupportedUnsupportedAndUnknown(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func(*fakeDevice)
		status   CapabilityStatus
		setHome  bool
		resolve  bool
		contains string
	}{
		{
			name:    "supported",
			status:  CapabilitySupported,
			setHome: true, resolve: true,
			contains: "supported HOME commands",
		},
		{
			name: "unsupported",
			prepare: func(d *fakeDevice) {
				d.capability = "Package manager (package) commands:\n  query-activities INTENT\n"
			},
			status:   CapabilityUnsupported,
			contains: "does not list",
		},
		{
			name: "unknown",
			prepare: func(d *fakeDevice) {
				d.capabilityErr = true
			},
			status:   CapabilityUnknown,
			contains: "probe failed",
		},
		{
			name: "diagnostic output stays unknown",
			prepare: func(d *fakeDevice) {
				d.capabilityExit = 2
			},
			status:   CapabilityUnknown,
			contains: "probe failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			device := newFakeDevice()
			if tc.prepare != nil {
				tc.prepare(device)
			}
			service := newTestService(t, device)
			capability := service.bind(testSerial).probeCapability(context.Background())
			if capability.Status != tc.status {
				t.Fatalf("status = %s, want %s (%+v)", capability.Status, tc.status, capability)
			}
			if capability.SetHomeActivity != tc.setHome || capability.ResolveActivity != tc.resolve {
				t.Fatalf("token detection = %+v", capability)
			}
			if !strings.Contains(capability.Detail, tc.contains) {
				t.Fatalf("detail = %q, want %q", capability.Detail, tc.contains)
			}
			for _, req := range device.recorded() {
				if req.Args[0] != "-s" || req.Args[1] != testSerial {
					t.Fatalf("capability probe was not pinned: %+v", req)
				}
				if -req.Timeout >= 0 {
					t.Fatalf("capability probe without a positive timeout: %+v", req)
				}
			}
			if device.sawCommand("set-home-activity") {
				t.Fatal("the capability probe must never write")
			}
		})
	}
}

// TestCapabilityWordingNamesPackageRoleSelection pins the updated wording: the probe
// can only prove that the commands exist, and it must say that the setter is
// package/role-backed instead of implying an exact-component guarantee.
func TestCapabilityWordingNamesPackageRoleSelection(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	capability := service.bind(testSerial).probeCapability(context.Background())
	if capability.Status != CapabilitySupported {
		t.Fatalf("status = %s, want supported", capability.Status)
	}
	for _, want := range []string{"PACKAGE/role-backed", "reduced to its package name", "several HOME activities is refused"} {
		if !strings.Contains(capability.Detail, want) {
			t.Fatalf("capability detail must state %q: %q", want, capability.Detail)
		}
	}
	// The probe stays token based: a help text without the package-only disclaimer is
	// still supported, because the single-HOME guard - not this probe - carries the
	// reversibility proof.
	device.capability = "Package manager (package) commands:\n  set-home-activity [--user USER_ID] TARGET-COMPONENT\n  resolve-activity [--user USER_ID] INTENT\n"
	capability = service.bind(testSerial).probeCapability(context.Background())
	if capability.Status != CapabilitySupported {
		t.Fatalf("a token-only help text must stay supported: %+v", capability)
	}
	if strings.Contains(capability.Detail, "exact component") {
		t.Fatalf("the detail must not promise exact-component semantics: %q", capability.Detail)
	}
}

func TestLaunchInterpretationRequiresMeaningfulOutput(t *testing.T) {
	cases := []struct {
		name     string
		stdout   string
		stderr   string
		launched bool
	}{
		{name: "starting", stdout: "Starting: Intent { act=android.intent.action.MAIN }\n", launched: true},
		{name: "already front", stdout: "Warning: Activity not started, its current task has been brought to the front\n", launched: true},
		{name: "error line", stdout: "Error: Activity not started, unable to resolve Intent\n", launched: false},
		{name: "error on stderr", stderr: "Error type 3\nError: Activity class does not exist\n", launched: false},
		{name: "empty", launched: false},
		{name: "unrecognized", stdout: "java.lang.SecurityException: Permission Denial\n", launched: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			launched, detail := interpretLaunch(tc.stdout, tc.stderr)
			if launched != tc.launched {
				t.Fatalf("interpretLaunch(%q, %q) = %v (%s)", tc.stdout, tc.stderr, launched, detail)
			}
			if !tc.launched && detail == "" {
				t.Fatal("a failed launch must explain why")
			}
		})
	}
}

func TestReadDeviceFailsClosedOnDiagnosticsAndOversizedOutput(t *testing.T) {
	service := NewService(t.TempDir(), nil)
	service.runCommand = func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		return &core.ExecResult{Stdout: "ok", Stderr: "WARNING: unexpected", ExitCode: 0}, nil
	}
	if _, err := service.bind(testSerial).read(context.Background(), "cmd", "package", "help"); err == nil {
		t.Fatal("a diagnostic on stderr must fail closed")
	}

	service.runCommand = func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		return &core.ExecResult{Stdout: strings.Repeat("x", maxReadBytes+1), ExitCode: 0}, nil
	}
	if _, err := service.bind(testSerial).read(context.Background(), "cmd", "package", "help"); err == nil {
		t.Fatal("oversized output must fail closed")
	}

	service.runCommand = func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		return &core.ExecResult{Stdout: "unused", Stderr: "unknown command", ExitCode: 1}, nil
	}
	if _, err := service.bind(testSerial).read(context.Background(), "cmd", "package", "help"); err == nil {
		t.Fatal("a non-zero exit code must fail closed")
	}
}

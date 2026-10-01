package launcher

import (
	"ADBKit/internal/core"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testSerial       = "SERIAL-1"
	testOriginal     = "com.stock.launcher/com.stock.launcher.HomeActivity"
	testCandidate    = "com.custom.launcher/.MainActivity"
	testCandidatePkg = "com.custom.launcher"
	testOriginalPkg  = "com.stock.launcher"
	testOther        = "com.other.launcher/.OtherHome"
)

// fakeDevice is a scripted adb. It records every request and answers from the
// scripted device state, so no real adb or device is ever involved.
type fakeDevice struct {
	mu       sync.Mutex
	requests []core.ExecRequest

	capability     string // stdout of `cmd package help`
	capabilityExit int
	capabilityErr  bool

	current    string
	components []string

	enabled  map[string]bool
	disabled map[string]bool

	// launchOutput is keyed by the launched component; empty means a successful
	// "Starting:" line.
	launchOutput map[string]string

	failMutation      bool
	keepOriginalAfter bool // the HOME command "succeeds" but HOME does not move
	moveElsewhere     string
	launchErrorFor    string // component whose HOME/launch does not start
	afterMutation     func()

	gateMu    sync.Mutex
	gate      chan struct{}
	gateMatch string
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		capability:   "Package manager (package) commands:\n  set-home-activity [--user USER_ID] TARGET-COMPONENT\n  resolve-activity [--user USER_ID] INTENT\n  query-activities [--user USER_ID] INTENT\n",
		current:      testOriginal,
		components:   []string{testOriginal, testCandidate},
		enabled:      map[string]bool{testOriginalPkg: true, testCandidatePkg: true},
		disabled:     map[string]bool{},
		launchOutput: map[string]string{},
	}
}

func (d *fakeDevice) runCtx() func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
	return func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		return d.handle(req), nil
	}
}

func (d *fakeDevice) recorded() []core.ExecRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]core.ExecRequest(nil), d.requests...)
}

func (d *fakeDevice) commands() []string {
	recorded := d.recorded()
	out := make([]string, 0, len(recorded))
	for _, req := range recorded {
		out = append(out, strings.Join(req.Args, " "))
	}
	return out
}

func (d *fakeDevice) sawCommand(fragment string) bool {
	for _, command := range d.commands() {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}

// waitForCommand blocks until the fake observed a command containing fragment.
func (d *fakeDevice) waitForCommand(t *testing.T, fragment string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if d.sawCommand(fragment) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("the fake device never saw a command containing %q", fragment)
}

func (d *fakeDevice) openGate(match string) {
	d.gateMu.Lock()
	d.gate = make(chan struct{})
	d.gateMatch = match
	d.gateMu.Unlock()
}

func (d *fakeDevice) releaseGate() {
	d.gateMu.Lock()
	if d.gate != nil {
		close(d.gate)
		d.gate = nil
	}
	d.gateMu.Unlock()
}

func (d *fakeDevice) waitGate(command string) {
	d.gateMu.Lock()
	gate, match := d.gate, d.gateMatch
	d.gateMu.Unlock()
	if gate != nil && strings.Contains(command, match) {
		<-gate
	}
}

func (d *fakeDevice) handle(req core.ExecRequest) *core.ExecResult {
	d.mu.Lock()
	d.requests = append(d.requests, req)
	d.mu.Unlock()

	if len(req.Args) < 3 || req.Args[0] != "-s" || req.Args[2] != "shell" {
		return &core.ExecResult{ExitCode: 1, Stderr: "unsupported adb invocation"}
	}
	command := strings.Join(req.Args[3:], " ")

	switch {
	case strings.HasPrefix(command, "cmd package help"):
		if d.capabilityErr {
			return &core.ExecResult{ExitCode: 1, Stderr: "adb: device offline"}
		}
		exit := d.capabilityExit
		return &core.ExecResult{Stdout: d.capability, ExitCode: exit}

	case strings.HasPrefix(command, "cmd package resolve-activity"):
		return d.resolveResult(command)

	case strings.HasPrefix(command, "cmd package query-activities"):
		return &core.ExecResult{Stdout: strings.Join(d.components, "\n") + "\n"}

	case strings.HasPrefix(command, "pm list packages"):
		return d.packageList(command)

	case strings.HasPrefix(command, "cmd package set-home-activity"):
		d.waitGate(command)
		return d.setHome(command)

	case strings.HasPrefix(command, "am start"):
		return d.launch(command)
	}
	return &core.ExecResult{ExitCode: 1, Stderr: "unexpected command: " + command}
}

func (d *fakeDevice) resolveResult(command string) *core.ExecResult {
	if strings.Contains(command, "-n ") {
		requested := unquoteShellArg(command[strings.Index(command, "-n ")+3:])
		normalized, err := normalizeComponent(requested)
		if err != nil {
			return &core.ExecResult{ExitCode: 1, Stderr: "invalid component"}
		}
		for _, listed := range d.components {
			if mustNormalize(listed) == normalized {
				// Resolving an explicit component that really is a HOME activity
				// returns that component, not the current default.
				return &core.ExecResult{Stdout: normalized + "\n"}
			}
		}
		return &core.ExecResult{ExitCode: 1, Stdout: "No activity found\n"}
	}
	switch {
	case d.current == "":
		return &core.ExecResult{ExitCode: 1, Stderr: "no HOME resolution"}
	default:
		return &core.ExecResult{Stdout: d.current + "\n"}
	}
}

func (d *fakeDevice) packageList(command string) *core.ExecResult {
	fields := strings.Fields(command)
	filter := fields[len(fields)-1]
	var listed []string
	switch {
	case strings.Contains(command, " -e "):
		if d.enabled[filter] {
			listed = append(listed, "package:"+filter)
		}
	case strings.Contains(command, " -d "):
		if d.disabled[filter] {
			listed = append(listed, "package:"+filter)
		}
	default:
		return &core.ExecResult{ExitCode: 1, Stderr: "unsupported pm list filter"}
	}
	if len(listed) == 0 {
		return &core.ExecResult{Stdout: "no packages\n"}
	}
	return &core.ExecResult{Stdout: strings.Join(listed, "\n") + "\n"}
}

func (d *fakeDevice) setHome(command string) *core.ExecResult {
	if d.failMutation {
		return &core.ExecResult{ExitCode: 1, Stderr: "java.lang.SecurityException"}
	}
	fields := strings.Fields(command)
	target := unquoteShellArg(fields[len(fields)-1])
	normalized, err := normalizeComponent(target)
	if err != nil {
		return &core.ExecResult{ExitCode: 1, Stderr: "invalid component"}
	}
	if d.afterMutation != nil {
		d.afterMutation()
	}
	switch {
	case d.moveElsewhere != "":
		d.current = d.moveElsewhere
	case d.keepOriginalAfter:
		d.current = testOriginal
	default:
		d.current = normalized
	}
	return &core.ExecResult{Stdout: "Success\n"}
}

func (d *fakeDevice) launch(command string) *core.ExecResult {
	if output, ok := d.launchOutput[command]; ok {
		return &core.ExecResult{Stdout: output}
	}
	if d.launchErrorFor != "" {
		if strings.Contains(command, "-n ") {
			requested, err := normalizeComponent(unquoteShellArg(command[strings.Index(command, "-n ")+3:]))
			if err == nil && requested == d.launchErrorFor {
				return &core.ExecResult{Stdout: "Error: Activity not started, unable to resolve Intent\n"}
			}
		} else if mustNormalize(d.current) == d.launchErrorFor {
			return &core.ExecResult{Stdout: "Error: Activity not started, unable to resolve Intent\n"}
		}
	}
	return &core.ExecResult{Stdout: "Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.HOME] }\n"}
}

func unquoteShellArg(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") {
		return strings.ReplaceAll(trimmed[1:len(trimmed)-1], `'"'"'`, "'")
	}
	return trimmed
}

func mustNormalize(component string) string {
	normalized, err := normalizeComponent(component)
	if err != nil {
		return component
	}
	return normalized
}

// newTestService wires the service to the fake device and a pinned tool path.
func newTestService(t *testing.T, device *fakeDevice) *Service {
	t.Helper()
	service := NewService(t.TempDir(), func() core.BinaryPaths {
		return core.BinaryPaths{Adb: "adb-pinned"}
	})
	service.runCommand = device.runCtx()
	return service
}

func testIdentity() Identity {
	return Identity{Serial: testSerial, Model: "Chromecast", Codename: "sabrina", SDKVersion: "30", IsTV: true}
}

func TestPreflightIsReadOnlyAndCapabilityGated(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	report, err := service.Preflight(context.Background(), testSerial, testIdentity())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Supported || !report.Restorable {
		t.Fatalf("expected a supported reversible preflight: %+v", report)
	}
	if report.CurrentHome != testOriginal {
		t.Fatalf("current HOME = %q", report.CurrentHome)
	}
	if len(report.HomeCandidates) != 2 {
		t.Fatalf("candidates = %+v", report.HomeCandidates)
	}
	for _, candidate := range report.HomeCandidates {
		if candidate.Component == testOriginal && !candidate.Selected {
			t.Fatal("the current HOME must be marked as selected")
		}
	}
	if device.sawCommand("set-home-activity") || device.sawCommand("am start") {
		t.Fatalf("preflight must be read-only: %v", device.commands())
	}
	for _, req := range device.recorded() {
		if req.Args[0] != "-s" || req.Args[1] != testSerial || req.Command != "adb-pinned" {
			t.Fatalf("unpinned probe: %+v", req)
		}
		if req.Timeout <= 0 {
			t.Fatalf("probe without a finite timeout: %+v", req)
		}
	}
}

func TestPreflightCannotAutoApplyWhenCapabilityIsUnknownOrMissing(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		device := newFakeDevice()
		device.capability = "Package manager (package) commands:\n  resolve-activity INTENT\n"
		service := newTestService(t, device)
		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported || report.Capability.Status != CapabilityUnsupported {
			t.Fatalf("missing capability must not be supported: %+v", report)
		}
		if _, err := service.Apply(context.Background(), ApplyRequest{
			OperationID: "op-missing-token", ExpectedSerial: testSerial,
			ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
		}); err == nil || !strings.Contains(err.Error(), "not confirmed") {
			t.Fatalf("apply must refuse an unsupported device: %v", err)
		}
		if device.sawCommand("set-home-activity") {
			t.Fatal("an unsupported device must not receive a HOME command")
		}
	})

	t.Run("probe failure", func(t *testing.T) {
		device := newFakeDevice()
		device.capabilityErr = true
		service := newTestService(t, device)
		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported || report.Capability.Status != CapabilityUnknown {
			t.Fatalf("an unreadable probe must stay unknown: %+v", report)
		}
		if !strings.Contains(report.Reason, "could not be probed") {
			t.Fatalf("reason = %q", report.Reason)
		}
	})
}

func TestPreflightRefusesAmbiguousOrChooserHome(t *testing.T) {
	t.Run("ambiguous resolution", func(t *testing.T) {
		device := newFakeDevice()
		device.current = testOriginal + "\ncom.other.launcher/.OtherHome"
		service := newTestService(t, device)
		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported || report.Restorable {
			t.Fatalf("ambiguous HOME must not be supported: %+v", report)
		}
	})

	t.Run("platform resolver", func(t *testing.T) {
		device := newFakeDevice()
		device.current = "android/com.android.internal.app.ResolverActivity"
		device.components = []string{"android/com.android.internal.app.ResolverActivity", testCandidate}
		device.enabled["android"] = true
		service := newTestService(t, device)
		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported || report.Restorable {
			t.Fatalf("a resolver HOME is not a reversible baseline: %+v", report)
		}
		if !strings.Contains(report.Reason, "resolver") {
			t.Fatalf("reason = %q", report.Reason)
		}
	})
}

func TestApplyRefusesInvalidCandidatesAndStaleCurrentHome(t *testing.T) {
	cases := []struct {
		name       string
		prepare    func(*fakeDevice)
		request    ApplyRequest
		wantDetail string
	}{
		{
			name: "candidate is not a HOME activity",
			request: ApplyRequest{OperationID: "op-not-home", ExpectedSerial: testSerial,
				ExpectedComponent: testOriginal, CandidateComponent: "com.other.launcher/.OtherHome"},
			wantDetail: "not a resolvable HOME activity",
		},
		{
			name: "candidate package disabled",
			prepare: func(d *fakeDevice) {
				d.enabled = map[string]bool{testOriginalPkg: true}
				d.disabled = map[string]bool{testCandidatePkg: true}
			},
			request: ApplyRequest{OperationID: "op-disabled", ExpectedSerial: testSerial,
				ExpectedComponent: testOriginal, CandidateComponent: testCandidate},
			wantDetail: "not enabled",
		},
		{
			name: "candidate package absent",
			prepare: func(d *fakeDevice) {
				d.enabled = map[string]bool{testOriginalPkg: true}
			},
			request: ApplyRequest{OperationID: "op-absent", ExpectedSerial: testSerial,
				ExpectedComponent: testOriginal, CandidateComponent: testCandidate},
			wantDetail: "not installed",
		},
		{
			name: "stale expected current HOME",
			request: ApplyRequest{OperationID: "op-stale-home", ExpectedSerial: testSerial,
				ExpectedComponent: testOther, CandidateComponent: testCandidate},
			wantDetail: "current HOME changed",
		},
		{
			name: "candidate equals current HOME",
			request: ApplyRequest{OperationID: "op-same-home", ExpectedSerial: testSerial,
				ExpectedComponent: testOriginal, CandidateComponent: testOriginal},
			wantDetail: "already the current HOME",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			device := newFakeDevice()
			if tc.prepare != nil {
				tc.prepare(device)
			}
			service := newTestService(t, device)
			_, err := service.Apply(context.Background(), tc.request)
			if err == nil {
				t.Fatal("expected a structured refusal")
			}
			opErr, ok := err.(*core.OperationError)
			if !ok {
				t.Fatalf("expected a structured error, got %T", err)
			}
			detail := opErr.Message + " " + opErr.Detail
			if !strings.Contains(detail, tc.wantDetail) {
				t.Fatalf("detail = %q, want %q", detail, tc.wantDetail)
			}
			if device.sawCommand("set-home-activity") {
				t.Fatalf("a refused apply must not reach the device: %v", device.commands())
			}
			if _, err := os.Stat(filepath.Join(service.dataDir, "launcher", tc.request.OperationID+".json")); err == nil {
				t.Fatal("a refused apply must not leave a durable record")
			}
		})
	}
}

func TestApplyPersistsIntentBeforeMutationAndVerifies(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	result, err := service.Apply(context.Background(), ApplyRequest{
		OperationID:        "op-happy-0001",
		ExpectedSerial:     testSerial,
		ExpectedComponent:  testOriginal,
		CandidateComponent: testCandidate,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.Verified || result.State != StateApplied || result.CurrentComponent != mustNormalize(testCandidate) {
		t.Fatalf("unexpected result: %+v", result)
	}

	// The durable record exists, is normalized and applied.
	record, err := service.store().load("op-happy-0001")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if record.State != StateApplied || record.OriginalComponent != testOriginal || record.CandidateComponent != mustNormalize(testCandidate) {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.Serial != testSerial || record.UserID != 0 || record.Action != ActionSetHome {
		t.Fatalf("record identity drifted: %+v", record)
	}

	// Every command is pinned to the serial and the live tool, and the only write
	// path is set-home-activity with explicit user 0 and a quoted component.
	writeSeen := false
	for _, req := range device.recorded() {
		if req.Command != "adb-pinned" || req.Args[0] != "-s" || req.Args[1] != testSerial {
			t.Fatalf("unpinned command: %+v", req)
		}
		command := strings.Join(req.Args, " ")
		if strings.Contains(command, "set-home-activity") {
			writeSeen = true
			if !strings.Contains(command, "--user 0") {
				t.Fatalf("missing explicit user scope: %s", command)
			}
			if !strings.Contains(command, "'"+mustNormalize(testCandidate)+"'") {
				t.Fatalf("candidate was not quoted for the remote shell: %s", command)
			}
		}
	}
	if !writeSeen {
		t.Fatal("no HOME command was recorded")
	}
	if device.sawCommand("pm disable") || device.sawCommand("disable-user") || device.sawCommand("pm uninstall") || device.sawCommand("pm enable") {
		t.Fatalf("the launcher path must never disable/uninstall a package: %v", device.commands())
	}
}

func TestApplyWriteFailureBlocksMutation(t *testing.T) {
	device := newFakeDevice()
	// A regular file where the data directory should be makes the record write fail.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(blocker, func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb-pinned"} })
	service.runCommand = device.runCtx()

	_, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-norecord1", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err == nil {
		t.Fatal("expected the record write failure to abort the apply")
	}
	if device.sawCommand("set-home-activity") {
		t.Fatalf("no device change may happen without a durable record: %v", device.commands())
	}
}

// TestApplyReportsNoChangeWhenHomeDidNotMove covers a "successful" HOME command
// that did not actually change anything: the result must be truthful, the original
// HOME must be recorded as restored and no rollback command must be sent.
func TestApplyReportsNoChangeWhenHomeDidNotMove(t *testing.T) {
	device := newFakeDevice()
	device.keepOriginalAfter = true
	service := newTestService(t, device)

	result, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-verify01", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err == nil {
		t.Fatal("an unverified change must not be reported as success")
	}
	if result.Verified {
		t.Fatalf("unverified result claimed success: %+v", result)
	}
	record, loadErr := service.store().load("op-verify01")
	if loadErr != nil {
		t.Fatalf("record: %v", loadErr)
	}
	if record.State != StateRestored {
		t.Fatalf("record state = %s, want restored: %+v", record.State, record)
	}
	if strings.Count(strings.Join(device.commands(), "\n"), "set-home-activity") != 1 {
		t.Fatalf("nothing had to be rolled back: %v", device.commands())
	}
	if !strings.Contains(err.Error(), "did not take effect") || !strings.Contains(result.Detail, "no change remained") {
		t.Fatalf("detail = %q err = %v", result.Detail, err)
	}
}

// TestApplyRollsBackWhenCandidateHomeDoesNotLaunch covers a verified HOME change
// whose launcher does not actually start: the service must restore the original and
// verify that restoration.
func TestApplyRollsBackWhenCandidateHomeDoesNotLaunch(t *testing.T) {
	device := newFakeDevice()
	device.launchErrorFor = mustNormalize(testCandidate)
	service := newTestService(t, device)

	result, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-launch01", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err == nil {
		t.Fatal("a launcher that does not start must not be reported as success")
	}
	if !result.RolledBack || result.State != StateRestored || result.CurrentComponent != testOriginal {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !device.sawCommand("'" + testOriginal + "'") {
		t.Fatalf("the rollback did not target the original component: %v", device.commands())
	}
	record, loadErr := service.store().load("op-launch01")
	if loadErr != nil {
		t.Fatalf("record: %v", loadErr)
	}
	if record.State != StateRestored {
		t.Fatalf("record state = %s, want restored: %+v", record.State, record)
	}
}

func TestApplyKeepsUnknownWhenHomeMovedElsewhere(t *testing.T) {
	device := newFakeDevice()
	device.moveElsewhere = testOther
	service := newTestService(t, device)

	result, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-moved001", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err == nil {
		t.Fatal("an unrelated HOME must not be reported as success")
	}
	if result.State != StateUnknown {
		t.Fatalf("result state = %s, want unknown: %+v", result.State, result)
	}
	if !strings.Contains(result.Detail, "manual recovery") || !strings.Contains(result.Detail, testSerial) {
		t.Fatalf("manual recovery instructions missing: %q", result.Detail)
	}
	record, loadErr := service.store().load("op-moved001")
	if loadErr != nil {
		t.Fatalf("record: %v", loadErr)
	}
	if record.State != StateUnknown {
		t.Fatalf("record state = %s, want unknown", record.State)
	}
	if strings.Count(strings.Join(device.commands(), "\n"), "set-home-activity") != 1 {
		t.Fatalf("the service must not bulldoze an unrelated HOME: %v", device.commands())
	}
}

func TestApplyCancelledAfterMutationRollsBackWithFreshContext(t *testing.T) {
	device := newFakeDevice()
	device.openGate("set-home-activity")
	service := newTestService(t, device)

	type outcome struct {
		result ApplyResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := service.Apply(context.Background(), ApplyRequest{
			OperationID: "op-cancel001", ExpectedSerial: testSerial,
			ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
		})
		done <- outcome{result: result, err: err}
	}()

	device.waitForCommand(t, "set-home-activity")

	// Cancel must return while the operation mutex is still held by Apply.
	cancelled := make(chan CancelResult, 1)
	cancelErr := make(chan error, 1)
	go func() {
		result, err := service.Cancel(CancelRequest{OperationID: "op-cancel001", ExpectedSerial: testSerial})
		cancelled <- result
		cancelErr <- err
	}()
	select {
	case err := <-cancelErr:
		if err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel blocked on the running operation")
	}
	if result := <-cancelled; !result.Cancelled {
		t.Fatalf("cancel did not signal the owned operation: %+v", result)
	}

	device.releaseGate()
	select {
	case got := <-done:
		if got.err == nil {
			t.Fatal("a cancelled apply must not be reported as success")
		}
		if got.result.State != StateRestored && got.result.State != StateUnknown {
			t.Fatalf("cancelled apply state = %s", got.result.State)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Apply did not finish after cancellation")
	}

	// The rollback ran on a fresh bounded context even though the operation context
	// was cancelled: it targeted the original component.
	if !device.sawCommand("'" + testOriginal + "'") {
		t.Fatalf("the rollback did not target the original component: %v", device.commands())
	}
	record, err := service.store().load("op-cancel001")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if record.State != StateRestored {
		t.Fatalf("record state = %s, want restored", record.State)
	}
}

func TestCancelRequiresOwnedSerialAndOperation(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	if _, err := service.Cancel(CancelRequest{OperationID: "op-nothing01", ExpectedSerial: testSerial}); err == nil {
		t.Fatal("cancelling an unowned operation must fail")
	}
	if _, err := service.Cancel(CancelRequest{OperationID: "short", ExpectedSerial: testSerial}); err == nil {
		t.Fatal("invalid operation IDs must be rejected")
	}
	if _, err := service.Cancel(CancelRequest{OperationID: "op-nothing01", ExpectedSerial: ""}); err == nil {
		t.Fatal("a blank serial must be rejected")
	}
	if len(device.recorded()) != 0 {
		t.Fatalf("Cancel must not touch the device: %v", device.commands())
	}
}

func TestTestCandidateIsExplicitSeparateAndMeaningful(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	result, err := service.TestCandidate(context.Background(), testSerial, testCandidate)
	if err != nil {
		t.Fatalf("TestCandidate: %v", err)
	}
	if !result.Resolved || !result.Launched {
		t.Fatalf("unexpected candidate test: %+v", result)
	}
	if device.sawCommand("set-home-activity") {
		t.Fatalf("the launch test must not change HOME: %v", device.commands())
	}
	if !device.sawCommand("am start -n " + mustNormalize(testCandidate)) {
		t.Fatalf("the launch test did not open the exact candidate: %v", device.commands())
	}

	// A launch that only reports an error must not be reported as usable.
	device.launchOutput["am start -n "+mustNormalize(testCandidate)] = "Error: Activity not started, unable to resolve Intent"
	result, err = service.TestCandidate(context.Background(), testSerial, testCandidate)
	if err != nil {
		t.Fatalf("TestCandidate: %v", err)
	}
	if result.Launched {
		t.Fatalf("an error output must not be reported as launched: %+v", result)
	}

	// An unknown candidate or an invalid component is refused.
	if _, err := service.TestCandidate(context.Background(), testSerial, "com.custom.launcher"); err == nil {
		t.Fatal("a package-only candidate must be refused")
	}
	if _, err := service.TestCandidate(context.Background(), testSerial, testOther); err == nil {
		t.Fatal("a component outside the HOME set must be refused")
	}
}

func TestCommandsStayPinnedWhenToolsChangeMidCall(t *testing.T) {
	device := newFakeDevice()
	paths := core.BinaryPaths{Adb: "adb-pinned"}
	calls := 0
	service := NewService(t.TempDir(), func() core.BinaryPaths {
		calls++
		if calls == 1 {
			return paths
		}
		// Any later read would see a different tool; the operation must not.
		return core.BinaryPaths{Adb: "adb-switched"}
	})
	service.runCommand = device.runCtx()

	if _, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-pinned001", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, req := range device.recorded() {
		if req.Command != "adb-pinned" {
			t.Fatalf("the tool path changed mid-operation: %+v", req)
		}
		if req.Args[1] != testSerial {
			t.Fatalf("the serial changed mid-operation: %+v", req)
		}
	}
}

func TestHomeCommandAndRecordUseOnlySupportedWritePath(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)
	if _, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-write0001", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	forbidden := []string{"disable-user", "pm disable", "pm uninstall", "pm enable", "pm clear", "pm hide", "clear-package-preferred-activities", "sh -c", "cmd.exe"}
	for _, command := range device.commands() {
		for _, bad := range forbidden {
			if strings.Contains(command, bad) {
				t.Fatalf("forbidden command %q in %q", bad, command)
			}
		}
	}
}

const (
	stockAltHome     = "com.stock.launcher/com.stock.launcher.AltHomeActivity"
	candidateMain    = "com.custom.launcher/.MainActivity"
	candidateAltHome = "com.custom.launcher/.AltActivity"
)

// TestApplyRefusesSeveralHomeActivitiesPerPackage covers the package/role-backed
// semantics of the supported setter: a package with more than one HOME activity can
// never prove which exact component HOME will resolve to, so the apply must be
// refused before the durable record and before any device change.
func TestApplyRefusesSeveralHomeActivitiesPerPackage(t *testing.T) {
	cases := []struct {
		name       string
		components []string
		current    string
		expected   string
		candidate  string
		wantDetail string
		// currentAmbiguous marks a device whose OWN current HOME package is ambiguous:
		// such a device must be reported unsupported. When only a candidate package is
		// ambiguous the device stays supported and the ambiguity is reported read-only.
		currentAmbiguous bool
		ambiguousPkg     string
	}{
		{
			name:             "original package declares two HOME activities",
			components:       []string{testOriginal, stockAltHome, candidateMain},
			current:          testOriginal,
			expected:         testOriginal,
			candidate:        candidateMain,
			wantDetail:       "current HOME package declares several HOME activities",
			currentAmbiguous: true,
			ambiguousPkg:     testOriginalPkg,
		},
		{
			name:         "candidate package declares two HOME activities",
			components:   []string{testOriginal, candidateMain, candidateAltHome},
			current:      testOriginal,
			expected:     testOriginal,
			candidate:    candidateMain,
			wantDetail:   "requested launcher package declares several HOME activities",
			ambiguousPkg: testCandidatePkg,
		},
		{
			name:             "original and candidate share one multi-HOME package",
			components:       []string{candidateMain, candidateAltHome},
			current:          candidateMain,
			expected:         candidateMain,
			candidate:        candidateAltHome,
			wantDetail:       "declares 2 HOME activities",
			currentAmbiguous: true,
			ambiguousPkg:     testCandidatePkg,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			device := newFakeDevice()
			device.components = tc.components
			device.current = mustNormalize(tc.current)
			service := newTestService(t, device)

			result, err := service.Apply(context.Background(), ApplyRequest{
				OperationID:        "op-multihome1",
				ExpectedSerial:     testSerial,
				ExpectedComponent:  tc.expected,
				CandidateComponent: tc.candidate,
			})
			if err == nil {
				t.Fatal("a multi-HOME package must not be changed")
			}
			if !strings.Contains(err.Error(), tc.wantDetail) {
				t.Fatalf("error = %v, want %q", err, tc.wantDetail)
			}
			if !strings.Contains(err.Error(), "HOME activities") {
				t.Fatalf("the refusal must name the ambiguity: %v", err)
			}
			if result.State != "" || result.Verified {
				t.Fatalf("a refused apply must not report an outcome: %+v", result)
			}
			if device.sawCommand("set-home-activity") {
				t.Fatalf("no device change may happen: %v", device.commands())
			}
			if device.sawCommand("am start") {
				t.Fatalf("a refused apply must not launch anything: %v", device.commands())
			}
			if _, statErr := os.Stat(filepath.Join(service.dataDir, "launcher", "op-multihome1.json")); statErr == nil {
				t.Fatal("a refused apply must not leave a durable record")
			}

			// The preflight must report the same ambiguity instead of claiming support.
			report, preflightErr := service.Preflight(context.Background(), testSerial, testIdentity())
			if preflightErr != nil {
				t.Fatal(preflightErr)
			}
			if !strings.Contains(report.ProbeDetail, "refused as targets") || !strings.Contains(report.ProbeDetail, tc.ambiguousPkg) {
				t.Fatalf("preflight detail must name the ambiguous package: %q", report.ProbeDetail)
			}
			if tc.currentAmbiguous {
				if report.Supported || report.Restorable {
					t.Fatalf("an ambiguous current HOME must not be supported: %+v", report)
				}
				if !strings.Contains(report.Reason, "HOME activities") {
					t.Fatalf("preflight reason = %q", report.Reason)
				}
			} else if !report.Supported || !report.Restorable {
				t.Fatalf("a device with an unambiguous current HOME stays supported: %+v", report)
			}
			if device.sawCommand("set-home-activity") {
				t.Fatalf("preflight must stay read-only: %v", device.commands())
			}
		})
	}
}

// TestRestoreRefusesSeveralHomeActivitiesBeforeAnyChange covers the restore side:
// the guard must run before the first write command and must leave the record as it
// was, while still offering the serial-pinned manual path.
func TestRestoreRefusesSeveralHomeActivitiesBeforeAnyChange(t *testing.T) {
	device := newFakeDevice()
	device.components = []string{testOriginal, stockAltHome, candidateMain}
	device.current = mustNormalize(candidateMain)
	service := newTestService(t, device)

	record := validRecord("op-restmulti", StateApplied)
	if err := service.store().persist(record); err != nil {
		t.Fatal(err)
	}

	result, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-restmulti"})
	if err == nil {
		t.Fatal("a multi-HOME original package must not be restored")
	}
	if !strings.Contains(err.Error(), "several HOME activities") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "exact restoration cannot be proven") {
		t.Fatalf("the refusal must state why: %v", err)
	}
	if device.sawCommand("set-home-activity") {
		t.Fatalf("no device change may happen before the guard: %v", device.commands())
	}
	if !strings.Contains(result.ManualCommand, testSerial) || !strings.Contains(result.ManualCommand, testOriginal) {
		t.Fatalf("the refusal must offer the serial-pinned manual path: %q", result.ManualCommand)
	}
	loaded, loadErr := service.store().load("op-restmulti")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if loaded.State != StateApplied {
		t.Fatalf("a refusal must not rewrite the record: %+v", loaded)
	}
}

// TestSingleHomePerPackagePathIsUnchanged is the guard's negative control: with one
// HOME activity per package the proven path still applies, verifies and restores the
// exact components, and the canonical argument keeps --user before the component.
func TestSingleHomePerPackagePathIsUnchanged(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	report, err := service.Preflight(context.Background(), testSerial, testIdentity())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Supported || !report.Restorable || report.CurrentHome != testOriginal {
		t.Fatalf("single-HOME preflight must stay supported: %+v", report)
	}

	applied, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-singlehome", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !applied.Verified || applied.State != StateApplied || applied.CurrentComponent != mustNormalize(testCandidate) {
		t.Fatalf("unexpected apply result: %+v", applied)
	}

	// The canonical argument order is preserved: --user 0 precedes the component.
	wantArgs := []string{"-s", testSerial, "shell", "cmd", "package", "set-home-activity", "--user", "0", "'" + mustNormalize(testCandidate) + "'"}
	found := false
	for _, req := range device.recorded() {
		if strings.Contains(strings.Join(req.Args, " "), "set-home-activity") {
			found = true
			if strings.Join(req.Args, "\x00") != strings.Join(wantArgs, "\x00") {
				t.Fatalf("set-home argv = %#v, want %#v", req.Args, wantArgs)
			}
		}
	}
	if !found {
		t.Fatal("no HOME command was recorded")
	}

	restored, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-singlehome"})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restored.Verified || restored.State != StateRestored || restored.CurrentComponent != testOriginal {
		t.Fatalf("unexpected restore result: %+v", restored)
	}
}

// TestPreflightSurfacesRecoveryReadFailure keeps a recovery-read failure visible:
// supported=false plus a reason, never a silently empty list.
func TestPreflightSurfacesRecoveryReadFailure(t *testing.T) {
	t.Run("unreadable launcher directory", func(t *testing.T) {
		device := newFakeDevice()
		service := newTestService(t, device)
		// A regular file where the record directory belongs makes the local read fail
		// deterministically, without touching the device.
		if err := os.WriteFile(filepath.Join(service.dataDir, "launcher"), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}

		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported {
			t.Fatalf("a recovery read failure must not be reported as supported: %+v", report)
		}
		if !strings.Contains(report.Reason, "recovery state is not trustworthy") || !strings.Contains(report.Reason, "could not be read") {
			t.Fatalf("reason = %q", report.Reason)
		}
		if !strings.Contains(report.ProbeDetail, "recovery records could not be read") {
			t.Fatalf("probe detail = %q", report.ProbeDetail)
		}
		if device.sawCommand("set-home-activity") {
			t.Fatalf("preflight must stay read-only: %v", device.commands())
		}
	})

	t.Run("unreadable record file is still listed", func(t *testing.T) {
		device := newFakeDevice()
		service := newTestService(t, device)
		dir := filepath.Join(service.dataDir, "launcher")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "op-broken001.json"), []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}

		report, err := service.Preflight(context.Background(), testSerial, testIdentity())
		if err != nil {
			t.Fatal(err)
		}
		if report.Supported {
			t.Fatalf("an unreadable record must not be reported as supported: %+v", report)
		}
		if !strings.Contains(report.Reason, "unreadable file") || !strings.Contains(report.ProbeDetail, "op-broken001.json") {
			t.Fatalf("reason = %q, detail = %q", report.Reason, report.ProbeDetail)
		}
		if len(report.Recovery) != 1 || !report.Recovery[0].Unreadable || report.Recovery[0].File != "op-broken001.json" {
			t.Fatalf("the unreadable record must stay listed: %+v", report.Recovery)
		}
	})
}

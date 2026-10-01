package launcher

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	probeTimeout   = 5 * time.Second
	mutateTimeout  = 15 * time.Second
	launchTimeout  = 10 * time.Second
	cleanupTimeout = 15 * time.Second
	maxReadBytes   = 64 * 1024
	androidUserID  = 0
)

// homeIntent is the HOME intent used by every read in this package.
var homeIntent = []string{"-a", "android.intent.action.MAIN", "-c", "android.intent.category.HOME"}

// Service performs the guarded launcher operations. Every method takes an explicit
// confirmed serial; nothing here follows the mutable global device selection, and
// no method mutates a device during startup or a read-only path.
type Service struct {
	dataDir    string
	getBinPath func() core.BinaryPaths
	// runCommand is the injected runner. Nil uses the production core.RunCommand.
	runCommand func(context.Context, core.ExecRequest) (*core.ExecResult, error)

	// operationMu serializes mutation and launch-test actions. Cancel never takes it.
	operationMu sync.Mutex

	// stateMu guards only the operation identity/cancel callback and is always held
	// briefly, so a cancellation never waits for device work.
	stateMu  sync.Mutex
	opID     string
	opSerial string
	opCancel context.CancelFunc
}

func NewService(dataDir string, getBinPath func() core.BinaryPaths) *Service {
	return &Service{dataDir: dataDir, getBinPath: getBinPath}
}

// store points at the launcher subdirectory of the existing resolved app-data
// directory. Records live beside, but separate from, Safe Tuning snapshots.
func (s *Service) store() *store {
	return newStore(s.dataDir)
}

func (s *Service) runner() func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
	if s.runCommand != nil {
		return s.runCommand
	}
	return core.RunCommand
}

// toolName resolves the live adb path for display and execution. A path stored in
// a record is never used here.
func (s *Service) toolName() string {
	if s.getBinPath != nil {
		if paths := s.getBinPath(); strings.TrimSpace(paths.Adb) != "" {
			return paths.Adb
		}
	}
	return core.BinaryNameAdb
}

func (s *Service) now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// target is one immutable operation target: the confirmed serial plus the tool path
// captured once when the operation starts. Every device command of that operation
// goes through this value, so a later configuration or selection change cannot
// retarget it.
type target struct {
	runner func(context.Context, core.ExecRequest) (*core.ExecResult, error)
	tool   string
	serial string
}

func (s *Service) bind(serial string) target {
	return target{runner: s.runner(), tool: s.toolName(), serial: serial}
}

// exec is the single place that builds a pinned device command.
func (t target) exec(ctx context.Context, timeout time.Duration, args ...string) (*core.ExecResult, error) {
	return t.runner(ctx, core.ExecRequest{
		Command: t.tool,
		Args:    append([]string{"-s", t.serial, "shell"}, args...),
		Timeout: timeout,
	})
}

type packageState struct {
	installed bool
	enabled   bool
}

type inspection struct {
	capability Capability
	current    string
	components []string
	states     map[string]packageState
	problems   []string
}

// read runs one read-only device command and requires a meaningful result:
// successful exit code, no diagnostic on stderr and bounded output. Unsupported or
// ambiguous firmware output therefore fails closed.
func (t target) read(ctx context.Context, args ...string) (string, error) {
	result, err := t.exec(ctx, probeTimeout, args...)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", core.NewOperationError("launcher_probe", "Device command failed", err.Error(), true)
	}
	if result == nil {
		return "", core.NewOperationError("launcher_probe", "Device command returned no result", "", true)
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return "", core.NewOperationError("launcher_probe", "Device command was not accepted", detail, false)
	}
	if diagnostic := strings.TrimSpace(result.Stderr); diagnostic != "" {
		return "", core.NewOperationError("launcher_probe", "Device command returned a diagnostic", diagnostic, false)
	}
	if len(result.Stdout) > maxReadBytes {
		return "", core.NewOperationError("launcher_probe", "Device output exceeds the supported size", "", false)
	}
	return strings.TrimSpace(result.Stdout), nil
}

// mutate runs one mutating device command with a finite timeout. It returns the raw
// output: the caller verifies the device state instead of trusting the exit code.
func (t target) mutate(ctx context.Context, args ...string) (string, error) {
	result, err := t.exec(ctx, mutateTimeout, args...)
	if err != nil {
		return "", core.NewOperationError("launcher_mutation", "Device command failed", err.Error(), true)
	}
	if result == nil {
		return "", core.NewOperationError("launcher_mutation", "Device command returned no result", "", true)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return "", core.NewOperationError("launcher_mutation", "HOME command was not accepted", detail, false)
	}
	return strings.TrimSpace(strings.Join([]string{result.Stdout, result.Stderr}, "\n")), nil
}

func nonEmptyLines(output string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func firstNonEmptyLine(output string) string {
	if lines := nonEmptyLines(output); len(lines) > 0 {
		return lines[0]
	}
	return ""
}

// currentHome returns the single resolved HOME component, failing closed when the
// device reports nothing or more than one line.
func (t target) currentHome(ctx context.Context) (string, error) {
	args := append([]string{"cmd", "package", "resolve-activity", "--components", "--user", "0"}, homeIntent...)
	output, err := t.read(ctx, args...)
	if err != nil {
		return "", err
	}
	lines := nonEmptyLines(output)
	if len(lines) != 1 {
		return "", core.NewOperationError("launcher_probe", "HOME resolution is unavailable or ambiguous", fmt.Sprintf("%d line(s)", len(lines)), false)
	}
	return normalizeComponent(lines[0])
}

// homeComponents returns every resolvable HOME component the device reports.
func (t target) homeComponents(ctx context.Context) ([]string, error) {
	args := append([]string{"cmd", "package", "query-activities", "--components", "--user", "0"}, homeIntent...)
	output, err := t.read(ctx, args...)
	if err != nil {
		return nil, err
	}
	lines := nonEmptyLines(output)
	if len(lines) == 0 {
		return nil, core.NewOperationError("launcher_probe", "No HOME activity is reported", "", false)
	}
	components := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		component, err := normalizeComponent(line)
		if err != nil {
			return nil, core.NewOperationError("launcher_probe", "HOME candidate output is not a valid component", line, false)
		}
		if seen[component] {
			continue
		}
		seen[component] = true
		components = append(components, component)
	}
	return components, nil
}

// resolveComponent asks the platform whether one exact component is a HOME activity
// it can resolve for user 0.
func (t target) resolveComponent(ctx context.Context, component string) (string, error) {
	args := append([]string{"cmd", "package", "resolve-activity", "--components", "--user", "0"}, homeIntent...)
	args = append(args, "-n", component)
	output, err := t.read(ctx, args...)
	if err != nil {
		return "", err
	}
	lines := nonEmptyLines(output)
	if len(lines) != 1 {
		return "", core.NewOperationError("launcher_probe", "Candidate resolution is unavailable or ambiguous", fmt.Sprintf("%d line(s)", len(lines)), false)
	}
	return normalizeComponent(lines[0])
}

// packageListed reports whether the package appears in one pm list filter.
func (t target) packageListed(ctx context.Context, flag, pkg string) (bool, error) {
	output, err := t.read(ctx, "pm", "list", "packages", flag, "--user", "0", pkg)
	if err != nil {
		return false, err
	}
	for _, line := range nonEmptyLines(output) {
		if strings.HasPrefix(line, "package:") && strings.TrimSpace(strings.TrimPrefix(line, "package:")) == pkg {
			return true, nil
		}
	}
	return false, nil
}

func (t target) packageState(ctx context.Context, pkg string) (packageState, error) {
	enabled, err := t.packageListed(ctx, "-e", pkg)
	if err != nil {
		return packageState{}, err
	}
	if enabled {
		return packageState{installed: true, enabled: true}, nil
	}
	disabled, err := t.packageListed(ctx, "-d", pkg)
	if err != nil {
		return packageState{}, err
	}
	return packageState{installed: disabled, enabled: false}, nil
}

// inspect performs every read-only probe the guarded flow needs.
func (t target) inspect(ctx context.Context) inspection {
	ins := inspection{states: map[string]packageState{}}
	ins.capability = t.probeCapability(ctx)

	current, err := t.currentHome(ctx)
	if err != nil {
		ins.problems = append(ins.problems, "current HOME: "+err.Error())
	} else {
		ins.current = current
	}

	components, err := t.homeComponents(ctx)
	if err != nil {
		ins.problems = append(ins.problems, "HOME candidates: "+err.Error())
	} else {
		ins.components = components
	}

	packages := map[string]bool{}
	for _, component := range components {
		packages[packageOf(component)] = true
	}
	if ins.current != "" {
		packages[packageOf(ins.current)] = true
	}
	for _, pkg := range sortedKeys(packages) {
		state, err := t.packageState(ctx, pkg)
		if err != nil {
			ins.problems = append(ins.problems, fmt.Sprintf("package %s: %v", pkg, err))
			continue
		}
		ins.states[pkg] = state
	}
	return ins
}

func (ins inspection) candidateViews() []HomeCandidate {
	views := make([]HomeCandidate, 0, len(ins.components))
	for _, component := range ins.components {
		state := ins.states[packageOf(component)]
		views = append(views, HomeCandidate{
			Component: component,
			Package:   packageOf(component),
			Installed: state.installed,
			Enabled:   state.enabled,
			Selected:  component == ins.current,
			Chooser:   isChooserOrResolver(component),
		})
	}
	return views
}

// countHomeForPackage reports how many of the listed HOME components belong to the
// package of the given component.
//
// The supported HOME setter selects HOME for a PACKAGE: since Android 10
// `cmd package set-home-activity` reduces its TARGET-COMPONENT argument to the
// package name and assigns the HOME role to that package, so the component that
// actually becomes HOME is chosen by the platform. Several HOME activities inside
// one package therefore make both the exact component and its restoration
// unprovable, and every such package is refused instead of being guessed.
func countHomeForPackage(components []string, component string) int {
	pkg := packageOf(component)
	count := 0
	for _, listed := range components {
		if packageOf(listed) == pkg {
			count++
		}
	}
	return count
}

// homeCount reports how many distinct normalized HOME components the package of the
// given component declares.
func (ins inspection) homeCount(component string) int {
	return countHomeForPackage(ins.components, component)
}

// restorable reports whether the current HOME can be restored by the supported
// command: exactly known, the only HOME activity of its package, listed as a HOME
// activity, installed and enabled, and not a platform resolver.
func (ins inspection) restorable() bool {
	if ins.current == "" || isChooserOrResolver(ins.current) {
		return false
	}
	if ins.homeCount(ins.current) != 1 {
		return false
	}
	found := false
	for _, component := range ins.components {
		if component == ins.current {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	state := ins.states[packageOf(ins.current)]
	return state.installed && state.enabled
}

// candidateFor validates one requested candidate against the live probes.
func (ins inspection) candidateFor(component string) error {
	if isChooserOrResolver(component) {
		return core.NewOperationError("launcher_candidate", "The requested component is a platform resolver/chooser", component, false)
	}
	found := false
	for _, listed := range ins.components {
		if listed == component {
			found = true
			break
		}
	}
	if !found {
		return core.NewOperationError("launcher_candidate", "The requested component is not a resolvable HOME activity", component, false)
	}
	if count := ins.homeCount(component); count != 1 {
		return core.NewOperationError("launcher_candidate", "The requested launcher package declares several HOME activities",
			fmt.Sprintf("%s declares %d HOME activities; the supported command selects HOME per package, so the exact component cannot be proven", packageOf(component), count), false)
	}
	state := ins.states[packageOf(component)]
	if !state.installed {
		return core.NewOperationError("launcher_candidate", "The requested launcher package is not installed", packageOf(component), false)
	}
	if !state.enabled {
		return core.NewOperationError("launcher_candidate", "The requested launcher package is not enabled", packageOf(component), false)
	}
	return nil
}

// Preflight reports the read-only capability picture. It never mutates a device;
// any incomplete or ambiguous probe leaves Supported=false.
func (s *Service) Preflight(ctx context.Context, serial string, identity Identity) (Preflight, error) {
	pinned, err := validateSerial(serial)
	if err != nil {
		return Preflight{}, err
	}
	if strings.TrimSpace(identity.Serial) != "" {
		other, err := validateSerial(identity.Serial)
		if err != nil || other != pinned {
			return Preflight{}, core.NewOperationError("launcher_preflight", "Device identity does not match the confirmed target", identity.Serial, false)
		}
	}
	identity.Serial = pinned

	ins := s.bind(pinned).inspect(ctx)
	report := Preflight{
		Identity:       identity,
		UserID:         androidUserID,
		Capability:     ins.capability,
		HomeCandidates: ins.candidateViews(),
	}
	if ins.current != "" {
		report.CurrentHome = ins.current
		report.ManualCommand = ManualSetHomeCommand(s.toolName(), pinned, androidUserID, ins.current, runtime.GOOS)
	}
	// Recovery browsing is local and read-only. A failure must never be rendered as
	// "nothing to recover": keep it visible, keep TestCandidate/Apply blocked, and
	// report the reason instead of an empty list.
	recoveryProblem := ""
	if recovery, recoveryErr := s.ReadRecovery(pinned); recoveryErr != nil {
		recoveryProblem = "recovery records could not be read: " + recoveryErr.Error()
	} else {
		report.Recovery = recovery.Records
		for _, entry := range recovery.Records {
			if entry.Unreadable {
				recoveryProblem = "recovery records include an unreadable file (" + entry.File + ")"
				break
			}
		}
	}
	// A candidate package with several HOME activities cannot be used as an exact
	// target. It does not block a device whose current HOME is unambiguous, so it is
	// reported read-only and refused by Apply/TestCandidate when actually chosen.
	ambiguousCandidates := make([]string, 0)
	reportedPackages := make(map[string]bool, len(ins.components))
	for _, component := range ins.components {
		pkg := packageOf(component)
		if reportedPackages[pkg] {
			continue
		}
		reportedPackages[pkg] = true
		if count := ins.homeCount(component); count > 1 {
			ambiguousCandidates = append(ambiguousCandidates, fmt.Sprintf("%s (%d HOME activities)", pkg, count))
		}
	}

	probeNotes := make([]string, 0, len(ins.problems)+2)
	probeNotes = append(probeNotes, ins.problems...)
	if recoveryProblem != "" {
		probeNotes = append(probeNotes, recoveryProblem)
	}
	if len(ambiguousCandidates) > 0 {
		probeNotes = append(probeNotes, "packages with several HOME activities are refused as targets: "+strings.Join(ambiguousCandidates, ", "))
	}
	report.ProbeDetail = strings.Join(probeNotes, "; ")

	switch {
	case recoveryProblem != "":
		report.Reason = "Launcher recovery state is not trustworthy: " + recoveryProblem + "; review the launcher directory before changing HOME"
	case ins.capability.Status == CapabilityUnknown:
		report.Reason = "The supported HOME commands could not be probed: " + ins.capability.Detail
	case ins.capability.Status != CapabilitySupported:
		report.Reason = ins.capability.Detail
	case len(ins.problems) > 0:
		report.Reason = "Device probing is incomplete: " + strings.Join(ins.problems, "; ")
	case ins.current == "":
		report.Reason = "The current HOME component is unknown or ambiguous"
	case isChooserOrResolver(ins.current):
		report.Reason = "The current HOME is a platform resolver/chooser, not a reversible launcher"
	case ins.homeCount(ins.current) > 1:
		report.Reason = fmt.Sprintf("The current HOME package %s declares %d HOME activities: the supported command selects HOME per package, so exact restoration cannot be proven",
			packageOf(ins.current), ins.homeCount(ins.current))
	default:
		report.Restorable = ins.restorable()
		if report.Restorable {
			report.Supported = true
		} else {
			report.Reason = "The current HOME is not restorable by the supported command"
		}
	}
	return report, nil
}

// TestCandidate is the explicit, separate launch test. It opens the candidate for a
// look and never changes the default HOME.
func (s *Service) TestCandidate(ctx context.Context, expectedSerial string, candidate string) (CandidateTestResult, error) {
	serial, err := validateSerial(expectedSerial)
	if err != nil {
		return CandidateTestResult{}, err
	}
	component, err := normalizeComponent(candidate)
	if err != nil {
		return CandidateTestResult{}, err
	}

	s.operationMu.Lock()
	defer s.operationMu.Unlock()

	tgt := s.bind(serial)
	ins := tgt.inspect(ctx)
	if ins.capability.Status != CapabilitySupported {
		detail := ins.capability.Detail
		if detail == "" {
			detail = strings.Join(ins.problems, "; ")
		}
		return CandidateTestResult{}, core.NewOperationError("launcher_candidate", "The supported HOME commands are not confirmed on this device", detail, false)
	}
	if err := ins.candidateFor(component); err != nil {
		return CandidateTestResult{}, err
	}
	resolved, err := tgt.resolveComponent(ctx, component)
	if err != nil {
		return CandidateTestResult{}, err
	}
	if resolved != component {
		return CandidateTestResult{}, core.NewOperationError("launcher_candidate", "The device resolved a different HOME activity", resolved, false)
	}
	launched, detail, err := tgt.launchComponent(ctx, component)
	if err != nil {
		return CandidateTestResult{}, err
	}
	return CandidateTestResult{
		Component: component,
		Resolved:  true,
		Launched:  launched,
		Detail:    detail,
		Note:      "This opens the app so it can be inspected; it does not change the default HOME.",
	}, nil
}

// launchComponent opens one exact component through the activity manager.
func (t target) launchComponent(ctx context.Context, component string) (bool, string, error) {
	result, err := t.exec(ctx, launchTimeout, "am", "start", "-n", component)
	if err != nil {
		return false, "", core.NewOperationError("launcher_candidate", "Failed to run the launch test", err.Error(), true)
	}
	if result == nil {
		return false, "", core.NewOperationError("launcher_candidate", "The launch test returned no result", "", true)
	}
	if ctx.Err() != nil {
		return false, "", ctx.Err()
	}
	launched, detail := interpretLaunch(result.Stdout, result.Stderr)
	return launched, detail, nil
}

// launchHome starts the HOME intent and requires a meaningful result.
func (t target) launchHome(ctx context.Context) (bool, string, error) {
	args := append([]string{"am", "start"}, homeIntent...)
	result, err := t.exec(ctx, launchTimeout, args...)
	if err != nil {
		return false, "", core.NewOperationError("launcher_verify", "Failed to run the HOME launch check", err.Error(), true)
	}
	if result == nil {
		return false, "", core.NewOperationError("launcher_verify", "The HOME launch check returned no result", "", true)
	}
	if ctx.Err() != nil {
		return false, "", ctx.Err()
	}
	launched, detail := interpretLaunch(result.Stdout, result.Stderr)
	return launched, detail, nil
}

// verifyHome re-reads HOME and requires the requested component.
func (t target) verifyHome(ctx context.Context, component string) (bool, string, error) {
	current, err := t.currentHome(ctx)
	if err != nil {
		return false, "", err
	}
	if current != component {
		return false, fmt.Sprintf("resolved HOME is %s", current), nil
	}
	return true, "resolved HOME is " + current, nil
}

// interpretLaunch decides from the command output; an exit code alone is never
// enough to claim that HOME works.
func interpretLaunch(stdout, stderr string) (bool, string) {
	output := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))
	if output == "" {
		return false, "the launch command returned no output"
	}
	for _, line := range nonEmptyLines(output) {
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "error") || strings.Contains(lower, "unable to resolve") {
			return false, line
		}
	}
	lowerOutput := strings.ToLower(output)
	if strings.Contains(output, "Starting:") || strings.Contains(lowerOutput, "activity not started") {
		return true, firstNonEmptyLine(output)
	}
	return false, "unrecognized launch output: " + firstNonEmptyLine(output)
}

// beginOperation registers the owned operation identity before any device work so
// the client can cancel while the synchronous call is still pending.
func (s *Service) beginOperation(parent context.Context, serial, operationID string) (context.Context, func(), error) {
	s.stateMu.Lock()
	if s.opID != "" {
		existing := s.opID
		s.stateMu.Unlock()
		return nil, nil, core.NewOperationError("launcher_operation", "Another launcher operation is still pending", existing, false)
	}
	ctx, cancel := context.WithCancel(parent)
	s.opID, s.opSerial, s.opCancel = operationID, serial, cancel
	s.stateMu.Unlock()

	end := func() {
		s.stateMu.Lock()
		if s.opID == operationID {
			s.opID, s.opSerial, s.opCancel = "", "", nil
		}
		s.stateMu.Unlock()
		cancel()
	}
	return ctx, end, nil
}

// Cancel signals an owned operation. It matches BOTH the explicit serial and the
// operation ID, never follows the current global selection, never touches the
// operation mutex and never waits for device work.
func (s *Service) Cancel(request CancelRequest) (CancelResult, error) {
	operationID, err := validateOperationID(request.OperationID)
	if err != nil {
		return CancelResult{}, err
	}
	serial, err := validateSerial(request.ExpectedSerial)
	if err != nil {
		return CancelResult{}, err
	}

	s.stateMu.Lock()
	ownedID, ownedSerial, cancelFn := s.opID, s.opSerial, s.opCancel
	s.stateMu.Unlock()

	if ownedID == "" || ownedID != operationID || ownedSerial != serial {
		return CancelResult{}, core.NewOperationError("launcher_cancel", "No owned launcher operation matches this cancellation", fmt.Sprintf("operation %s", operationID), false)
	}
	if cancelFn != nil {
		cancelFn()
	}
	return CancelResult{OperationID: operationID, Serial: serial, Cancelled: true}, nil
}

// ReadRecovery is a local, read-only listing that stays usable with no device.
func (s *Service) ReadRecovery(expectedSerial string) (Recovery, error) {
	serial := strings.TrimSpace(expectedSerial)
	if serial != "" {
		if _, err := validateSerial(serial); err != nil {
			return Recovery{}, err
		}
	}

	// A regular file where the record directory belongs cannot hold records. Report
	// that state instead of degrading into "no records" (some platforms map the
	// resulting read error to "not exist", which would look like an empty recovery).
	if info, statErr := os.Stat(s.store().dir); statErr == nil && !info.IsDir() {
		return Recovery{}, core.NewOperationError("launcher_record", "Launcher record path is not a directory", s.store().dir, false)
	}

	records, unreadable, err := s.store().list()
	if err != nil {
		return Recovery{}, err
	}

	tool := s.toolName()
	summaries := make([]RecordSummary, 0, len(records)+len(unreadable))
	for _, record := range sortedRecords(records) {
		if serial != "" && record.Serial != serial {
			continue
		}
		summaries = append(summaries, recordSummary(record, tool))
	}
	for _, name := range unreadable {
		summaries = append(summaries, RecordSummary{Unreadable: true, File: name})
	}
	return Recovery{
		Records: summaries,
		Note:    "Recovery browsing is local and read-only: no device command is sent.",
	}, nil
}

func recordSummary(record Record, tool string) RecordSummary {
	return RecordSummary{
		ID:                 record.ID,
		Serial:             record.Serial,
		Action:             record.Action,
		State:              record.State,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
		OriginalComponent:  record.OriginalComponent,
		CandidateComponent: record.CandidateComponent,
		CurrentComponent:   record.CurrentComponent,
		LastError:          record.LastError,
		NeedsRecovery:      record.State.needsRecovery(),
		BlocksApply:        record.State.blocksNewApply(),
		ManualCommand:      record.manualCommand(tool, runtime.GOOS),
	}
}

func sortedRecords(records []Record) []Record {
	sorted := append([]Record{}, records...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].UpdatedAt != sorted[j].UpdatedAt {
			return sorted[i].UpdatedAt > sorted[j].UpdatedAt
		}
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ensureNoBlockingRecord refuses a new HOME change while an unresolved record for
// the same serial exists. Unreadable records block as well: they could hide one.
func (s *Service) ensureNoBlockingRecord(serial string) error {
	records, unreadable, err := s.store().list()
	if err != nil {
		return err
	}
	if len(unreadable) > 0 {
		return core.NewOperationError("launcher_apply", "Launcher records cannot be read", strings.Join(unreadable, ", ")+"; review the launcher directory before changing HOME", false)
	}
	for _, record := range records {
		if record.Serial == serial && record.State.blocksNewApply() {
			return core.NewOperationError("launcher_apply", "An unresolved launcher operation must be reviewed first", fmt.Sprintf("record %s is %s", record.ID, record.State), false)
		}
	}
	return nil
}

// Apply performs the confirmed HOME replacement. It persists the durable intent
// before any mutation and leaves the record in a truthful state afterwards.
func (s *Service) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	operationID, err := validateOperationID(request.OperationID)
	if err != nil {
		return ApplyResult{}, err
	}
	serial, err := validateSerial(request.ExpectedSerial)
	if err != nil {
		return ApplyResult{}, err
	}
	expected, err := normalizeComponent(request.ExpectedComponent)
	if err != nil {
		return ApplyResult{}, err
	}
	candidate, err := normalizeComponent(request.CandidateComponent)
	if err != nil {
		return ApplyResult{}, err
	}
	if candidate == expected {
		return ApplyResult{}, core.NewOperationError("launcher_apply", "The requested launcher is already the current HOME", candidate, false)
	}

	opCtx, endOperation, err := s.beginOperation(ctx, serial, operationID)
	if err != nil {
		return ApplyResult{}, err
	}
	defer endOperation()

	s.operationMu.Lock()
	defer s.operationMu.Unlock()

	if err := opCtx.Err(); err != nil {
		return ApplyResult{}, core.NewOperationError("launcher_apply", "Launcher change cancelled before any device change", "", false)
	}
	if err := s.ensureNoBlockingRecord(serial); err != nil {
		return ApplyResult{}, err
	}

	tgt := s.bind(serial)
	ins := tgt.inspect(opCtx)
	if err := applyPreconditions(ins, expected, candidate); err != nil {
		return ApplyResult{}, err
	}
	if err := opCtx.Err(); err != nil {
		return ApplyResult{}, core.NewOperationError("launcher_apply", "Launcher change cancelled before any device change", "", false)
	}

	record := Record{
		SchemaVersion:      SchemaVersion,
		ID:                 operationID,
		Serial:             serial,
		UserID:             androidUserID,
		Action:             ActionSetHome,
		State:              StatePending,
		CreatedAt:          s.now(),
		UpdatedAt:          s.now(),
		OriginalComponent:  expected,
		CandidateComponent: candidate,
		ToolPath:           tgt.tool,
	}
	// The durable intent must exist before the device can change.
	if err := s.store().saveNew(record); err != nil {
		return ApplyResult{}, err
	}

	if err := opCtx.Err(); err != nil {
		record.State = StatePlanned
		record.LastError = "cancelled before any device change"
		record.UpdatedAt = s.now()
		_ = s.store().persist(record)
		return ApplyResult{
			OperationID:        operationID,
			Serial:             serial,
			State:              StatePlanned,
			OriginalComponent:  expected,
			CandidateComponent: candidate,
			ManualCommand:      record.manualCommand(tgt.tool, runtime.GOOS),
			Detail:             "cancelled before any device change",
		}, core.NewOperationError("launcher_apply", "Launcher change cancelled before any device change", "", false)
	}

	if _, err := tgt.mutate(opCtx, setHomeArgs(candidate)...); err != nil {
		return s.recoverAfterFailure(tgt, record, "the HOME command failed: "+err.Error())
	}
	if err := opCtx.Err(); err != nil {
		return s.recoverAfterFailure(tgt, record, "the launcher change was cancelled after the HOME command")
	}
	if verified, detail, err := tgt.verifyHome(opCtx, candidate); err != nil || !verified {
		reason := "HOME verification failed: " + detail
		if err != nil {
			reason = "HOME verification failed: " + err.Error()
		}
		return s.recoverAfterFailure(tgt, record, reason)
	}
	if launched, detail, err := tgt.launchHome(opCtx); err != nil || !launched {
		reason := "the HOME launch check failed: " + detail
		if err != nil {
			reason = "the HOME launch check failed: " + err.Error()
		}
		return s.recoverAfterFailure(tgt, record, reason)
	}
	if err := opCtx.Err(); err != nil {
		return s.recoverAfterFailure(tgt, record, "the launcher change was cancelled before it could be confirmed")
	}

	record.State = StateApplied
	record.CurrentComponent = candidate
	record.LastError = ""
	record.UpdatedAt = s.now()
	if err := s.store().persist(record); err != nil {
		return ApplyResult{
			OperationID:        operationID,
			Serial:             serial,
			State:              StatePending,
			OriginalComponent:  expected,
			CandidateComponent: candidate,
			CurrentComponent:   candidate,
			Verified:           true,
			ManualCommand:      record.manualCommand(tgt.tool, runtime.GOOS),
			Detail:             "HOME replaced and verified, but the durable record could not be updated",
		}, core.NewOperationError("launcher_apply", "HOME was replaced and verified but the record could not be updated", err.Error(), true)
	}

	return ApplyResult{
		OperationID:        operationID,
		Serial:             serial,
		State:              StateApplied,
		OriginalComponent:  expected,
		CandidateComponent: candidate,
		CurrentComponent:   candidate,
		Verified:           true,
		ManualCommand:      record.manualCommand(tgt.tool, runtime.GOOS),
		Detail:             "HOME replaced and verified",
	}, nil
}

// setHomeArgs is the only supported HOME write path.
func setHomeArgs(component string) []string {
	return []string{"cmd", "package", "set-home-activity", "--user", "0", core.QuoteShellArg(component)}
}

// applyPreconditions encodes the fail-closed rules for a reversible apply.
func applyPreconditions(ins inspection, expected, candidate string) error {
	if len(ins.problems) > 0 {
		return core.NewOperationError("launcher_apply", "Device probing is incomplete", strings.Join(ins.problems, "; "), false)
	}
	if ins.capability.Status != CapabilitySupported {
		return core.NewOperationError("launcher_apply", "The supported HOME commands are not confirmed on this device", ins.capability.Detail, false)
	}
	if ins.current == "" {
		return core.NewOperationError("launcher_apply", "The current HOME component is unknown or ambiguous", "", false)
	}
	if ins.current != expected {
		return core.NewOperationError("launcher_apply", "The current HOME changed; inspect the device again", fmt.Sprintf("expected %s, current %s", expected, ins.current), false)
	}
	if isChooserOrResolver(ins.current) {
		return core.NewOperationError("launcher_apply", "The current HOME is a platform resolver/chooser", ins.current, false)
	}
	if count := ins.homeCount(ins.current); count != 1 {
		return core.NewOperationError("launcher_apply", "The current HOME package declares several HOME activities",
			fmt.Sprintf("%s declares %d HOME activities; the supported command selects HOME per package, so exact restoration cannot be proven and no change was made", packageOf(ins.current), count), false)
	}
	if !ins.restorable() {
		return core.NewOperationError("launcher_apply", "The current HOME is not restorable by the supported command", ins.current, false)
	}
	return ins.candidateFor(candidate)
}

// recoverAfterFailure keeps the device recoverable: it uses a fresh bounded cleanup
// context with the same pinned target, never follows another device, and refuses to
// bulldoze a HOME that moved to an unrelated component.
func (s *Service) recoverAfterFailure(tgt target, record Record, reason string) (ApplyResult, error) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	current, currentErr := tgt.currentHome(cleanupCtx)
	switch {
	case currentErr != nil:
		return s.markUnknown(tgt, record, reason+"; the current HOME could not be read: "+currentErr.Error())
	case current == record.OriginalComponent:
		record.State = StateRestored
		record.CurrentComponent = current
		record.LastError = reason
		record.UpdatedAt = s.now()
		_ = s.store().persist(record)
		return ApplyResult{
			OperationID:        record.ID,
			Serial:             record.Serial,
			State:              StateRestored,
			OriginalComponent:  record.OriginalComponent,
			CandidateComponent: record.CandidateComponent,
			CurrentComponent:   current,
			RolledBack:         true,
			ManualCommand:      record.manualCommand(tgt.tool, runtime.GOOS),
			Detail:             reason + "; the original HOME was still current, so no change remained",
		}, core.NewOperationError("launcher_apply", "Launcher change did not take effect", reason+"; the original HOME is still current", true)
	case current != record.CandidateComponent:
		return s.markUnknown(tgt, record, fmt.Sprintf("%s; HOME is now %s, which is neither the original nor the candidate", reason, current))
	}

	record.State = StateRestorePending
	record.CurrentComponent = current
	record.LastError = reason
	record.UpdatedAt = s.now()
	if err := s.store().persist(record); err != nil {
		return s.markUnknown(tgt, record, reason+"; the rollback intent could not be persisted: "+err.Error())
	}
	if _, err := tgt.mutate(cleanupCtx, setHomeArgs(record.OriginalComponent)...); err != nil {
		return s.markUnknown(tgt, record, reason+"; restoring the original HOME failed: "+err.Error())
	}
	if verified, detail, err := tgt.verifyHome(cleanupCtx, record.OriginalComponent); err != nil || !verified {
		if err != nil {
			detail = err.Error()
		}
		return s.markUnknown(tgt, record, reason+"; the restored HOME could not be verified: "+detail)
	}
	if launched, detail, err := tgt.launchHome(cleanupCtx); err != nil || !launched {
		if err != nil {
			detail = err.Error()
		}
		return s.markUnknown(tgt, record, reason+"; the restored HOME did not launch: "+detail)
	}

	record.State = StateRestored
	record.CurrentComponent = record.OriginalComponent
	record.UpdatedAt = s.now()
	if err := s.store().persist(record); err != nil {
		return s.markUnknown(tgt, record, reason+"; the restored state could not be persisted: "+err.Error())
	}
	return ApplyResult{
		OperationID:        record.ID,
		Serial:             record.Serial,
		State:              StateRestored,
		OriginalComponent:  record.OriginalComponent,
		CandidateComponent: record.CandidateComponent,
		CurrentComponent:   record.OriginalComponent,
		RolledBack:         true,
		ManualCommand:      record.manualCommand(tgt.tool, runtime.GOOS),
		Detail:             reason + "; the original HOME was restored and verified",
	}, core.NewOperationError("launcher_apply", "Launcher change failed and the original HOME was restored", reason, true)
}

func (s *Service) markUnknown(tgt target, record Record, reason string) (ApplyResult, error) {
	record.State = StateUnknown
	record.LastError = reason
	record.UpdatedAt = s.now()
	_ = s.store().persist(record)
	manual := record.manualCommand(tgt.tool, runtime.GOOS)
	return ApplyResult{
		OperationID:        record.ID,
		Serial:             record.Serial,
		State:              StateUnknown,
		OriginalComponent:  record.OriginalComponent,
		CandidateComponent: record.CandidateComponent,
		CurrentComponent:   record.CurrentComponent,
		ManualCommand:      manual,
		Detail:             reason + "; manual recovery: " + manual,
	}, core.NewOperationError("launcher_recovery", "Launcher recovery is required", reason+"; manual recovery: "+manual, false)
}

// Restore applies the recorded original HOME after an explicit live admission.
func (s *Service) Restore(ctx context.Context, request RestoreRequest) (RestoreResult, error) {
	serial, err := validateSerial(request.ExpectedSerial)
	if err != nil {
		return RestoreResult{}, err
	}
	recordID, err := validateOperationID(request.RecordID)
	if err != nil {
		return RestoreResult{}, err
	}

	s.operationMu.Lock()
	defer s.operationMu.Unlock()

	record, err := s.store().load(recordID)
	if err != nil {
		return RestoreResult{}, err
	}
	if record.Serial != serial {
		return RestoreResult{}, core.NewOperationError("launcher_restore", "The record belongs to a different device", record.Serial, false)
	}

	tgt := s.bind(serial)
	manual := record.manualCommand(tgt.tool, runtime.GOOS)
	if !record.State.needsRecovery() {
		return RestoreResult{
			RecordID:          record.ID,
			Serial:            serial,
			State:             record.State,
			RestoredComponent: record.OriginalComponent,
			CurrentComponent:  record.CurrentComponent,
			ManualCommand:     manual,
			Detail:            "this record needs no recovery",
		}, nil
	}

	// The restore path must prove exact restoration before it touches the device:
	// the supported command acts on a PACKAGE, so a package with several HOME
	// activities cannot be restored to one exact component.
	components, err := tgt.homeComponents(ctx)
	if err != nil {
		return RestoreResult{}, core.NewOperationError("launcher_restore", "The HOME activity list could not be read", err.Error(), true)
	}
	if count := countHomeForPackage(components, record.OriginalComponent); count != 1 {
		return RestoreResult{
				RecordID:          record.ID,
				Serial:            serial,
				State:             record.State,
				RestoredComponent: record.OriginalComponent,
				CurrentComponent:  record.CurrentComponent,
				ManualCommand:     manual,
				Detail:            "the original HOME package declares several HOME activities; no device change was made",
			}, core.NewOperationError("launcher_restore", "The original HOME package declares several HOME activities",
				fmt.Sprintf("%s declares %d HOME activities; the supported command selects HOME per package, so exact restoration cannot be proven and no device change was made; manual recovery: %s", packageOf(record.OriginalComponent), count, manual), false)
	}
	if count := countHomeForPackage(components, record.CandidateComponent); count != 1 {
		return RestoreResult{
				RecordID:          record.ID,
				Serial:            serial,
				State:             record.State,
				RestoredComponent: record.OriginalComponent,
				CurrentComponent:  record.CurrentComponent,
				ManualCommand:     manual,
				Detail:            "the recorded candidate package declares several HOME activities; no device change was made",
			}, core.NewOperationError("launcher_restore", "The recorded candidate package declares several HOME activities",
				fmt.Sprintf("%s declares %d HOME activities; this record predates the single-HOME guard, so its candidate identity is not proven; no device change was made; manual recovery: %s", packageOf(record.CandidateComponent), count, manual), false)
	}

	current, err := tgt.currentHome(ctx)
	if err != nil {
		return RestoreResult{}, core.NewOperationError("launcher_restore", "The current HOME could not be read", err.Error(), true)
	}
	if current == record.OriginalComponent {
		record.State = StateRestored
		record.CurrentComponent = current
		record.UpdatedAt = s.now()
		if err := s.store().persist(record); err != nil {
			return RestoreResult{}, err
		}
		return RestoreResult{
			RecordID:          record.ID,
			Serial:            serial,
			State:             StateRestored,
			RestoredComponent: record.OriginalComponent,
			CurrentComponent:  current,
			Verified:          true,
			ManualCommand:     manual,
			Detail:            "the original HOME is already current",
		}, nil
	}
	if current != record.CandidateComponent {
		record.State = StateUnknown
		record.CurrentComponent = current
		record.LastError = "current HOME is neither the original nor the candidate"
		record.UpdatedAt = s.now()
		_ = s.store().persist(record)
		return RestoreResult{
			RecordID:          record.ID,
			Serial:            serial,
			State:             StateUnknown,
			RestoredComponent: record.OriginalComponent,
			CurrentComponent:  current,
			ManualCommand:     manual,
			Detail:            "current HOME is neither the original nor the candidate; review the device before recovery; manual recovery: " + manual,
		}, core.NewOperationError("launcher_restore", "The current HOME moved elsewhere", "current "+current+"; manual recovery: "+manual, false)
	}

	record.State = StateRestorePending
	record.CurrentComponent = current
	record.UpdatedAt = s.now()
	if err := s.store().persist(record); err != nil {
		return RestoreResult{}, err
	}
	if _, err := tgt.mutate(ctx, setHomeArgs(record.OriginalComponent)...); err != nil {
		return s.restoreFailed(tgt, record, "restoring the original HOME failed: "+err.Error())
	}
	if verified, detail, err := tgt.verifyHome(ctx, record.OriginalComponent); err != nil || !verified {
		if err != nil {
			detail = err.Error()
		}
		return s.restoreFailed(tgt, record, "the restored HOME could not be verified: "+detail)
	}
	if launched, detail, err := tgt.launchHome(ctx); err != nil || !launched {
		if err != nil {
			detail = err.Error()
		}
		return s.restoreFailed(tgt, record, "the restored HOME did not launch: "+detail)
	}

	record.State = StateRestored
	record.CurrentComponent = record.OriginalComponent
	record.LastError = ""
	record.UpdatedAt = s.now()
	if err := s.store().persist(record); err != nil {
		return RestoreResult{}, err
	}
	return RestoreResult{
		RecordID:          record.ID,
		Serial:            serial,
		State:             StateRestored,
		RestoredComponent: record.OriginalComponent,
		CurrentComponent:  record.OriginalComponent,
		Verified:          true,
		ManualCommand:     manual,
		Detail:            "the original HOME was restored and verified",
	}, nil
}

func (s *Service) restoreFailed(tgt target, record Record, reason string) (RestoreResult, error) {
	record.State = StateUnknown
	record.LastError = reason
	record.UpdatedAt = s.now()
	_ = s.store().persist(record)
	manual := record.manualCommand(tgt.tool, runtime.GOOS)
	return RestoreResult{
		RecordID:          record.ID,
		Serial:            record.Serial,
		State:             StateUnknown,
		RestoredComponent: record.OriginalComponent,
		CurrentComponent:  record.CurrentComponent,
		ManualCommand:     manual,
		Detail:            reason + "; manual recovery: " + manual,
	}, core.NewOperationError("launcher_recovery", "Launcher recovery is required", reason+"; manual recovery: "+manual, false)
}

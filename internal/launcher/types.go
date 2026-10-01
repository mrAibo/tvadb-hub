// Package launcher implements the guarded TV custom-launcher backend: a read-only
// capability preflight, an explicit candidate launch test, a confirmed HOME
// replacement with a durable intent record, bounded cancellation, safe rollback
// and offline-capable manual recovery.
//
// Boundaries this package keeps: official adb CLI only, exact components instead
// of packages/roles, no SDK≥29 assumption, no stock-launcher disable/uninstall,
// no execution of a tool path taken from a stored record, and no device mutation
// from a read-only path.
package launcher

import (
	"ADBKit/internal/core"
	"strings"
)

// SchemaVersion is the only durable record schema this package writes or accepts.
const SchemaVersion = 1

// Action is the launcher-specific operation vocabulary. It deliberately does not
// reuse the Safe Tuning disable/uninstall action enum.
type Action string

const (
	ActionSetHome     Action = "set-home"
	ActionRestoreHome Action = "restore-home"
)

// State is the durable record lifecycle.
type State string

const (
	// StatePlanned means an intent was recorded and no device change was attempted.
	StatePlanned State = "planned"
	// StatePending means the mutation may have started; the outcome is unresolved.
	StatePending State = "pending"
	// StateApplied means the requested HOME change was applied and verified.
	StateApplied State = "applied"
	// StateUnknown means the device state is uncertain and must be reviewed.
	StateUnknown State = "unknown"
	// StateRestorePending means a rollback was requested and is unconfirmed.
	StateRestorePending State = "restore-pending"
	// StateRestored means the original HOME was restored and verified.
	StateRestored State = "restored"
)

// needsRecovery reports whether the record still describes something the user may
// want to restore.
func (s State) needsRecovery() bool {
	switch s {
	case StatePending, StateApplied, StateUnknown, StateRestorePending:
		return true
	default:
		return false
	}
}

// blocksNewApply reports whether an unresolved outcome must be reviewed before
// another HOME change is allowed.
func (s State) blocksNewApply() bool {
	switch s {
	case StatePending, StateUnknown, StateRestorePending:
		return true
	default:
		return false
	}
}

// Identity is the read-only device summary a preflight reports. The app fills it
// from the existing device service; the launcher package never reads props itself.
type Identity struct {
	Serial         string `json:"serial"`
	Model          string `json:"model,omitempty"`
	Manufacturer   string `json:"manufacturer,omitempty"`
	Codename       string `json:"codename,omitempty"`
	AndroidVersion string `json:"androidVersion,omitempty"`
	SDKVersion     string `json:"sdkVersion,omitempty"`
	IsTV           bool   `json:"isTV,omitempty"`
}

// CapabilityStatus is the outcome of the live capability probe. Unknown must never
// be treated as supported.
type CapabilityStatus string

const (
	CapabilitySupported   CapabilityStatus = "supported"
	CapabilityUnsupported CapabilityStatus = "unsupported"
	CapabilityUnknown     CapabilityStatus = "unknown"
)

// Capability records which supported commands the device actually advertises.
type Capability struct {
	Status          CapabilityStatus `json:"status"`
	SetHomeActivity bool             `json:"setHomeActivity"`
	ResolveActivity bool             `json:"resolveActivity"`
	Detail          string           `json:"detail,omitempty"`
}

// HomeCandidate is one resolvable HOME activity with its live package state.
type HomeCandidate struct {
	Component string `json:"component"`
	Package   string `json:"package"`
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Selected  bool   `json:"selected"`
	Chooser   bool   `json:"chooser,omitempty"`
}

// Record is the durable intent of one launcher operation. It stores exact
// components, never packages or roles.
type Record struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	Serial        string `json:"serial"`
	UserID        int    `json:"userId"`
	Action        Action `json:"action"`
	State         State  `json:"state"`

	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`

	OriginalComponent  string `json:"originalComponent"`
	CandidateComponent string `json:"candidateComponent"`
	CurrentComponent   string `json:"currentComponent,omitempty"`

	// ToolPath is display/forensic data for the manual command. It is never used
	// to execute anything: every command uses the live tool configuration.
	ToolPath  string `json:"toolPath,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

// RecordSummary is the read-only view of a record for the recovery surface.
type RecordSummary struct {
	ID                 string `json:"id"`
	Serial             string `json:"serial"`
	Action             Action `json:"action"`
	State              State  `json:"state"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	OriginalComponent  string `json:"originalComponent"`
	CandidateComponent string `json:"candidateComponent"`
	CurrentComponent   string `json:"currentComponent,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	NeedsRecovery      bool   `json:"needsRecovery"`
	BlocksApply        bool   `json:"blocksApply"`
	ManualCommand      string `json:"manualCommand"`
	Unreadable         bool   `json:"unreadable,omitempty"`
	File               string `json:"file,omitempty"`
}

// Preflight is the read-only report. Supported is true only when every condition
// for a reversible HOME change was confirmed on the live device.
type Preflight struct {
	Identity       Identity        `json:"identity"`
	UserID         int             `json:"userId"`
	Capability     Capability      `json:"capability"`
	Supported      bool            `json:"supported"`
	Reason         string          `json:"reason,omitempty"`
	CurrentHome    string          `json:"currentHome,omitempty"`
	Restorable     bool            `json:"restorable"`
	HomeCandidates []HomeCandidate `json:"homeCandidates"`
	Recovery       []RecordSummary `json:"recovery"`
	ManualCommand  string          `json:"manualCommand,omitempty"`
	ProbeDetail    string          `json:"probeDetail,omitempty"`
}

// CandidateTestResult is the outcome of the explicit candidate launch test.
type CandidateTestResult struct {
	Component string `json:"component"`
	Resolved  bool   `json:"resolved"`
	Launched  bool   `json:"launched"`
	Detail    string `json:"detail,omitempty"`
	Note      string `json:"note"`
}

// ApplyRequest asks for a confirmed HOME replacement. OperationID is generated by
// the client so it can cancel while the synchronous call is still running.
type ApplyRequest struct {
	OperationID        string `json:"operationId"`
	ExpectedSerial     string `json:"expectedSerial"`
	ExpectedComponent  string `json:"expectedComponent"`
	CandidateComponent string `json:"candidateComponent"`
}

// ApplyResult reports the truthful outcome of a confirmed apply.
type ApplyResult struct {
	OperationID        string `json:"operationId"`
	Serial             string `json:"serial"`
	State              State  `json:"state"`
	OriginalComponent  string `json:"originalComponent"`
	CandidateComponent string `json:"candidateComponent"`
	CurrentComponent   string `json:"currentComponent,omitempty"`
	Verified           bool   `json:"verified"`
	RolledBack         bool   `json:"rolledBack"`
	ManualCommand      string `json:"manualCommand,omitempty"`
	Detail             string `json:"detail,omitempty"`
}

// RestoreRequest asks to restore the original HOME of one recorded operation.
type RestoreRequest struct {
	OperationID    string `json:"operationId,omitempty"`
	ExpectedSerial string `json:"expectedSerial"`
	RecordID       string `json:"recordId"`
}

// RestoreResult reports the truthful outcome of a restore.
type RestoreResult struct {
	RecordID          string `json:"recordId"`
	Serial            string `json:"serial"`
	State             State  `json:"state"`
	RestoredComponent string `json:"restoredComponent"`
	CurrentComponent  string `json:"currentComponent,omitempty"`
	Verified          bool   `json:"verified"`
	ManualCommand     string `json:"manualCommand,omitempty"`
	Detail            string `json:"detail,omitempty"`
}

// CancelRequest cancels an owned operation by explicit serial and operation ID.
type CancelRequest struct {
	OperationID    string `json:"operationId"`
	ExpectedSerial string `json:"expectedSerial"`
}

// CancelResult reports whether an owned operation was signalled.
type CancelResult struct {
	OperationID string `json:"operationId"`
	Serial      string `json:"serial"`
	Cancelled   bool   `json:"cancelled"`
}

// Recovery is the local, offline-capable recovery listing.
type Recovery struct {
	Records []RecordSummary `json:"records"`
	Note    string          `json:"note,omitempty"`
}

const (
	minOperationIDLength = 8
	maxOperationIDLength = 64
	maxSerialLength      = 128
)

// validateOperationID accepts the client-generated UUID-ish identifier pattern.
func validateOperationID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if len(trimmed) < minOperationIDLength || len(trimmed) > maxOperationIDLength {
		return "", core.NewOperationError("launcher_operation", "Invalid operation ID", "expected 8 to 64 characters", false)
	}
	for _, r := range trimmed {
		allowed := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !allowed {
			return "", core.NewOperationError("launcher_operation", "Invalid operation ID", trimmed, false)
		}
	}
	return trimmed, nil
}

// validateSerial trims and constrains the confirmed device target.
func validateSerial(serial string) (string, error) {
	trimmed := strings.TrimSpace(serial)
	if trimmed == "" {
		return "", core.NewOperationError("launcher_target", "Confirmed device is required", "select and confirm an ADB device", false)
	}
	if len(trimmed) > maxSerialLength {
		return "", core.NewOperationError("launcher_target", "Confirmed device serial is not supported", trimmed, false)
	}
	for _, r := range trimmed {
		allowed := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == ':' || r == '-' || r == '_'
		if !allowed {
			return "", core.NewOperationError("launcher_target", "Confirmed device serial is not supported", trimmed, false)
		}
	}
	return trimmed, nil
}

// normalizeComponent validates an Android component and returns the canonical
// "package/full.Class" form.
//
// The supported HOME command selects one exact component, so a package-only value
// is refused: a package with several HOME activities would otherwise be an
// ambiguous target whose restoration cannot be proven. Short class names are
// expanded, and `$`-bearing inner-class names are preserved.
func normalizeComponent(component string) (string, error) {
	trimmed := strings.TrimSpace(component)
	if trimmed == "" {
		return "", core.NewOperationError("launcher_component", "HOME component is required", "expected package/Class", false)
	}
	pkg, err := core.AndroidComponentPackage(trimmed)
	if err != nil {
		return "", core.NewOperationError("launcher_component", "Invalid Android component", trimmed, false)
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", core.NewOperationError("launcher_component", "Invalid Android component", trimmed, false)
	}
	class := parts[1]
	if strings.HasPrefix(class, ".") {
		class = pkg + class
	}
	if class == "" || strings.ContainsAny(class, "/ ") {
		return "", core.NewOperationError("launcher_component", "Invalid Android component class", trimmed, false)
	}
	return pkg + "/" + class, nil
}

// packageOf returns the package part of a normalized component.
func packageOf(component string) string {
	parts := strings.SplitN(component, "/", 2)
	return parts[0]
}

// isChooserOrResolver reports whether a component is a platform dispatcher rather
// than a real launcher. Such an original HOME is not a reversible baseline.
func isChooserOrResolver(component string) bool {
	pkg := strings.ToLower(packageOf(component))
	if pkg == "android" || pkg == "com.android.intentresolver" {
		return true
	}
	lower := strings.ToLower(component)
	return strings.Contains(lower, "resolveractivity") || strings.Contains(lower, "chooseractivity") || strings.Contains(lower, "resolver")
}

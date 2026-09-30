package tuning

type Risk string

const (
	RiskSafe      Risk = "safe"
	RiskCaution   Risk = "caution"
	RiskDangerous Risk = "dangerous"
	RiskBlocked   Risk = "blocked"
)

type ActionMode string

const (
	ActionDisable       ActionMode = "disable"
	ActionUninstallUser ActionMode = "uninstall-user"
)

type MatchCriteria struct {
	Manufacturers []string `json:"manufacturers,omitempty"`
	Brands        []string `json:"brands,omitempty"`
	Models        []string `json:"models,omitempty"`
	Codenames     []string `json:"codenames,omitempty"`
	TVOnly        bool     `json:"tvOnly,omitempty"`
	Generic       bool     `json:"generic,omitempty"`
}

type PackageRule struct {
	PackageName     string `json:"packageName"`
	Label           string `json:"label"`
	Category        string `json:"category"`
	Risk            Risk   `json:"risk"`
	Reason          string `json:"reason"`
	DefaultSelected bool   `json:"defaultSelected"`
}

type Profile struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	DeviceFamily  string        `json:"deviceFamily"`
	Description   string        `json:"description"`
	SourceName    string        `json:"sourceName"`
	SourceURL     string        `json:"sourceUrl"`
	SourceLicense string        `json:"sourceLicense"`
	Criteria      MatchCriteria `json:"criteria"`
	Keep          []string      `json:"keep"`
	Rules         []PackageRule `json:"rules"`
}

type ProfileSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DeviceFamily  string `json:"deviceFamily"`
	Description   string `json:"description"`
	SourceName    string `json:"sourceName"`
	SourceURL     string `json:"sourceUrl"`
	SourceLicense string `json:"sourceLicense"`
	MatchScore    int    `json:"matchScore"`
	Recommended   bool   `json:"recommended"`
}

type PackageMatch struct {
	PackageRule
	IsEnabled   bool `json:"isEnabled"`
	IsSystemApp bool `json:"isSystemApp"`
	Protected   bool `json:"protected"`
	Actionable  bool `json:"actionable"`
}

type Analysis struct {
	Serial             string           `json:"serial"`
	HardwareSerial     string           `json:"hardwareSerial,omitempty"`
	Model              string           `json:"model"`
	Manufacturer       string           `json:"manufacturer"`
	AndroidVersion     string           `json:"androidVersion"`
	IsTV               bool             `json:"isTV"`
	SelectedProfile    ProfileSummary   `json:"selectedProfile"`
	AvailableProfiles  []ProfileSummary `json:"availableProfiles"`
	Matches            []PackageMatch   `json:"matches"`
	InstalledCount     int              `json:"installedCount"`
	DefaultSelected    []string         `json:"defaultSelected"`
	ProtectedInstalled []string         `json:"protectedInstalled"`
}

type ApplyRequest struct {
	ExpectedSerial         string     `json:"expectedSerial"`
	ExpectedHardwareSerial string     `json:"expectedHardwareSerial,omitempty"`
	ProfileID              string     `json:"profileId"`
	PackageNames           []string   `json:"packageNames"`
	Mode                   ActionMode `json:"mode"`
	AcknowledgeCaution     bool       `json:"acknowledgeCaution"`
}

type ApplyResult struct {
	SnapshotID string            `json:"snapshotId"`
	Changed    []string          `json:"changed"`
	Skipped    []string          `json:"skipped"`
	Failed     map[string]string `json:"failed"`
}

type SnapshotItem struct {
	PackageName string        `json:"packageName"`
	WasEnabled  bool          `json:"wasEnabled"`
	IsSystemApp bool          `json:"isSystemApp"`
	Risk        Risk          `json:"risk"`
	Action      ActionMode    `json:"action"`
	Applied     bool          `json:"applied"`
	State       JournalState  `json:"state,omitempty"`
	Before      *PackageState `json:"before,omitempty"`
	LastError   string        `json:"lastError,omitempty"`
}

type JournalState string

const (
	JournalPlanned        JournalState = "planned"
	JournalPending        JournalState = "pending"
	JournalApplied        JournalState = "applied"
	JournalUnknown        JournalState = "unknown"
	JournalRestorePending JournalState = "restore-pending"
	JournalRestored       JournalState = "restored"
)

// Enabled preserves Android's exact DEFAULT/ENABLED/DISABLED/DISABLED_USER/
// DISABLED_UNTIL_USED setting, rather than a lossy effective-enabled boolean.
type PackageState struct {
	Installed bool `json:"installed"`
	Enabled   int  `json:"enabled"`
}

type Snapshot struct {
	Version        int            `json:"version,omitempty"`
	UserID         int            `json:"userId"`
	HardwareSerial string         `json:"hardwareSerial,omitempty"`
	ID             string         `json:"id"`
	CreatedAt      string         `json:"createdAt"`
	Serial         string         `json:"serial"`
	ProfileID      string         `json:"profileId"`
	ProfileName    string         `json:"profileName"`
	Mode           ActionMode     `json:"mode"`
	Items          []SnapshotItem `json:"items"`
}

type SnapshotSummary struct {
	Version     int        `json:"version"`
	Recoverable int        `json:"recoverable"`
	Uncertain   int        `json:"uncertain"`
	ID          string     `json:"id"`
	CreatedAt   string     `json:"createdAt"`
	Serial      string     `json:"serial"`
	ProfileID   string     `json:"profileId"`
	ProfileName string     `json:"profileName"`
	Mode        ActionMode `json:"mode"`
	Applied     int        `json:"applied"`
}

type RestoreResult struct {
	SnapshotID string            `json:"snapshotId"`
	Restored   []string          `json:"restored"`
	Failed     map[string]string `json:"failed"`
}

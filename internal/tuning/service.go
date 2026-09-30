package tuning

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	packagemgr "ADBKit/internal/package_mgr"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	dataDir             string
	resolveActiveSerial func(context.Context) (string, error)
	getBinPath          func() core.BinaryPaths
	packages            *packagemgr.Service
	feed                *feedManager
	operationMu         sync.Mutex
	runCommand          func(context.Context, core.ExecRequest) (*core.ExecResult, error)
	writeSnapshot       func(Snapshot) error
}

func NewService(
	dataDir string,
	resolveActiveSerial func(context.Context) (string, error),
	getBinPath func() core.BinaryPaths,
	packages *packagemgr.Service,
) *Service {
	return &Service{
		dataDir: dataDir, resolveActiveSerial: resolveActiveSerial,
		getBinPath: getBinPath, packages: packages, feed: newFeedManager(dataDir),
	}
}

func (s *Service) GetProfiles(info device.Info) []ProfileSummary {
	return profilesForDevice(s.currentProfiles(), info)
}

func (s *Service) Analyze(ctx context.Context, info device.Info, profileID string) (Analysis, error) {
	return s.bound().analyze(ctx, info, profileID)
}

// A fresh operation owns an immutable tool-path snapshot. Do not copy mutexes.
func (s *Service) bound() *Service {
	paths := s.getBinPath()
	return &Service{dataDir: s.dataDir, resolveActiveSerial: s.resolveActiveSerial,
		getBinPath: func() core.BinaryPaths { return paths }, packages: s.packages, feed: s.feed,
		runCommand: s.runCommand, writeSnapshot: s.writeSnapshot}
}

func (s *Service) analyze(ctx context.Context, info device.Info, profileID string) (Analysis, error) {
	serial := strings.TrimSpace(info.Serial)
	if serial == "" {
		return Analysis{}, core.NewOperationError("tuning_analyze", "explicit device target is required", "", false)
	}
	profiles := s.currentProfiles()
	profile, err := resolveProfileFrom(profiles, info, profileID)
	if err != nil {
		return Analysis{}, err
	}
	allProfiles := profilesForDevice(profiles, info)
	selectedSummary := profileSummary(profile, profileMatchScore(profile, info), false)
	for _, summary := range allProfiles {
		if summary.ID == profile.ID {
			selectedSummary = summary
			break
		}
	}

	installed, err := s.packages.ForTarget(serial, 0).ListPackages(ctx, "all")
	if err != nil {
		return Analysis{}, err
	}
	installedMap := make(map[string]packagemgr.Info, len(installed))
	for _, pkg := range installed {
		installedMap[pkg.PackageName] = pkg
	}
	protected, riskFloors := deviceSafetyFloor(info, profile)

	matches := make([]PackageMatch, 0)
	defaultSelected := make([]string, 0)
	protectedInstalled := make([]string, 0)
	for _, rule := range profile.Rules {
		rule = enforceRuleFloor(rule, protected, riskFloors)
		pkg, ok := installedMap[rule.PackageName]
		if !ok {
			continue
		}
		_, isProtected := protected[rule.PackageName]
		actionable := !isProtected && rule.Risk != RiskDangerous && rule.Risk != RiskBlocked
		match := PackageMatch{
			PackageRule: rule, IsEnabled: pkg.IsEnabled, IsSystemApp: pkg.IsSystemApp,
			Protected: isProtected, Actionable: actionable,
		}
		matches = append(matches, match)
		if actionable && rule.DefaultSelected && pkg.IsEnabled {
			defaultSelected = append(defaultSelected, rule.PackageName)
		}
	}
	for pkg := range protected {
		if _, ok := installedMap[pkg]; ok {
			protectedInstalled = append(protectedInstalled, pkg)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Category != matches[j].Category {
			return matches[i].Category < matches[j].Category
		}
		return matches[i].PackageName < matches[j].PackageName
	})
	sort.Strings(protectedInstalled)

	return Analysis{
		Serial: serial, HardwareSerial: info.HardwareSerial, Model: info.Model, Manufacturer: info.Manufacturer,
		AndroidVersion: info.AndroidVersion, IsTV: info.IsTV,
		SelectedProfile: selectedSummary, AvailableProfiles: allProfiles, Matches: matches,
		InstalledCount: len(installed), DefaultSelected: defaultSelected,
		ProtectedInstalled: protectedInstalled,
	}, nil
}

func (s *Service) Apply(ctx context.Context, info device.Info, request ApplyRequest) (ApplyResult, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	if strings.TrimSpace(request.ExpectedSerial) == "" || request.ExpectedSerial != info.Serial {
		return ApplyResult{}, core.NewOperationError("tuning_apply", "device target changed; analyze and confirm again", "", false)
	}
	if request.Mode != ActionDisable && request.Mode != ActionUninstallUser {
		return ApplyResult{}, core.NewOperationError("tuning_apply", "unsupported tuning action", string(request.Mode), false)
	}
	if info.HardwareSerial != request.ExpectedHardwareSerial {
		return ApplyResult{}, core.NewOperationError("tuning_apply", "device identity changed; analyze and confirm again", "", false)
	}
	return s.bound().apply(ctx, info, request)
}

func (s *Service) apply(ctx context.Context, info device.Info, request ApplyRequest) (ApplyResult, error) {
	analysis, err := s.analyze(ctx, info, request.ProfileID)
	if err != nil {
		return ApplyResult{}, err
	}
	if len(request.PackageNames) == 0 {
		return ApplyResult{}, core.NewOperationError("tuning_apply", "no packages selected", "", false)
	}

	matchByName := make(map[string]PackageMatch, len(analysis.Matches))
	for _, match := range analysis.Matches {
		matchByName[match.PackageName] = match
	}

	seen := map[string]struct{}{}
	items := make([]SnapshotItem, 0, len(request.PackageNames))
	skipped := make([]string, 0)
	for _, packageName := range request.PackageNames {
		packageName = strings.TrimSpace(packageName)
		if packageName == "" {
			continue
		}
		if _, duplicate := seen[packageName]; duplicate {
			continue
		}
		seen[packageName] = struct{}{}
		match, exists := matchByName[packageName]
		if !exists {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "selected package is not part of the active profile or is not installed", packageName, false)
		}
		if match.Protected || isHardProtectedPackage(packageName) {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "protected package cannot be changed", packageName, false)
		}
		if match.Risk == RiskDangerous || match.Risk == RiskBlocked {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "high-risk or blocked package cannot be changed by Safe Tuning", packageName, false)
		}
		if match.Risk == RiskCaution && !request.AcknowledgeCaution {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "caution package requires explicit acknowledgement", packageName, false)
		}
		if !match.IsEnabled && request.Mode == ActionDisable {
			skipped = append(skipped, packageName)
			continue
		}
		if request.Mode == ActionUninstallUser && !match.IsSystemApp {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "user-installed apps are not eligible for reversible uninstall-user mode", packageName, false)
		}
		items = append(items, SnapshotItem{
			PackageName: packageName, WasEnabled: match.IsEnabled, IsSystemApp: match.IsSystemApp,
			Risk: match.Risk, Action: request.Mode,
			State: JournalPlanned,
		})
	}
	if len(items) == 0 {
		return ApplyResult{Changed: []string{}, Skipped: skipped, Failed: map[string]string{}}, nil
	}

	snapshot := Snapshot{
		Version: 2, UserID: 0, HardwareSerial: info.HardwareSerial,
		ID: snapshotID(), CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Serial: analysis.Serial, ProfileID: analysis.SelectedProfile.ID, ProfileName: analysis.SelectedProfile.Name,
		Mode: request.Mode, Items: items,
	}
	// Capture every original state before writing the plan or changing anything.
	for i := range snapshot.Items {
		state, err := s.readPackageState(ctx, analysis.Serial, snapshot.Items[i].PackageName)
		if err != nil {
			return ApplyResult{}, err
		}
		if !state.Installed {
			return ApplyResult{}, core.NewOperationError("tuning_apply", "package is no longer installed", snapshot.Items[i].PackageName, false)
		}
		snapshot.Items[i].Before = &state
	}
	if err := s.persistSnapshot(snapshot); err != nil {
		return ApplyResult{}, err
	}

	result := ApplyResult{
		SnapshotID: snapshot.ID, Changed: []string{}, Skipped: skipped, Failed: map[string]string{},
	}
	return s.applyJournal(ctx, &snapshot, result)
}

func (s *Service) ListSnapshots(ctx context.Context) ([]SnapshotSummary, error) {
	serial, err := s.resolveActiveSerial(ctx)
	if err != nil {
		return nil, err
	}
	return s.ListSnapshotsForSerial(serial)
}

func (s *Service) ListSnapshotsForSerial(serial string) ([]SnapshotSummary, error) {
	dir := s.snapshotDir(serial)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []SnapshotSummary{}, nil
	}
	if err != nil {
		return nil, core.NewOperationError("tuning_snapshots", "failed to read tuning snapshots", err.Error(), true)
	}
	out := make([]SnapshotSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		snapshot, err := s.loadSnapshot(serial, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		applied := 0
		recoverable, uncertain := 0, 0
		for _, item := range snapshot.Items {
			if needsRecovery(item) {
				recoverable++
				if item.Applied {
					applied++
				} else {
					uncertain++
				}
			}
		}
		out = append(out, SnapshotSummary{
			Version: snapshot.Version, Recoverable: recoverable, Uncertain: uncertain,
			ID: snapshot.ID, CreatedAt: snapshot.CreatedAt, Serial: snapshot.Serial,
			ProfileID: snapshot.ProfileID, ProfileName: snapshot.ProfileName,
			Mode: snapshot.Mode, Applied: applied,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func (s *Service) Restore(ctx context.Context, snapshotID string) (RestoreResult, error) {
	serial, err := s.resolveActiveSerial(ctx)
	if err != nil {
		return RestoreResult{}, err
	}
	return s.RestoreForSerial(ctx, serial, snapshotID)
}

func (s *Service) RestoreForSerial(ctx context.Context, serial, snapshotID string) (RestoreResult, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	return s.bound().restore(ctx, serial, snapshotID)
}

func (s *Service) restore(ctx context.Context, serial, snapshotID string) (RestoreResult, error) {
	snapshot, err := s.loadSnapshot(serial, snapshotID)
	if err != nil {
		return RestoreResult{}, err
	}
	if snapshot.Serial != serial {
		return RestoreResult{}, core.NewOperationError("tuning_restore", "snapshot belongs to a different device", snapshot.Serial, false)
	}
	if snapshot.HardwareSerial != "" {
		output, err := s.runADBOutput(ctx, []string{"-s", serial, "shell", "getprop", "ro.serialno"}, "cannot verify snapshot device identity")
		if err != nil {
			return RestoreResult{}, err
		}
		if strings.TrimSpace(output) == "" || strings.EqualFold(strings.TrimSpace(output), "unknown") {
			output, err = s.runADBOutput(ctx, []string{"-s", serial, "shell", "getprop", "ro.boot.serialno"}, "cannot verify snapshot device identity")
			if err != nil {
				return RestoreResult{}, err
			}
		}
		if strings.TrimSpace(output) != snapshot.HardwareSerial {
			return RestoreResult{}, core.NewOperationError("tuning_restore", "snapshot device identity does not match", "", false)
		}
	}
	return s.restoreJournal(ctx, &snapshot)
}

func resolveProfileFrom(profiles []Profile, info device.Info, profileID string) (Profile, error) {
	if strings.TrimSpace(profileID) != "" {
		profile, ok := findProfile(profiles, profileID)
		if !ok {
			return Profile{}, core.NewOperationError("tuning_profile", "tuning profile not found", profileID, false)
		}
		allowed := false
		for _, summary := range profilesForDevice(profiles, info) {
			if summary.ID == profile.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			return Profile{}, core.NewOperationError("tuning_profile", "profile does not match the active device", profile.Name, false)
		}
		return profile, nil
	}
	profile, ok := recommendedProfile(profiles, info)
	if !ok {
		return Profile{}, core.NewOperationError("tuning_profile", "no compatible tuning profile is available", info.Model, false)
	}
	return profile, nil
}

func (s *Service) uninstallForUser(ctx context.Context, serial, packageName string) error {
	return s.runPM(ctx, serial, []string{"uninstall", "-k", "--user", "0", packageName}, "reversible user uninstall failed")
}

func (s *Service) installExistingForUser(ctx context.Context, serial, packageName string) error {
	return s.runShell(ctx, serial, []string{"cmd", "package", "install-existing", "--user", "0", packageName}, "restore install-existing failed")
}

func (s *Service) runPM(ctx context.Context, serial string, args []string, userMessage string) error {
	command := []string{"-s", serial, "shell", "pm"}
	command = append(command, args...)
	return s.runADB(ctx, command, userMessage)
}

func (s *Service) runShell(ctx context.Context, serial string, args []string, userMessage string) error {
	command := []string{"-s", serial, "shell"}
	command = append(command, args...)
	return s.runADB(ctx, command, userMessage)
}

func (s *Service) runADB(ctx context.Context, args []string, userMessage string) error {
	_, err := s.runADBOutput(ctx, args, userMessage)
	return err
}

func (s *Service) runADBOutput(ctx context.Context, args []string, userMessage string) (string, error) {
	run := s.runCommand
	if run == nil {
		run = core.RunCommand
	}
	result, err := run(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb, Args: args, Timeout: 30 * time.Second,
	})
	if err != nil {
		return "", core.NewOperationError("safe_tuning", userMessage, err.Error(), true)
	}
	if result == nil {
		return "", core.NewOperationError("safe_tuning", userMessage, "no command result", true)
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return "", core.NewOperationError("safe_tuning", userMessage, detail, true)
	}
	return result.Stdout, nil
}

func snapshotID() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}

func safeSerial(serial string) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", "[", "_", "]", "_", " ", "_")
	clean := replacer.Replace(strings.TrimSpace(serial))
	if clean == "" {
		return "unknown"
	}
	return clean
}

func (s *Service) snapshotDir(serial string) string {
	return filepath.Join(s.dataDir, "tuning-snapshots", safeSerial(serial))
}

func (s *Service) saveSnapshot(snapshot Snapshot) error {
	dir := s.snapshotDir(snapshot.Serial)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return core.NewOperationError("tuning_snapshot", "failed to create snapshot directory", err.Error(), true)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return core.NewOperationError("tuning_snapshot", "failed to encode tuning snapshot", err.Error(), true)
	}
	path := filepath.Join(dir, snapshot.ID+".json")
	if err := core.WriteFileAtomicWithMode(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

func (s *Service) persistSnapshot(snapshot Snapshot) error {
	if s.writeSnapshot != nil {
		return s.writeSnapshot(snapshot)
	}
	return s.saveSnapshot(snapshot)
}

func (s *Service) loadSnapshot(serial, id string) (Snapshot, error) {
	id = strings.TrimSpace(id)
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, "/\\") {
		return Snapshot{}, core.NewOperationError("tuning_snapshot", "invalid snapshot id", id, false)
	}
	path := filepath.Join(s.snapshotDir(serial), id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, core.NewOperationError("tuning_snapshot", "failed to read tuning snapshot", err.Error(), true)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, core.NewOperationError("tuning_snapshot", "tuning snapshot is invalid", err.Error(), false)
	}
	if snapshot.ID != id {
		return Snapshot{}, core.NewOperationError("tuning_snapshot", "snapshot id mismatch", fmt.Sprintf("%s != %s", snapshot.ID, id), false)
	}
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

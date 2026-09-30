package tuning

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"regexp"
	"strconv"
)

var userStateLine = regexp.MustCompile(`(?m)^\s*User 0:\s*([^\r\n]+)`)
var installedField = regexp.MustCompile(`(?:^|\s)installed=(true|false)(?:\s|$)`)
var enabledField = regexp.MustCompile(`(?:^|\s)enabled=([0-4])(?:\s|$)`)

func parsePackageState(output string) (PackageState, error) {
	lines := userStateLine.FindAllStringSubmatch(output, -1)
	if len(lines) != 1 {
		return PackageState{}, fmt.Errorf("cannot identify a unique Android user 0 package state")
	}
	installed := installedField.FindStringSubmatch(lines[0][1])
	enabled := enabledField.FindStringSubmatch(lines[0][1])
	if len(installed) != 2 || len(enabled) != 2 {
		return PackageState{}, fmt.Errorf("installed/enabled package state is unavailable")
	}
	value, _ := strconv.Atoi(enabled[1])
	return PackageState{Installed: installed[1] == "true", Enabled: value}, nil
}

func (s *Service) readPackageState(ctx context.Context, serial, pkg string) (PackageState, error) {
	if !validPackageName(pkg) {
		return PackageState{}, core.NewOperationError("tuning_state", "invalid package name", pkg, false)
	}
	output, err := s.runADBOutput(ctx, []string{"-s", serial, "shell", "dumpsys", "package", pkg}, "cannot read exact package state")
	if err != nil {
		return PackageState{}, err
	}
	state, err := parsePackageState(output)
	if err != nil {
		return PackageState{}, core.NewOperationError("tuning_state", "exact package state is unavailable; no change attempted", err.Error(), false)
	}
	return state, nil
}

func desiredApplied(item SnapshotItem) PackageState {
	desired := *item.Before
	if item.Action == ActionDisable {
		desired.Enabled = 3
	} else {
		desired.Installed = false
	}
	return desired
}

func appliedStateMatches(item SnapshotItem, observed PackageState) bool {
	if item.Action == ActionUninstallUser {
		return !observed.Installed
	}
	return observed == desiredApplied(item)
}

func recoveryStateMatches(item SnapshotItem, observed PackageState) bool {
	if appliedStateMatches(item, observed) {
		return true
	}
	// install-existing may restore installation before the enabled-state command
	// completes. Accept only that documented intermediate state on resumption.
	return item.State == JournalRestorePending && item.Action == ActionUninstallUser &&
		observed.Installed && (observed.Enabled == 0 || observed.Enabled == item.Before.Enabled)
}

func needsRecovery(item SnapshotItem) bool {
	return item.Applied || item.State == JournalPending || item.State == JournalApplied ||
		item.State == JournalUnknown || item.State == JournalRestorePending || item.State == ""
}

func validateSnapshot(snapshot Snapshot) error {
	invalid := func(detail string) error {
		return core.NewOperationError("tuning_snapshot", "invalid tuning snapshot", detail, false)
	}
	if snapshot.Version != 0 && snapshot.Version != 2 {
		return invalid("unsupported version")
	}
	if snapshot.UserID != 0 {
		return invalid("only Android user 0 snapshots are supported")
	}
	if snapshot.Serial == "" || len(snapshot.Items) > 5000 {
		return invalid("invalid target or item count")
	}
	seen := map[string]bool{}
	for _, item := range snapshot.Items {
		if !validPackageName(item.PackageName) || seen[item.PackageName] {
			return invalid("invalid or duplicate package")
		}
		seen[item.PackageName] = true
		if item.Action != ActionDisable && item.Action != ActionUninstallUser {
			return invalid("unsupported action")
		}
		if snapshot.Version == 2 && (item.Before == nil || !item.Before.Installed || item.Before.Enabled < 0 || item.Before.Enabled > 4) {
			return invalid("original state missing or invalid")
		}
		switch item.State {
		case "", JournalPlanned, JournalPending, JournalApplied, JournalUnknown, JournalRestorePending, JournalRestored:
		default:
			return invalid("unknown journal state")
		}
	}
	return nil
}

func (s *Service) applyJournal(ctx context.Context, snapshot *Snapshot, result ApplyResult) (ApplyResult, error) {
	for i := range snapshot.Items {
		item := &snapshot.Items[i]
		if err := ctx.Err(); err != nil {
			return result, err
		}
		current, err := s.readPackageState(ctx, snapshot.Serial, item.PackageName)
		if err != nil {
			return result, err
		}
		if current != *item.Before {
			return result, core.NewOperationError("tuning_apply", "package state changed; analyze again", item.PackageName, false)
		}
		// Re-read device recovery roles before each disabling action. A launcher
		// or keyboard may have changed after analysis or an earlier batch item.
		if err := s.requireNonRecoveryPackage(ctx, snapshot.Serial, item.PackageName); err != nil {
			return result, err
		}
		item.State = JournalPending
		// This durable intent closes the command-to-result-write interruption gap.
		if err := s.persistSnapshot(*snapshot); err != nil {
			return result, err
		}
		if item.Action == ActionDisable {
			err = s.runPM(ctx, snapshot.Serial, []string{"disable-user", "--user", "0", item.PackageName}, "disable failed")
		} else {
			err = s.uninstallForUser(ctx, snapshot.Serial, item.PackageName)
		}
		if err == nil {
			var observed PackageState
			observed, err = s.readPackageState(ctx, snapshot.Serial, item.PackageName)
			if err == nil && !appliedStateMatches(*item, observed) {
				err = fmt.Errorf("post-change package state does not match the plan")
			}
		}
		if err != nil {
			item.State = JournalUnknown
			item.LastError = err.Error()
			result.Failed[item.PackageName] = err.Error()
			if saveErr := s.persistSnapshot(*snapshot); saveErr != nil {
				return result, saveErr
			}
			// Stop at the first uncertain outcome. Recovery reconciles pending/unknown
			// items against the original state instead of trusting the command result.
			return result, core.NewOperationError("tuning_apply", "change outcome needs recovery", fmt.Sprintf("%s; snapshot %s", err, snapshot.ID), true)
		}
		item.State = JournalApplied
		item.Applied = true
		item.LastError = ""
		result.Changed = append(result.Changed, item.PackageName)
		if err := s.persistSnapshot(*snapshot); err != nil {
			return result, core.NewOperationError("tuning_apply", "change applied; journal update failed", fmt.Sprintf("restore snapshot %s: %s", snapshot.ID, err), true)
		}
	}
	return result, nil
}

func exactEnabledCommand(state int) (string, error) {
	commands := []string{"default-state", "enable", "disable", "disable-user", "disable-until-used"}
	if state < 0 || state >= len(commands) {
		return "", fmt.Errorf("unsupported enabled state %d", state)
	}
	return commands[state], nil
}

func (s *Service) restoreJournal(ctx context.Context, snapshot *Snapshot) (RestoreResult, error) {
	result := RestoreResult{SnapshotID: snapshot.ID, Restored: []string{}, Failed: map[string]string{}}
	for i := len(snapshot.Items) - 1; i >= 0; i-- {
		item := &snapshot.Items[i]
		if !needsRecovery(*item) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		current, err := s.readPackageState(ctx, snapshot.Serial, item.PackageName)
		if err != nil {
			result.Failed[item.PackageName] = err.Error()
			continue
		}
		original := item.Before
		if original == nil {
			// Legacy snapshots only captured an effective boolean. Preserve that
			// limitation explicitly; do not invent an exact original DEFAULT state.
			enabled := 3
			if item.WasEnabled {
				enabled = 1
			}
			original = &PackageState{Installed: true, Enabled: enabled}
		}
		if current != *original {
			if item.Before != nil && !recoveryStateMatches(*item, current) {
				result.Failed[item.PackageName] = "state differs from both original and planned change; review device state before recovery"
				continue
			}
			if item.Before == nil && !item.Applied && current.Installed && (item.WasEnabled == (current.Enabled == 0 || current.Enabled == 1)) {
				item.State = JournalRestored
				if err := s.persistSnapshot(*snapshot); err != nil {
					return result, err
				}
				continue
			}
			if item.Before == nil && current.Installed && current.Enabled != 3 {
				result.Failed[item.PackageName] = "legacy snapshot cannot explain the current state; review before recovery"
				continue
			}
			if isHardProtectedPackage(item.PackageName) && original.Enabled >= 2 {
				result.Failed[item.PackageName] = "recovery cannot disable a protected package"
				continue
			}
			// Restoring a disabled original state is still a disabling operation.
			// Do not let an imported/old journal disable today's OEM HOME or IME.
			// Enabling/reinstalling recovery remains possible if HOME is broken.
			if original.Enabled >= 2 {
				if err := s.requireNonRecoveryPackage(ctx, snapshot.Serial, item.PackageName); err != nil {
					result.Failed[item.PackageName] = err.Error()
					continue
				}
			}
			item.State = JournalRestorePending
			if err := s.persistSnapshot(*snapshot); err != nil {
				return result, err
			}
			if !current.Installed {
				if item.Action != ActionUninstallUser || !item.IsSystemApp {
					result.Failed[item.PackageName] = "package is missing and cannot be reinstalled from a system package"
					continue
				}
				if err = s.installExistingForUser(ctx, snapshot.Serial, item.PackageName); err != nil {
					result.Failed[item.PackageName] = err.Error()
					continue
				}
			}
			command, _ := exactEnabledCommand(original.Enabled)
			if err = s.runPM(ctx, snapshot.Serial, []string{command, "--user", "0", item.PackageName}, "restore exact package state failed"); err != nil {
				result.Failed[item.PackageName] = err.Error()
				continue
			}
			observed, err := s.readPackageState(ctx, snapshot.Serial, item.PackageName)
			if err != nil || observed != *original {
				result.Failed[item.PackageName] = "restore verification failed"
				if err != nil {
					result.Failed[item.PackageName] += "; " + err.Error()
				}
				continue
			}
		}
		item.State = JournalRestored
		item.Applied = false
		item.LastError = ""
		result.Restored = append(result.Restored, item.PackageName)
		if err := s.persistSnapshot(*snapshot); err != nil {
			return result, err
		}
	}
	return result, nil
}

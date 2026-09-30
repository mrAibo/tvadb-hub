package tuning

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"context"
	"strings"
)

func (s *Service) recoveryPackages(ctx context.Context, serial string) (map[string]struct{}, error) {
	packages, err := core.AndroidRecoveryPackages(ctx, s.getBinPath().Adb, serial, s.runCommand)
	if err != nil {
		return nil, core.NewOperationError("tuning_safety", "cannot establish HOME/IME protection; no disabling action attempted", err.Error(), true)
	}
	return packages, nil
}

func (s *Service) requireNonRecoveryPackage(ctx context.Context, serial, pkg string) error {
	if isHardProtectedPackage(pkg) {
		return core.NewOperationError("tuning_safety", "protected package cannot be disabled or removed", pkg, false)
	}
	protected, err := s.recoveryPackages(ctx, serial)
	if err != nil {
		return err
	}
	if _, exists := protected[pkg]; exists {
		return core.NewOperationError("tuning_safety", "HOME/IME recovery package cannot be disabled or removed", pkg, false)
	}
	return nil
}

// hardProtectedPackages is the non-overridable Safe Tuning safety floor.
// A signed metadata feed may add stronger protection, but it cannot make
// these core packages actionable. Keep this list intentionally narrow and
// limited to packages whose loss can break Android management, permissions,
// networking or the DroidSphere recovery path.
var hardProtectedPackages = map[string]struct{}{
	"android":                                 {},
	"com.android.systemui":                    {},
	"com.android.settings":                    {},
	"com.android.tv.settings":                 {},
	"com.android.permissioncontroller":        {},
	"com.google.android.permissioncontroller": {},
	"com.android.packageinstaller":            {},
	"com.google.android.packageinstaller":     {},
	"com.android.providers.settings":          {},
	"com.android.networkstack":                {},
	"com.google.android.networkstack":         {},
	"com.android.shell":                       {},
	"com.google.android.gms":                  {},
	"com.google.android.gsf":                  {},
	"com.amazon.tv.launcher":                  {},
	"com.amazon.tv.ime":                       {},
	"com.amazon.fireinputdevices":             {},
	"com.google.android.apps.tv.launcherx":    {},
	"com.google.android.leanbacklauncher":     {},
	"com.android.tv.launcher":                 {},
	"com.android.launcher3":                   {},
	"com.google.android.apps.nexuslauncher":   {},
	"com.oneplus.launcher":                    {},
	"com.mi.android.globallauncher":           {},
}

func isHardProtectedPackage(packageName string) bool {
	_, ok := hardProtectedPackages[strings.TrimSpace(packageName)]
	return ok
}

// Safety is derived from the target and built-ins, never from a feed's profile
// ID. Choosing a newly named external profile cannot discard device protections.
func deviceSafetyFloor(info device.Info, selected Profile) (map[string]struct{}, map[string]Risk) {
	protected := make(map[string]struct{})
	risks := make(map[string]Risk)
	for pkg := range hardProtectedPackages {
		protected[pkg] = struct{}{}
	}
	for _, profile := range builtinProfiles {
		if profileMatchScore(profile, info) <= 0 {
			continue
		}
		for _, pkg := range profile.Keep {
			protected[pkg] = struct{}{}
		}
		for _, rule := range profile.Rules {
			if old, ok := risks[rule.PackageName]; !ok || riskSeverity(rule.Risk) > riskSeverity(old) {
				risks[rule.PackageName] = rule.Risk
			}
		}
	}
	for _, pkg := range selected.Keep {
		protected[pkg] = struct{}{}
	}
	return protected, risks
}

func enforceRuleFloor(rule PackageRule, protected map[string]struct{}, risks map[string]Risk) PackageRule {
	if _, ok := protected[rule.PackageName]; ok {
		rule.Risk = RiskBlocked
	}
	if floor, ok := risks[rule.PackageName]; ok && riskSeverity(floor) > riskSeverity(rule.Risk) {
		rule.Risk = floor
	}
	if rule.Risk != RiskSafe {
		rule.DefaultSelected = false
	}
	return rule
}

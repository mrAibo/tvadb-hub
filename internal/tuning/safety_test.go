package tuning

import (
	"ADBKit/internal/device"
	"testing"
)

func TestHardProtectedPackagesContainRecoveryCriticalComponents(t *testing.T) {
	for _, packageName := range []string{
		"android",
		"com.android.systemui",
		"com.android.settings",
		"com.android.permissioncontroller",
		"com.android.shell",
		"com.google.android.gms",
	} {
		if !isHardProtectedPackage(packageName) {
			t.Fatalf("%s must be hard protected", packageName)
		}
	}
}

func TestNewExternalProfileIDCannotBypassBuiltinDevicePolicy(t *testing.T) {
	info := device.Info{Manufacturer: "Amazon", Model: "AFTKRT", Codename: "karat", IsTV: true}
	external := Profile{ID: "community-new-id", Criteria: MatchCriteria{Generic: true}}
	protected, risks := deviceSafetyFloor(info, external)
	for _, pkg := range []string{"com.amazon.tv.launcher", "com.amazon.tv.keypolicymanager", "com.amazon.device.controllermanager"} {
		rule := enforceRuleFloor(PackageRule{PackageName: pkg, Risk: RiskSafe, DefaultSelected: true}, protected, risks)
		if rule.Risk != RiskBlocked || rule.DefaultSelected {
			t.Fatalf("external profile bypassed keep floor: %+v", rule)
		}
	}
	pixel := device.Info{Manufacturer: "Google", Brand: "google"}
	protected, risks = deviceSafetyFloor(pixel, external)
	rule := enforceRuleFloor(PackageRule{PackageName: "com.google.android.apps.dialer", Risk: RiskSafe, DefaultSelected: true}, protected, risks)
	if rule.Risk != RiskDangerous || rule.DefaultSelected {
		t.Fatalf("new ID bypassed dangerous floor: %+v", rule)
	}
	if external.Keep != nil {
		t.Fatal("policy mutated signed profile")
	}
}

func TestHardProtectionDoesNotCatchOrdinaryOptionalApps(t *testing.T) {
	if isHardProtectedPackage("com.example.optional") {
		t.Fatal("ordinary optional package must not be hard protected")
	}
}

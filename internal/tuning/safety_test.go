package tuning

import "testing"

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

func TestHardProtectionDoesNotCatchOrdinaryOptionalApps(t *testing.T) {
	if isHardProtectedPackage("com.example.optional") {
		t.Fatal("ordinary optional package must not be hard protected")
	}
}

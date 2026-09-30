package tuning

import "strings"

// hardProtectedPackages is the non-overridable Safe Tuning safety floor.
// A signed metadata feed may add stronger protection, but it cannot make
// these core packages actionable. Keep this list intentionally narrow and
// limited to packages whose loss can break Android management, permissions,
// networking or the DroidSphere recovery path.
var hardProtectedPackages = map[string]struct{}{
	"android":                         {},
	"com.android.systemui":            {},
	"com.android.settings":            {},
	"com.android.tv.settings":         {},
	"com.android.permissioncontroller": {},
	"com.google.android.permissioncontroller": {},
	"com.android.packageinstaller":    {},
	"com.google.android.packageinstaller": {},
	"com.android.providers.settings":  {},
	"com.android.networkstack":        {},
	"com.google.android.networkstack": {},
	"com.android.shell":               {},
	"com.google.android.gms":          {},
	"com.google.android.gsf":          {},
}

func isHardProtectedPackage(packageName string) bool {
	_, ok := hardProtectedPackages[strings.TrimSpace(packageName)]
	return ok
}

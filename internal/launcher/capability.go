package launcher

import (
	"context"
	"strings"
)

// capabilityTokens are the supported commands a device must advertise before the
// guarded HOME replacement path is considered available at all.
var capabilityTokens = []string{"set-home-activity", "resolve-activity"}

// probeCapability asks the device which supported commands it actually advertises.
// The SDK level is never consulted: firmware that fails or omits the probe stays
// unsupported, and an unreadable probe stays Unknown so it can never auto-apply.
//
// The probe can only prove that the commands exist; it cannot prove whether the
// setter honours one exact component or a package. Since Android 10 the shell
// command reduces its TARGET-COMPONENT argument to the package name and assigns the
// HOME role to that package (the help text still calls the argument
// TARGET-COMPONENT). The single-HOME-per-package guard in service.go therefore
// carries the reversibility proof, not this probe.
func (t target) probeCapability(ctx context.Context) Capability {
	output, err := t.read(ctx, "cmd", "package", "help")
	if err != nil {
		return Capability{
			Status: CapabilityUnknown,
			Detail: "cmd package help probe failed: " + err.Error(),
		}
	}

	capability := Capability{
		Status:          CapabilitySupported,
		SetHomeActivity: strings.Contains(output, "set-home-activity"),
		ResolveActivity: strings.Contains(output, "resolve-activity"),
	}
	missing := make([]string, 0, len(capabilityTokens))
	for _, token := range capabilityTokens {
		if !strings.Contains(output, token) {
			missing = append(missing, token)
		}
	}
	if len(missing) > 0 {
		capability.Status = CapabilityUnsupported
		capability.Detail = "cmd package help does not list: " + strings.Join(missing, ", ")
		return capability
	}
	capability.Detail = "cmd package help lists the supported HOME commands. The setter is PACKAGE/role-backed: on Android 10 and later the accepted component is reduced to its package name, so HOME is selected per package and a package with several HOME activities is refused. The exact option order is confirmed only on a live device."
	return capability
}

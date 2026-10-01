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
	capability.Detail = "cmd package help lists the supported HOME commands; the exact option order is confirmed only on a live device"
	return capability
}

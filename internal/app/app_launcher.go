package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"ADBKit/internal/launcher"
)

// The launcher facade is additive and keeps the guarded contract:
//
//   - preflight, candidate test, apply and restore admit the caller-confirmed
//     serial first and then run on a service that pins serial/tool/user;
//   - cancel deliberately does NOT take the admission path: it matches the owned
//     operation ID plus the explicit serial, so a device switch cannot retarget or
//     block a cancellation;
//   - recovery browsing is local and read-only, so it stays usable with no online
//     device and needs no admission.

func (a *App) launcherService() (*launcher.Service, error) {
	if a.lauSvc == nil {
		return nil, core.NewOperationError("launcher_unavailable", "Launcher service is not ready", "start the application first", true)
	}
	return a.lauSvc, nil
}

func launcherIdentity(info device.Info) launcher.Identity {
	return launcher.Identity{
		Serial:         info.Serial,
		Model:          info.Model,
		Manufacturer:   info.Manufacturer,
		Codename:       info.Codename,
		AndroidVersion: info.AndroidVersion,
		SDKVersion:     info.SDKVersion,
		IsTV:           info.IsTV,
	}
}

// PreflightLauncher is the read-only capability picture for one confirmed device.
func (a *App) PreflightLauncher(expectedSerial string) (launcher.Preflight, error) {
	return auditAction(a, "launcher_preflight", func() (launcher.Preflight, error) {
		service, err := a.launcherService()
		if err != nil {
			return launcher.Preflight{}, err
		}
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return launcher.Preflight{}, err
		}
		if a.devSvc == nil {
			return launcher.Preflight{}, core.NewOperationError("launcher_preflight", "Device service is not ready", "start the application first", true)
		}
		info, err := a.devSvc.GetDeviceInfo(a.ctx, serial)
		if err != nil {
			return launcher.Preflight{}, err
		}
		return service.Preflight(a.ctx, serial, launcherIdentity(*info))
	})
}

// TestLauncherCandidate opens one candidate for inspection only. It never changes
// the default HOME.
func (a *App) TestLauncherCandidate(expectedSerial string, candidateComponent string) (launcher.CandidateTestResult, error) {
	return auditAction(a, "launcher_candidate_test", func() (launcher.CandidateTestResult, error) {
		service, err := a.launcherService()
		if err != nil {
			return launcher.CandidateTestResult{}, err
		}
		serial, err := a.expectedActive(expectedSerial)
		if err != nil {
			return launcher.CandidateTestResult{}, err
		}
		return service.TestCandidate(a.ctx, serial, candidateComponent)
	})
}

// ApplyLauncher performs the confirmed HOME replacement. The request carries the
// client-generated operation ID so the caller can cancel while this call runs.
func (a *App) ApplyLauncher(request launcher.ApplyRequest) (launcher.ApplyResult, error) {
	return auditAction(a, "launcher_apply", func() (launcher.ApplyResult, error) {
		service, err := a.launcherService()
		if err != nil {
			return launcher.ApplyResult{}, err
		}
		serial, err := a.expectedActive(request.ExpectedSerial)
		if err != nil {
			return launcher.ApplyResult{}, err
		}
		request.ExpectedSerial = serial
		return service.Apply(a.ctx, request)
	})
}

// RestoreLauncher restores the recorded original HOME after a strict live admission.
func (a *App) RestoreLauncher(request launcher.RestoreRequest) (launcher.RestoreResult, error) {
	return auditAction(a, "launcher_restore", func() (launcher.RestoreResult, error) {
		service, err := a.launcherService()
		if err != nil {
			return launcher.RestoreResult{}, err
		}
		serial, err := a.expectedActive(request.ExpectedSerial)
		if err != nil {
			return launcher.RestoreResult{}, err
		}
		request.ExpectedSerial = serial
		return service.Restore(a.ctx, request)
	})
}

// CancelLauncherOperation signals the owned operation. It intentionally skips the
// admission check: the operation ID and the explicit serial must match, and the
// current global selection is irrelevant.
func (a *App) CancelLauncherOperation(request launcher.CancelRequest) (launcher.CancelResult, error) {
	service, err := a.launcherService()
	if err != nil {
		return launcher.CancelResult{}, err
	}
	return service.Cancel(request)
}

// ReadLauncherRecovery lists durable launcher records. It is local and read-only,
// stays available with no online device and sends no device command.
func (a *App) ReadLauncherRecovery(expectedSerial string) (launcher.Recovery, error) {
	service, err := a.launcherService()
	if err != nil {
		return launcher.Recovery{}, err
	}
	return service.ReadRecovery(expectedSerial)
}

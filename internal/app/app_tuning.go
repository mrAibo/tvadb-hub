package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"ADBKit/internal/tuning"
)

func (a *App) AnalyzeSafeTuning(profileID string) (tuning.Analysis, error) {
	return a.AnalyzeSafeTuningForDevice("", profileID)
}

func (a *App) AnalyzeSafeTuningForDevice(serial, profileID string) (tuning.Analysis, error) {
	return auditAction(a, "analyze_safe_tuning", func() (tuning.Analysis, error) {
		info, err := a.tuningDeviceInfo(serial)
		if err != nil {
			return tuning.Analysis{}, err
		}
		return a.tuneSvc.Analyze(a.ctx, *info, profileID)
	})
}

func (a *App) ApplySafeTuning(request tuning.ApplyRequest) (tuning.ApplyResult, error) {
	return auditAction(a, "apply_safe_tuning", func() (tuning.ApplyResult, error) {
		if request.ExpectedSerial == "" {
			return tuning.ApplyResult{}, core.NewOperationError("tuning_apply", "explicit confirmed device target is required", "analyze and confirm again", false)
		}
		info, err := a.tuningDeviceInfo(request.ExpectedSerial)
		if err != nil {
			return tuning.ApplyResult{}, err
		}
		return a.tuneSvc.Apply(a.ctx, *info, request)
	})
}

func (a *App) ListTuningSnapshots() ([]tuning.SnapshotSummary, error) {
	return auditAction(a, "list_tuning_snapshots", func() ([]tuning.SnapshotSummary, error) {
		return a.tuneSvc.ListSnapshots(a.ctx)
	})
}

func (a *App) ListTuningSnapshotsForDevice(serial string) ([]tuning.SnapshotSummary, error) {
	if _, err := a.tuningTarget(serial); err != nil {
		return nil, err
	}
	return a.tuneSvc.ListSnapshotsForSerial(serial)
}

func (a *App) RestoreTuningSnapshot(snapshotID string) (tuning.RestoreResult, error) {
	return auditAction(a, "restore_tuning_snapshot", func() (tuning.RestoreResult, error) {
		return a.tuneSvc.Restore(a.ctx, snapshotID)
	})
}

func (a *App) RestoreTuningSnapshotForDevice(serial, snapshotID string) (tuning.RestoreResult, error) {
	return auditAction(a, "restore_tuning_snapshot", func() (tuning.RestoreResult, error) {
		target, err := a.tuningTarget(serial)
		if err != nil {
			return tuning.RestoreResult{}, err
		}
		return a.tuneSvc.RestoreForSerial(a.ctx, target, snapshotID)
	})
}

func (a *App) tuningTarget(expected string) (string, error) {
	serial, err := a.resolveActiveSerial(a.ctx)
	if err != nil {
		return "", err
	}
	if expected != "" && serial != expected {
		return "", core.NewOperationError("safe_tuning", "selected device changed; analyze and confirm again", expected, false)
	}
	return serial, nil
}

func (a *App) tuningDeviceInfo(expected string) (*device.Info, error) {
	serial, err := a.tuningTarget(expected)
	if err != nil {
		return nil, err
	}
	return a.devSvc.GetDeviceInfo(a.ctx, serial)
}

func (a *App) GetSafeTuningFeedStatus() tuning.FeedStatus {
	if a.tuneSvc == nil {
		return tuning.FeedStatus{Source: "builtin", Message: "Safe Tuning service is not ready."}
	}
	return a.tuneSvc.FeedStatus()
}

func (a *App) RefreshSafeTuningFeed() (tuning.FeedStatus, error) {
	return auditAction(a, "refresh_safe_tuning_feed", func() (tuning.FeedStatus, error) {
		return a.tuneSvc.RefreshFeed(a.ctx)
	})
}

func (a *App) RollbackSafeTuningFeed() (tuning.FeedStatus, error) {
	return auditAction(a, "rollback_safe_tuning_feed", func() (tuning.FeedStatus, error) {
		return a.tuneSvc.RollbackFeed()
	})
}

func (a *App) currentSafeTuningFeedConfig() tuning.FeedConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return tuning.FeedConfig{}
	}
	return tuning.FeedConfig{
		URL:       a.cfg.SafeTuningFeedURL,
		PublicKey: a.cfg.SafeTuningFeedPublicKey,
	}
}

func (a *App) GetSafeTuningFeedConfig() tuning.FeedConfig {
	return a.currentSafeTuningFeedConfig()
}

func (a *App) ConfigureSafeTuningFeed(config tuning.FeedConfig) (tuning.FeedStatus, error) {
	return auditAction(a, "configure_safe_tuning_feed", func() (tuning.FeedStatus, error) {
		normalized, err := tuning.NormalizeFeedConfig(config)
		if err != nil {
			return tuning.FeedStatus{}, err
		}

		a.mu.Lock()
		if a.cfg == nil {
			a.mu.Unlock()
			return tuning.FeedStatus{}, core.NewOperationError(
				"safe_tuning_feed_config",
				"Application configuration is not available",
				"",
				false,
			)
		}
		a.cfg.SafeTuningFeedURL = normalized.URL
		a.cfg.SafeTuningFeedPublicKey = normalized.PublicKey
		err = core.SaveConfig(a.dataDir, a.cfg)
		a.mu.Unlock()
		if err != nil {
			return tuning.FeedStatus{}, err
		}
		return a.tuneSvc.FeedStatus(), nil
	})
}

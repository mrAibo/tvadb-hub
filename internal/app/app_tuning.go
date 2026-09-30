package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/device"
	"ADBKit/internal/tuning"
)

func (a *App) AnalyzeSafeTuning(profileID string) (tuning.Analysis, error) {
	return auditAction(a, "analyze_safe_tuning", func() (tuning.Analysis, error) {
		info, err := a.activeTuningDeviceInfo()
		if err != nil {
			return tuning.Analysis{}, err
		}
		return a.tuneSvc.Analyze(a.ctx, *info, profileID)
	})
}

func (a *App) ApplySafeTuning(request tuning.ApplyRequest) (tuning.ApplyResult, error) {
	return auditAction(a, "apply_safe_tuning", func() (tuning.ApplyResult, error) {
		info, err := a.activeTuningDeviceInfo()
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

func (a *App) RestoreTuningSnapshot(snapshotID string) (tuning.RestoreResult, error) {
	return auditAction(a, "restore_tuning_snapshot", func() (tuning.RestoreResult, error) {
		return a.tuneSvc.Restore(a.ctx, snapshotID)
	})
}

func (a *App) activeTuningDeviceInfo() (*device.Info, error) {
	serial, err := a.resolveActiveSerial(a.ctx)
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

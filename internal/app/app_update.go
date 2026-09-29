package app

import "ADBKit/internal/updater"

func (a *App) CheckForUpdates() (updater.ReleaseInfo, error) {
	return auditAction(a, "check_for_updates", func() (updater.ReleaseInfo, error) {
		return a.updSvc.Check(a.ctx)
	})
}

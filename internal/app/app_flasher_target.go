package app

import (
	"ADBKit/internal/core"
	"ADBKit/internal/flasher"
	"strings"
)

// The confirmed fastboot flash RPCs are additive twins of FlashPartition and
// FlashRomFolder:
//
//   - the legacy RPCs keep their signatures and their fallback to the ADB active
//     device, so nothing that already ships changes behaviour;
//   - these twins accept a caller-confirmed fastboot serial and never fall back to
//     the ADB selection, because fastboot devices are chosen independently in the
//     UI and the backend does not own an authoritative fastboot selection;
//   - a blank serial is refused before any binary is resolved, before the fastboot
//     service is touched and before any command is built;
//   - the whole operation (single flash or multi-step batch) runs through one bound
//     flasher.Target, so the serial and the fastboot executable stay immutable even
//     if the configuration or the global selection changes while the batch runs.

// confirmedFastbootSerial trims the caller-confirmed serial and refuses a blank one.
func confirmedFastbootSerial(serial string) (string, error) {
	trimmed := strings.TrimSpace(serial)
	if trimmed == "" {
		return "", core.NewOperationError("confirmed_fastboot_target", "Confirmed fastboot device is required", "capture the fastboot serial before flashing", false)
	}
	return trimmed, nil
}

// FlashPartitionForDevice flashes one partition to the confirmed fastboot serial.
func (a *App) FlashPartitionForDevice(confirmedSerial string, partition string, filePath string) (string, error) {
	return auditAction(a, "flash_partition_confirmed", func() (string, error) {
		serial, err := confirmedFastbootSerial(confirmedSerial)
		if err != nil {
			return "", err
		}
		// Validation keeps the legacy precedence: partition and image are checked
		// before any binary is resolved.
		trimmedPartition := strings.ToLower(strings.TrimSpace(partition))
		if err := core.ValidateFlashPartition(trimmedPartition); err != nil {
			return "", err
		}
		trimmedFilePath := strings.TrimSpace(filePath)
		if err := core.ValidateFlashFile(trimmedFilePath); err != nil {
			return "", err
		}
		if a.fbSvc == nil {
			return "", core.NewOperationError("flash_partition_confirmed", "fastboot service is not ready", "", true)
		}
		target, err := a.fbSvc.ForTarget(serial)
		if err != nil {
			return "", err
		}
		return target.FlashPartition(a.ctx, trimmedPartition, trimmedFilePath)
	})
}

// FlashRomFolderForDevice flashes a validated plan to the confirmed fastboot serial,
// pinning one fastboot executable for every step of the batch.
func (a *App) FlashRomFolderForDevice(confirmedSerial string, folderPath string, plan flasher.Plan) (string, error) {
	return auditAction(a, "flash_rom_folder_confirmed", func() (string, error) {
		serial, err := confirmedFastbootSerial(confirmedSerial)
		if err != nil {
			return "", err
		}
		// The plan is validated before any binary is resolved, exactly like the
		// legacy batch path validates before it starts flashing.
		if err := flasher.ValidateFlashPlan(folderPath, plan); err != nil {
			return "", err
		}
		if a.fbSvc == nil {
			return "", core.NewOperationError("flash_rom_folder_confirmed", "fastboot service is not ready", "", true)
		}
		if a.fpSvc == nil {
			return "", core.NewOperationError("flash_rom_folder_confirmed", "flash plan executor is not ready", "", true)
		}
		target, err := a.fbSvc.ForTarget(serial)
		if err != nil {
			return "", err
		}
		return a.fpSvc.FlashRomFolderForTarget(a.ctx, target, folderPath, plan)
	})
}

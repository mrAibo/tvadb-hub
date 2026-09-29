package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"ADBKit/internal/core"
)

// OpenPathLocation opens a configured binary or directory in the host file
// manager. For files, platforms that support it highlight the file; Linux
// falls back to opening the containing directory.
func (a *App) OpenPathLocation(path string) error {
	return auditVoidAction(a, "open_path_location", func() error {
		target := strings.TrimSpace(path)
		if target == "" {
			return core.NewOperationError("open_path_location", "path is required", "", false)
		}

		info, err := os.Stat(target)
		if err != nil {
			return core.NewOperationError("open_path_location", "path does not exist", err.Error(), false)
		}

		name, args := hostOpenCommand(runtime.GOOS, target, info.IsDir())
		if name == "" {
			return core.NewOperationError(
				"open_path_location",
				"opening paths is not supported on this platform",
				runtime.GOOS,
				false,
			)
		}

		if err := exec.Command(name, args...).Start(); err != nil {
			return core.NewOperationError(
				"open_path_location",
				"failed to open path in file manager",
				fmt.Sprintf("%s: %v", target, err),
				true,
			)
		}
		return nil
	})
}

func hostOpenCommand(goos string, target string, isDir bool) (string, []string) {
	switch goos {
	case "windows":
		if isDir {
			return "explorer.exe", []string{target}
		}
		return "explorer.exe", []string{"/select,", filepath.Clean(target)}
	case "darwin":
		if isDir {
			return "open", []string{target}
		}
		return "open", []string{"-R", target}
	case "linux":
		if !isDir {
			target = filepath.Dir(target)
		}
		return "xdg-open", []string{target}
	default:
		return "", nil
	}
}

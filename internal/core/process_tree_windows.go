//go:build windows

package core

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// TerminateProcessTree uses the Windows taskkill tree flag so descendants of
// adb, fastboot or scrcpy are terminated together with the launched process.
func TerminateProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid > 0 {
		treeKill := exec.Command(
			"taskkill",
			"/PID", strconv.Itoa(pid),
			"/T",
			"/F",
		)
		treeKill.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:     true,
			CreationFlags: createNoWindow,
		}
		if err := treeKill.Run(); err == nil {
			return nil
		}
	}

	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

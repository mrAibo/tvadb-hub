//go:build !windows

package core

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// TerminateProcessTree kills the whole process group created by
// ConfigureChildProcess. This prevents long-running adb/scrcpy children from
// surviving cancellation.
func TerminateProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid > 0 {
		if err := syscall.Kill(-pid, syscall.SIGKILL); err == nil || errors.Is(err, syscall.ESRCH) {
			return nil
		}
	}

	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

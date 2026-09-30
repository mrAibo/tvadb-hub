package core

import (
	"context"
	"os/exec"
	"time"
)

// NewCommandContext creates a cancellable child process with the platform
// process-group settings used by DroidSphere. Cancellation terminates the
// complete process tree rather than only the immediate child.
func NewCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	ConfigureChildProcess(cmd)
	cmd.Cancel = func() error {
		return TerminateProcessTree(cmd)
	}
	cmd.WaitDelay = 3 * time.Second
	return cmd
}

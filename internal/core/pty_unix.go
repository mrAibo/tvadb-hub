//go:build !windows

package core

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// runWithPTY runs a command attached to a pseudo-terminal so that tools like
// adb emit their interactive progress output (e.g. "[ 45%] /path/file"), which
// they suppress when connected to a plain pipe. Each output line (split on \n
// and \r) is forwarded to onLine. Returns the captured output after exit.
func runWithPTY(ctx context.Context, command string, args []string, onLine func(line string)) (*ExecResult, error) {
	cmd := exec.Command(command, args...)
	start := time.Now()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, errPTYUnsupported
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = TerminateProcessTree(cmd)
			}
			_ = ptmx.Close()
		case <-done:
		}
	}()

	var buf boundedOutput

	scanner := bufio.NewScanner(ptmx)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanProgressLines)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			buf.append(line + "\n")
		}
		if onLine != nil {
			onLine(line)
		}
	}
	readErr := scanner.Err()
	// Linux PTYs return EIO at normal EOF, not an output-reader failure.
	if readErr != nil && !errors.Is(readErr, syscall.EIO) {
		_ = TerminateProcessTree(cmd)
	}

	waitErr := cmd.Wait()
	close(done)

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, NewOperationError("exec", "process wait failed", waitErr.Error(), true)
		}
	}

	result := &ExecResult{
		Stdout:   buf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}
	if readErr != nil && !errors.Is(readErr, syscall.EIO) {
		return result, NewOperationError("exec", "failed to read process output", readErr.Error(), true)
	}
	return result, waitErr
}

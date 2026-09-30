package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CommandRunner func(context.Context, ExecRequest) (*ExecResult, error)

var clipboardSetHelp = regexp.MustCompile(`(?m)^\s*set(?:\s+[^\r\n]*)?$`)
var clipboardGetHelp = regexp.MustCompile(`(?m)^\s*get(?:\s+[^\r\n]*)?$`)

func clipboardOutputUnsupported(result *ExecResult, includeStdout bool) bool {
	if result == nil {
		return true
	}
	text := strings.ToLower(result.Stderr)
	if includeStdout {
		text += "\n" + strings.ToLower(result.Stdout)
	}
	for _, marker := range []string{"no shell command implementation", "unknown command", "unknown option", "can't find service", "not found"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func clipboardCommand(ctx context.Context, run CommandRunner, adbPath, serial, command string) (*ExecResult, error) {
	result, err := run(ctx, ExecRequest{Command: adbPath, Args: []string{"-s", serial, "shell", command}, Timeout: 5 * time.Second})
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		return result, err
	}
	if result == nil || result.ExitCode != 0 || clipboardOutputUnsupported(result, command != "cmd clipboard get") {
		return result, fmt.Errorf("Android clipboard shell command is unsupported or failed")
	}
	return result, nil
}

func probeAndroidClipboard(ctx context.Context, run CommandRunner, adbPath, serial string) error {
	result, err := clipboardCommand(ctx, run, adbPath, serial, "cmd clipboard help")
	if err != nil {
		return err
	}
	if !clipboardSetHelp.MatchString(result.Stdout) || !clipboardGetHelp.MatchString(result.Stdout) {
		return fmt.Errorf("Android clipboard help does not advertise supported set/get commands")
	}
	return nil
}

// SetAndroidClipboard uses a detected vendor shell implementation and verifies
// exact readback. Stock Android may return exit 0 for an unimplemented shell
// command, so exit status alone must never authorize a subsequent PASTE event.
func SetAndroidClipboard(ctx context.Context, adbPath, serial, text string, run CommandRunner) error {
	if strings.TrimSpace(serial) == "" || text == "" || strings.ContainsRune(text, '\x00') {
		return fmt.Errorf("invalid clipboard target or text")
	}
	if run == nil {
		run = RunCommand
	}
	if err := probeAndroidClipboard(ctx, run, adbPath, serial); err != nil {
		return err
	}
	if _, err := clipboardCommand(ctx, run, adbPath, serial, "cmd clipboard set "+QuoteShellArg(text)); err != nil {
		return err
	}
	result, err := clipboardCommand(ctx, run, adbPath, serial, "cmd clipboard get")
	if err != nil {
		return err
	}
	if result.Stdout != text && result.Stdout != text+"\n" && result.Stdout != text+"\r\n" {
		return fmt.Errorf("Android clipboard readback did not match requested text; paste was not attempted")
	}
	return nil
}

func ReadAndroidClipboard(ctx context.Context, adbPath, serial string, run CommandRunner) (string, error) {
	if strings.TrimSpace(serial) == "" {
		return "", fmt.Errorf("device serial is required")
	}
	if run == nil {
		run = RunCommand
	}
	if err := probeAndroidClipboard(ctx, run, adbPath, serial); err != nil {
		return "", err
	}
	result, err := clipboardCommand(ctx, run, adbPath, serial, "cmd clipboard get")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(result.Stdout, "\n"), "\r"), nil
}

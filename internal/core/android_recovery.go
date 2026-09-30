package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var androidComponentPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)/(\.?[A-Za-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)*)$`)
var imeSubtypePattern = regexp.MustCompile(`^-?[0-9]+$`)

// AndroidComponentPackage accepts a flattened Android component, not shell
// text, dumpsys fragments or a substring which happens to look like a package.
func AndroidComponentPackage(component string) (string, error) {
	if len(component) > 1024 {
		return "", fmt.Errorf("Android component exceeds the supported length")
	}
	match := androidComponentPattern.FindStringSubmatch(component)
	if match == nil {
		return "", fmt.Errorf("invalid Android component")
	}
	return match[1], nil
}

func recoveryRead(ctx context.Context, run CommandRunner, adbPath, serial string, args ...string) (string, error) {
	command := append([]string{"-s", serial, "shell"}, args...)
	result, err := run(ctx, ExecRequest{Command: adbPath, Args: command, Timeout: 5 * time.Second})
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("recovery capability read failed: %w", err)
	}
	if result == nil || result.ExitCode != 0 || strings.TrimSpace(result.Stderr) != "" {
		return "", fmt.Errorf("recovery capability read returned an error or diagnostic")
	}
	if len(result.Stdout) > 64*1024 {
		return "", fmt.Errorf("recovery capability output exceeds the supported size")
	}
	return strings.TrimSpace(result.Stdout), nil
}

// AndroidRecoveryPackages discovers user 0 HOME handlers/current resolution and
// selected/enabled IMEs. This is a read-only, non-cacheable safety floor, not
// firmware capability authorization for launcher replacement.
func AndroidRecoveryPackages(ctx context.Context, adbPath, serial string, run CommandRunner) (map[string]struct{}, error) {
	if strings.TrimSpace(adbPath) == "" || strings.TrimSpace(serial) == "" {
		return nil, fmt.Errorf("explicit recovery target and ADB path are required")
	}
	if run == nil {
		run = RunCommand
	}
	protected := map[string]struct{}{}
	intent := []string{"--components", "--user", "0", "-a", "android.intent.action.MAIN", "-c", "android.intent.category.HOME"}
	for _, verb := range []string{"query-activities", "resolve-activity"} {
		args := append([]string{"cmd", "package", verb}, intent...)
		output, err := recoveryRead(ctx, run, adbPath, serial, args...)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(output, "\n")
		if output == "" || (verb == "resolve-activity" && len(lines) != 1) {
			return nil, fmt.Errorf("HOME resolution is unavailable or ambiguous")
		}
		for _, line := range lines {
			pkg, err := AndroidComponentPackage(strings.TrimSpace(line))
			if err != nil {
				return nil, fmt.Errorf("HOME capability output is unsupported or malformed: %w", err)
			}
			protected[pkg] = struct{}{}
		}
	}
	for _, key := range []string{"default_input_method", "enabled_input_methods"} {
		output, err := recoveryRead(ctx, run, adbPath, serial, "settings", "--user", "0", "get", "secure", key)
		if err != nil {
			return nil, err
		}
		// Explicit Android null is a known absence. Empty/diagnostic output is
		// not evidence that no keyboard exists (including exit-0 unsupported).
		if output == "null" {
			continue
		}
		if key == "default_input_method" {
			pkg, err := AndroidComponentPackage(output)
			if err != nil {
				return nil, fmt.Errorf("selected IME is unavailable or malformed: %w", err)
			}
			protected[pkg] = struct{}{}
			continue
		}
		for _, entry := range strings.Split(output, ":") {
			fields := strings.Split(entry, ";")
			pkg, err := AndroidComponentPackage(fields[0])
			if err != nil {
				return nil, fmt.Errorf("enabled IME list is unavailable or malformed: %w", err)
			}
			for _, subtype := range fields[1:] {
				if !imeSubtypePattern.MatchString(subtype) {
					return nil, fmt.Errorf("enabled IME subtype is malformed")
				}
			}
			protected[pkg] = struct{}{}
		}
	}
	return protected, nil
}

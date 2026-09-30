package main

import (
	"ADBKit/internal/adbproto"
	"ADBKit/internal/core"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type stats struct {
	min    time.Duration
	median time.Duration
	p95    time.Duration
	max    time.Duration
}

func main() {
	adbPath := flag.String("adb", "adb", "path to the official adb executable")
	server := flag.String("server", adbproto.DefaultServerAddress, "ADB server smart-socket address")
	iterations := flag.Int("n", 50, "measured iterations per implementation")
	warmup := flag.Int("warmup", 3, "warmup iterations per implementation")
	timeout := flag.Duration("timeout", 5*time.Second, "timeout for one operation")
	flag.Parse()

	if *iterations <= 0 || *warmup < 0 || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "-n must be > 0, -warmup must be >= 0 and -timeout must be > 0")
		os.Exit(2)
	}

	client := adbproto.NewClient(*server)
	client.DialTimeout = *timeout
	client.IOTimeout = *timeout

	cliQuery := func(ctx context.Context) (string, error) {
		result, err := core.RunCommand(ctx, core.ExecRequest{
			Command: *adbPath,
			Args:    []string{"devices", "-l"},
			Timeout: *timeout,
		})
		if err != nil {
			return "", fmt.Errorf("adb devices -l: %w", err)
		}
		if result.ExitCode != 0 {
			return "", fmt.Errorf("adb devices -l exited with %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
		}
		return result.Stdout, nil
	}
	directQuery := func(ctx context.Context) (string, error) {
		return client.ListDevicesLong(ctx)
	}

	// Warm the official CLI first so adb-server startup is not charged to only
	// one side of the comparison. The direct client deliberately never starts
	// or upgrades adb-server on its own.
	if _, err := runOnce(*timeout, cliQuery); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := runOnce(*timeout, directQuery); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for i := 0; i < *warmup; i++ {
		if _, err := runOnce(*timeout, cliQuery); err != nil {
			fmt.Fprintln(os.Stderr, "CLI warmup:", err)
			os.Exit(1)
		}
		if _, err := runOnce(*timeout, directQuery); err != nil {
			fmt.Fprintln(os.Stderr, "direct warmup:", err)
			os.Exit(1)
		}
	}

	cliSnapshot, err := runOnce(*timeout, cliQuery)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	directSnapshot, err := runOnce(*timeout, directQuery)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cliDurations, err := measure(*iterations, *timeout, cliQuery)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CLI benchmark:", err)
		os.Exit(1)
	}
	directDurations, err := measure(*iterations, *timeout, directQuery)
	if err != nil {
		fmt.Fprintln(os.Stderr, "direct benchmark:", err)
		os.Exit(1)
	}

	fmt.Printf("ADB smart-socket benchmark (%d iterations, warm server)\n", *iterations)
	fmt.Printf("server: %s\n", *server)
	fmt.Printf("adb:    %s\n\n", *adbPath)
	printStats("adb devices -l", summarize(cliDurations))
	printStats("host:devices-l", summarize(directDurations))

	if normalizedCLI(cliSnapshot) != strings.TrimSpace(directSnapshot) {
		fmt.Println("\nwarning: CLI and direct snapshots differed; repeat with a stable device set before drawing conclusions")
	}
}

func runOnce(timeout time.Duration, query func(context.Context) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return query(ctx)
}

func measure(iterations int, timeout time.Duration, query func(context.Context) (string, error)) ([]time.Duration, error) {
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		started := time.Now()
		_, err := query(ctx)
		duration := time.Since(started)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("iteration %d: %w", i+1, err)
		}
		durations = append(durations, duration)
	}
	return durations, nil
}

func summarize(values []time.Duration) stats {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return stats{
		min:    ordered[0],
		median: percentile(ordered, 0.50),
		p95:    percentile(ordered, 0.95),
		max:    ordered[len(ordered)-1],
	}
}

func percentile(ordered []time.Duration, p float64) time.Duration {
	if len(ordered) == 0 {
		return 0
	}
	index := int(float64(len(ordered)-1)*p + 0.5)
	if index < 0 {
		index = 0
	}
	if index >= len(ordered) {
		index = len(ordered) - 1
	}
	return ordered[index]
}

func printStats(name string, s stats) {
	fmt.Printf("%-16s min=%-10s median=%-10s p95=%-10s max=%s\n", name, s.min, s.median, s.p95, s.max)
}

func normalizedCLI(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "List of devices attached") || strings.HasPrefix(trimmed, "* daemon ") {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, "\n")
}

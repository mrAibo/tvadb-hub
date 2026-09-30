package main

import (
	"ADBKit/internal/adbproto"
	"ADBKit/internal/core"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

type endpoint struct{ address, host, port, source string }

func resolveEndpoint(explicit string, getenv func(string) string) (endpoint, error) {
	address, source := explicit, "-server"
	if address == "" {
		source, address = "default", adbproto.DefaultServerAddress
		if socket := getenv("ADB_SERVER_SOCKET"); socket != "" {
			source = "ADB_SERVER_SOCKET"
			if !strings.HasPrefix(socket, "tcp:") {
				return endpoint{}, fmt.Errorf("only local TCP ADB_SERVER_SOCKET is comparable; override explicitly with -server")
			}
			address = strings.TrimPrefix(socket, "tcp:")
			if !strings.Contains(address, ":") {
				address = net.JoinHostPort("127.0.0.1", address)
			}
		} else if host, port := getenv("ANDROID_ADB_SERVER_ADDRESS"), getenv("ANDROID_ADB_SERVER_PORT"); host != "" || port != "" {
			source = "ANDROID_ADB_SERVER_ADDRESS/PORT"
			if host == "" {
				host = "127.0.0.1"
			}
			if port == "" {
				port = "5037"
			}
			address = net.JoinHostPort(host, port)
		}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return endpoint{}, fmt.Errorf("invalid local ADB endpoint: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return endpoint{}, fmt.Errorf("ADB port must be between 1 and 65535")
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return endpoint{}, fmt.Errorf("benchmark only supports an explicit loopback TCP ADB server")
	}
	host, port = ip.String(), strconv.Itoa(n)
	cliHost := host
	if strings.Contains(host, ":") {
		cliHost = "[" + host + "]"
	}
	return endpoint{address: net.JoinHostPort(host, port), host: cliHost, port: port, source: source}, nil
}

func cliQuery(adbPath string, server endpoint, timeout time.Duration, run core.CommandRunner) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		result, err := run(ctx, core.ExecRequest{Command: adbPath, Args: []string{"-H", server.host, "-P", server.port, "devices", "-l"}, Timeout: timeout})
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			return "", fmt.Errorf("adb devices -l: %w", err)
		}
		if result == nil || result.ExitCode != 0 {
			return "", fmt.Errorf("adb devices -l did not return a successful result")
		}
		if !strings.Contains(result.Stdout, "List of devices attached") {
			return "", fmt.Errorf("adb devices -l returned unsupported output")
		}
		return result.Stdout, nil
	}
}

type summary struct {
	MinNS    int64 `json:"minNs"`
	MedianNS int64 `json:"medianNs"`
	P95NS    int64 `json:"p95Ns"`
	MaxNS    int64 `json:"maxNs"`
}
type pairSample struct {
	Index          int    `json:"index"`
	First          string `json:"first"`
	CLINS          int64  `json:"cliNs"`
	SocketNS       int64  `json:"socketNs"`
	CLISnapshot    string `json:"cliSnapshot"`
	SocketSnapshot string `json:"socketSnapshot"`
	Match          bool   `json:"match"`
}
type report struct {
	SchemaVersion    int          `json:"schemaVersion"`
	GeneratedAt      string       `json:"generatedAt"`
	HostOS           string       `json:"hostOS"`
	HostArch         string       `json:"hostArch"`
	GoVersion        string       `json:"goVersion"`
	SourceRevision   string       `json:"sourceRevision"`
	SourceDirty      *bool        `json:"sourceDirty"`
	ADBPath          string       `json:"adbPath"`
	ADBVersion       string       `json:"adbVersion"`
	Server           string       `json:"server"`
	EndpointSource   string       `json:"endpointSource"`
	Scenario         string       `json:"scenario"`
	Iterations       int          `json:"iterations"`
	Warmup           int          `json:"warmup"`
	TimeoutNS        int64        `json:"timeoutNs"`
	Samples          []pairSample `json:"samples"`
	CLI              summary      `json:"cli"`
	Socket           summary      `json:"socket"`
	MismatchedPairs  int          `json:"mismatchedPairs"`
	DeviceSetChanged bool         `json:"deviceSetChanged"`
	Comparable       bool         `json:"comparable"`
	Error            string       `json:"error,omitempty"`
}

func main() {
	adbPath := flag.String("adb", "adb", "path to official adb executable")
	server := flag.String("server", "", "loopback TCP ADB endpoint; explicit flag overrides server environment")
	n := flag.Int("n", 50, "measured pairs")
	warmup := flag.Int("warmup", 3, "alternating warmup pairs")
	timeout := flag.Duration("timeout", 5*time.Second, "timeout per operation")
	scenario := flag.String("scenario", "unspecified", "evidence label: no-device, USB, wireless or multiple")
	jsonOutput := flag.Bool("json", false, "write raw/summary JSON evidence to stdout")
	flag.Parse()
	if *n <= 0 || *n > 100000 || *warmup < 0 || *warmup > 100000 || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "invalid iteration/warmup/timeout bounds")
		os.Exit(2)
	}
	ep, err := resolveEndpoint(*server, os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := adbproto.NewClient(ep.address)
	client.DialTimeout, client.IOTimeout = *timeout, *timeout
	r := report{SchemaVersion: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339), HostOS: runtime.GOOS, HostArch: runtime.GOARCH, GoVersion: runtime.Version(), ADBPath: *adbPath, Server: ep.address, EndpointSource: ep.source, Scenario: *scenario, Iterations: *n, Warmup: *warmup, TimeoutNS: int64(*timeout), Samples: []pairSample{}}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				r.SourceRevision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty := setting.Value == "true"
				r.SourceDirty = &dirty
			}
		}
	}
	version, versionErr := core.RunCommand(ctx, core.ExecRequest{Command: *adbPath, Args: []string{"version"}, Timeout: *timeout})
	if versionErr != nil || version == nil || version.ExitCode != 0 {
		err = fmt.Errorf("cannot record official ADB version: %v", versionErr)
	} else {
		r.ADBVersion = strings.TrimSpace(version.Stdout)
		err = runBenchmark(ctx, &r, *timeout, cliQuery(*adbPath, ep, *timeout, core.RunCommand), client.ListDevicesLong)
	}
	if err != nil {
		r.Error = err.Error()
	}
	r.Comparable = err == nil && len(r.Samples) == *n && r.MismatchedPairs == 0 && !r.DeviceSetChanged
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(r); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			os.Exit(1)
		}
	} else {
		fmt.Printf("ADB benchmark: %d paired samples, %s (%s)\n", len(r.Samples), ep.address, ep.source)
		printStats("adb devices -l", r.CLI)
		printStats("host:devices-l", r.Socket)
		fmt.Printf("comparable: %t; mismatched pairs: %d; device set changed: %t\n", r.Comparable, r.MismatchedPairs, r.DeviceSetChanged)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !r.Comparable {
		fmt.Fprintln(os.Stderr, "snapshots changed or differed; this run is not adoption evidence")
		os.Exit(1)
	}
}

func runQuery(ctx context.Context, timeout time.Duration, query func(context.Context) (string, error)) (string, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	output, err := query(ctx)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return output, time.Since(started), err
}

func runBenchmark(ctx context.Context, r *report, timeout time.Duration, cli, socket func(context.Context) (string, error)) error {
	// Official CLI owns server startup/version negotiation; setup is excluded.
	if _, _, err := runQuery(ctx, timeout, cli); err != nil {
		return fmt.Errorf("CLI setup: %w", err)
	}
	if _, _, err := runQuery(ctx, timeout, socket); err != nil {
		return fmt.Errorf("socket setup: %w", err)
	}
	cliTimes, socketTimes := []time.Duration{}, []time.Duration{}
	defer func() { r.CLI = summarize(cliTimes); r.Socket = summarize(socketTimes) }()
	baseline, haveBaseline := "", false
	for i := 0; i < r.Warmup+r.Iterations; i++ {
		cliOutput, socketOutput := "", ""
		var cliTime, socketTime time.Duration
		first := "cli"
		queries := []string{"cli", "socket"}
		if i%2 == 1 {
			first = "socket"
			queries = []string{"socket", "cli"}
		}
		for _, name := range queries {
			var err error
			if name == "cli" {
				cliOutput, cliTime, err = runQuery(ctx, timeout, cli)
			} else {
				socketOutput, socketTime, err = runQuery(ctx, timeout, socket)
			}
			if err != nil {
				return fmt.Errorf("pair %d (%s): %w", i+1, name, err)
			}
		}
		if i < r.Warmup {
			continue
		}
		cliSnapshot, socketSnapshot := normalizedSnapshot(cliOutput, true), normalizedSnapshot(socketOutput, false)
		match := cliSnapshot == socketSnapshot
		if !match {
			r.MismatchedPairs++
		}
		if !haveBaseline {
			baseline = cliSnapshot
			haveBaseline = true
		}
		if cliSnapshot != baseline || socketSnapshot != baseline {
			r.DeviceSetChanged = true
		}
		r.Samples = append(r.Samples, pairSample{Index: i - r.Warmup + 1, First: first, CLINS: int64(cliTime), SocketNS: int64(socketTime), CLISnapshot: cliSnapshot, SocketSnapshot: socketSnapshot, Match: match})
		cliTimes = append(cliTimes, cliTime)
		socketTimes = append(socketTimes, socketTime)
	}
	return nil
}

func normalizedSnapshot(output string, cli bool) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		row := strings.Join(strings.Fields(line), " ")
		if row == "" || (cli && row == "List of devices attached") {
			continue
		}
		rows = append(rows, row)
	}
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

func summarize(values []time.Duration) summary {
	if len(values) == 0 {
		return summary{}
	}
	ordered := append([]time.Duration{}, values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return summary{MinNS: int64(ordered[0]), MedianNS: int64(percentile(ordered, .50)), P95NS: int64(percentile(ordered, .95)), MaxNS: int64(ordered[len(ordered)-1])}
}

func percentile(ordered []time.Duration, p float64) time.Duration {
	if len(ordered) == 0 {
		return 0
	}
	index := int(float64(len(ordered)-1)*p + .5)
	if index < 0 {
		index = 0
	}
	if index >= len(ordered) {
		index = len(ordered) - 1
	}
	return ordered[index]
}

func printStats(name string, s summary) {
	fmt.Printf("%-16s min=%s median=%s p95=%s max=%s\n", name, time.Duration(s.MinNS), time.Duration(s.MedianNS), time.Duration(s.P95NS), time.Duration(s.MaxNS))
}

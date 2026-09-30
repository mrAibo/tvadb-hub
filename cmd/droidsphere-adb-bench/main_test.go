package main

import (
	"ADBKit/internal/core"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEndpointPrecedenceAndValidation(t *testing.T) {
	cases := []struct {
		name, explicit string
		env            map[string]string
		want, source   string
		invalid        bool
	}{
		{name: "default", want: "127.0.0.1:5037", source: "default"},
		{name: "explicit overrides socket", explicit: "127.0.0.1:15037", env: map[string]string{"ADB_SERVER_SOCKET": "localfilesystem:/adb"}, want: "127.0.0.1:15037", source: "-server"},
		{name: "tcp port", env: map[string]string{"ADB_SERVER_SOCKET": "tcp:15037"}, want: "127.0.0.1:15037", source: "ADB_SERVER_SOCKET"},
		{name: "tcp host", env: map[string]string{"ADB_SERVER_SOCKET": "tcp:localhost:15037", "ANDROID_ADB_SERVER_PORT": "5037"}, want: "127.0.0.1:15037", source: "ADB_SERVER_SOCKET"},
		{name: "legacy environment", env: map[string]string{"ANDROID_ADB_SERVER_ADDRESS": "127.0.0.2", "ANDROID_ADB_SERVER_PORT": "15037"}, want: "127.0.0.2:15037", source: "ANDROID_ADB_SERVER_ADDRESS/PORT"},
		{name: "ipv6", explicit: "[::1]:5037", want: "[::1]:5037", source: "-server"},
		{name: "unsupported socket", env: map[string]string{"ADB_SERVER_SOCKET": "localabstract:adb"}, invalid: true},
		{name: "zero port", explicit: "127.0.0.1:0", invalid: true},
		{name: "large port", explicit: "127.0.0.1:65536", invalid: true},
		{name: "missing host", explicit: ":5037", invalid: true},
		{name: "non numeric port", explicit: "127.0.0.1:adb", invalid: true},
		{name: "remote", explicit: "192.0.2.1:5037", invalid: true},
		{name: "ambiguous hostname", explicit: "adb.example:5037", invalid: true},
		{name: "unspecified", explicit: "0.0.0.0:5037", invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveEndpoint(tc.explicit, func(name string) string { return tc.env[name] })
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid endpoint accepted")
				}
				return
			}
			if err != nil || got.address != tc.want || got.source != tc.source {
				t.Fatalf("endpoint=%+v err=%v", got, err)
			}
			if tc.name == "ipv6" && got.host != "[::1]" {
				t.Fatalf("ADB host not bracketed: %s", got.host)
			}
		})
	}
}

func TestCLIQueryPinsSameEndpoint(t *testing.T) {
	ep, err := resolveEndpoint("127.0.0.1:15037", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	query := cliQuery("official-adb", ep, time.Second, func(_ context.Context, req core.ExecRequest) (*core.ExecResult, error) {
		if req.Command != "official-adb" || !reflect.DeepEqual(req.Args, []string{"-H", "127.0.0.1", "-P", "15037", "devices", "-l"}) || req.Timeout != time.Second {
			t.Fatalf("wrong CLI endpoint: %+v", req)
		}
		return &core.ExecResult{Stdout: "List of devices attached\n"}, nil
	})
	if _, err := query(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, result := range []*core.ExecResult{nil, {ExitCode: 1}, {Stdout: "unknown command"}} {
		q := cliQuery("adb", ep, time.Second, func(context.Context, core.ExecRequest) (*core.ExecResult, error) { return result, nil })
		if _, err := q(context.Background()); err == nil {
			t.Fatal("unsuccessful CLI output accepted")
		}
	}
}

func TestPairedBenchmarkAlternatesAndExcludesSetupAndWarmup(t *testing.T) {
	order := []string{}
	query := func(name, output string) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { order = append(order, name); return output, nil }
	}
	r := report{Warmup: 2, Iterations: 3}
	err := runBenchmark(context.Background(), &r, time.Second, query("C", "List of devices attached\r\nB device model:B\r\nA  device  model:A\r\n"), query("S", "A\tdevice model:A\nB\tdevice model:B\n"))
	if err != nil || !reflect.DeepEqual(order, []string{"C", "S", "C", "S", "S", "C", "C", "S", "S", "C", "C", "S"}) || len(r.Samples) != 3 || r.MismatchedPairs != 0 || r.DeviceSetChanged {
		t.Fatalf("order=%v report=%+v err=%v", order, r, err)
	}
	// Alternation, warmup exclusion and retained samples are asserted here; the
	// measured durations deliberately are not. A fake runner returns instantly,
	// and a host whose monotonic clock has a coarse tick may report 0 ns, so a
	// positive MaxNS would be a platform assertion rather than a product one.
	if r.Samples[0].First != "cli" || r.Samples[1].First != "socket" || r.Samples[2].First != "cli" {
		t.Fatalf("missing alternation evidence: %+v", r)
	}
	bytes, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(bytes), `"cliNs"`) || !strings.Contains(string(bytes), `"samples"`) {
		t.Fatalf("missing raw JSON evidence: %s %v", bytes, err)
	}
}

func TestBenchmarkDetectsChangesBetweenEveryPair(t *testing.T) {
	calls := 0
	cli := func(context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "List of devices attached\nA device", nil
		}
		return "List of devices attached\nB device", nil
	}
	socketCalls := 0
	socket := func(context.Context) (string, error) {
		socketCalls++
		if socketCalls < 3 {
			return "A\tdevice", nil
		}
		return "B\tdevice", nil
	}
	r := report{Iterations: 2}
	if err := runBenchmark(context.Background(), &r, time.Second, cli, socket); err != nil || !r.DeviceSetChanged || r.MismatchedPairs != 0 {
		t.Fatalf("stable pairs hid changing device set: %+v %v", r, err)
	}
}

func TestBenchmarkDetectsMismatchAndRetainsPartialSamplesOnFailure(t *testing.T) {
	calls := 0
	cli := func(context.Context) (string, error) { return "List of devices attached\nA device", nil }
	socket := func(context.Context) (string, error) {
		calls++
		if calls == 3 {
			return "", errors.New("socket failure")
		}
		return "B device", nil
	}
	r := report{Iterations: 3}
	if err := runBenchmark(context.Background(), &r, time.Second, cli, socket); err == nil || len(r.Samples) != 1 || r.MismatchedPairs != 1 || !r.DeviceSetChanged || r.Samples[0].First != "cli" {
		t.Fatalf("partial evidence lost: %+v %v", r, err)
	}
}

// TestSummarizeIsDeterministicForSyntheticDurations pins the statistics that the
// paired benchmark reports. It uses explicit synthetic durations instead of
// measured samples, so the assertions do not depend on the host clock tick (an
// instantaneous fake runner can legitimately measure 0 ns on Windows).
func TestSummarizeIsDeterministicForSyntheticDurations(t *testing.T) {
	values := []time.Duration{5 * time.Millisecond, time.Millisecond, 3 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
	got := summarize(values)
	want := summary{MinNS: int64(time.Millisecond), MedianNS: int64(3 * time.Millisecond), P95NS: int64(5 * time.Millisecond), MaxNS: int64(5 * time.Millisecond)}
	if got != want {
		t.Fatalf("summarize=%+v want %+v", got, want)
	}
	if !reflect.DeepEqual(values, []time.Duration{5 * time.Millisecond, time.Millisecond, 3 * time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}) {
		t.Fatal("summarize must not reorder the caller slice")
	}

	ordered := []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}
	for _, tc := range []struct {
		name     string
		p        float64
		values   []time.Duration
		expected time.Duration
	}{
		{name: "lower bound", p: 0, values: ordered, expected: time.Millisecond},
		{name: "median", p: .50, values: ordered, expected: 2 * time.Millisecond},
		{name: "upper bound", p: 1, values: ordered, expected: 3 * time.Millisecond},
		{name: "empty", p: .95, values: nil, expected: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentile(tc.values, tc.p); got != tc.expected {
				t.Fatalf("percentile(%v, %v)=%v want %v", tc.values, tc.p, got, tc.expected)
			}
		})
	}
}

func TestRunQueryPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, _, err := runQuery(ctx, time.Second, func(context.Context) (string, error) { cancel(); return "ignored", nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden: %v", err)
	}
	if summarize(nil) != (summary{}) {
		t.Fatal("empty summary must not panic")
	}
}

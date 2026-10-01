package shell

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogcatProcessHelper(t *testing.T) {
	switch os.Args[len(os.Args)-1] {
	case "logcat-burst":
		for i := 0; i < 2048; i++ {
			fmt.Fprintf(os.Stdout, "01-02 03:04:05.000 123 456 I TestTag: message-%d\n", i)
		}
		fmt.Fprint(os.Stdout, "01-02 03:04:05.000 123 456 I TestTag: final-partial")
		fmt.Fprintln(os.Stderr, "last stderr diagnostic")
		os.Exit(0)
	case "logcat-long-line":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 2*1024*1024))
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "logcat-wait":
		fmt.Fprintln(os.Stdout, "01-02 03:04:05.000 123 456 I TestTag: ready")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
}

func lifecycleService() (*LogcatService, *[]LogcatEntry, *[]string, *sync.Mutex) {
	var entries []LogcatEntry
	var events []string
	mu := &sync.Mutex{}
	s := &LogcatService{streams: make(map[string]*logcatStream), emitBatch: func(batch []LogcatEntry) {
		mu.Lock()
		defer mu.Unlock()
		entries = append(entries, batch...)
		events = append(events, "batch")
	}, emitStatus: func(e LogcatStatusEvent) { mu.Lock(); defer mu.Unlock(); events = append(events, e.Status) }}
	return s, &entries, &events, mu
}

func startHelperStream(t *testing.T, s *LogcatService, mode string, ctx context.Context) *logcatStream {
	t.Helper()
	childCtx, cancel := context.WithCancel(ctx)
	_, processCancel := context.WithCancel(childCtx)
	cmd := core.NewCommandContext(childCtx, os.Args[0], "-test.run=^TestLogcatProcessHelper$", "--", mode)
	stream, err := s.startCommand(
		childCtx,
		"TEST-TV",
		cmd,
		cancel,
		newProcessNameCache(nil),
		processCancel,
	)
	if err != nil {
		processCancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); processCancel(); <-stream.finished })
	return stream
}

func waitStream(t *testing.T, stream *logcatStream) {
	t.Helper()
	select {
	case <-stream.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("stream lifecycle did not finish")
	}
}

func TestLogcatNaturalExitDrainsFinalBatchBeforeStatus(t *testing.T) {
	s, entries, events, mu := lifecycleService()
	stream := startHelperStream(t, s, "logcat-burst", context.Background())
	waitStream(t, stream)
	mu.Lock()
	defer mu.Unlock()
	if len(*entries) != 2050 {
		t.Fatalf("lost output: %d entries", len(*entries))
	}
	found := false
	for _, e := range *entries {
		if e.Message == "final-partial" {
			found = true
		}
	}
	if !found {
		t.Fatal("final partial line lost")
	}
	if (*events)[0] != "started" || (*events)[len(*events)-1] != "stopped" {
		t.Fatalf("bad status ordering: %#v", *events)
	}
	if stream.cmd.ProcessState == nil || !stream.cmd.ProcessState.Exited() {
		t.Fatal("child not reaped")
	}
}

func TestLogcatReaderFailureIsErrorNotSuccessfulStop(t *testing.T) {
	s, entries, events, mu := lifecycleService()
	stream := startHelperStream(t, s, "logcat-long-line", context.Background())
	waitStream(t, stream)
	mu.Lock()
	defer mu.Unlock()
	if (*events)[len(*events)-1] != "error" {
		t.Fatalf("reader failure hidden: %#v", *events)
	}
	if len(*entries) != 1 || (*entries)[0].Level != "E" || !strings.Contains((*entries)[0].Message, "reader failed") {
		t.Fatalf("missing reader diagnostic: %#v", *entries)
	}
}

func TestLogcatStopReapsBeforeReturningAndFlushes(t *testing.T) {
	s, _, events, mu := lifecycleService()
	stream := startHelperStream(t, s, "logcat-wait", context.Background())
	if err := s.StopStream("TEST-TV"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.finished:
	default:
		t.Fatal("Stop returned before cleanup")
	}
	if stream.cmd.ProcessState == nil {
		t.Fatal("Stop did not wait for process reaping")
	}
	mu.Lock()
	defer mu.Unlock()
	if (*events)[len(*events)-1] != "stopped" {
		t.Fatalf("intentional stop reported as failure: %#v", *events)
	}
}

func TestLogcatContextCancellationDoesNotEmitFalseError(t *testing.T) {
	s, entries, events, mu := lifecycleService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := startHelperStream(t, s, "logcat-wait", ctx)
	cancel()
	waitStream(t, stream)
	mu.Lock()
	defer mu.Unlock()
	if (*events)[len(*events)-1] != "stopped" {
		t.Fatalf("cancel reported as failure: %#v", *events)
	}
	for _, entry := range *entries {
		if entry.Level == "E" {
			t.Fatal("false cancellation error")
		}
	}
}

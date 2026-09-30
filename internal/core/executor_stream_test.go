package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamingProcessHelper(t *testing.T) {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "stream-burst":
		for i := 0; i < 4096; i++ {
			fmt.Fprintf(os.Stdout, "stdout-%04d\n", i)
			fmt.Fprintf(os.Stderr, "stderr-%04d\n", i)
		}
		fmt.Fprint(os.Stdout, "final-stdout")
		fmt.Fprint(os.Stderr, "final-stderr")
		os.Exit(0)
	case "stream-long-line":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 2*maxCapturedOutput))
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "stream-wait":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
}

func helperStreamRequest(mode string) StreamingExecRequest {
	return StreamingExecRequest{Command: os.Args[0], Args: []string{"-test.run=^TestStreamingProcessHelper$", "--", mode}}
}

func TestPipeStreamingDrainsBothFinalTails(t *testing.T) {
	var lines atomic.Int64
	req := helperStreamRequest("stream-burst")
	req.OnStderrLine = func(string) { lines.Add(1) }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runWithPipes(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if lines.Load() != 8194 {
		t.Fatalf("lost lines: %d", lines.Load())
	}
	if !strings.HasSuffix(result.Stdout, "final-stdout\n") || !strings.HasSuffix(result.Stderr, "final-stderr\n") {
		t.Fatalf("final output lost: %#v", result)
	}
	if !strings.Contains(result.Stderr, "stderr-0000") || result.Duration <= 0 {
		t.Fatal("stderr or timing missing")
	}
}

func TestPipeStreamingRejectsReaderFailureAndReapsChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	_, err := runWithPipes(ctx, helperStreamRequest("stream-long-line"))
	if err == nil || !strings.Contains(err.Error(), "failed to read process output") {
		t.Fatalf("hidden reader error: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("reader failure left process running")
	}
}

func TestPipeStreamingCancellationDoesNotWaitForOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runWithPipes(ctx, helperStreamRequest("stream-wait"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("cancelled process was not reaped")
	}
}

func TestProcessLineWriterChunkBoundariesAndFinalPartial(t *testing.T) {
	var lines []string
	w := NewProcessLineWriter(func(s string) { lines = append(lines, s) }, nil)
	for _, chunk := range []string{"a", "b\rc", "d\ne", "f"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if strings.Join(lines, "|") != "ab|cd|ef" {
		t.Fatalf("bad split: %#v", lines)
	}
}

func TestCapturedOutputIsBoundedAndKeepsTail(t *testing.T) {
	var b boundedOutput
	for i := 0; i < 5000; i++ {
		b.append(strings.Repeat("x", 1024))
	}
	b.append("last-diagnostic")
	if b.buffer.Len() > 2*maxCapturedOutput || len(b.String()) != maxCapturedOutput || !strings.HasSuffix(b.String(), "last-diagnostic") {
		t.Fatal("capture bound or final diagnostic lost")
	}
}

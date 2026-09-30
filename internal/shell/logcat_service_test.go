package shell

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBuildLogcatFilterSpec(t *testing.T) {
	cases := []struct {
		name     string
		levels   string
		tag      string
		expected string
	}{
		{"empty both", "", "", ""},
		{"only levels", "W", "", "*:W"},
		{"only tag", "", "MyTag", "MyTag:V"},
		{"both", "E", "MyTag", "MyTag:E"},
		{"levels lowercased to upper", "w", "MyTag", "MyTag:W"},
		{"levels trimmed", "  E  ", "MyTag", "MyTag:E"},
		{"tag trimmed", "I", "  MyTag  ", "MyTag:I"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildLogcatFilterSpec(c.levels, c.tag)
			if got != c.expected {
				t.Errorf("buildLogcatFilterSpec(%q, %q) = %q, want %q", c.levels, c.tag, got, c.expected)
			}
		})
	}
}

func TestParseLogcatEntry_AllLevels(t *testing.T) {
	cases := []struct {
		level   string
		message string
	}{
		{"V", "verbose"},
		{"D", "debug"},
		{"I", "info"},
		{"W", "warn"},
		{"E", "error"},
		{"F", "fatal"},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			line := "11-23 12:34:56.789 1000 2000 " + c.level + " TestTag: " + c.message
			entry := parseLogcatEntry("ABC123", line)
			if entry.Level != c.level {
				t.Errorf("expected level %q, got %q", c.level, entry.Level)
			}
			if entry.Message != c.message {
				t.Errorf("expected message %q, got %q", c.message, entry.Message)
			}
			if entry.Serial != "ABC123" {
				t.Errorf("expected serial 'ABC123', got %q", entry.Serial)
			}
			if entry.Tag != "TestTag" {
				t.Errorf("expected tag 'TestTag', got %q", entry.Tag)
			}
		})
	}
}

func TestParseLogcatEntry_NoMatchKeepsRawMessage(t *testing.T) {
	line := "this is not a logcat-formatted line"
	entry := parseLogcatEntry("ABC123", line)
	if entry.Level != "V" {
		t.Errorf("expected fallback level V for non-matching line, got %q", entry.Level)
	}
	if entry.Message != line {
		t.Errorf("expected raw message preserved, got %q", entry.Message)
	}
	if entry.Tag != "" {
		t.Errorf("expected empty tag for non-matching line, got %q", entry.Tag)
	}
}

func TestParseLogcatEntry_PreservesRawField(t *testing.T) {
	line := "01-02 03:04:05.000 123 456 I TestTag: hello world"
	entry := parseLogcatEntry("ABC123", line)
	if entry.Raw != line {
		t.Errorf("expected raw field to equal input, got %q", entry.Raw)
	}
	if entry.PID != "123" {
		t.Errorf("expected pid '123', got %q", entry.PID)
	}
	if entry.TID != "456" {
		t.Errorf("expected tid '456', got %q", entry.TID)
	}
}

func FuzzParseLogcatEntry(f *testing.F) {
	f.Add("01-02 03:04:05.000 123 456 I TestTag: hello world")
	f.Add("vendor-specific unstructured output")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		entry := parseLogcatEntry("FUZZ", input)
		if entry.Serial != "FUZZ" {
			t.Fatalf("serial changed during parsing: %q", entry.Serial)
		}
	})
}


func TestRunLogcatBatcherFlushesAtMaxSizeAndPreservesOrdering(t *testing.T) {
	entries := make(chan LogcatEntry, 5)
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		entries <- LogcatEntry{ID: id, Serial: "ABC123"}
	}
	close(entries)

	var batches [][]LogcatEntry
	runLogcatBatcher(entries, time.Hour, 3, func(batch []LogcatEntry) {
		batches = append(batches, batch)
	})

	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}

	got := []string{
		batches[0][0].ID,
		batches[0][1].ID,
		batches[0][2].ID,
		batches[1][0].ID,
		batches[1][1].ID,
	}
	want := []string{"1", "2", "3", "4", "5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d out of order: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunLogcatBatcherFlushesFinalPartialBatch(t *testing.T) {
	entries := make(chan LogcatEntry, 2)
	entries <- LogcatEntry{ID: "1", Serial: "ABC123"}
	entries <- LogcatEntry{ID: "2", Serial: "ABC123"}
	close(entries)

	var batches [][]LogcatEntry
	runLogcatBatcher(entries, time.Hour, 10, func(batch []LogcatEntry) {
		batches = append(batches, batch)
	})

	if len(batches) != 1 {
		t.Fatalf("expected 1 final batch, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Fatalf("expected 2 entries in final batch, got %d", len(batches[0]))
	}
}

func TestRunLogcatBatcherFlushesOnInterval(t *testing.T) {
	entries := make(chan LogcatEntry, 1)
	emitted := make(chan []LogcatEntry, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		runLogcatBatcher(entries, 10*time.Millisecond, 10, func(batch []LogcatEntry) {
			emitted <- batch
		})
	}()

	entries <- LogcatEntry{ID: "interval", Serial: "ABC123"}

	select {
	case batch := <-emitted:
		if len(batch) != 1 || batch[0].ID != "interval" {
			t.Fatalf("unexpected interval batch: %#v", batch)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for interval flush")
	}

	close(entries)
	<-done
}

func TestCloseStreamFlushesPendingBatchOnCancellation(t *testing.T) {
	var mu sync.Mutex
	var batches [][]LogcatEntry
	var statuses []LogcatStatusEvent

	service := &LogcatService{
		streams: make(map[string]*logcatStream),
		emitBatch: func(batch []LogcatEntry) {
			mu.Lock()
			defer mu.Unlock()
			batches = append(batches, batch)
		},
		emitStatus: func(event LogcatStatusEvent) {
			mu.Lock()
			defer mu.Unlock()
			statuses = append(statuses, event)
		},
	}

	stream := &logcatStream{
		serial:    "ABC123",
		entries:   make(chan LogcatEntry, 4),
		batchDone: make(chan struct{}),
	}
	service.streams[stream.serial] = stream
	go service.emitLogcatBatches(stream)

	stream.entries <- LogcatEntry{ID: "pending", Serial: stream.serial}
	stream.stopping.Store(true)
	service.closeStream(stream, "stopped", context.Canceled)

	mu.Lock()
	defer mu.Unlock()

	if len(batches) != 1 || len(batches[0]) != 1 || batches[0][0].ID != "pending" {
		t.Fatalf("pending batch was not flushed on cancellation: %#v", batches)
	}
	if len(statuses) != 1 || statuses[0].Status != "stopped" {
		t.Fatalf("expected stopped status after cancellation, got %#v", statuses)
	}
	if _, ok := service.streams[stream.serial]; ok {
		t.Fatal("cancelled stream was not removed from active streams")
	}
}

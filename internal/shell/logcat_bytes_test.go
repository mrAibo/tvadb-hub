package shell

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These tests pin the byte-volume ceilings of the logcat pipeline. The old
// behaviour was bounded only by entry count (512 queued + 128 batched), which
// permits hundreds of MiB when each line is near the 1 MiB per-line hard cap.
// They use small synthetic limits/budgets; no exploit payloads or devices.

func TestLogcatEntryBytesCountsStringContentIncludingMetadata(t *testing.T) {
	entry := LogcatEntry{Message: "hello", Raw: "hello-world", ProcessName: "process", Tag: "tag"}
	if got := logcatEntryBytes(entry); got != len("hello")+len("hello-world")+len("process")+len("tag") {
		t.Fatalf("entry byte size = %d, want %d", got, len("hello")+len("hello-world")+len("process")+len("tag"))
	}
	if got := logcatEntryBytes(LogcatEntry{}); got != 0 {
		t.Fatalf("empty entry byte size = %d, want 0", got)
	}
}

func TestLogcatByteQueueAccountsForMetadataOnlyEntries(t *testing.T) {
	var stopping atomic.Bool
	q := newLogcatByteQueue(make(chan LogcatEntry, 1), &stopping, 16)
	if sent, tooLarge := q.send(LogcatEntry{ProcessName: strings.Repeat("p", 17)}, context.Background()); sent || !tooLarge {
		t.Fatalf("oversized metadata admitted: sent=%v tooLarge=%v", sent, tooLarge)
	}
}

func TestLogcatByteQueueDoesNotMissAnAlreadyReleasedCredit(t *testing.T) {
	var stopping atomic.Bool
	q := newLogcatByteQueue(make(chan LogcatEntry, 1), &stopping, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if !q.waitForRoom(ctx, 8) {
		t.Fatal("free credits must not wait for another release notification")
	}
}

func TestLogcatByteQueueCancelsWhenChannelIsFull(t *testing.T) {
	var stopping atomic.Bool
	entries := make(chan LogcatEntry, 1)
	q := newLogcatByteQueue(entries, &stopping, 64)
	q.send(LogcatEntry{Message: "first"}, context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { sent, _ := q.send(LogcatEntry{Message: "second"}, ctx); done <- sent }()
	deadline := time.Now().Add(time.Second)
	for {
		q.mu.Lock()
		pending := q.inFlight
		q.mu.Unlock()
		if pending == len("first")+len("second") {
			break // The enqueue reserved credits and is blocked on the full channel.
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("second enqueue did not reach the full channel")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case sent := <-done:
		if sent {
			t.Fatal("cancelled enqueue must not replace the queued entry")
		}
	case <-time.After(time.Second):
		t.Fatal("full channel prevented cancellation")
	}
	q.mu.Lock()
	retained := q.inFlight
	q.mu.Unlock()
	if retained != len("first") {
		t.Fatalf("cancelled enqueue leaked credits: %d", retained)
	}
	q.release(logcatEntryBytes(<-entries))
}

func TestLogcatByteQueueBlocksProducerUntilConsumerReleasesCredits(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	var stopping atomic.Bool
	q := newLogcatByteQueue(entries, &stopping, 16)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		sent, tooLarge := q.send(LogcatEntry{Message: "01234567"}, ctx)
		if !sent || tooLarge {
			t.Fatalf("entry %d not admitted: sent=%v tooLarge=%v", i, sent, tooLarge)
		}
	}
	if q.inFlight != 16 {
		t.Fatalf("in-flight bytes = %d, want 16", q.inFlight)
	}

	blocked := make(chan bool, 1)
	go func() {
		sent, _ := q.send(LogcatEntry{Message: "01234567"}, ctx)
		blocked <- sent
	}()

	select {
	case sent := <-blocked:
		t.Fatalf("producer admitted an entry over the byte budget: sent=%v", sent)
	case <-time.After(50 * time.Millisecond):
	}

	<-entries // consumer receives one entry and returns exactly its bytes
	q.release(8)

	select {
	case sent := <-blocked:
		if !sent {
			t.Fatal("producer did not resume after credits were released")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked producer never resumed")
	}

	<-entries
	q.release(8)
	<-entries
	q.release(8)
	if q.inFlight != 0 {
		t.Fatalf("credit accounting leaked: in-flight = %d, want 0", q.inFlight)
	}
}

func TestLogcatByteQueueWaitWakesOnContextCancellation(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	var stopping atomic.Bool
	q := newLogcatByteQueue(entries, &stopping, 8)
	q.send(LogcatEntry{Message: "01234567"}, context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := make(chan bool, 1)
	go func() {
		sent, _ := q.send(LogcatEntry{Message: "01234567"}, ctx)
		result <- sent
	}()

	select {
	case sent := <-result:
		if sent {
			t.Fatal("cancelled producer admitted an entry")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled producer stayed blocked on the byte budget")
	}
	if q.inFlight != 8 {
		t.Fatalf("failed reservation changed in-flight bytes: %d", q.inFlight)
	}
}

func TestLogcatByteQueueStopWakesBlockedProducer(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	var stopping atomic.Bool
	q := newLogcatByteQueue(entries, &stopping, 8)
	q.send(LogcatEntry{Message: "01234567"}, context.Background())

	woke := make(chan bool, 1)
	go func() {
		sent, _ := q.send(LogcatEntry{Message: "01234567"}, context.Background())
		woke <- sent
	}()
	time.Sleep(50 * time.Millisecond)

	stopping.Store(true)
	q.wakeWaiters()

	select {
	case sent := <-woke:
		if sent {
			t.Fatal("stopping queue admitted an entry")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not wake a producer blocked on the byte budget")
	}
}

func TestLogcatByteQueueOversizedEntryIsRejectedNotWaitedOn(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	var stopping atomic.Bool
	q := newLogcatByteQueue(entries, &stopping, 8)

	done := make(chan bool, 1)
	go func() {
		sent, tooLarge := q.send(LogcatEntry{Message: "0123456789"}, context.Background())
		done <- sent || !tooLarge
	}()

	select {
	case failed := <-done:
		if failed {
			t.Fatal("entry larger than the whole budget was not flagged too large")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("oversized entry waited instead of failing explicitly")
	}
	if len(entries) != 0 || q.inFlight != 0 {
		t.Fatalf("oversized entry was enqueued or retained: len=%d inFlight=%d", len(entries), q.inFlight)
	}
}

func TestLogcatByteQueueConcurrentProducersStayWithinBudget(t *testing.T) {
	const (
		limit   = 4096
		workers = 8
		sends   = 200
	)
	entries := make(chan LogcatEntry, 64)
	var stopping atomic.Bool
	q := newLogcatByteQueue(entries, &stopping, limit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var maxSeen atomic.Int64
	overshoot := make(chan int, 1)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			q.mu.Lock()
			seen := q.inFlight
			q.mu.Unlock()
			if int64(seen) > maxSeen.Load() {
				maxSeen.Store(int64(seen))
			}
			if seen > limit {
				select {
				case overshoot <- seen:
				default:
				}
				return
			}
			time.Sleep(50 * time.Microsecond)
		}
	}()

	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			for i := 0; i < sends; i++ {
				entry := LogcatEntry{Message: strings.Repeat("x", 64)}
				if sent, _ := q.send(entry, ctx); !sent {
					errs <- fmt.Errorf("worker %d send %d rejected", id, i)
					return
				}
				<-entries
				q.release(logcatEntryBytes(entry))
			}
			errs <- nil
		}(w)
	}

	for w := 0; w < workers; w++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent producers deadlocked")
		}
	}
	cancel()
	<-monitorDone

	select {
	case over := <-overshoot:
		t.Fatalf("byte budget exceeded under concurrency: %d > %d", over, limit)
	default:
	}
	if q.inFlight != 0 {
		t.Fatalf("credit accounting leaked: in-flight = %d", q.inFlight)
	}
}

func TestLogcatStreamSendReportsOversizedEntryAsFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entries := make(chan LogcatEntry, 4)
	stream := &logcatStream{
		serial:  "TEST-TV",
		entries: entries,
		ctx:     ctx,
		cancel:  cancel,
	}
	stream.queue = newLogcatByteQueue(stream.entries, &stream.stopping, 16)

	err := stream.send(LogcatEntry{Message: strings.Repeat("x", 17)})
	if err == nil {
		t.Fatal("oversized entry was accepted instead of failing explicitly")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if stream.fatalErr == nil || !strings.Contains(stream.fatalErr.Error(), "16 byte queue budget") {
		t.Fatalf("fatal error not retained for the stream diagnostic: %v", stream.fatalErr)
	}
	if len(entries) != 0 {
		t.Fatalf("oversized entry was enqueued: %d", len(entries))
	}
	if q := stream.queue; q.inFlight != 0 {
		t.Fatalf("oversized entry retained credits: %d", q.inFlight)
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("fatal entry did not cancel the actual stream context")
	}
}

func TestRunLogcatBatcherFlushesBeforeByteBound(t *testing.T) {
	entries := make(chan LogcatEntry, 16)
	big := strings.Repeat("x", 600)
	for i := 0; i < 6; i++ {
		entries <- LogcatEntry{ID: fmt.Sprintf("%d", i), Message: big, Raw: big}
	}
	close(entries)

	var batches [][]LogcatEntry
	runLogcatBatcher(entries, time.Hour, 1000, 4096, func(batch []LogcatEntry) {
		batches = append(batches, batch)
	}, nil)

	if len(batches) < 2 {
		t.Fatalf("byte bound did not split batches: %d batches", len(batches))
	}
	total := 0
	for i, batch := range batches {
		bytes := 0
		for _, entry := range batch {
			bytes += logcatEntryBytes(entry)
		}
		if bytes > 4096 {
			t.Fatalf("batch %d exceeded the byte bound: %d > 4096", i, bytes)
		}
		total += len(batch)
	}
	if total != 6 {
		t.Fatalf("entries lost across byte-bounded batches: %d", total)
	}
}

func TestRunLogcatBatcherKeepsSingleEntryLargerThanBatchBound(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	huge := strings.Repeat("y", 2000)
	entries <- LogcatEntry{ID: "huge", Message: huge}
	close(entries)

	var batches [][]LogcatEntry
	runLogcatBatcher(entries, time.Hour, 128, 512, func(batch []LogcatEntry) {
		batches = append(batches, batch)
	}, nil)

	if len(batches) != 1 || len(batches[0]) != 1 || batches[0][0].ID != "huge" {
		t.Fatalf("single over-bound entry was dropped or split: %#v", batches)
	}
}

func TestRunLogcatBatcherReleasesQueueCreditsAsItReceives(t *testing.T) {
	entries := make(chan LogcatEntry, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stopping atomic.Bool
	queue := newLogcatByteQueue(entries, &stopping, 4096)

	for i := 0; i < 3; i++ {
		if sent, _ := queue.send(LogcatEntry{Message: strings.Repeat("z", 100)}, ctx); !sent {
			t.Fatal("entry rejected")
		}
	}
	if queue.inFlight == 0 {
		t.Fatal("credits were not reserved")
	}
	close(entries)

	runLogcatBatcher(entries, time.Hour, 128, 4096, func([]LogcatEntry) {}, queue)

	if queue.inFlight != 0 {
		t.Fatalf("batcher did not release all credits: %d", queue.inFlight)
	}
}

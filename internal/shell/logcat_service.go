package shell

import (
	"ADBKit/internal/binary"
	"ADBKit/internal/core"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventBatch  = "logcat_batch"
	EventStatus = "logcat_status"

	logcatBatchInterval    = 75 * time.Millisecond
	logcatBatchMaxEntries  = 128
	logcatBatchChannelSize = logcatBatchMaxEntries * 4

	// Byte-volume ceilings for the logcat pipeline. The entry count alone
	// (512 queued + 128 batched) bounds nothing useful because a single line may
	// be up to 1 MiB (core.maxCapturedOutput), so the queue is additionally
	// bounded by UTF-8 string-content bytes, including metadata. A producer reserves
	// bytes before enqueueing and the batcher releases them as it receives, so
	// retained bytes never exceed the budget; backpressure (blocking the adb
	// output copier) is preserved rather than replaced by dropping entries.
	logcatQueueMaxBytes = 4 << 20
	logcatBatchMaxBytes = 256 << 10
)

// logcatEntryBytes counts UTF-8 string content, including metadata. Raw and
// Message may share storage, so this is conservative for string data, not an
// exact heap or JSON-wire size (JSON escaping and fixed field overhead differ).
// Entries stay immutable between reservation and release.
func logcatEntryBytes(entry LogcatEntry) int {
	return len(entry.ID) + len(entry.Serial) + len(entry.Date) + len(entry.Time) +
		len(entry.PID) + len(entry.TID) + len(entry.ProcessName) + len(entry.Level) +
		len(entry.Tag) + len(entry.Message) + len(entry.Raw) + len(entry.Timestamp)
}

// logcatByteQueue is a byte-credit wrapper around the entry channel. Credits are
// reserved atomically under mu; a producer that cannot fit an entry waits for a
// release or for cancellation/stop, so concurrent producers cannot partially
// acquire a reservation.
type logcatByteQueue struct {
	entries chan LogcatEntry
	limit   int

	mu       sync.Mutex
	inFlight int
	stopping *atomic.Bool
	waiterID int
	waiters  map[int]chan struct{}
}

func newLogcatByteQueue(entries chan LogcatEntry, stop *atomic.Bool, limit int) *logcatByteQueue {
	return &logcatByteQueue{
		entries:  entries,
		limit:    limit,
		stopping: stop,
		waiters:  make(map[int]chan struct{}),
	}
}

// reserve admits one entry of size bytes. ok=false means either the queue is
// stopping/cancelled, or the entry itself is too large to ever fit (tooLarge).
func (q *logcatByteQueue) reserve(ctx context.Context, size int) (ok, tooLarge bool) {
	if q.stopping.Load() || ctx.Err() != nil {
		return false, false
	}
	if size > q.limit {
		return false, true
	}
	for {
		q.mu.Lock()
		if q.stopping.Load() || ctx.Err() != nil {
			q.mu.Unlock()
			return false, false
		}
		if q.inFlight+size <= q.limit {
			q.inFlight += size
			q.mu.Unlock()
			return true, false
		}
		q.mu.Unlock()

		if !q.waitForRoom(ctx, size) {
			return false, false
		}
	}
}

// waitForRoom blocks until a credit is released or ctx/stop makes progress
// impossible. It never holds mu while waiting.
func (q *logcatByteQueue) waitForRoom(ctx context.Context, size int) bool {
	q.mu.Lock()
	if q.stopping.Load() || ctx.Err() != nil {
		q.mu.Unlock()
		return false
	}
	// A release may precede registration; recheck under the same lock to avoid
	// waiting forever for a notification that already happened.
	if q.inFlight+size <= q.limit {
		q.mu.Unlock()
		return true
	}
	ch := make(chan struct{})
	q.waiterID++
	id := q.waiterID
	q.waiters[id] = ch
	q.mu.Unlock()

	select {
	case <-ctx.Done():
	case <-ch:
	}
	q.cancelWaiter(id)
	return !q.stopping.Load() && ctx.Err() == nil
}

func (q *logcatByteQueue) cancelWaiter(id int) {
	q.mu.Lock()
	if ch, ok := q.waiters[id]; ok {
		delete(q.waiters, id)
		close(ch)
	}
	q.mu.Unlock()
}

// release returns size bytes to the budget and wakes every waiter. It is called
// exactly once per admitted entry, when the consumer receives it.
func (q *logcatByteQueue) release(size int) {
	q.mu.Lock()
	q.inFlight -= size
	q.mu.Unlock()
	q.wakeWaiters()
}

func (q *logcatByteQueue) wakeWaiters() {
	q.mu.Lock()
	for id, ch := range q.waiters {
		delete(q.waiters, id)
		close(ch)
	}
	q.mu.Unlock()
}

// send reserves and enqueues one entry. It returns false when the entry was not
// admitted: the queue is stopping/cancelled (drop, already shutting down), the
// context is done, or the single entry exceeds the whole queue budget
// (tooLarge, reported as a stream error instead of waiting forever or silently
// discarding the line).
func (q *logcatByteQueue) send(entry LogcatEntry, ctx context.Context) (sent, tooLarge bool) {
	size := logcatEntryBytes(entry)
	ok, tooLarge := q.reserve(ctx, size)
	if !ok {
		return false, tooLarge
	}
	if q.stopping.Load() {
		q.release(size)
		return false, false
	}
	select {
	case q.entries <- entry:
		return true, false
	case <-ctx.Done():
		q.release(size)
		return false, false
	}
}

var logcatPattern = regexp.MustCompile(`^(\d{2}-\d{2})\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+(\d+)\s+(\d+)\s+([VDIWEF])\s+(.+?):\s?(.*)$`)

type LogcatEntry struct {
	ID          string `json:"id"`
	Serial      string `json:"serial"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	PID         string `json:"pid"`
	TID         string `json:"tid"`
	ProcessName string `json:"processName,omitempty"`
	Level       string `json:"level"`
	Tag         string `json:"tag"`
	Message     string `json:"message"`
	Raw         string `json:"raw"`
	Timestamp   string `json:"timestamp"`
}

type LogcatStatusEvent struct {
	Serial string `json:"serial"`
	Status string `json:"status"`
}

type logcatStream struct {
	serial        string
	cmd           *exec.Cmd
	stdout        *core.ProcessLineWriter
	stderr        *core.ProcessLineWriter
	entries       chan LogcatEntry
	queue         *logcatByteQueue
	batchDone     chan struct{}
	finished      chan struct{}
	cancel        context.CancelFunc
	ctx           context.Context
	once          sync.Once
	stopping      atomic.Bool
	fatalMu       sync.Mutex
	fatalErr      error
	processNames  *processNameCache
	processCancel context.CancelFunc
}

// send enqueues one parsed entry under the queue byte budget. A nil fatal error
// result means the entry was admitted; while the stream is stopping or its
// context is cancelled the entry is simply dropped, which is the existing
// shutdown semantic.
func (stream *logcatStream) send(entry LogcatEntry) error {
	sent, tooLarge := stream.queue.send(entry, stream.ctx)
	if sent || !tooLarge {
		return nil
	}
	err := fmt.Errorf(
		"logcat entry of %d bytes exceeds the %d byte queue budget",
		logcatEntryBytes(entry), stream.queue.limit,
	)
	stream.fatalMu.Lock()
	if stream.fatalErr == nil {
		stream.fatalErr = err
	}
	stream.fatalMu.Unlock()
	if stream.cancel != nil {
		stream.cancel() // Stop the actual logcat command, not only its ps poller.
	}
	if stream.processCancel != nil {
		stream.processCancel()
	}
	return err
}

type LogcatService struct {
	ctx           context.Context
	binaryService *binary.Service
	getConfig     func() *core.AppConfig

	opMu       sync.Mutex
	mu         sync.Mutex
	streams    map[string]*logcatStream
	emitBatch  func([]LogcatEntry)
	emitStatus func(LogcatStatusEvent)
}

func NewLogcatService(ctx context.Context, binaryService *binary.Service, getConfig func() *core.AppConfig) *LogcatService {
	return &LogcatService{
		ctx:           ctx,
		binaryService: binaryService,
		getConfig:     getConfig,
		streams:       make(map[string]*logcatStream),
		emitBatch: func(entries []LogcatEntry) {
			application.Get().Event.Emit(EventBatch, entries)
		},
		emitStatus: func(event LogcatStatusEvent) {
			application.Get().Event.Emit(EventStatus, event)
		},
	}
}

func (s *LogcatService) StartStream(ctx context.Context, serial string, levels string, tagFilter string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return core.NewOperationError("start_logcat_stream", "Device serial is required", "serial must not be empty", false)
	}

	adbPath, err := s.resolveADBPath()
	if err != nil {
		return err
	}

	if s.hasStream(trimmedSerial) {
		if err := s.stopStream(trimmedSerial); err != nil {
			return err
		}
	}

	args := []string{"-s", trimmedSerial, "logcat", "-v", "threadtime"}
	filterSpec := buildLogcatFilterSpec(levels, tagFilter)
	if filterSpec != "" {
		args = append(args, filterSpec)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	initialProcessNames, _ := queryLogcatProcessNames(streamCtx, adbPath, trimmedSerial)
	processNames := newProcessNameCache(initialProcessNames)
	processCtx, processCancel := context.WithCancel(streamCtx)
	_, err = s.startCommand(
		streamCtx,
		trimmedSerial,
		core.NewCommandContext(streamCtx, adbPath, args...),
		cancel,
		processNames,
		processCancel,
	)
	if err != nil {
		processCancel()
		return err
	}
	go refreshLogcatProcessNames(processCtx, processNames, adbPath, trimmedSerial)
	return nil
}

// The assigned writers let Wait drain output before closing the batch channel.
func (s *LogcatService) startCommand(
	ctx context.Context,
	serial string,
	cmd *exec.Cmd,
	cancel context.CancelFunc,
	processNames *processNameCache,
	processCancel context.CancelFunc,
) (*logcatStream, error) {
	stream := &logcatStream{
		serial:        serial,
		cmd:           cmd,
		entries:       make(chan LogcatEntry, logcatBatchChannelSize),
		batchDone:     make(chan struct{}),
		finished:      make(chan struct{}),
		cancel:        cancel,
		ctx:           ctx,
		processNames:  processNames,
		processCancel: processCancel,
	}
	stream.queue = newLogcatByteQueue(stream.entries, &stream.stopping, logcatQueueMaxBytes)
	lineWriter := func(isError bool) *core.ProcessLineWriter {
		return core.NewProcessLineWriter(func(line string) {
			entry := parseLogcatEntry(serial, line)
			entry.ProcessName = strings.Clone(stream.processNames.get(entry.PID))
			if isError && entry.Tag == "" {
				entry.Level = "W"
			}
			// An entry beyond the whole queue budget can never be admitted, so
			// it fails the stream explicitly (existing error path) rather than
			// blocking forever or silently dropping the line.
			_ = stream.send(entry)
		}, cancel)
	}
	stream.stdout, stream.stderr = lineWriter(false), lineWriter(true)
	cmd.Stdout, cmd.Stderr = stream.stdout, stream.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, core.NewOperationError("start_logcat_stream", "Failed to start logcat stream", err.Error(), true)
	}

	s.mu.Lock()
	s.streams[serial] = stream
	s.mu.Unlock()

	go s.emitLogcatBatches(stream)

	s.emitStatus(LogcatStatusEvent{
		Serial: serial,
		Status: "started",
	})
	go s.waitForStreamExit(stream)

	return stream, nil
}

func (s *LogcatService) StopStream(serial string) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return s.stopStream(serial)
}

func (s *LogcatService) stopStream(serial string) error {
	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return core.NewOperationError("stop_logcat_stream", "Device serial is required", "serial must not be empty", false)
	}

	s.mu.Lock()
	stream, ok := s.streams[trimmedSerial]
	s.mu.Unlock()
	if !ok {
		return core.NewOperationError("stop_logcat_stream", "Logcat stream was not found", fmt.Sprintf("stream '%s' is not active", trimmedSerial), true)
	}

	stream.stopping.Store(true)
	stream.cancel()
	<-stream.finished // Wait reaps the process and joins output readers first.
	return nil
}

func (s *LogcatService) Shutdown() {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.mu.Lock()
	streams := make([]*logcatStream, 0, len(s.streams))
	for _, stream := range s.streams {
		streams = append(streams, stream)
	}
	s.mu.Unlock()

	for _, stream := range streams {
		stream.stopping.Store(true)
		stream.cancel()
	}
	for _, stream := range streams {
		<-stream.finished
	}
}

func (s *LogcatService) waitForStreamExit(stream *logcatStream) {
	err := stream.cmd.Wait()
	stream.stdout.Flush()
	stream.stderr.Flush()
	readErr := errors.Join(stream.stdout.ReadError(), stream.stderr.ReadError())
	stream.fatalMu.Lock()
	fatalErr := stream.fatalErr
	stream.fatalMu.Unlock()
	if fatalErr != nil {
		// A single over-budget entry aborted the stream; keep that diagnostic
		// instead of the generic reader error.
		err = fatalErr
		stream.stopping.Store(true)
	} else if readErr != nil {
		err = fmt.Errorf("logcat output reader failed: %w", readErr)
	} else if stream.ctx.Err() != nil {
		stream.stopping.Store(true)
	}
	// Kill yang dipicu user (StopStream/Shutdown) membuat Wait() mengembalikan
	// signal error; itu bukan kegagalan jadi tetap dilaporkan sebagai "stopped".
	if err != nil {
		s.closeStream(stream, "error", err)
		return
	}

	s.closeStream(stream, "stopped", nil)
}

func (s *LogcatService) closeStream(stream *logcatStream, status string, streamErr error) {
	stream.once.Do(func() {
		intentionalStop := stream.stopping.Load()
		stream.stopping.Store(true)

		if streamErr != nil && !intentionalStop {
			// Terminal diagnostic after Wait joined the output copiers, when
			// every byte credit is already released. It uses a non-blocking
			// direct enqueue because the queue itself is stopping by now; the
			// full channel is the only failure mode and is already covered by
			// the emitted "error" status, so the queue can never block here.
			entry := LogcatEntry{
				ID:      uuid.NewString(),
				Serial:  stream.serial,
				Level:   "E",
				Message: fmt.Sprintf("logcat stream ended: %s", streamErr.Error()),
				Raw:     streamErr.Error(),
			}
			select {
			case stream.entries <- entry:
			default:
			}
			status = "error"
		} else if intentionalStop {
			status = "stopped"
		}

		if stream.processCancel != nil {
			stream.processCancel()
		}

		// Wait has already joined the output copiers, including final lines.
		close(stream.entries)
		<-stream.batchDone

		s.emitStatus(LogcatStatusEvent{
			Serial: stream.serial,
			Status: status,
		})
		s.mu.Lock()
		if s.streams[stream.serial] == stream {
			delete(s.streams, stream.serial)
		}
		s.mu.Unlock()
		if stream.cancel != nil {
			stream.cancel()
		}
		if stream.finished != nil {
			close(stream.finished)
		}
	})
}

func (s *LogcatService) emitLogcatBatches(stream *logcatStream) {
	defer close(stream.batchDone)

	runLogcatBatcher(
		stream.entries,
		logcatBatchInterval,
		logcatBatchMaxEntries,
		logcatBatchMaxBytes,
		s.emitBatch,
		stream.queue,
	)
}

// runLogcatBatcher drains immutable batches bounded by entry count and string
// content volume. Flush before crossing maxBytes; a single larger entry is emitted
// alone and remains limited by the queue budget. JSON escaping/field overhead is
// not included in these content bytes. Each received entry returns its credits.
func runLogcatBatcher(
	entries <-chan LogcatEntry,
	flushInterval time.Duration,
	maxEntries int,
	maxBytes int,
	emit func([]LogcatEntry),
	queue *logcatByteQueue,
) {
	if flushInterval <= 0 {
		flushInterval = logcatBatchInterval
	}
	if maxEntries <= 0 {
		maxEntries = logcatBatchMaxEntries
	}
	if maxBytes <= 0 {
		maxBytes = logcatBatchMaxBytes
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]LogcatEntry, 0, min(maxEntries, 32))
	batchBytes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}

		// Emit a detached slice so the next batch can safely reuse capacity
		// even if the Wails runtime serializes the payload asynchronously.
		ready := append([]LogcatEntry(nil), batch...)
		emit(ready)
		batch = batch[:0]
		batchBytes = 0
	}

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				flush()
				return
			}
			if queue != nil {
				queue.release(logcatEntryBytes(entry))
			}

			size := logcatEntryBytes(entry)
			if len(batch) > 0 && (len(batch) >= maxEntries || batchBytes+size > maxBytes) {
				flush()
			}
			batch = append(batch, entry)
			batchBytes += size
			if len(batch) >= maxEntries {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *LogcatService) resolveADBPath() (string, error) {
	if s.binaryService == nil {
		return "", core.NewOperationError("resolve_logcat_binary", "Required binary service is not available", "binary service is nil", false)
	}

	if s.getConfig == nil {
		return "", core.NewOperationError("resolve_logcat_binary", "Application config is unavailable", "config getter is nil", false)
	}
	status := s.binaryService.GetBinaryStatus(s.getConfig())
	if status.Adb == nil || status.Adb.Status != core.BinaryReady || status.Adb.Path == "" {
		return "", core.NewOperationError("resolve_logcat_binary", "Required binary is not ready", "binary 'adb' is unavailable", true)
	}

	return status.Adb.Path, nil
}

func (s *LogcatService) hasStream(serial string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.streams[serial]
	return ok
}

func buildLogcatFilterSpec(levels string, tagFilter string) string {
	normalizedLevels := strings.ToUpper(strings.TrimSpace(levels))
	normalizedTag := strings.TrimSpace(tagFilter)

	if normalizedTag == "" {
		if normalizedLevels == "" {
			return ""
		}
		return fmt.Sprintf("*:%s", normalizedLevels)
	}

	if normalizedLevels == "" {
		normalizedLevels = "V"
	}

	return fmt.Sprintf("%s:%s", normalizedTag, normalizedLevels)
}

func parseLogcatEntry(serial string, rawLine string) LogcatEntry {
	trimmedLine := strings.Clone(strings.TrimRight(rawLine, "\r\n"))
	entry := LogcatEntry{
		ID:      uuid.NewString(),
		Serial:  serial,
		Level:   "V",
		Message: trimmedLine,
		Raw:     trimmedLine,
	}

	matches := logcatPattern.FindStringSubmatch(trimmedLine)
	if len(matches) != 8 {
		return entry
	}

	entry.Date = matches[1]
	entry.Time = matches[2]
	entry.PID = matches[3]
	entry.TID = matches[4]
	entry.Level = matches[5]
	entry.Tag = strings.TrimSpace(matches[6])
	entry.Message = matches[7]
	entry.Timestamp = fmt.Sprintf("%s %s", entry.Date, entry.Time)

	return entry
}

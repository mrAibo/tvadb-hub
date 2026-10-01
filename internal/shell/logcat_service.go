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
)

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
	Tag       string `json:"tag"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
	Timestamp string `json:"timestamp"`
}

type LogcatStatusEvent struct {
	Serial string `json:"serial"`
	Status string `json:"status"`
}

type logcatStream struct {
	serial    string
	cmd       *exec.Cmd
	stdout    *core.ProcessLineWriter
	stderr    *core.ProcessLineWriter
	entries   chan LogcatEntry
	batchDone chan struct{}
	finished  chan struct{}
	cancel    context.CancelFunc
	ctx          context.Context
	once         sync.Once
	stopping     atomic.Bool
	processNames *processNameCache
	processCancel context.CancelFunc
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
	stream, err := s.startCommand(
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
		serial:    serial,
		cmd:       cmd,
		entries:   make(chan LogcatEntry, logcatBatchChannelSize),
		batchDone: make(chan struct{}),
		finished:  make(chan struct{}),
		cancel:        cancel,
		ctx:           ctx,
		processNames:  processNames,
		processCancel: processCancel,
	}
	lineWriter := func(isError bool) *core.ProcessLineWriter {
		return core.NewProcessLineWriter(func(line string) {
			entry := parseLogcatEntry(serial, line)
			entry.ProcessName = stream.processNames.get(entry.PID)
			if isError && entry.Tag == "" {
				entry.Level = "W"
			}
			stream.entries <- entry
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
	if readErr != nil {
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
			stream.entries <- LogcatEntry{
				ID:      uuid.NewString(),
				Serial:  stream.serial,
				Level:   "E",
				Message: fmt.Sprintf("logcat stream ended: %s", streamErr.Error()),
				Raw:     streamErr.Error(),
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
		s.emitBatch,
	)
}

func runLogcatBatcher(
	entries <-chan LogcatEntry,
	flushInterval time.Duration,
	maxEntries int,
	emit func([]LogcatEntry),
) {
	if flushInterval <= 0 {
		flushInterval = logcatBatchInterval
	}
	if maxEntries <= 0 {
		maxEntries = logcatBatchMaxEntries
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]LogcatEntry, 0, maxEntries)
	flush := func() {
		if len(batch) == 0 {
			return
		}

		// Emit a detached slice so the next batch can safely reuse capacity
		// even if the Wails runtime serializes the payload asynchronously.
		ready := append([]LogcatEntry(nil), batch...)
		emit(ready)
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				flush()
				return
			}

			batch = append(batch, entry)
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
	trimmedLine := strings.TrimRight(rawLine, "\r\n")
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

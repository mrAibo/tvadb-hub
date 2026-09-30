package shell

import (
	"ADBKit/internal/binary"
	"ADBKit/internal/core"
	"bufio"
	"context"
	"fmt"
	"io"
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
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	entries       chan LogcatEntry
	batchDone     chan struct{}
	readers       sync.WaitGroup
	once          sync.Once
	stopping      atomic.Bool
	processMu     sync.RWMutex
	processNames  map[string]string
	processCancel context.CancelFunc
}

type LogcatService struct {
	ctx           context.Context
	binaryService *binary.Service
	getConfig     func() *core.AppConfig

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
		streams: make(map[string]*logcatStream),
		emitBatch: func(entries []LogcatEntry) {
			application.Get().Event.Emit(EventBatch, entries)
		},
		emitStatus: func(event LogcatStatusEvent) {
			application.Get().Event.Emit(EventStatus, event)
		},
	}
}

func (s *LogcatService) StartStream(ctx context.Context, serial string, levels string, tagFilter string) error {
	trimmedSerial := strings.TrimSpace(serial)
	if trimmedSerial == "" {
		return core.NewOperationError("start_logcat_stream", "Device serial is required", "serial must not be empty", false)
	}

	adbPath, err := s.resolveADBPath()
	if err != nil {
		return err
	}

	if s.hasStream(trimmedSerial) {
		if err := s.StopStream(trimmedSerial); err != nil {
			return err
		}
	}

	initialProcessNames, _ := queryLogcatProcessNames(ctx, adbPath, trimmedSerial)

	args := []string{"-s", trimmedSerial, "logcat", "-v", "threadtime"}
	filterSpec := buildLogcatFilterSpec(levels, tagFilter)
	if filterSpec != "" {
		args = append(args, filterSpec)
	}

	cmd := core.NewCommandContext(ctx, adbPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return core.NewOperationError("start_logcat_stream", "Failed to open logcat stdout", err.Error(), true)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return core.NewOperationError("start_logcat_stream", "Failed to open logcat stderr", err.Error(), true)
	}

	if err := cmd.Start(); err != nil {
		return core.NewOperationError("start_logcat_stream", "Failed to start logcat stream", err.Error(), true)
	}

	processCtx, processCancel := context.WithCancel(ctx)
	stream := &logcatStream{
		serial:        trimmedSerial,
		cmd:           cmd,
		stdout:        stdoutPipe,
		stderr:        stderrPipe,
		entries:       make(chan LogcatEntry, logcatBatchChannelSize),
		batchDone:     make(chan struct{}),
		processNames:  initialProcessNames,
		processCancel: processCancel,
	}

	s.mu.Lock()
	s.streams[trimmedSerial] = stream
	s.mu.Unlock()

	go s.emitLogcatBatches(stream)
	go s.refreshLogcatProcessNames(processCtx, stream, adbPath)

	stream.readers.Add(2)
	go func() {
		defer stream.readers.Done()
		s.readLogcatOutput(stream, stdoutPipe, false)
	}()
	go func() {
		defer stream.readers.Done()
		s.readLogcatOutput(stream, stderrPipe, true)
	}()
	go s.waitForStreamExit(stream)

	s.emitStatus(LogcatStatusEvent{
		Serial: trimmedSerial,
		Status: "started",
	})

	return nil
}

func (s *LogcatService) StopStream(serial string) error {
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
	s.closeStream(stream, "stopped", nil)
	return nil
}

func (s *LogcatService) Shutdown() {
	s.mu.Lock()
	streams := make([]*logcatStream, 0, len(s.streams))
	for _, stream := range s.streams {
		streams = append(streams, stream)
	}
	s.mu.Unlock()

	for _, stream := range streams {
		stream.stopping.Store(true)
		s.closeStream(stream, "stopped", nil)
	}
}

func (s *LogcatService) readLogcatOutput(stream *logcatStream, pipe io.ReadCloser, isError bool) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	for scanner.Scan() {
		entry := parseLogcatEntry(stream.serial, scanner.Text())
		entry.ProcessName = stream.processName(entry.PID)
		// adb menulis diagnostik non-fatal ke stderr (mis. "waiting for device").
		// Hanya tandai sebagai warning bila baris bukan format logcat valid;
		// baris yang sudah terparse mempertahankan level aslinya.
		if isError && entry.Tag == "" {
			entry.Level = "W"
		}
		stream.entries <- entry
	}

	if err := scanner.Err(); err != nil && !stream.stopping.Load() {
		stream.entries <- LogcatEntry{
			ID:      uuid.NewString(),
			Serial:  stream.serial,
			Level:   "E",
			Message: err.Error(),
			Raw:     err.Error(),
		}
	}
}

func (s *LogcatService) waitForStreamExit(stream *logcatStream) {
	err := stream.cmd.Wait()
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

		s.mu.Lock()
		delete(s.streams, stream.serial)
		s.mu.Unlock()

		if stream.processCancel != nil {
			stream.processCancel()
		}

		if stream.stdout != nil {
			_ = stream.stdout.Close()
		}
		if stream.stderr != nil {
			_ = stream.stderr.Close()
		}
		if stream.cmd != nil && stream.cmd.Process != nil {
			_ = core.TerminateProcessTree(stream.cmd)
		}

		// Readers may already hold parsed lines when cancellation closes the
		// pipes. Wait for them before closing the batch channel so those final
		// entries are emitted instead of being dropped.
		stream.readers.Wait()
		close(stream.entries)
		<-stream.batchDone

		s.emitStatus(LogcatStatusEvent{
			Serial: stream.serial,
			Status: status,
		})
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

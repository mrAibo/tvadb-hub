package scrcpy

import (
	"ADBKit/internal/audit"
	"ADBKit/internal/core"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	recordingHelperEnv        = "DROIDSPHERE_RECORDING_HELPER"
	recordingHelperOutputEnv  = "DROIDSPHERE_RECORDING_OUTPUT"
	recordingHelperPayloadEnv = "DROIDSPHERE_RECORDING_PAYLOAD"
	recordingHelperExitEnv    = "DROIDSPHERE_RECORDING_EXIT"
)

func TestStartRecording_RejectsEmptySerial(t *testing.T) {
	svc := &Service{}
	err := svc.StartRecording("", "/tmp/recording.mp4", Options{})
	if err == nil {
		t.Fatal("expected error for empty serial")
	}
	if !strings.Contains(err.Error(), "Device serial is required") {
		t.Errorf("expected friendly error, got %v", err)
	}
}

func TestStartRecording_RejectsEmptyOutputPath(t *testing.T) {
	svc := &Service{}
	err := svc.StartRecording("ABC123", "", Options{})
	if err == nil {
		t.Fatal("expected error for empty output path")
	}
	if !strings.Contains(err.Error(), "Output file path is required") {
		t.Errorf("expected friendly error, got %v", err)
	}
}

func TestStopRecording_NoActiveRecording(t *testing.T) {
	svc := &Service{}
	_, err := svc.StopRecording()
	if err == nil {
		t.Fatal("expected error when no active recording")
	}
	if !strings.Contains(err.Error(), "No active recording") {
		t.Errorf("expected friendly error, got %v", err)
	}
}

func TestTakeScreenshot_RequiresActiveSession(t *testing.T) {
	svc := &Service{}
	_, err := svc.TakeScreenshot("nonexistent", "/tmp/ss.png")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestTakeScreenshot_RejectsEmptyOutputPath(t *testing.T) {
	svc := &Service{}
	_, err := svc.TakeScreenshot("", "")
	if err == nil {
		t.Fatal("expected error for empty output path")
	}
}

// TestStartRecordingRejectsActiveRecordingSlot proves the single-recording guard
// runs before binary resolution: a service without binaries must still refuse a
// second recording while the slot is taken.
func TestStartRecordingRejectsActiveRecordingSlot(t *testing.T) {
	svc := &Service{}
	svc.recording = &recordingProcess{done: make(chan struct{})}

	err := svc.StartRecording("SERIAL-1", filepath.Join(t.TempDir(), "recording.mp4"), Options{})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("occupied recording slot was not reported: %v", err)
	}
}

// TestStopRecordingSingleWaitOwnerManualStop is the regression guard for the
// double cmd.Wait: the owner goroutine reaps the process, StopRecording only waits
// for that owner, and a requested stop is neither published nor audited as an
// unexpected recording exit.
func TestStopRecordingSingleWaitOwnerManualStop(t *testing.T) {
	svc, auditLog := newRecordingTestService(t)
	rec, _ := startFakeRecording(t, svc, "payload", false)

	waitForRecordingOutput(t, rec.path)

	outputPath, err := svc.StopRecording()
	if err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	if outputPath != rec.path {
		t.Fatalf("StopRecording returned %q, want %q", outputPath, rec.path)
	}
	svc.recordingMu.Lock()
	active := svc.recording
	svc.recordingMu.Unlock()
	if active != nil {
		t.Fatal("recording slot was not released")
	}
	select {
	case <-rec.done:
	default:
		t.Fatal("StopRecording returned before the single Wait owner finished")
	}
	if rec.cmd.ProcessState == nil {
		t.Fatal("the recording process was not reaped by its owner")
	}
	if hasAuditEntry(auditLog, "recording_exit") {
		t.Fatal("a manual stop was audited as an unexpected recording exit")
	}
	if !hasSuccessfulAuditEntry(auditLog, "stop_scrcpy_recording") {
		t.Fatal("the truthful stop audit entry is missing")
	}
}

// TestStopRecordingAfterRecordingEndedIsNotFound covers a recording that exits on
// its own: the owner releases the slot exactly once, and a later StopRecording
// reports no active recording instead of waiting on a second Wait.
func TestStopRecordingAfterRecordingEndedIsNotFound(t *testing.T) {
	svc, auditLog := newRecordingTestService(t)
	rec, _ := startFakeRecording(t, svc, "payload", true)

	select {
	case <-rec.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the recording owner did not finish")
	}
	if rec.waitErr != nil {
		t.Fatalf("clean recorder exit reported as %v", rec.waitErr)
	}
	svc.recordingMu.Lock()
	active := svc.recording
	svc.recordingMu.Unlock()
	if active != nil {
		t.Fatal("a cleanly finished recording kept the slot")
	}

	if _, err := svc.StopRecording(); err == nil || !strings.Contains(err.Error(), "No active recording") {
		t.Fatalf("expected no active recording, got %v", err)
	}
	if hasAuditEntry(auditLog, "recording_exit") {
		t.Fatal("a clean exit was audited as a failed recording exit")
	}
}

// TestStopRecordingReportsEmptyOutput keeps the fail-closed reporting: an empty
// output file is an error, never a successful stop.
func TestStopRecordingReportsEmptyOutput(t *testing.T) {
	svc, auditLog := newRecordingTestService(t)
	rec, _ := startFakeRecording(t, svc, "", false)

	waitForRecordingOutput(t, rec.path)

	if _, err := svc.StopRecording(); err == nil || !strings.Contains(err.Error(), "Recording file is empty") {
		t.Fatalf("empty recording was not reported truthfully: %v", err)
	}
	svc.recordingMu.Lock()
	active := svc.recording
	svc.recordingMu.Unlock()
	if active != nil {
		t.Fatal("recording slot was not released after a failed stop")
	}
	if hasSuccessfulAuditEntry(auditLog, "stop_scrcpy_recording") {
		t.Fatal("a failed stop was audited as a successful stop")
	}
}

// TestStopRecordingUnconfirmedCompletionFailsClosed models an owner that never
// confirms completion (for example a process tree that does not die) while a
// non-empty output file already exists. The stop must report a structured failure
// with the truthful output state, audit the unsuccessful attempt, keep the record
// tracked so no second recording can start, and answer a repeated stop without
// pretending there is nothing left to stop.
func TestStopRecordingUnconfirmedCompletionFailsClosed(t *testing.T) {
	svc, auditLog := newRecordingTestService(t)
	svc.recordingStopGrace = time.Millisecond
	rec, _, _ := startFakeRecordingProcess(t, svc, "payload", false)
	// No Wait owner is started for this cmd, so completion cannot be confirmed here.
	t.Cleanup(func() { reapRecordingProcess(t, rec.cmd) })

	waitForRecordingOutput(t, rec.path)

	_, err := svc.StopRecording()
	if err == nil {
		t.Fatal("a stop with unconfirmed completion was reported as success")
	}
	opErr, ok := err.(*core.OperationError)
	if !ok || opErr.Operation != "stop_scrcpy_recording" || !opErr.Retryable {
		t.Fatalf("unexpected error shape: %#v", err)
	}
	if !strings.Contains(opErr.Message, "could not be confirmed") {
		t.Fatalf("the failure does not state that completion was unconfirmed: %v", opErr)
	}
	if !strings.Contains(opErr.Detail, "completion unconfirmed") {
		t.Fatalf("the failure detail hides the process state: %v", opErr.Detail)
	}
	wantSize := fmt.Sprintf("output size=%d", len("payload"))
	if !strings.Contains(opErr.Detail, wantSize) {
		t.Fatalf("the failure detail hides the output state (%s): %v", wantSize, opErr.Detail)
	}

	if hasSuccessfulAuditEntry(auditLog, "stop_scrcpy_recording") {
		t.Fatal("an unconfirmed stop was audited as successful")
	}
	if !hasFailedAuditEntry(auditLog, "stop_scrcpy_recording") {
		t.Fatal("the unsuccessful stop attempt was not audited")
	}

	// The record stays owned and stopping, so nothing may start on top of it.
	svc.recordingMu.Lock()
	stillTracked := svc.recording == rec
	stopping := rec.stopping
	svc.recordingMu.Unlock()
	if !stillTracked || !stopping {
		t.Fatalf("an unconfirmed stop released the recording slot (tracked=%t stopping=%t)", stillTracked, stopping)
	}
	startErr := svc.StartRecording("SERIAL-1", filepath.Join(t.TempDir(), "second.mp4"), Options{})
	if startErr == nil || !strings.Contains(startErr.Error(), "already in progress") {
		t.Fatalf("a new recording was allowed while the previous stop was unconfirmed: %v", startErr)
	}
	if _, repeated := svc.StopRecording(); repeated == nil || !strings.Contains(repeated.Error(), "already in progress") {
		t.Fatalf("a repeated stop was not reported truthfully: %v", repeated)
	}
}

// TestRecordingHelperProcess is the owned fake recorder used by the lifecycle
// tests. It acts only when the environment variables are set, so a normal test run
// leaves it a no-op. It writes the requested payload and then either exits or
// blocks on stdin until the recording is stopped.
func TestRecordingHelperProcess(t *testing.T) {
	if os.Getenv(recordingHelperEnv) != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv(recordingHelperOutputEnv), []byte(os.Getenv(recordingHelperPayloadEnv)), 0o600); err != nil {
		t.Fatalf("recording helper: %v", err)
	}
	if os.Getenv(recordingHelperExitEnv) == "1" {
		return
	}
	_, _ = os.Stdin.Read(make([]byte, 1))
}

func newRecordingTestService(t *testing.T) (*Service, *audit.Log) {
	t.Helper()
	auditLog, err := audit.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A short bounded wait keeps the termination fallback observable on Windows
	// without spending five seconds per stop.
	return &Service{auditLog: auditLog, recordingStopGrace: 200 * time.Millisecond}, auditLog
}

func startFakeRecording(t *testing.T, svc *Service, payload string, exitImmediately bool) (*recordingProcess, *os.File) {
	t.Helper()
	rec, stderrPipe, stdinWrite := startFakeRecordingProcess(t, svc, payload, exitImmediately)
	go svc.monitorRecordingProcess(rec, stderrPipe)
	return rec, stdinWrite
}

// startFakeRecordingProcess starts the fake recorder and registers it without a
// Wait owner, which lets a test model an owner that never confirms completion.
func startFakeRecordingProcess(t *testing.T, svc *Service, payload string, exitImmediately bool) (*recordingProcess, io.ReadCloser, *os.File) {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "recording.mp4")

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdinWrite.Close() })

	cmd := core.NewCommandContext(context.Background(), os.Args[0], "-test.run=^TestRecordingHelperProcess$")
	cmd.Env = append(os.Environ(),
		recordingHelperEnv+"=1",
		recordingHelperOutputEnv+"="+outputPath,
		recordingHelperPayloadEnv+"="+payload,
	)
	if exitImmediately {
		cmd.Env = append(cmd.Env, recordingHelperExitEnv+"=1")
	}
	cmd.Stdin = stdinRead
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	rec := &recordingProcess{
		cmd:    cmd,
		path:   outputPath,
		serial: "SERIAL-1",
		done:   make(chan struct{}),
	}
	svc.recordingMu.Lock()
	svc.recording = rec
	svc.recordingMu.Unlock()

	return rec, stderrPipe, stdinWrite
}

// reapRecordingProcess terminates and reaps a helper process whose Wait had no
// owner in the test, with a bounded wait so a stuck process cannot hang the suite.
func reapRecordingProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	_ = core.TerminateProcessTree(cmd)
	if cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("the recording helper did not exit")
	}
}

func waitForRecordingOutput(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the fake recorder never created %s", path)
}

func hasAuditEntry(log *audit.Log, operation string) bool {
	for _, entry := range log.EntriesWithLimit(0) {
		if entry.Operation == operation {
			return true
		}
	}
	return false
}

func hasSuccessfulAuditEntry(log *audit.Log, operation string) bool {
	for _, entry := range log.EntriesWithLimit(0) {
		if entry.Operation == operation && entry.Success {
			return true
		}
	}
	return false
}

func hasFailedAuditEntry(log *audit.Log, operation string) bool {
	for _, entry := range log.EntriesWithLimit(0) {
		if entry.Operation == operation && !entry.Success {
			return true
		}
	}
	return false
}

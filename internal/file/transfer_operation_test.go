package file

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeTransferService() *Service {
	return &Service{resolveActiveSerial: func(context.Context) (string, error) { return "A", nil }, getBinPath: func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb-A"} }, runCommand: func(context.Context, core.ExecRequest) (*core.ExecResult, error) { return &core.ExecResult{}, nil }, runStreaming: func(context.Context, core.StreamingExecRequest) (*core.ExecResult, error) {
		return &core.ExecResult{Stdout: "OK"}, nil
	}}
}

func TestTransferAdmissionPreservesCancellationOwnership(t *testing.T) {
	s := fakeTransferService()
	first, releaseFirst, err := s.beginTransfer(context.Background(), "A")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()
	if _, _, err := s.beginTransfer(context.Background(), "A"); err == nil {
		t.Fatal("overlapping operation was admitted")
	}
	s.CancelTransferFor("stale-id")
	if first.ctx.Err() != nil {
		t.Fatal("stale cancellation cancelled active transfer")
	}
	s.CancelTransferFor(first.id)
	if first.ctx.Err() == nil {
		t.Fatal("active cancellation lost")
	}
	releaseFirst()
	second, releaseSecond, err := s.beginTransfer(context.Background(), "A")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSecond()
	releaseFirst() // must not clear a later operation's cancellation function
	s.CancelTransferFor(first.id)
	if second.ctx.Err() != nil {
		t.Fatal("old operation affected new operation")
	}
	s.CancelTransferFor(second.id)
	if second.ctx.Err() == nil {
		t.Fatal("old release cleared new cancellation function")
	}
}

func TestBatchPinsTargetToolAndPreferences(t *testing.T) {
	s := fakeTransferService()
	serial, tool, compression, verify := "A", "adb-A", "off", false
	serialCalls, pathCalls, transferCalls := 0, 0, 0
	s.resolveActiveSerial = func(context.Context) (string, error) { serialCalls++; return serial, nil }
	s.getBinPath = func() core.BinaryPaths { pathCalls++; return core.BinaryPaths{Adb: tool} }
	s.SetTransferCompressionResolver(func() string { return compression })
	s.SetTransferVerificationResolver(func() bool { return verify })
	s.runCommand = func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		return &core.ExecResult{Stdout: "-z ALGORITHM compression any zstd\n-Z disable compression"}, nil
	}
	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	s.runStreaming = func(_ context.Context, req core.StreamingExecRequest) (*core.ExecResult, error) {
		transferCalls++
		if req.Command != "adb-A" || req.Args[1] != "A" || !strings.Contains(strings.Join(req.Args, " "), "-Z") {
			t.Fatalf("batch retargeted or settings changed: %#v", req)
		}
		serial, tool, compression, verify = "B", "adb-B", "zstd", true
		return &core.ExecResult{Stdout: "OK"}, nil
	}
	result, err := s.PushMultipleFilesDetailed(context.Background(), "A", []string{filePath, filePath}, "/sdcard")
	if err != nil {
		t.Fatal(err)
	}
	if result.Serial != "A" || result.Completed != 2 || transferCalls != 2 || serialCalls != 1 || pathCalls != 1 {
		t.Fatalf("batch not pinned: %#v calls %d/%d/%d", result, transferCalls, serialCalls, pathCalls)
	}
}

func TestTransferRejectsChangedConfirmationBeforeCommand(t *testing.T) {
	s := fakeTransferService()
	s.runStreaming = func(context.Context, core.StreamingExecRequest) (*core.ExecResult, error) {
		t.Fatal("unexpected transfer")
		return nil, nil
	}
	if _, err := s.PullMultipleFilesDetailed(context.Background(), "B", []string{"/sdcard/a"}, t.TempDir()); err == nil {
		t.Fatal("stale target admitted")
	}
	if s.cancelFunc != nil {
		t.Fatal("failed admission left transfer busy")
	}
}

func TestBatchReportsPartialFailureAndLegacyReturnsError(t *testing.T) {
	s := fakeTransferService()
	s.runStreaming = func(_ context.Context, req core.StreamingExecRequest) (*core.ExecResult, error) {
		if strings.Contains(strings.Join(req.Args, " "), "bad") {
			return &core.ExecResult{ExitCode: 1, Stderr: "permission denied"}, errors.New("exit status 1")
		}
		return &core.ExecResult{Stdout: "OK"}, nil
	}
	result, err := s.PullMultipleFilesDetailed(context.Background(), "A", []string{"/sdcard/good", "/sdcard/bad"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 || result.Failed != 1 || len(result.Items) != 2 || !strings.Contains(result.Items[1].Message, "permission denied") {
		t.Fatalf("dishonest partial result: %#v", result)
	}
	if _, err := s.PullMultipleFiles(context.Background(), []string{"/sdcard/good", "/sdcard/bad"}, t.TempDir()); err == nil {
		t.Fatal("legacy API reported partial failure as success")
	}
}

func TestBatchCancellationKeepsCompletedAndUnattemptedItems(t *testing.T) {
	s := fakeTransferService()
	calls := 0
	s.runStreaming = func(context.Context, core.StreamingExecRequest) (*core.ExecResult, error) {
		calls++
		s.CancelTransfer()
		return &core.ExecResult{Stdout: "OK"}, nil
	}
	result, err := s.PullMultipleFilesDetailed(context.Background(), "A", []string{"/sdcard/a", "/sdcard/b", "/sdcard/c"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || result.Completed != 1 || result.Cancelled != 1 || result.Skipped != 1 {
		t.Fatalf("cancelled batch lost outcomes: %#v", result)
	}
}

func TestTransferRetriesActualStderrAndKeepsDiagnostics(t *testing.T) {
	s := fakeTransferService()
	calls := 0
	s.runStreaming = func(context.Context, core.StreamingExecRequest) (*core.ExecResult, error) {
		calls++
		if calls == 1 {
			return &core.ExecResult{ExitCode: 1, Stderr: "device offline"}, errors.New("exit status 1")
		}
		return &core.ExecResult{Stdout: "OK"}, nil
	}
	if _, err := s.PullFile(context.Background(), "/sdcard/a", filepath.Join(t.TempDir(), "a")); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("actual ADB diagnostic did not drive retry: %d", calls)
	}
	detail := transferDiagnostic(&core.ExecResult{Stderr: "device offline", Stdout: strings.Repeat("noise", 10000)}, errors.New("exit status 1"))
	if !strings.Contains(detail, "device offline") || len(detail) > 4096 {
		t.Fatal("stdout hid stderr or diagnostics unbounded")
	}
}

func TestMissingHostHashIsFailureNotUnavailable(t *testing.T) {
	_, err := verifyTransferredFile(context.Background(), "adb", "A", filepath.Join(t.TempDir(), "missing"), "/sdcard/a", func(context.Context, core.ExecRequest) (*core.ExecResult, error) {
		t.Fatal("remote hashing should not run after host I/O failure")
		return nil, nil
	})
	if err == nil || errors.Is(err, errHostSHA256Unsupported) {
		t.Fatalf("host I/O failure hidden: %v", err)
	}
}

func TestMissingHashCommandDiagnosticsDoNotMatchMissingFiles(t *testing.T) {
	for _, detail := range []string{"sha256sum: /sdcard/file: not found", "/sdcard/not found", "unknown command file.txt", "permission denied"} {
		if isRemoteHashCommandUnavailable(detail) {
			t.Fatalf("file/execution error classified unavailable: %q", detail)
		}
	}
	for _, detail := range []string{"/system/bin/sh: sha256sum: not found", "sh: 1: toybox: not found", "toybox: Unknown command sha256sum"} {
		if !isRemoteHashCommandUnavailable(detail) {
			t.Fatalf("missing hash tool not recognized: %q", detail)
		}
	}
}

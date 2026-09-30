package file

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type transferOperation struct {
	ctx          context.Context
	id           string
	serial       string
	adbPath      string
	compression  string
	capabilities adbCompressionCapabilities
	verify       bool
}

func runTransferBatch(op *transferOperation, items []TransferItemResult, transfer func(TransferItemResult) (string, error)) TransferBatchResult {
	result := TransferBatchResult{OperationID: op.id, Serial: op.serial, Items: items}
	stopReason := ""
	for i := range result.Items {
		item := &result.Items[i]
		if stopReason != "" {
			item.Status = "skipped"
			item.Message = stopReason
			result.Skipped++
			continue
		}
		if err := op.ctx.Err(); err != nil {
			item.Status = "cancelled"
			item.Message = "Batch cancelled before this item was attempted"
			result.Cancelled++
			stopReason = item.Message
			continue
		}
		message, err := transfer(*item)
		if err == nil {
			item.Status = "success"
			item.Message = boundedTransferMessage(message)
			result.Completed++
			continue
		}
		item.Message = boundedTransferMessage(err.Error())
		if op.ctx.Err() != nil || isCancelledError(err) {
			item.Status = "cancelled"
			result.Cancelled++
			stopReason = "Batch cancelled; item was not attempted"
			continue
		}
		item.Status = "failed"
		result.Failed++
		if isVerificationMismatchError(err) {
			stopReason = "Batch stopped after a SHA-256 mismatch; item was not attempted"
		}
	}
	return result
}

func boundedTransferMessage(message string) string {
	if len(message) > 4096 {
		return message[:4096] + "… (truncated)"
	}
	return message
}

func legacyBatchResult(direction string, result TransferBatchResult, err error) (string, error) {
	op := direction + "_multiple_files"
	label := "Push"
	if direction == "pull" {
		label = "Pull"
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || isCancelledError(err) {
			return "", core.NewOperationError(op, label+" batch cancelled", err.Error(), false)
		}
		return "", err
	}
	message := fmt.Sprintf("%s batch on %s: %d completed, %d failed, %d cancelled, %d not attempted", label, result.Serial, result.Completed, result.Failed, result.Cancelled, result.Skipped)
	if result.Failed > 0 || result.Cancelled > 0 || result.Skipped > 0 {
		for _, item := range result.Items {
			if item.Status != "success" {
				message += ". " + item.Source + ": " + item.Message
			}
		}
		if result.Cancelled > 0 {
			return message, core.NewOperationError(op, label+" batch cancelled", message, false)
		}
		return message, core.NewOperationError(op, label+" batch incomplete", message, true)
	}
	return message, nil
}

func (s *Service) beginTransfer(ctx context.Context, expectedSerial string, direction ...string) (*transferOperation, func(), error) {
	s.mu.Lock()
	if s.cancelFunc != nil {
		s.mu.Unlock()
		return nil, nil, core.NewOperationError("file_transfer", "Another file transfer is already active", "cancel or wait for the current operation", true)
	}
	transferCtx, cancel := context.WithCancel(ctx)
	s.nextOperationID++
	id := fmt.Sprintf("transfer-%d", s.nextOperationID)
	s.cancelFunc, s.activeOperationID = cancel, id
	s.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			cancel()
			s.mu.Lock()
			if s.activeOperationID == id {
				s.cancelFunc = nil
				s.activeOperationID = ""
			}
			s.mu.Unlock()
		})
	}
	fail := func(err error) (*transferOperation, func(), error) { release(); return nil, nil, err }
	serial, err := s.requireActiveSerial(transferCtx)
	if err != nil {
		return fail(err)
	}
	if strings.TrimSpace(serial) == "" || (expectedSerial != "" && expectedSerial != serial) {
		return fail(core.NewOperationError("file_transfer", "Confirmed device no longer matches", "refresh the selected target before transferring", false))
	}
	if s.getBinPath == nil {
		return fail(core.NewOperationError("file_transfer", "ADB path resolver is unavailable", "binary paths are not configured", false))
	}
	op := &transferOperation{ctx: transferCtx, id: id, serial: serial, adbPath: s.getBinPath().Adb, compression: s.transferCompressionPreference(), verify: s.transferVerificationEnabled()}
	if strings.TrimSpace(op.adbPath) == "" {
		return fail(core.NewOperationError("file_transfer", "ADB path is unavailable", "binary path is empty", false))
	}
	if err := transferCtx.Err(); err != nil {
		return fail(err)
	}
	// Acknowledge ownership before capability probing or a quiet transfer, so
	// cancellation can target this operation rather than a later one.
	if len(direction) > 0 {
		s.emitOperationProgress(op, "", direction[0], 0, "", "")
	}
	op.capabilities = s.getADBCompressionCapabilities(transferCtx, op.adbPath)
	if err := transferCtx.Err(); err != nil {
		return fail(err)
	}
	return op, release, nil
}

func (s *Service) runTransferCommand(ctx context.Context, req core.StreamingExecRequest) (*core.ExecResult, error) {
	if s.runStreaming != nil {
		return s.runStreaming(ctx, req)
	}
	return core.RunCommandStreaming(ctx, req)
}

func (s *Service) runTransferProbe(ctx context.Context, req core.ExecRequest) (*core.ExecResult, error) {
	if s.runCommand != nil {
		return s.runCommand(ctx, req)
	}
	return core.RunCommand(ctx, req)
}

func transferDiagnostic(result *core.ExecResult, err error) string {
	var details []string
	add := func(text string) {
		text = strings.TrimSpace(text)
		if len(text) > 1024 {
			text = text[len(text)-1024:]
		}
		if text != "" {
			details = append(details, text)
		}
	}
	if result != nil {
		add(result.Stderr)
		add(result.Stdout)
	}
	if err != nil {
		add(err.Error())
	}
	detail := strings.Join(details, "\n")
	if len(detail) > 4096 {
		detail = detail[len(detail)-4096:]
	}
	if detail == "" {
		detail = "ADB returned no successful transfer result"
	}
	return detail
}

func waitTransferRetry(ctx context.Context) error {
	timer := time.NewTimer(transferDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

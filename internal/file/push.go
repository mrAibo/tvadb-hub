package file

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *Service) PushFile(ctx context.Context, localPath string, remotePath string) (string, error) {
	return s.PushFileForDevice(ctx, "", localPath, remotePath)
}

func (s *Service) PushFileForDevice(ctx context.Context, expectedSerial string, localPath string, remotePath string) (string, error) {
	op, release, err := s.beginTransfer(ctx, expectedSerial, "push")
	if err != nil {
		return "", err
	}
	defer release()
	return s.pushFile(op, localPath, remotePath)
}

func (s *Service) pushFile(op *transferOperation, localPath string, remotePath string) (string, error) {

	trimmedLocalPath := strings.TrimSpace(localPath)
	if trimmedLocalPath == "" {
		return "", core.NewOperationError("push_file", "Local file path is required", "local file path is empty", false)
	}

	localInfo, statErr := validateReadableHostPath("push_file", trimmedLocalPath)
	if statErr != nil {
		return "", statErr
	}
	trimmedLocalPath, pathErr := filepath.Abs(trimmedLocalPath)
	if pathErr != nil {
		return "", core.NewOperationError("push_file", "Cannot resolve host path", pathErr.Error(), false)
	}

	normalizedRemotePath, err := normalizeRemotePath(remotePath)
	if err != nil {
		return "", err
	}
	if err := validateRemoteMutationPath("push_file", normalizedRemotePath); err != nil {
		return "", err
	}

	fileName := localInfo.Name()

	transferCtx, adbPath, serial := op.ctx, op.adbPath, op.serial
	args := buildADBPushArgs(serial, trimmedLocalPath, normalizedRemotePath, op.compression, op.capabilities)

	var result *core.ExecResult
	var cmdErr error
	for attempt := 1; attempt <= transferRetries; attempt++ {
		result, cmdErr = s.runTransferCommand(transferCtx, core.StreamingExecRequest{
			Command: adbPath,
			Args:    args,
			OnStderrLine: func(line string) {
				if m := adbProgressPattern.FindStringSubmatch(line); len(m) > 1 {
					name := fileName
					if len(m) > 2 {
						if base := filepath.Base(strings.TrimSpace(m[2])); base != "" && base != "." && base != "/" {
							name = base
						}
					}
					s.emitOperationProgress(op, name, "push", parseAdbPercent(m[1]), "", "")
				}
			},
		})
		if cmdErr == nil && result != nil && result.ExitCode == 0 {
			s.emitOperationProgress(op, fileName, "push", 100, "", "")
			message := fallbackMessage(result.Stdout, fmt.Sprintf("Pushed to %s", normalizedRemotePath))
			verification, verifyErr := s.verifyTransferIfEnabled(
				op,
				transferCtx,
				adbPath,
				serial,
				trimmedLocalPath,
				normalizedRemotePath,
				fileName,
				"push",
			)
			if verifyErr != nil {
				if transferCtx.Err() != nil {
					return "", core.NewOperationError("push_file", "Push cancelled by user", "transfer context cancelled during verification", false)
				}
				return "", core.NewOperationError("push_file", "SHA-256 verification failed", verifyErr.Error(), true)
			}
			if verification.Status == VerificationStatusMismatch {
				return "", core.NewOperationError("push_file", "SHA-256 verification mismatch", verification.Detail, false)
			}
			return appendVerificationMessage(message, verification), nil
		}

		if transferCtx.Err() != nil {
			return "", core.NewOperationError("push_file", "Push cancelled by user", "transfer context cancelled", false)
		}

		if !isTransientADBError(transferDiagnostic(result, cmdErr)) || attempt == transferRetries {
			break
		}

		if waitTransferRetry(transferCtx) != nil {
			return "", core.NewOperationError("push_file", "Push cancelled by user", "transfer context cancelled", false)
		}
	}

	return "", core.NewOperationError("push_file", "Failed to push file", transferDiagnostic(result, cmdErr), true)
}

func (s *Service) PushMultipleFiles(ctx context.Context, localPaths []string, remoteDirectory string) (string, error) {
	result, err := s.PushMultipleFilesDetailed(ctx, "", localPaths, remoteDirectory)
	return legacyBatchResult("push", result, err)
}

func (s *Service) PushMultipleFilesDetailed(ctx context.Context, expectedSerial string, localPaths []string, remoteDirectory string) (TransferBatchResult, error) {
	if len(localPaths) == 0 {
		return TransferBatchResult{}, core.NewOperationError("push_multiple_files", "No files were selected", "local path list is empty", false)
	}
	if len(localPaths) > 1000 {
		return TransferBatchResult{}, core.NewOperationError("push_multiple_files", "Batch exceeds 1000 items", "select a smaller batch or transfer a directory", false)
	}

	normalizedRemoteDir, err := normalizeRemotePath(remoteDirectory)
	if err != nil {
		return TransferBatchResult{}, err
	}
	if err := validateRemoteMutationPath("push_multiple_files", normalizedRemoteDir); err != nil {
		return TransferBatchResult{}, err
	}

	op, release, err := s.beginTransfer(ctx, expectedSerial, "push")
	if err != nil {
		return TransferBatchResult{}, err
	}
	defer release()
	items := make([]TransferItemResult, len(localPaths))
	for i, localPath := range localPaths {
		items[i] = TransferItemResult{Source: localPath, Destination: path.Join(normalizedRemoteDir, filepath.Base(strings.TrimSpace(localPath)))}
	}
	return runTransferBatch(op, items, func(item TransferItemResult) (string, error) { return s.pushFile(op, item.Source, item.Destination) }), nil
}

func isCancelledError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err != nil
	}

	var operationErr *core.OperationError
	return errors.As(err, &operationErr) && strings.Contains(strings.ToLower(operationErr.Message), "cancel")
}

func (s *Service) emitTransferProgress(fileName, direction string, percent int) {
	s.emitTransferProgressState(fileName, direction, percent, "", "")
}

func (s *Service) emitTransferVerification(fileName, direction, status, detail string) {
	s.emitTransferProgressState(fileName, direction, 100, status, detail)
}

func (s *Service) emitTransferProgressState(fileName, direction string, percent int, verification, detail string) {
	s.emitOperationProgress(nil, fileName, direction, percent, verification, detail)
}

func (s *Service) emitOperationProgress(op *transferOperation, fileName, direction string, percent int, verification, detail string) {
	if s.wailsCtx == nil {
		return
	}
	progress := TransferProgress{
		FileName:           fileName,
		Direction:          direction,
		Percent:            percent,
		Verification:       verification,
		VerificationDetail: detail,
	}
	if op != nil {
		progress.OperationID = op.id
		progress.Serial = op.serial
	}
	application.Get().Event.Emit(TransferProgressEvent, progress)
}

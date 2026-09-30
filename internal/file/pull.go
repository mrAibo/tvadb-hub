package file

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func (s *Service) PullFile(ctx context.Context, remotePath string, localPath string) (string, error) {
	return s.PullFileForDevice(ctx, "", remotePath, localPath)
}

func (s *Service) PullFileForDevice(ctx context.Context, expectedSerial string, remotePath string, localPath string) (string, error) {
	op, release, err := s.beginTransfer(ctx, expectedSerial, "pull")
	if err != nil {
		return "", err
	}
	defer release()
	return s.pullFile(op, remotePath, localPath)
}

func (s *Service) pullFile(op *transferOperation, remotePath string, localPath string) (string, error) {

	normalizedRemotePath, err := normalizeRemotePath(remotePath)
	if err != nil {
		return "", err
	}

	trimmedLocalPath := strings.TrimSpace(localPath)
	if trimmedLocalPath == "" {
		return "", core.NewOperationError("pull_file", "Destination path is required", "local destination path is empty", false)
	}
	trimmedLocalPath, pathErr := filepath.Abs(trimmedLocalPath)
	if pathErr != nil {
		return "", core.NewOperationError("pull_file", "Cannot resolve host destination", pathErr.Error(), false)
	}

	fileName := path.Base(normalizedRemotePath)

	transferCtx, adbPath, serial := op.ctx, op.adbPath, op.serial
	args := buildADBPullArgs(serial, normalizedRemotePath, trimmedLocalPath, op.compression, op.capabilities)

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
						if base := path.Base(strings.TrimSpace(m[2])); base != "" && base != "." && base != "/" {
							name = base
						}
					}
					s.emitOperationProgress(op, name, "pull", parseAdbPercent(m[1]), "", "")
				}
			},
		})
		if cmdErr == nil && result != nil && result.ExitCode == 0 {
			s.emitOperationProgress(op, fileName, "pull", 100, "", "")
			message := fallbackMessage(result.Stdout, fmt.Sprintf("Saved file to %s", trimmedLocalPath))
			verification, verifyErr := s.verifyTransferIfEnabled(
				op,
				transferCtx,
				adbPath,
				serial,
				trimmedLocalPath,
				normalizedRemotePath,
				fileName,
				"pull",
			)
			if verifyErr != nil {
				if transferCtx.Err() != nil {
					return "", core.NewOperationError("pull_file", "Pull cancelled by user", "transfer context cancelled during verification", false)
				}
				return "", core.NewOperationError("pull_file", "SHA-256 verification failed", verifyErr.Error(), true)
			}
			if verification.Status == VerificationStatusMismatch {
				return "", core.NewOperationError("pull_file", "SHA-256 verification mismatch", verification.Detail, false)
			}
			return appendVerificationMessage(message, verification), nil
		}

		if transferCtx.Err() != nil {
			return "", core.NewOperationError("pull_file", "Pull cancelled by user", "transfer context cancelled", false)
		}

		if !isTransientADBError(transferDiagnostic(result, cmdErr)) || attempt == transferRetries {
			break
		}

		if waitTransferRetry(transferCtx) != nil {
			return "", core.NewOperationError("pull_file", "Pull cancelled by user", "transfer context cancelled", false)
		}
	}

	return "", core.NewOperationError("pull_file", "Failed to pull file", transferDiagnostic(result, cmdErr), true)
}

func (s *Service) PullMultipleFiles(ctx context.Context, remotePaths []string, localDirectory string) (string, error) {
	result, err := s.PullMultipleFilesDetailed(ctx, "", remotePaths, localDirectory)
	return legacyBatchResult("pull", result, err)
}

func (s *Service) PullMultipleFilesDetailed(ctx context.Context, expectedSerial string, remotePaths []string, localDirectory string) (TransferBatchResult, error) {
	trimmedLocalDir := strings.TrimSpace(localDirectory)
	if trimmedLocalDir == "" {
		return TransferBatchResult{}, core.NewOperationError("pull_multiple_files", "Destination directory is required", "local destination directory is empty", false)
	}
	if len(remotePaths) == 0 {
		return TransferBatchResult{}, core.NewOperationError("pull_multiple_files", "No files were selected", "remote path list is empty", false)
	}
	if len(remotePaths) > 1000 {
		return TransferBatchResult{}, core.NewOperationError("pull_multiple_files", "Batch exceeds 1000 items", "select a smaller batch or transfer a directory", false)
	}

	op, release, err := s.beginTransfer(ctx, expectedSerial, "pull")
	if err != nil {
		return TransferBatchResult{}, err
	}
	defer release()
	items := make([]TransferItemResult, len(remotePaths))
	for i, remotePath := range remotePaths {
		items[i] = TransferItemResult{Source: remotePath, Destination: filepath.Join(trimmedLocalDir, path.Base(strings.TrimSpace(remotePath)))}
	}
	return runTransferBatch(op, items, func(item TransferItemResult) (string, error) {
		name := path.Base(strings.TrimSpace(item.Source))
		if !filepath.IsLocal(name) || filepath.Base(name) != name {
			return "", core.NewOperationError("pull_file", "Remote name cannot be mapped safely to a host file", name, false)
		}
		return s.pullFile(op, item.Source, item.Destination)
	}), nil
}

func parseAdbPercent(s string) int {
	pct := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			pct = pct*10 + int(c-'0')
		}
	}
	return pct
}

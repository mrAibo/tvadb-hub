package file

import (
	"ADBKit/internal/core"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type transferVerificationResult struct {
	Status       string
	Detail       string
	LocalDigest  string
	RemoteDigest string
}

type verificationCommandRunner func(context.Context, core.ExecRequest) (*core.ExecResult, error)

func (s *Service) transferVerificationEnabled() bool {
	s.mu.Lock()
	resolve := s.getTransferVerification
	s.mu.Unlock()

	return resolve != nil && resolve()
}

func (s *Service) verifyTransferIfEnabled(
	ctx context.Context,
	adbPath string,
	serial string,
	localPath string,
	remotePath string,
	fileName string,
	direction string,
) (transferVerificationResult, error) {
	if !s.transferVerificationEnabled() {
		return transferVerificationResult{}, nil
	}

	s.emitTransferVerification(
		fileName,
		direction,
		VerificationStatusVerifying,
		"Computing host and Android SHA-256 digests",
	)

	result, err := verifyTransferredFile(
		ctx,
		adbPath,
		serial,
		localPath,
		remotePath,
		core.RunCommand,
	)
	if err != nil {
		return transferVerificationResult{}, err
	}

	s.emitTransferVerification(fileName, direction, result.Status, result.Detail)
	return result, nil
}

func verifyTransferredFile(
	ctx context.Context,
	adbPath string,
	serial string,
	localPath string,
	remotePath string,
	run verificationCommandRunner,
) (transferVerificationResult, error) {
	localDigest, err := computeHostSHA256(ctx, localPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return transferVerificationResult{}, err
		}
		return transferVerificationResult{
			Status: VerificationStatusUnavailable,
			Detail: fmt.Sprintf("host SHA-256 unavailable: %s", err.Error()),
		}, nil
	}

	remoteDigest, err := computeRemoteSHA256(ctx, adbPath, serial, remotePath, run)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return transferVerificationResult{}, err
		}
		return transferVerificationResult{
			Status:      VerificationStatusUnavailable,
			Detail:      err.Error(),
			LocalDigest: localDigest,
		}, nil
	}

	if localDigest != remoteDigest {
		return transferVerificationResult{
			Status:       VerificationStatusMismatch,
			Detail:       fmt.Sprintf("host %s does not match Android %s", localDigest, remoteDigest),
			LocalDigest:  localDigest,
			RemoteDigest: remoteDigest,
		}, nil
	}

	return transferVerificationResult{
		Status:       VerificationStatusVerified,
		Detail:       fmt.Sprintf("SHA-256 %s", localDigest),
		LocalDigest:  localDigest,
		RemoteDigest: remoteDigest,
	}, nil
}

func computeHostSHA256(ctx context.Context, filePath string) (string, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("verification is only available for regular host files")
	}

	fileHandle, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fileHandle.Close()

	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		readCount, readErr := fileHandle.Read(buffer)
		if readCount > 0 {
			if _, err := hasher.Write(buffer[:readCount]); err != nil {
				return "", err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func computeRemoteSHA256(
	ctx context.Context,
	adbPath string,
	serial string,
	remotePath string,
	run verificationCommandRunner,
) (string, error) {
	quotedPath := quoteShellArg(remotePath)
	commands := []string{
		"sha256sum < " + quotedPath,
		"toybox sha256sum < " + quotedPath,
	}

	failures := make([]string, 0, len(commands))
	for _, command := range commands {
		result, err := run(ctx, core.ExecRequest{
			Command: adbPath,
			Args:    []string{"-s", serial, "shell", command},
		})
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}

		if err != nil || result == nil || result.ExitCode != 0 {
			detail := "command failed"
			if result != nil {
				if trimmed := strings.TrimSpace(result.Stderr); trimmed != "" {
					detail = trimmed
				}
			}
			if err != nil && strings.TrimSpace(err.Error()) != "" {
				detail = err.Error()
			}
			failures = append(failures, detail)
			continue
		}

		digest, parseErr := parseSHA256Output(result.Stdout)
		if parseErr != nil {
			failures = append(failures, parseErr.Error())
			continue
		}
		return digest, nil
	}

	detail := strings.Join(failures, "; ")
	if detail == "" {
		detail = "sha256sum and toybox sha256sum are unavailable"
	}
	return "", fmt.Errorf("Android SHA-256 unavailable: %s", detail)
}

func parseSHA256Output(output string) (string, error) {
	line := extractFirstLine(output)
	if line == "" {
		return "", fmt.Errorf("SHA-256 command returned no digest")
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("SHA-256 command returned malformed output")
	}

	digest := strings.ToLower(strings.TrimSpace(fields[0]))
	if len(digest) != sha256.Size*2 {
		return "", fmt.Errorf("SHA-256 command returned malformed digest %q", fields[0])
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("SHA-256 command returned malformed digest %q", fields[0])
	}
	return digest, nil
}

func appendVerificationMessage(message string, verification transferVerificationResult) string {
	switch verification.Status {
	case VerificationStatusVerified:
		return message + ". SHA-256 verified."
	case VerificationStatusUnavailable:
		return message + ". SHA-256 verification unavailable: " + verification.Detail
	default:
		return message
	}
}

func isVerificationMismatchError(err error) bool {
	if err == nil {
		return false
	}

	var operationErr *core.OperationError
	return errors.As(err, &operationErr) &&
		strings.Contains(strings.ToLower(operationErr.Message), "verification mismatch")
}

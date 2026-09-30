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
	"regexp"
	"strings"
	"time"
)

type transferVerificationResult struct {
	Status       string
	Detail       string
	LocalDigest  string
	RemoteDigest string
}

type verificationCommandRunner func(context.Context, core.ExecRequest) (*core.ExecResult, error)

var errRemoteSHA256Unavailable = errors.New("remote SHA-256 tools unavailable")
var errHostSHA256Unsupported = errors.New("verification is only available for regular host files")

var missingHashCommandPattern = regexp.MustCompile(`^(?:(?:/system/bin/)?sh: (?:[0-9]+: )?)?(?:sha256sum|toybox): (?:inaccessible or )?not found$`)
var missingToyboxAppletPattern = regexp.MustCompile(`^toybox: unknown command ['"]?sha256sum['"]?(?:\s.*)?$`)

func (s *Service) transferVerificationEnabled() bool {
	s.mu.Lock()
	resolve := s.getTransferVerification
	s.mu.Unlock()

	return resolve != nil && resolve()
}

func (s *Service) verifyTransferIfEnabled(
	op *transferOperation,
	ctx context.Context,
	adbPath string,
	serial string,
	localPath string,
	remotePath string,
	fileName string,
	direction string,
) (transferVerificationResult, error) {
	if !op.verify {
		return transferVerificationResult{}, nil
	}

	s.emitOperationProgress(
		op,
		fileName,
		direction,
		100,
		VerificationStatusVerifying,
		"Computing host and Android SHA-256 digests",
	)

	result, err := verifyTransferredFile(
		ctx,
		adbPath,
		serial,
		localPath,
		remotePath,
		s.runTransferProbe,
	)
	if err != nil {
		s.emitOperationProgress(op, fileName, direction, 100, VerificationStatusFailure, err.Error())
		return transferVerificationResult{}, err
	}

	s.emitOperationProgress(op, fileName, direction, 100, result.Status, result.Detail)
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
		if !errors.Is(err, errHostSHA256Unsupported) {
			return transferVerificationResult{}, fmt.Errorf("host SHA-256 verification failed: %w", err)
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
		if errors.Is(err, errRemoteSHA256Unavailable) {
			return transferVerificationResult{
				Status:      VerificationStatusUnavailable,
				Detail:      err.Error(),
				LocalDigest: localDigest,
			}, nil
		}
		return transferVerificationResult{}, err
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errHostSHA256Unsupported
	}

	fileHandle, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fileHandle.Close()

	openedInfo, err := fileHandle.Stat()
	if err != nil {
		return "", err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return "", fmt.Errorf("host file changed or became unsafe before verification")
	}

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

	after, err := fileHandle.Stat()
	if err != nil {
		return "", err
	}
	current, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if !current.Mode().IsRegular() || !os.SameFile(openedInfo, current) || after.Size() != openedInfo.Size() || !after.ModTime().Equal(openedInfo.ModTime()) {
		return "", fmt.Errorf("host file changed during verification")
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
	unavailableCount := 0
	for _, command := range commands {
		result, err := run(ctx, core.ExecRequest{
			Command: adbPath,
			Args:    []string{"-s", serial, "shell", command},
			Timeout: 10 * time.Minute,
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
			if detail == "command failed" && err != nil && strings.TrimSpace(err.Error()) != "" {
				detail = err.Error()
			}
			// Android shells conventionally use exit code 127 for a missing
			// command. Some builds emit no useful stderr in that case, so do
			// not depend on diagnostic text alone.
			if (result != nil && result.ExitCode == 127) || isRemoteHashCommandUnavailable(detail) {
				unavailableCount++
			} else {
				failures = append(failures, detail)
			}
			continue
		}

		digest, parseErr := parseSHA256Output(result.Stdout)
		if parseErr != nil {
			failures = append(failures, parseErr.Error())
			continue
		}
		return digest, nil
	}

	if unavailableCount == len(commands) && len(failures) == 0 {
		return "", fmt.Errorf("%w: sha256sum and toybox sha256sum are unavailable", errRemoteSHA256Unavailable)
	}

	detail := strings.Join(failures, "; ")
	if detail == "" {
		detail = "Android hashing failed for an unknown reason"
	}
	return "", fmt.Errorf("Android SHA-256 verification failed: %s", detail)
}

func isRemoteHashCommandUnavailable(detail string) bool {
	for _, line := range strings.Split(strings.ToLower(detail), "\n") {
		line = strings.TrimSpace(line)
		if missingHashCommandPattern.MatchString(line) || missingToyboxAppletPattern.MatchString(line) {
			return true
		}
	}
	return false
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

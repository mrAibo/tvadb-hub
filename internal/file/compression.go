package file

import (
	"ADBKit/internal/core"
	"context"
	"strings"
	"time"
)

const adbCompressionProbeTimeout = 3 * time.Second

type adbCompressionCapabilities struct {
	UseCompression     bool
	DisableCompression bool
	Algorithms         map[string]bool
}

func (s *Service) transferCompressionPreference() string {
	s.mu.Lock()
	resolve := s.getTransferCompression
	s.mu.Unlock()

	if resolve == nil {
		return core.DefaultFileTransferCompression
	}

	value := strings.ToLower(strings.TrimSpace(resolve()))
	if !core.IsValidFileTransferCompression(value) {
		return core.DefaultFileTransferCompression
	}
	return value
}

func (s *Service) getADBCompressionCapabilities(ctx context.Context, adbPath string) adbCompressionCapabilities {
	s.mu.Lock()
	cached, ok := s.compressionCache[adbPath]
	s.mu.Unlock()
	if ok {
		return cached
	}

	result, err := s.runTransferProbe(ctx, core.ExecRequest{
		Command: adbPath,
		Args:    []string{"help"},
		Timeout: adbCompressionProbeTimeout,
	})
	if err != nil || result == nil || result.ExitCode != 0 {
		return adbCompressionCapabilities{}
	}

	capabilities := parseADBCompressionCapabilities(result.Stdout + "\n" + result.Stderr)

	s.mu.Lock()
	if s.compressionCache == nil {
		s.compressionCache = make(map[string]adbCompressionCapabilities)
	}
	s.compressionCache[adbPath] = capabilities
	s.mu.Unlock()

	return capabilities
}

func parseADBCompressionCapabilities(help string) adbCompressionCapabilities {
	lower := strings.ToLower(help)
	useCompression := strings.Contains(help, "-z") && strings.Contains(lower, "compression") && strings.Contains(lower, "any")

	capabilities := adbCompressionCapabilities{
		UseCompression:     useCompression,
		DisableCompression: strings.Contains(help, "-Z"),
		Algorithms:         make(map[string]bool),
	}
	if !useCompression {
		return capabilities
	}

	for _, algorithm := range []string{
		core.FileTransferCompressionZstd,
		core.FileTransferCompressionLZ4,
		core.FileTransferCompressionBrotli,
	} {
		capabilities.Algorithms[algorithm] = strings.Contains(lower, algorithm)
	}

	return capabilities
}

func transferCompressionArgs(mode string, capabilities adbCompressionCapabilities) []string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if !core.IsValidFileTransferCompression(normalized) {
		normalized = core.DefaultFileTransferCompression
	}

	switch normalized {
	case core.FileTransferCompressionOff:
		if capabilities.DisableCompression {
			return []string{"-Z"}
		}
		return nil
	case core.FileTransferCompressionZstd,
		core.FileTransferCompressionLZ4,
		core.FileTransferCompressionBrotli:
		if capabilities.UseCompression && capabilities.Algorithms[normalized] {
			return []string{"-z", normalized}
		}
		if capabilities.UseCompression {
			return []string{"-z", "any"}
		}
		return nil
	default:
		if capabilities.UseCompression {
			return []string{"-z", "any"}
		}
		return nil
	}
}

func buildADBPushArgs(
	serial string,
	localPath string,
	remotePath string,
	mode string,
	capabilities adbCompressionCapabilities,
) []string {
	args := []string{"-s", serial, "push"}
	args = append(args, transferCompressionArgs(mode, capabilities)...)
	args = append(args, localPath, remotePath)
	return args
}

func buildADBPullArgs(
	serial string,
	remotePath string,
	localPath string,
	mode string,
	capabilities adbCompressionCapabilities,
) []string {
	args := []string{"-s", serial, "pull", "-a"}
	args = append(args, transferCompressionArgs(mode, capabilities)...)
	args = append(args, remotePath, localPath)
	return args
}

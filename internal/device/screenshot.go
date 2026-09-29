package device

import (
	"ADBKit/internal/core"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

type ScreenshotResult struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func (s *Service) CaptureScreenshot(ctx context.Context, serial string, localPath string) (ScreenshotResult, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"device serial is required",
			"serial must not be empty",
			false,
		)
	}

	targetPath, err := normalizeScreenshotPath(localPath)
	if err != nil {
		return ScreenshotResult{}, err
	}
	targetDir := filepath.Dir(targetPath)
	if info, statErr := os.Stat(targetDir); statErr != nil || !info.IsDir() {
		detail := targetDir
		if statErr != nil {
			detail = statErr.Error()
		}
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"screenshot destination directory is not available",
			detail,
			false,
		)
	}

	tmp, err := os.CreateTemp(targetDir, ".tvadb-screenshot-*.png")
	if err != nil {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"failed to create temporary screenshot file",
			err.Error(),
			true,
		)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	captureCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		captureCtx,
		s.getBinPath().Adb,
		"-s",
		serial,
		"exec-out",
		"screencap",
		"-p",
	)
	core.ConfigureChildProcess(cmd)

	var stderr bytes.Buffer
	cmd.Stdout = tmp
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	closeErr := tmp.Close()
	if captureCtx.Err() != nil {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"screenshot capture timed out",
			captureCtx.Err().Error(),
			true,
		)
	}
	if runErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = runErr.Error()
		}
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"ADB screenshot capture failed",
			detail,
			true,
		)
	}
	if closeErr != nil {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"failed to finalize screenshot file",
			closeErr.Error(),
			true,
		)
	}

	size, err := validatePNGFile(tmpPath)
	if err != nil {
		return ScreenshotResult{}, err
	}

	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Remove(targetPath); err != nil {
			return ScreenshotResult{}, core.NewOperationError(
				"capture_screenshot",
				"failed to replace existing screenshot",
				err.Error(),
				true,
			)
		}
	} else if !os.IsNotExist(err) {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"failed to inspect screenshot destination",
			err.Error(),
			true,
		)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return ScreenshotResult{}, core.NewOperationError(
			"capture_screenshot",
			"failed to save screenshot",
			err.Error(),
			true,
		)
	}

	return ScreenshotResult{Path: targetPath, Bytes: size}, nil
}

func normalizeScreenshotPath(localPath string) (string, error) {
	path := strings.TrimSpace(localPath)
	if path == "" {
		return "", core.NewOperationError(
			"capture_screenshot",
			"screenshot path is required",
			"choose a local PNG file destination",
			false,
		)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "":
		path += ".png"
	case ".png":
	default:
		return "", core.NewOperationError(
			"capture_screenshot",
			"screenshot destination must be a PNG file",
			fmt.Sprintf("unsupported extension %q", ext),
			false,
		)
	}

	return filepath.Clean(path), nil
}

func validatePNGFile(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, core.NewOperationError(
			"capture_screenshot",
			"failed to validate screenshot",
			err.Error(),
			true,
		)
	}
	defer file.Close()

	header := make([]byte, len(pngSignature))
	n, err := file.Read(header)
	if err != nil || n != len(pngSignature) || !bytes.Equal(header, pngSignature) {
		return 0, core.NewOperationError(
			"capture_screenshot",
			"ADB returned an invalid screenshot",
			"captured output is not a PNG image",
			true,
		)
	}

	info, err := file.Stat()
	if err != nil {
		return 0, core.NewOperationError(
			"capture_screenshot",
			"failed to inspect screenshot",
			err.Error(),
			true,
		)
	}
	if info.Size() <= int64(len(pngSignature)) {
		return 0, core.NewOperationError(
			"capture_screenshot",
			"ADB returned an empty screenshot",
			"PNG output contains no image payload",
			true,
		)
	}
	return info.Size(), nil
}

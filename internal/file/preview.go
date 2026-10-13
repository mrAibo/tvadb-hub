package file

import (
	"ADBKit/internal/core"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxPreviewBytes int64 = 2 * 1024 * 1024

type PreviewResult struct {
	Source       string `json:"source"`
	Serial       string `json:"serial,omitempty"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	MimeType     string `json:"mimeType"`
	Text         string `json:"text,omitempty"`
	ImageDataURL string `json:"imageDataUrl,omitempty"`
	Bytes        int64  `json:"bytes"`
}

var previewImages = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}
var previewTexts = map[string]bool{
	".txt": true, ".md": true, ".log": true, ".csv": true,
	".json": true, ".xml": true, ".yaml": true, ".yml": true,
	".ini": true, ".conf": true, ".cfg": true, ".properties": true,
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".sh": true, ".css": true, ".html": true, ".htm": true,
}

func previewKind(filename string) (string, error) {
	extension := strings.ToLower(path.Ext(filename))
	switch {
	case previewImages[extension]:
		return "image", nil
	case previewTexts[extension]:
		return "text", nil
	default:
		return "", core.NewOperationError("preview_file", "Preview not supported", "only text and PNG/JPEG/GIF/WebP images are supported", false)
	}
}

func decodePreview(source, serial, filePath, kind string, data []byte) (PreviewResult, error) {
	if len(data) > int(MaxPreviewBytes) {
		return PreviewResult{}, core.NewOperationError("preview_file", "Preview too large", "file exceeds 2 MiB preview limit", false)
	}
	result := PreviewResult{Source: source, Serial: serial, Path: filePath, Name: path.Base(filePath), Kind: kind, Bytes: int64(len(data))}
	if source == "local" {
		result.Name = filepath.Base(filePath)
	}
	if kind == "text" {
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return PreviewResult{}, core.NewOperationError("preview_file", "Not a text file", "preview requires valid UTF-8 text without NUL bytes", false)
		}
		result.MimeType = "text/plain; charset=utf-8"
		result.Text = string(data)
		return result, nil
	}
	contentType := http.DetectContentType(data)
	// WebP detection is not consistent across all supported Go sniffers.
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		contentType = "image/webp"
	}
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		result.MimeType = contentType
		result.ImageDataURL = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
		return result, nil
	default:
		return PreviewResult{}, core.NewOperationError("preview_file", "Image format mismatch", "file does not contain a supported image", false)
	}
}

func (s *Service) PreviewLocalFile(localPath string) (PreviewResult, error) {
	kind, err := previewKind(filepath.Base(localPath))
	if err != nil {
		return PreviewResult{}, err
	}
	info, err := os.Lstat(localPath)
	if err != nil {
		return PreviewResult{}, err
	}
	if !info.Mode().IsRegular() {
		return PreviewResult{}, core.NewOperationError("preview_file", "Unsupported file type", "directories, symlinks and special files cannot be previewed", false)
	}
	if info.Size() > MaxPreviewBytes {
		return PreviewResult{}, core.NewOperationError("preview_file", "Preview too large", "file exceeds 2 MiB preview limit", false)
	}
	reader, err := os.Open(localPath)
	if err != nil {
		return PreviewResult{}, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, MaxPreviewBytes+1))
	if err != nil {
		return PreviewResult{}, err
	}
	return decodePreview("local", "", localPath, kind, data)
}

func (s *Service) PreviewRemoteFileForDevice(ctx context.Context, serial, remotePath string) (PreviewResult, error) {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return PreviewResult{}, core.NewOperationError("preview_file", "Confirmed device required", "select an ADB target before previewing", false)
	}
	normalized, err := normalizeRemotePath(remotePath)
	if err != nil {
		return PreviewResult{}, err
	}
	kind, err := previewKind(path.Base(normalized))
	if err != nil {
		return PreviewResult{}, err
	}
	if s.getBinPath == nil || strings.TrimSpace(s.getBinPath().Adb) == "" {
		return PreviewResult{}, errors.New("ADB binary is not configured")
	}
	boundedCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var data []byte
	if s.readPreviewRemote != nil {
		data, err = s.readPreviewRemote(boundedCtx, s.getBinPath().Adb, serial, normalized)
	} else {
		data, err = readRemotePreview(boundedCtx, s.getBinPath().Adb, serial, normalized)
	}
	if err != nil {
		return PreviewResult{}, fmt.Errorf("ADB preview failed: %w", err)
	}
	return decodePreview("device", serial, normalized, kind, data)
}

type limitedPreviewError struct{ bytes.Buffer }

func (b *limitedPreviewError) Write(data []byte) (int, error) {
	n := len(data)
	if b.Len() < 4096 {
		remaining := 4096 - b.Len()
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.Buffer.Write(data)
	}
	return n, nil
}

// readRemotePreview never captures more than MaxPreviewBytes+1 bytes. ADB
// keeps its official CLI semantics, with a confirmed serial and context.
func readRemotePreview(ctx context.Context, binary, serial, remotePath string) ([]byte, error) {
	cmd := core.NewCommandContext(ctx, binary, "-s", serial, "exec-out", "cat", quoteShellArg(remotePath))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr limitedPreviewError
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, MaxPreviewBytes+1))
	if readErr != nil || len(data) > int(MaxPreviewBytes) {
		_ = core.TerminateProcessTree(cmd)
		_ = cmd.Wait()
		if readErr != nil {
			return nil, readErr
		}
		return nil, core.NewOperationError("preview_file", "Preview too large", "remote file exceeds 2 MiB preview limit", false)
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("adb exec-out failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return data, nil
}

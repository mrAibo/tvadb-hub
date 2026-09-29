package device

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeScreenshotPath(t *testing.T) {
	t.Run("keeps png extension", func(t *testing.T) {
		got, err := normalizeScreenshotPath(filepath.Join("tmp", "tv.png"))
		if err != nil {
			t.Fatalf("normalizeScreenshotPath returned error: %v", err)
		}
		if filepath.Ext(got) != ".png" {
			t.Fatalf("expected .png extension, got %q", got)
		}
	})

	t.Run("adds png extension when omitted", func(t *testing.T) {
		got, err := normalizeScreenshotPath(filepath.Join("tmp", "tv-shot"))
		if err != nil {
			t.Fatalf("normalizeScreenshotPath returned error: %v", err)
		}
		if filepath.Ext(got) != ".png" {
			t.Fatalf("expected .png extension, got %q", got)
		}
	})

	t.Run("rejects non png extension", func(t *testing.T) {
		if _, err := normalizeScreenshotPath("tv.jpg"); err == nil {
			t.Fatal("expected unsupported extension error")
		}
	})
}

func TestValidatePNGFile(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.png")
	valid := append(append([]byte{}, pngSignature...), []byte("payload")...)
	if err := os.WriteFile(validPath, valid, 0o600); err != nil {
		t.Fatalf("write valid fixture: %v", err)
	}
	size, err := validatePNGFile(validPath)
	if err != nil {
		t.Fatalf("validatePNGFile(valid) returned error: %v", err)
	}
	if size != int64(len(valid)) {
		t.Fatalf("validatePNGFile(valid) size = %d, want %d", size, len(valid))
	}

	invalidPath := filepath.Join(dir, "invalid.png")
	if err := os.WriteFile(invalidPath, []byte("not-a-png"), 0o600); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}
	if _, err := validatePNGFile(invalidPath); err == nil {
		t.Fatal("expected invalid PNG error")
	}
}

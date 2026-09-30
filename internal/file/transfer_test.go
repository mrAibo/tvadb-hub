package file

import (
	"ADBKit/internal/core"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPushMultipleFiles_CancelledReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		cancelOnCall  int
		includeBroken bool
	}{
		{name: "first file", cancelOnCall: 1},
		{name: "after previous file", cancelOnCall: 2, includeBroken: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			validPath := filepath.Join(dir, "valid.txt")
			if err := os.WriteFile(validPath, []byte("test"), 0o600); err != nil {
				t.Fatalf("write test file: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			svc := &Service{
				resolveActiveSerial: func(context.Context) (string, error) {
					calls++
					if calls == tt.cancelOnCall {
						cancel()
					}
					return "test-device", nil
				},
				getBinPath: func() core.BinaryPaths {
					return core.BinaryPaths{Adb: "adb"}
				},
			}

			paths := []string{validPath}
			if tt.includeBroken {
				paths = []string{filepath.Join(dir, "missing.txt"), validPath}
			}

			_, err := svc.PushMultipleFiles(ctx, paths, "/sdcard")
			if err == nil {
				t.Fatal("expected cancelled push batch to return an error")
			}

			var operationErr *core.OperationError
			if !errors.As(err, &operationErr) {
				t.Fatalf("expected OperationError, got %T: %v", err, err)
			}
			if operationErr.Operation != "push_multiple_files" {
				t.Errorf("operation = %q, want push_multiple_files", operationErr.Operation)
			}
			if operationErr.Message != "Push batch cancelled" {
				t.Errorf("message = %q, want Push batch cancelled", operationErr.Message)
			}
		})
	}
}


func TestParseADBCompressionCapabilities(t *testing.T) {
	help := `file transfer:
  -z ALGORITHM    use compression with specified algorithm (any/none/brotli/lz4/zstd)
  -Z              disable compression
`

	caps := parseADBCompressionCapabilities(help)
	if !caps.UseCompression {
		t.Fatal("expected compression support")
	}
	if !caps.DisableCompression {
		t.Fatal("expected -Z support")
	}
	for _, algorithm := range []string{"zstd", "lz4", "brotli"} {
		if !caps.Algorithms[algorithm] {
			t.Errorf("expected %s to be detected", algorithm)
		}
	}
}

func TestBuildADBTransferArgsCompressionAndFallback(t *testing.T) {
	modern := adbCompressionCapabilities{
		UseCompression:     true,
		DisableCompression: true,
		Algorithms: map[string]bool{
			core.FileTransferCompressionZstd:   true,
			core.FileTransferCompressionLZ4:    true,
			core.FileTransferCompressionBrotli: true,
		},
	}
	withoutZstd := adbCompressionCapabilities{
		UseCompression:     true,
		DisableCompression: true,
		Algorithms: map[string]bool{
			core.FileTransferCompressionLZ4: true,
		},
	}
	legacy := adbCompressionCapabilities{}

	tests := []struct {
		name string
		mode string
		caps adbCompressionCapabilities
		push []string
		pull []string
	}{
		{
			name: "auto uses negotiated compression",
			mode: core.FileTransferCompressionAuto,
			caps: modern,
			push: []string{"-s", "SERIAL", "push", "-z", "any", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "-z", "any", "/sdcard/remote.bin", "local.bin"},
		},
		{
			name: "explicit zstd",
			mode: core.FileTransferCompressionZstd,
			caps: modern,
			push: []string{"-s", "SERIAL", "push", "-z", "zstd", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "-z", "zstd", "/sdcard/remote.bin", "local.bin"},
		},
		{
			name: "unsupported explicit algorithm falls back to auto",
			mode: core.FileTransferCompressionZstd,
			caps: withoutZstd,
			push: []string{"-s", "SERIAL", "push", "-z", "any", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "-z", "any", "/sdcard/remote.bin", "local.bin"},
		},
		{
			name: "off uses disable flag when supported",
			mode: core.FileTransferCompressionOff,
			caps: modern,
			push: []string{"-s", "SERIAL", "push", "-Z", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "-Z", "/sdcard/remote.bin", "local.bin"},
		},
		{
			name: "legacy adb receives no compression flags",
			mode: core.FileTransferCompressionAuto,
			caps: legacy,
			push: []string{"-s", "SERIAL", "push", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "/sdcard/remote.bin", "local.bin"},
		},
		{
			name: "off on legacy adb also receives no unsupported flag",
			mode: core.FileTransferCompressionOff,
			caps: legacy,
			push: []string{"-s", "SERIAL", "push", "local.bin", "/sdcard/remote.bin"},
			pull: []string{"-s", "SERIAL", "pull", "-a", "/sdcard/remote.bin", "local.bin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPush := buildADBPushArgs("SERIAL", "local.bin", "/sdcard/remote.bin", tt.mode, tt.caps)
			if !reflect.DeepEqual(gotPush, tt.push) {
				t.Fatalf("push args = %#v, want %#v", gotPush, tt.push)
			}

			gotPull := buildADBPullArgs("SERIAL", "/sdcard/remote.bin", "local.bin", tt.mode, tt.caps)
			if !reflect.DeepEqual(gotPull, tt.pull) {
				t.Fatalf("pull args = %#v, want %#v", gotPull, tt.pull)
			}
		})
	}
}

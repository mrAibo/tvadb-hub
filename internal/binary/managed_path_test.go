package binary

import (
	"path/filepath"
	"testing"
)

func TestGetManagedBinaryPath(t *testing.T) {
	dataDir := filepath.Join("tmp", "tvadb")
	service := NewService(dataDir)

	got, err := service.GetManagedBinaryPath(BinaryNameAdb)
	if err != nil {
		t.Fatalf("GetManagedBinaryPath(adb) returned error: %v", err)
	}
	want := joinManagedPath(dataDir, BinaryNameAdb)
	if got != want {
		t.Fatalf("GetManagedBinaryPath(adb) = %q, want %q", got, want)
	}

	if _, err := service.GetManagedBinaryPath("unknown"); err == nil {
		t.Fatal("expected unsupported binary error")
	}
}

package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAtomicConcurrentWritesDoNotShareTemporaryFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(value byte) {
			defer wg.Done()
			if err := WriteFileAtomicWithMode(path, bytes.Repeat([]byte{value}, 8192), 0o600); err != nil {
				t.Errorf("write: %v", err)
			}
		}(byte('a' + i))
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 8192 || !bytes.Equal(data, bytes.Repeat(data[:1], 8192)) {
		t.Fatal("atomic file contains mixed/truncated writes")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %+v", entries)
	}
}

// TestWriteFileAtomicCommitRefusalPreservesDestinationAndCleansTemp pins the
// fail-closed contract of the commit reservation: when the replacement cannot
// be performed the caller receives a structured error, the existing destination
// entry is left untouched and the temporary file is removed. A directory at the
// destination makes the rename fail deterministically on every supported
// platform, so no timing or permission manipulation is involved.
func TestWriteFileAtomicCommitRefusalPreservesDestinationAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomicWithMode(path, []byte("payload"), 0o600)
	if err == nil {
		t.Fatal("expected a refused commit to be reported as an error")
	}
	var opErr *OperationError
	if !errors.As(err, &opErr) || opErr.Operation != "fsutil" {
		t.Fatalf("expected a structured fsutil operation error, got %v", err)
	}

	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("existing destination was not preserved: info=%v err=%v", info, statErr)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 || entries[0].Name() != "journal.json" {
		t.Fatalf("temporary file left behind after a refused commit: %+v", entries)
	}
}

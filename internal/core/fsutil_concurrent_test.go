package core

import (
	"bytes"
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

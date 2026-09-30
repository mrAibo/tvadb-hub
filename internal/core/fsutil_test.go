package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicWithModeReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")

	if err := WriteFileAtomicWithMode(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomicWithMode(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "second" {
		t.Fatalf("content=%q want second", content)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remained after atomic write: %v", err)
	}
}

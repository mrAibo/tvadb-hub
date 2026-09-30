package core

import (
	"os"
	"path/filepath"
)

func WriteFileAtomic(path string, data []byte) error {
	return WriteFileAtomicWithMode(path, data, 0o644)
}

func WriteFileAtomicWithMode(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return NewOperationError("fsutil", "failed to create directory", err.Error(), true)
	}

	tmp := path + ".tmp"
	_ = os.Remove(tmp)

	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return NewOperationError("fsutil", "failed to create temp file", err.Error(), true)
	}

	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tmp)
	}

	if _, err := file.Write(data); err != nil {
		cleanup()
		return NewOperationError("fsutil", "failed to write temp file", err.Error(), true)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return NewOperationError("fsutil", "failed to sync temp file", err.Error(), true)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return NewOperationError("fsutil", "failed to close temp file", err.Error(), true)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return NewOperationError("fsutil", "failed to rename temp file", err.Error(), true)
	}

	// Best-effort directory sync makes the rename durable across sudden power
	// loss on filesystems that support syncing directories. Windows may reject
	// directory Sync, so it is deliberately not a fatal error.
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}

	return nil
}

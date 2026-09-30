package core

import (
	"os"
	"path/filepath"
	"sync"
)

// commitMu reserves the final replacement step of an atomic write within this
// process. On Windows two overlapping
// MoveFileEx(..., MOVEFILE_REPLACE_EXISTING) calls against the same destination
// make the loser fail with ERROR_ACCESS_DENIED ("Access is denied"), so
// concurrent writers of one path could observe a refused commit. Serializing
// exactly the rename makes the commit step deterministic while temporary file
// creation, chmod, write, sync and close stay fully parallel.
//
// Scope: this is a process-local mutex for all goroutines of one process, not a
// cross-process lock. Two DroidSphere processes (or a second application
// instance) writing the same path remain unprotected, and a file-level reader
// holding the destination open can still make the replacement fail on Windows.
// Such a refusal is a returned error, never silent success; the rename itself
// stays atomic within this API contract.
var commitMu sync.Mutex

func WriteFileAtomic(path string, data []byte) error {
	return WriteFileAtomicWithMode(path, data, 0o644)
}

func WriteFileAtomicWithMode(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return NewOperationError("fsutil", "failed to create directory", err.Error(), true)
	}

	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return NewOperationError("fsutil", "failed to create temp file", err.Error(), true)
	}
	tmp := file.Name()
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return NewOperationError("fsutil", "failed to set temp file permissions", err.Error(), true)
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

	// The reservation covers only the destination replacement. Everything above
	// (temp creation, permissions, write, sync, close) stays outside the lock.
	commitMu.Lock()
	renameErr := os.Rename(tmp, path)
	commitMu.Unlock()
	if renameErr != nil {
		_ = os.Remove(tmp)
		return NewOperationError("fsutil", "failed to rename temp file", renameErr.Error(), true)
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

// Package atomicfile provides private atomic file replacement for local snapshots.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const privateDirectoryMode = 0o700

type syncCloser interface {
	Sync() error
	Close() error
}

// Replace writes data to a private temporary file in the destination directory,
// syncs and closes it, then renames it over path and syncs the directory ancestry.
// Readers see the old or complete new file. Newly created directories use 0700
// and files use 0600; existing directory permissions are unchanged.
// Pending temporary files are removed on failure. An error after rename leaves
// the new file visible but its durability unconfirmed. Callers must not treat
// that error as a committed reservation.
// Durability depends on the filesystem honoring file and directory syncs.
// Callers own encoding, validation, and store-specific error context.
func Replace(path, pattern string, data []byte) error {
	return replace(path, pattern, data, syncDirectory)
}

func replace(path, pattern string, data []byte, syncDir func(string) error) error {
	dir := filepath.Dir(path)
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	if err := os.MkdirAll(dir, privateDirectoryMode); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	defer func() { _ = os.Remove(file.Name()) }()
	defer func() { _ = file.Close() }()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	// Sync deepest first, including existing ancestors: lock setup may have
	// just created them, and a prior failed attempt may have left them visible
	// without committing their parent entries. Retrying must sync them again.
	for {
		if err := syncDir(dir); err != nil {
			return fmt.Errorf("commit directory %q: %w", dir, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	return syncAndClose(dir)
}

func syncAndClose(dir syncCloser) error {
	var syncErr, closeErr error
	if err := dir.Sync(); err != nil {
		syncErr = fmt.Errorf("sync directory: %w", err)
	}
	if err := dir.Close(); err != nil {
		closeErr = fmt.Errorf("close directory: %w", err)
	}
	return errors.Join(syncErr, closeErr)
}

// Package atomicfile provides private atomic file replacement for local snapshots.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replace writes data to a private temporary file in the destination directory,
// syncs and closes it, then renames it over path. Readers see the old or complete
// new file. Newly created directories use 0700 and files use 0600; existing
// directory permissions are unchanged. Pending temporary files are removed on
// failure. Callers own encoding, validation, and store-specific error context.
func Replace(path, pattern string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
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
	return nil
}

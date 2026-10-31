package appidentity

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/cliffom/bambustand/internal/atomicfile"
	"golang.org/x/sys/unix"
)

var ErrMigration = errors.New("storage migration incomplete or conflicting: stop all older hosts, preserve both directories, reconcile conflicting files from backup, and retry; no history was reset")

// Lock never replaces or unlinks a lock inode. Callers retain it through writes.
func Lock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// CopyMissing copies a snapshot without replacing different destination bytes.
// A failed/interrupted copy can be resumed: identical files are recommitted to
// repair ambiguous directory-sync outcomes. Callers hold ownership and publish
// their completion marker only after every artifact succeeds. Symlinks and
// special files fail closed; neither secrets nor source paths enter errors.
func CopyMissing(source, destination string) error {
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return ErrMigration
	}
	if info.IsDir() {
		if existing, err := os.Lstat(destination); err == nil && !existing.IsDir() {
			return ErrMigration
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return ErrMigration
		}
		if err := os.MkdirAll(destination, 0o700); err != nil {
			return ErrMigration
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return ErrMigration
		}
		for _, entry := range entries {
			if err := CopyMissing(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return ErrMigration
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return ErrMigration
	}
	if existing, err := os.Lstat(destination); err == nil {
		if !existing.Mode().IsRegular() {
			return ErrMigration
		}
		saved, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(data, saved) {
			return ErrMigration
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrMigration
	}
	if err := atomicfile.Replace(destination, ".migration-*", data); err != nil {
		return ErrMigration
	}
	return nil
}

// Completed validates a durable marker rather than treating any file as success.
func Completed(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || string(data) != Namespace+"\n" {
		return false, ErrMigration
	}
	return true, nil
}

func Complete(path string) error {
	if err := atomicfile.Replace(path, ".migration-*", []byte(Namespace+"\n")); err != nil {
		return ErrMigration
	}
	return nil
}

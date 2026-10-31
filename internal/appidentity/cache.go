package appidentity

import (
	"errors"
	"os"
	"path/filepath"
)

// MigrateCache retains existing diagnostic exports. Fresh installs do not create
// a legacy directory. Cache conflicts fail closed just like account snapshots.
func MigrateCache() error {
	base, err := os.UserCacheDir()
	if err != nil {
		return ErrMigration
	}
	source := filepath.Join(base, LegacyDirectory, "diagnostics")
	if _, err := os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return ErrMigration
	}
	root := filepath.Join(base, Namespace)
	lock, err := Lock(filepath.Join(root, ".migration.lock"))
	if err != nil {
		return ErrMigration
	}
	defer func() { _ = lock.Close() }()
	marker := filepath.Join(root, ".migrated")
	done, err := Completed(marker)
	if done || err != nil {
		return err
	}
	if err := CopyMissing(source, filepath.Join(root, "diagnostics")); err != nil {
		return err
	}
	return Complete(marker)
}

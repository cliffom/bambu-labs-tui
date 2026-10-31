package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/cliffom/bambustand/internal/appidentity"
	"github.com/cliffom/bambustand/internal/atomicfile"
)

// Only the default path is migrated. Explicit paths keep their original theme
// base. A valid existing destination always wins; unreadable destinations fail
// in the normal loader instead of falling back to old configuration.
func migrate(path string) error {
	current, err := Path()
	if err != nil || filepath.Clean(path) != current {
		return nil
	}
	if _, err := os.Lstat(current); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return appidentity.ErrMigration
	}
	legacy := filepath.Join(filepath.Dir(filepath.Dir(current)), appidentity.LegacyDirectory, configFilename)
	if _, err := os.Lstat(legacy); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return appidentity.ErrMigration
	}
	lock, err := appidentity.Lock(filepath.Join(filepath.Dir(current), ".config-migration.lock"))
	if err != nil {
		return appidentity.ErrMigration
	}
	defer func() { _ = lock.Close() }()
	if _, err := os.Lstat(current); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return appidentity.ErrMigration
	}
	cfg, err := load(legacy)
	if err != nil {
		return appidentity.ErrMigration
	}
	if cfg.ThemeFile != "" && !filepath.IsAbs(cfg.ThemeFile) {
		cfg.ThemeFile = filepath.Join(filepath.Dir(legacy), cfg.ThemeFile)
	}
	var data bytes.Buffer
	if err := toml.NewEncoder(&data).Encode(cfg); err != nil {
		return appidentity.ErrMigration
	}
	if err := atomicfile.Replace(current, ".config-*", data.Bytes()); err != nil {
		return appidentity.ErrMigration
	}
	return nil
}

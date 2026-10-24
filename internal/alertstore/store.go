// Package alertstore persists account-scoped printer snoozes separately from reviews.
package alertstore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store holds one account/region snapshot of printer snooze deadlines.
type Store struct{ Path string }

type document struct {
	Version int                  `json:"version"`
	Snoozes map[string]time.Time `json:"snoozes"`
}

// New derives a stable account/region path without using identifiers as filenames.
func New(region, username string) (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	// Account identifiers never become path components. Tokens are not used.
	id := sha256.Sum256([]byte(region + "\x00" + username))
	return Store{Path: filepath.Join(dir, "bambu-tui", "alerts", fmt.Sprintf("%x.json", id))}, nil
}

// Load reads deadlines without discarding expired entries before the UI can save cleanup.
func (s Store) Load() (map[string]time.Time, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read saved alert snoozes: %w", err)
	}
	var d document
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("decode saved alert snoozes: %w", err)
	}
	if d.Version != 1 {
		return nil, fmt.Errorf("unsupported saved alert version %d", d.Version)
	}
	for id, until := range d.Snoozes {
		if id == "" || until.IsZero() {
			return nil, fmt.Errorf("invalid saved alert snooze")
		}
	}
	return d.Snoozes, nil
}

// Save replaces the complete snapshot atomically so interruption cannot leave a
// partially written document. Readers see either the old file or the new file.
func (s Store) Save(snoozes map[string]time.Time) error {
	b, err := json.MarshalIndent(document{Version: 1, Snoozes: snoozes}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode saved alert snoozes: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create alert directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".alerts-*")
	if err != nil {
		return fmt.Errorf("create saved alert file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write saved alert snoozes: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync saved alert snoozes: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close saved alert snoozes: %w", err)
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return fmt.Errorf("replace saved alert snoozes: %w", err)
	}
	return nil
}

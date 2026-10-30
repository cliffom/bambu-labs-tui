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

	"github.com/cliffom/bambustand/internal/atomicfile"
)

const schemaVersion = 1

type (
	// Store holds one account/region snapshot of printer snooze deadlines.
	Store struct{ Path string }

	document struct {
		Version int                  `json:"version"`
		Snoozes map[string]time.Time `json:"snoozes"`
	}
)

// New derives a stable account/region path without using identifiers as filenames.
func New(region, username string) (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	// Account identifiers never become path components. Tokens are not used.
	id := sha256.Sum256([]byte(region + "\x00" + username))
	// Retain the original namespace for upgrade compatibility; see docs/user/usage.md.
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
	if d.Version != schemaVersion {
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
// Cloud hosts must hold accountstore ownership across loading and all saves;
// atomic replacement alone does not protect independent session snapshots.
func (s Store) Save(snoozes map[string]time.Time) error {
	b, err := json.MarshalIndent(document{Version: schemaVersion, Snoozes: snoozes}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode saved alert snoozes: %w", err)
	}
	if err := atomicfile.Replace(s.Path, ".alerts-*", append(b, '\n')); err != nil {
		return fmt.Errorf("save alert snoozes: %w", err)
	}
	return nil
}

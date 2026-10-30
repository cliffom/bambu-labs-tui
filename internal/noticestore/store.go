// Package noticestore persists local notice decisions separately from credentials.
package noticestore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cliffom/bambustand/internal/atomicfile"
	"github.com/cliffom/bambustand/internal/farm"
)

const schemaVersion = 1

type (
	Store struct{ Path string }

	document struct {
		Version int                    `json:"version"`
		Reviews map[string]farm.Review `json:"reviews"`
	}
)

func New(region, username string) (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	// Account identifiers never become path components. Tokens are not used.
	id := sha256.Sum256([]byte(region + "\x00" + username))
	// Retain the original namespace for upgrade compatibility; see docs/user/usage.md.
	return Store{Path: filepath.Join(dir, "bambu-tui", "notices", fmt.Sprintf("%x.json", id))}, nil
}

func (s Store) Load() (map[string]farm.Review, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read saved notices: %w", err)
	}
	var d document
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("decode saved notices: %w", err)
	}
	if d.Version != schemaVersion {
		return nil, fmt.Errorf("unsupported saved notice version %d", d.Version)
	}
	return d.Reviews, nil
}

// Save replaces the complete snapshot atomically so interruption cannot leave a
// partially written document. Readers see either the old file or the new file.
// Cloud hosts must hold accountstore ownership across loading and all saves;
// atomic replacement alone does not protect independent session snapshots.
func (s Store) Save(reviews map[string]farm.Review) error {
	b, err := json.MarshalIndent(document{Version: schemaVersion, Reviews: reviews}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode saved notices: %w", err)
	}
	if err := atomicfile.Replace(s.Path, ".notices-*", append(b, '\n')); err != nil {
		return fmt.Errorf("save notices: %w", err)
	}
	return nil
}

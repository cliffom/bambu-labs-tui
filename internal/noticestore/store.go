// Package noticestore persists local notice decisions separately from credentials.
package noticestore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cliffom/bambu-labs-tui/internal/farm"
)

type Store struct{ Path string }

type document struct {
	Version int                    `json:"version"`
	Reviews map[string]farm.Review `json:"reviews"`
}

func New(region, username string) (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	// Account identifiers never become path components. Tokens are not used.
	id := sha256.Sum256([]byte(region + "\x00" + username))
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
	if d.Version != 1 {
		return nil, fmt.Errorf("unsupported saved notice version %d", d.Version)
	}
	return d.Reviews, nil
}

// Save replaces the complete snapshot atomically so interruption cannot leave a
// partially written document. Readers see either the old file or the new file.
func (s Store) Save(reviews map[string]farm.Review) error {
	b, err := json.MarshalIndent(document{Version: 1, Reviews: reviews}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode saved notices: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create notice directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".notices-*")
	if err != nil {
		return fmt.Errorf("create saved notice file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write saved notices: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync saved notices: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close saved notices: %w", err)
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return fmt.Errorf("replace saved notices: %w", err)
	}
	return nil
}

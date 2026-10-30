// Package accountstore owns notice and snooze persistence for a cloud session.
package accountstore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cliffom/bambustand/internal/alertstore"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/noticestore"
	"golang.org/x/sys/unix"
)

var (
	ErrBusy   = errors.New("another cloud TUI or serve process owns saved notices and snoozes for this account/region; stop it and retry")
	ErrClosed = errors.New("account persistence ownership has ended")
)

// Store holds exclusive ownership from before loading until all accepted writes
// finish. Close rejects delayed jobs without waiting for a stuck filesystem.
type Store struct {
	mu      sync.Mutex
	file    *os.File
	closed  bool
	writes  int
	notices noticestore.Store
	snoozes alertstore.Store
}

func Open(region, username string) (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("cannot locate account persistence directory")
	}
	id := sha256.Sum256([]byte(region + "\x00" + username))
	// Retain the original namespace for upgrade compatibility; see docs/user/usage.md.
	dir = filepath.Join(dir, "bambu-tui", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, errors.New("cannot create account persistence ownership directory; check configuration directory access")
	}
	// Never unlink this file: replacing the inode could allow two owners.
	file, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%x.lock", id)), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, errors.New("cannot open account persistence lock; check configuration directory access")
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, errors.New("cannot lock account persistence; monitoring was not started")
	}
	notices, noticeErr := noticestore.New(region, username)
	snoozes, snoozeErr := alertstore.New(region, username)
	if err := errors.Join(noticeErr, snoozeErr); err != nil {
		_ = file.Close()
		return nil, errors.New("cannot locate saved notices and snoozes")
	}
	return &Store{file: file, notices: notices, snoozes: snoozes}, nil
}

// Close is idempotent and bounded. An active write retains the OS lock until
// completion; process exit (including a crash) releases it automatically.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.release()
}

func (s *Store) release() {
	if s.closed && s.writes == 0 && s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
}

func (s *Store) write(save func() error) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	s.writes++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.writes--
		s.release()
	}()
	return save()
}

// Loads run during startup while the host owns the lease, before jobs start.
func (s *Store) LoadReviews() (map[string]farm.Review, error) { return s.notices.Load() }

func (s *Store) LoadSnoozes() (map[string]time.Time, error) { return s.snoozes.Load() }

func (s *Store) SaveReviews(reviews map[string]farm.Review) error {
	return s.write(func() error { return s.notices.Save(reviews) })
}

func (s *Store) SaveSnoozes(snoozes map[string]time.Time) error {
	return s.write(func() error { return s.snoozes.Save(snoozes) })
}

package notification_state

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// Owner holds notification delivery ownership for one account/region. The OS
// releases it on process exit, including a crash. The lock file is never removed:
// replacing its inode could allow two owners to hold different locks.
type Owner struct {
	file                 *os.File
	previousToken, token string
	once                 sync.Once
	err                  error
}

func (s Store) AcquireOwner() (*Owner, error) {
	if s.Path == "" {
		return nil, errors.New("notification state path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create notification owner directory: %w", err)
	}
	file, err := os.OpenFile(s.Path+".owner.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open notification owner lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock notification owner: %w", err)
	}
	previous, token, err := rotateOwnerToken(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("record notification ownership continuity: %w", err)
	}
	return &Owner{file: file, previousToken: previous, token: token}, nil
}

func (o *Owner) Close() error {
	o.once.Do(func() { o.err = o.file.Close() })
	return o.err
}

// Continues verifies that no other lease acquisition occurred since previous,
// including owners that exited before this process could observe contention.
func (o *Owner) Continues(previous *Owner) bool {
	return previous != nil && o.previousToken == previous.token
}

func rotateOwnerToken(file *os.File) (string, string, error) {
	var saved [nonceBytes + 1]byte
	n, err := file.ReadAt(saved[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	if n != 0 && n != nonceBytes {
		return "", "", errors.New("invalid ownership continuity token")
	}
	var token [nonceBytes]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", "", err
	}
	if _, err := file.WriteAt(token[:], 0); err != nil {
		return "", "", err
	}
	if err := file.Sync(); err != nil {
		return "", "", err
	}
	return string(saved[:n]), string(token[:]), nil
}

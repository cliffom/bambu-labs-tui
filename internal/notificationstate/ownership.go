package notificationstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// Owner holds notification delivery ownership for one account/region. The OS
// releases it on process exit, including a crash. The lock file is never removed:
// replacing its inode could allow two owners to hold different locks.
type Owner struct {
	file *os.File
	once sync.Once
	err  error
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
	return &Owner{file: file}, nil
}

func (o *Owner) Close() error {
	o.once.Do(func() { o.err = o.file.Close() })
	return o.err
}

package accountstore

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/cliffom/bambustand/internal/appidentity"
	"github.com/cliffom/bambustand/internal/notification_state"
	"golang.org/x/sys/unix"
)

// migrate runs under both account leases, before any load or notification worker.
// Keep original lock inodes in place so supported older hosts cannot run beside
// this owner. Data remains as a backup; after completion canonical state wins.
func (s *Store) migrate(base, id, region, username string) error {
	legacy := filepath.Join(base, appidentity.LegacyDirectory)
	if _, err := os.Lstat(legacy); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return appidentity.ErrMigration
	}
	lock, err := appidentity.Lock(filepath.Join(legacy, "sessions", id+".lock"))
	if err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return ErrBusy
		}
		return appidentity.ErrMigration
	}
	s.legacyLocks = append(s.legacyLocks, lock)
	marker := filepath.Join(base, appidentity.Namespace, "sessions", id+".migrated")
	ledger, err := notification_state.New(region, username)
	if err != nil {
		return appidentity.ErrMigration
	}
	// Older notification workers also retain independent owner locks. Preserve
	// these inodes and guard both native and browser channels during the copy.
	name := filepath.Base(ledger.Path)
	for _, channel := range []string{name, name + ".browser"} {
		for _, suffix := range []string{".owner.lock", ".lock"} {
			path := filepath.Join(legacy, "notifications", channel+suffix)
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return appidentity.ErrMigration
			}
			lock, err := appidentity.Lock(path)
			if err != nil {
				if errors.Is(err, unix.EWOULDBLOCK) {
					return ErrBusy
				}
				return appidentity.ErrMigration
			}
			s.legacyLocks = append(s.legacyLocks, lock)
		}
	}
	done, err := appidentity.Completed(marker)
	if err != nil || done {
		return err
	}
	for _, folder := range []string{"notices", "alerts"} {
		if err := appidentity.CopyMissing(filepath.Join(legacy, folder, id+".json"), filepath.Join(base, appidentity.Namespace, folder, id+".json")); err != nil {
			return err
		}
	}
	// Archive buckets and format markers travel with each ledger. Never copy lock
	// token contents: new ownership gets a fresh independent continuity chain.
	for _, channel := range []string{name, name + ".browser"} {
		for _, suffix := range []string{".jobs", ""} {
			if err := appidentity.CopyMissing(filepath.Join(legacy, "notifications", channel+suffix), filepath.Join(base, appidentity.Namespace, "notifications", channel+suffix)); err != nil {
				return err
			}
		}
	}
	if err := appidentity.Complete(marker); err != nil {
		return err
	}
	return nil
}

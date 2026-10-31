package accountstore

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/farm"
)

func openTestStore(t *testing.T, region, account string) *Store {
	t.Helper()
	s, err := Open(region, account)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestExclusiveScopeAndStaleSnapshots(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	first := openTestStore(t, "global", "synthetic-account")
	oldReviews, err := first.LoadReviews()
	if err != nil {
		t.Fatal(err)
	}
	oldSnoozes, err := first.LoadSnoozes()
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Open("global", "synthetic-account"); !errors.Is(err, ErrBusy) {
		if second != nil {
			second.Close()
		}
		t.Fatal("independent session loaded stale snapshots", err)
	}
	for _, scope := range [][2]string{{"china", "synthetic-account"}, {"global", "other-account"}} {
		openTestStore(t, scope[0], scope[1])
	}
	if filepath.Base(filepath.Dir(first.file.Name())) != "sessions" || filepath.Base(filepath.Dir(filepath.Dir(first.file.Name()))) != "com.suitepotato.bambustand" {
		t.Fatal("unexpected account namespace", first.file.Name())
	}
	info, err := first.file.Stat()
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != 0 {
		t.Fatal("lock not private/empty", err)
	}
	directory, err := os.Stat(filepath.Dir(first.file.Name()))
	if err != nil || directory.Mode().Perm() != 0o700 {
		t.Fatal("directory not private", err)
	}
	path := first.file.Name()
	first.Close()
	second := openTestStore(t, "global", "synthetic-account")
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	reviews := map[string]farm.Review{"SYNTHETIC/fault": {Level: "error", Acknowledged: true, Dismissed: true, Since: at}}
	snoozes := map[string]time.Time{"SYNTHETIC": at.Add(time.Hour)}
	if err := second.SaveReviews(reviews); err != nil {
		t.Fatal(err)
	}
	if err := second.SaveSnoozes(snoozes); err != nil {
		t.Fatal(err)
	}
	// A delayed immutable job from the previous owner cannot erase the new owner's edits.
	if err := first.SaveReviews(oldReviews); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if err := first.SaveSnoozes(oldSnoozes); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	first.Close()
	if third, err := Open("global", "synthetic-account"); !errors.Is(err, ErrBusy) {
		if third != nil {
			third.Close()
		}
		t.Fatal("old close released new owner", err)
	}
	second.Close()
	third := openTestStore(t, "global", "synthetic-account")
	gotReviews, err := third.LoadReviews()
	if err != nil || !reflect.DeepEqual(reviews, gotReviews) {
		t.Fatal("lost reviews", err)
	}
	gotSnoozes, err := third.LoadSnoozes()
	if err != nil || !reflect.DeepEqual(snoozes, gotSnoozes) {
		t.Fatal("lost snoozes", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("lock inode replaced", err)
	}
}

func TestCloseRetainsOwnershipUntilActiveWriteFinishes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	first := openTestStore(t, "global", "synthetic-account")
	entered, finish, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() { done <- first.write(func() error { close(entered); <-finish; return nil }) }()
	<-entered
	first.Close() // Must not wait for the blocked write.
	if next, err := Open("global", "synthetic-account"); !errors.Is(err, ErrBusy) {
		if next != nil {
			next.Close()
		}
		t.Fatal("released during active write", err)
	}
	if err := first.SaveReviews(nil); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	openTestStore(t, "global", "synthetic-account")
}

func TestOwnerIsReleasedByNormalExitAndCrash(t *testing.T) {
	for _, crash := range []bool{false, true} {
		name := "normal"
		if crash {
			name = "crash"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			readyRead, readyWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = readyRead.Close(); _ = readyWrite.Close() }()
			holdRead, holdWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holdRead.Close(); _ = holdWrite.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOwnershipHelperProcess$")
			child.Env = append(os.Environ(), "BAMBU_STAND_ACCOUNT_OWNER_TEST=1")
			child.ExtraFiles = []*os.File{readyWrite}
			child.Stdin = holdRead
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if child.ProcessState == nil {
					_ = child.Process.Kill()
					_ = child.Wait()
				}
			})
			_ = readyWrite.Close()
			_ = holdRead.Close()
			ready := make(chan error, 1)
			go func() { var b [1]byte; _, err := io.ReadFull(readyRead, b[:]); ready <- err }()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatal("child failed before taking ownership", err)
				}
			case <-ctx.Done():
				t.Fatal("child did not acquire ownership")
			}
			if owner, err := Open("global", "synthetic-account"); !errors.Is(err, ErrBusy) {
				if owner != nil {
					owner.Close()
				}
				t.Fatal("child ownership was not exclusive", err)
			}
			if crash {
				if err := child.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				_ = holdWrite.Close()
			}
			if err := child.Wait(); err != nil && !crash {
				t.Fatal(err)
			}
			owner, err := Open("global", "synthetic-account")
			if err != nil {
				t.Fatal("OS did not release ownership", err)
			}
			reviews, err := owner.LoadReviews()
			if err != nil || !reviews["SYNTHETIC/fault"].Acknowledged {
				t.Fatal("committed review lost on exit", err)
			}
			snoozes, err := owner.LoadSnoozes()
			if err != nil || len(snoozes) != 1 {
				t.Fatal("committed snooze lost on exit", err)
			}
			owner.Close()
		})
	}
}

func TestOwnershipHelperProcess(t *testing.T) {
	path := os.Getenv("BAMBU_STAND_ACCOUNT_OWNER_TEST")
	if path == "" {
		return
	}
	owner, err := Open("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err := owner.SaveReviews(map[string]farm.Review{"SYNTHETIC/fault": {Level: "error", Acknowledged: true}}); err != nil {
		t.Fatal(err)
	}
	if err := owner.SaveSnoozes(map[string]time.Time{"SYNTHETIC": time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	ready := os.NewFile(3, "owner-ready")
	if _, err := ready.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = ready.Close()
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func TestOwnershipAccessFailureIsSanitizedAndStopsStartup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "com.suitepotato.bambustand"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "com.suitepotato.bambustand", "sessions"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open("global", "private-synthetic-account")
	if store != nil || err == nil || strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "private-synthetic-account") {
		t.Fatal("ownership failure allowed startup or leaked context", err)
	}
}

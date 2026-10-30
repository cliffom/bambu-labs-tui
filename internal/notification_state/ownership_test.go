package notification_state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnerIsExclusivePrivateAndScopedToLedger(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "nested", "account-region.json")}
	first, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	result := make(chan error, 1)
	go func() {
		owner, err := store.AcquireOwner()
		if owner != nil {
			_ = owner.Close()
		}
		result <- err
	}()
	select {
	case err := <-result:
		if !errors.Is(err, ErrBusy) {
			t.Fatal("second owner was allowed", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owner acquisition blocked")
	}
	info, err := os.Stat(store.Path + ".owner.lock")
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != nonceBytes {
		t.Fatal("owner token is not bounded/private", err)
	}
	directory, err := os.Stat(filepath.Dir(store.Path))
	if err != nil || directory.Mode().Perm() != 0o700 {
		t.Fatal("owner directory is not private", err)
	}
	other := Store{Path: filepath.Join(filepath.Dir(store.Path), "other-account-region.json")}
	independent, err := other.AcquireOwner()
	if err != nil {
		t.Fatal("independent scope was blocked", err)
	}
	if err := independent.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	replacement, err := store.AcquireOwner()
	if err != nil {
		t.Fatal("released owner was not available", err)
	}
	defer func() { _ = replacement.Close() }()
	// Re-closing the old handle must not release a newly reused descriptor.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if owner, err := store.AcquireOwner(); !errors.Is(err, ErrBusy) {
		if owner != nil {
			_ = owner.Close()
		}
		t.Fatal("old owner close released new lease", err)
	}
	after, err := os.Stat(store.Path + ".owner.lock")
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("ownership lock inode was replaced", err)
	}
	if _, err := os.Stat(store.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("acquiring ownership created delivery identities", err)
	}
}

func TestOwnerIsReleasedByNormalExitAndCrash(t *testing.T) {
	for _, crash := range []bool{false, true} {
		name := "normal"
		if crash {
			name = "crash"
		}
		t.Run(name, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
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
			child.Env = append(os.Environ(), "BAMBU_STAND_OWNER_TEST="+store.Path)
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
			if owner, err := store.AcquireOwner(); !errors.Is(err, ErrBusy) {
				if owner != nil {
					_ = owner.Close()
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
			owner, err := store.AcquireOwner()
			if err != nil {
				t.Fatal("OS did not release ownership", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOwnershipHelperProcess(t *testing.T) {
	path := os.Getenv("BAMBU_STAND_OWNER_TEST")
	if path == "" {
		return
	}
	owner, err := (Store{Path: path}).AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	ready := os.NewFile(3, "owner-ready")
	if _, err := ready.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = ready.Close()
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerContinuityDetectsInterveningAcquisition(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	first, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	next, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	if !next.Continues(first) || next.Continues(nil) {
		t.Fatal("uninterrupted reacquisition not identified")
	}
	_ = next.Close()
	intervening, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	_ = intervening.Close()
	last, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = last.Close() }()
	if last.Continues(next) || !last.Continues(intervening) {
		t.Fatal("unseen handoff did not break continuity")
	}
}

func TestOwnerRejectsMalformedContinuityTokenWithoutOverwriting(t *testing.T) {
	for _, size := range []int{1, nonceBytes - 1, nonceBytes + 1, 1024} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
			original := bytes.Repeat([]byte{'x'}, size)
			if err := os.WriteFile(store.Path+".owner.lock", original, 0o600); err != nil {
				t.Fatal(err)
			}
			if owner, err := store.AcquireOwner(); err == nil {
				_ = owner.Close()
				t.Fatal("malformed token allowed ownership")
			}
			after, err := os.ReadFile(store.Path + ".owner.lock")
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("malformed token overwritten", err)
			}
		})
	}
}

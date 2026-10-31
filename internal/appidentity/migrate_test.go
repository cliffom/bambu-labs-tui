package appidentity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationCopiesPrivateFilesRejectsLinksAndPreservesConflicts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "new", "snapshot")
	if err := os.WriteFile(source, []byte("synthetic snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := CopyMissing(source, destination); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal(info, err)
	}
	info, err = os.Stat(filepath.Dir(destination))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatal(info, err)
	}
	if err := os.WriteFile(destination, []byte("newer snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CopyMissing(source, destination); !errors.Is(err, ErrMigration) {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil || string(saved) != "newer snapshot" {
		t.Fatal(string(saved), err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if err := CopyMissing(link, filepath.Join(root, "copied")); !errors.Is(err, ErrMigration) {
		t.Fatal("source link accepted", err)
	}
	if err := CopyMissing(source, link); !errors.Is(err, ErrMigration) {
		t.Fatal("destination link accepted", err)
	}
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, []byte("interrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if done, err := Completed(marker); done || !errors.Is(err, ErrMigration) {
		t.Fatal("invalid marker accepted", done, err)
	}
}

package atomicfile

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceUsesPrivateFilesAndAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "snapshot.json")
	before := []byte("old complete snapshot\n")
	after := bytes.Repeat([]byte("new complete snapshot\n"), 1024)
	if err := Replace(path, ".snapshot-*", before); err != nil {
		t.Fatal(err)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || dir.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions: %v, %v", dir, err)
	}
	// Replacing a less restrictive existing file must still install a private file.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Close() }()
	if err := Replace(path, ".snapshot-*", after); err != nil {
		t.Fatal(err)
	}
	stillOld, err := io.ReadAll(old)
	if err != nil || !bytes.Equal(stillOld, before) {
		t.Fatal("replacement modified a reader's already-open old file")
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, after) {
		t.Fatal("destination is not the complete new snapshot")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions: %v, %v", info, err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 || files[0].Name() != filepath.Base(path) {
		t.Fatalf("temporary files leaked: %v, %v", files, err)
	}
}

func TestFailedReplacementCleansTemporaryFileAndPreservesDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "destination")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "existing")
	if err := os.WriteFile(sentinel, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	err := Replace(target, ".snapshot-*", []byte("replacement"))
	var renameError *os.LinkError
	if err == nil || !strings.HasPrefix(err.Error(), "replace file:") || !errors.As(err, &renameError) {
		t.Fatalf("rename failure lost its context or underlying error: %v", err)
	}
	body, err := os.ReadFile(sentinel)
	if err != nil || string(body) != "original" {
		t.Fatal("failed rename changed destination")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != "destination" {
		t.Fatalf("failed replacement leaked temporary file: %v, %v", files, err)
	}
}

func TestCreationFailuresPreserveExistingDataAndPropagateErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	err := Replace(filepath.Join(path, "blocked.json"), ".snapshot-*", nil)
	var pathError *os.PathError
	if err == nil || !strings.HasPrefix(err.Error(), "create directory:") || !errors.As(err, &pathError) {
		t.Fatalf("directory error lost context or cause: %v", err)
	}
	err = Replace(path, "invalid/pattern", []byte("new"))
	if err == nil || !strings.HasPrefix(err.Error(), "create temporary file:") {
		t.Fatalf("temporary file error swallowed: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "existing" {
		t.Fatal("creation failure changed existing file")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("creation failure left artifacts: %v, %v", files, err)
	}
}

package noticestore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
)

func TestRoundTripAndReplacement(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "notices", "account.json")}
	if reviews, err := s.Load(); err != nil || len(reviews) != 0 {
		t.Fatalf("missing file: %v, %v", reviews, err)
	}
	want := map[string]farm.Review{"TEST/fault/1234": {Level: "error", Code: "1234", Since: time.Now().UTC(), Acknowledged: true, Dismissed: true}}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: %v, %v", got, err)
	}
	info, err := os.Stat(s.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions: %v, %v", info, err)
	}
	if err := s.Save(map[string]farm.Review{}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || len(got) != 0 {
		t.Fatal("empty replacement retained decisions")
	}
	files, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", files, err)
	}
}

func TestInvalidFilesAndWriteFailure(t *testing.T) {
	for _, body := range []string{`{`, `{"version":2,"reviews":{}}`} {
		s := Store{Path: filepath.Join(t.TempDir(), "account.json")}
		if err := os.WriteFile(s.Path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Load(); err == nil {
			t.Fatal("accepted invalid/unsupported saved data")
		}
	}
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := Store{Path: filepath.Join(blocker, "account.json")}
	if err := s.Save(nil); err == nil {
		t.Fatal("write failure swallowed")
	}
}

func TestAccountAndRegionIsolation(t *testing.T) {
	a, err := New("global", "user-a")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New("global", "user-b")
	c, _ := New("china", "user-a")
	d, _ := New("global", "user-a")
	if a.Path == b.Path || a.Path == c.Path || a.Path != d.Path {
		t.Fatal("notice paths are not stable and isolated by region/account")
	}
}

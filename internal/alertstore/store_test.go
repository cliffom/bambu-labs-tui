package alertstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSnoozeStoreRoundTripAndIsolation checks private, atomic account/region snapshots.
func TestSnoozeStoreRoundTripAndIsolation(t *testing.T) {
	a, err := New("global", "account-a")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New("global", "account-b")
	c, _ := New("china", "account-a")
	if a.Path == b.Path || a.Path == c.Path {
		t.Fatal("account/region paths collide")
	}
	s := Store{Path: filepath.Join(t.TempDir(), "alerts", "account.json")}
	if v, err := s.Load(); err != nil || len(v) != 0 {
		t.Fatal("missing store not empty")
	}
	until := time.Now().UTC().Add(time.Hour)
	if err := s.Save(map[string]time.Time{"H": until}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || !got["H"].Equal(until) {
		t.Fatalf("deadline changed: %v, %v", got, err)
	}
	info, err := os.Stat(s.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("snooze permissions not private")
	}
	if err := s.Save(nil); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || len(got) != 0 {
		t.Fatal("empty snapshot retained deadlines")
	}
	files, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil || len(files) != 1 {
		t.Fatal("temporary files leaked")
	}
}

// TestSnoozeStoreRejectsInvalidData checks load errors protect bad and future data.
func TestSnoozeStoreRejectsInvalidData(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "account.json")}
	for _, body := range []string{`{`, `{"version":2,"snoozes":{}}`, `{"version":1,"snoozes":{"H":"bad date"}}`, `{"version":1,"snoozes":{"H":"0001-01-01T00:00:00Z"}}`, `{"version":1,"snoozes":{"":"2030-01-01T00:00:00Z"}}`} {
		if err := os.WriteFile(s.Path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Load(); err == nil {
			t.Fatalf("accepted invalid store: %s", body)
		}
	}
	s.Path = filepath.Join(s.Path, "blocked.json")
	if err := s.Save(nil); err == nil {
		t.Fatal("write failure swallowed")
	}
}

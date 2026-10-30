package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/alertstore"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/noticestore"
)

// Existing installations keep their configuration and account snapshots in place.
// Seed the old paths directly rather than saving through the current constructors.
func TestUpgradeReadsOriginalConfigurationAndDecisions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "bambu-tui")
	write := func(path, contents string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "config.toml")
	write(path, "region = \"china\"\nlow_filament_percent = 23\n")
	gotPath, err := config.Path()
	if err != nil || gotPath != path {
		t.Fatalf("configuration moved: %q, %v", gotPath, err)
	}
	cfg, err := config.Load(gotPath)
	if err != nil || cfg.Region != "china" || cfg.LowFilamentPercent != 23 {
		t.Fatal(cfg, err)
	}
	id := sha256.Sum256([]byte("global\x00synthetic-account"))
	filename := fmt.Sprintf("%x.json", id)
	write(filepath.Join(root, "notices", filename), `{"version":1,"reviews":{"synthetic/fault":{"level":"warning","code":"fixture","since":"2026-10-29T12:00:00Z","acknowledged":true,"dismissed":true}}}`)
	notices, err := noticestore.New("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	reviews, err := notices.Load()
	if err != nil || !reviews["synthetic/fault"].Acknowledged || !reviews["synthetic/fault"].Dismissed {
		t.Fatal(reviews, err)
	}
	write(filepath.Join(root, "alerts", filename), `{"version":1,"snoozes":{"synthetic":"2030-01-01T00:00:00Z"}}`)
	alerts, err := alertstore.New("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	snoozes, err := alerts.Load()
	if err != nil || !snoozes["synthetic"].Equal(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal(snoozes, err)
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/accountstore"
	"github.com/cliffom/bambustand/internal/alertstore"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/diagnostics"
	"github.com/cliffom/bambustand/internal/noticestore"
	"github.com/cliffom/bambustand/internal/notification_state"
	"github.com/cliffom/bambustand/internal/ui"
)

func TestCanonicalStorageAndExplicitConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	path, err := config.Path()
	if err != nil || path != filepath.Join(root, "com.suitepotato.bambustand", "config.toml") {
		t.Fatal(path, err)
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.Region != "global" {
		t.Fatal(cfg, err)
	}
	paths := map[string]bool{}
	for _, region := range []string{"global", "china"} {
		for _, account := range []string{"synthetic-one", "synthetic-two"} {
			notices, err := noticestore.New(region, account)
			if err != nil {
				t.Fatal(err)
			}
			snoozes, err := alertstore.New(region, account)
			if err != nil {
				t.Fatal(err)
			}
			ledger, err := notification_state.New(region, account)
			if err != nil {
				t.Fatal(err)
			}
			for folder, path := range map[string]string{"notices": notices.Path, "alerts": snoozes.Path, "notifications": ledger.Path} {
				if filepath.Dir(path) != filepath.Join(root, "com.suitepotato.bambustand", folder) || paths[path] {
					t.Fatal("incorrect namespace or shared account storage", path)
				}
				paths[path] = true
			}
			owner, err := accountstore.Open(region, account)
			if err != nil {
				t.Fatal(err)
			}
			owner.Close()
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	destination, err := diagnostics.Destination(time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC))
	if err != nil || filepath.Dir(destination) != filepath.Join(cache, "com.suitepotato.bambustand", "diagnostics") {
		t.Fatal(destination, err)
	}
	// Explicit configuration keeps relative themes beside that TOML file.
	directory := t.TempDir()
	explicit := filepath.Join(directory, "config.toml")
	if err := os.WriteFile(explicit, []byte("region = \"china\"\ntheme_file = \"custom.json\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "custom.json"), []byte(`{"version":1,"name":"Synthetic","colors":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(explicit)
	if err != nil || cfg.Region != "china" || cfg.ThemeFile != "custom.json" {
		t.Fatal(cfg, err)
	}
	theme, err := ui.LoadTheme(explicit, cfg.ThemeFile)
	if err != nil || theme.Source != filepath.Join(directory, "custom.json") {
		t.Fatal(theme.Source, err)
	}
}

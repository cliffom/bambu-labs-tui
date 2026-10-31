package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/accountstore"
	"github.com/cliffom/bambustand/internal/appidentity"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/notification_state"
	"github.com/cliffom/bambustand/internal/ui"
)

func upgradeRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func upgradeWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeConfigurationPrecedenceAndRelativeTheme(t *testing.T) {
	root := upgradeRoot(t)
	legacy := filepath.Join(root, appidentity.LegacyDirectory, "config.toml")
	upgradeWrite(t, legacy, "region = \"china\"\nlow_filament_percent = 23\ntheme_file = \"custom.json\"\n")
	theme := filepath.Join(filepath.Dir(legacy), "custom.json")
	upgradeWrite(t, theme, `{"version":1,"name":"Synthetic","colors":{}}`)
	current, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(current) != filepath.Join(root, appidentity.Namespace) {
		t.Fatal(current)
	}
	cfg, err := config.Load(current)
	if err != nil || cfg.Region != "china" || cfg.LowFilamentPercent != 23 || cfg.ThemeFile != theme {
		t.Fatal(cfg, err)
	}
	if _, err := ui.LoadTheme(current, cfg.ThemeFile); err != nil {
		t.Fatal(err)
	}
	// Explicit config paths preserve their relative theme interpretation.
	explicit, err := config.Load(legacy)
	if err != nil || explicit.ThemeFile != "custom.json" {
		t.Fatal(explicit, err)
	}
	// Newer destination configuration wins on repeat launch.
	upgradeWrite(t, current, "region = \"global\"\n")
	again, err := config.Load(current)
	if err != nil || again.Region != "global" {
		t.Fatal(again, err)
	}
}

func TestUpgradeAccountDecisionsArchivesAndReplayProtection(t *testing.T) {
	root := upgradeRoot(t)
	for _, region := range []string{"global", "china"} {
		for _, username := range []string{"synthetic-one", "synthetic-two"} {
			t.Run(region+"/"+username, func(t *testing.T) {
				id := fmt.Sprintf("%x", sha256.Sum256([]byte(region+"\x00"+username)))
				legacy := filepath.Join(root, appidentity.LegacyDirectory)
				upgradeWrite(t, filepath.Join(legacy, "notices", id+".json"), `{"version":1,"reviews":{"synthetic/fault":{"level":"warning","code":"fixture","since":"2026-10-29T12:00:00Z","acknowledged":true,"dismissed":true}}}`)
				upgradeWrite(t, filepath.Join(legacy, "alerts", id+".json"), `{"version":1,"snoozes":{"synthetic":"2030-01-01T00:00:00Z"}}`)
				current, err := notification_state.New(region, username)
				if err != nil {
					t.Fatal(err)
				}
				snapshot := func(task, state string) farm.NotificationSnapshot {
					return farm.NotificationSnapshot{"synthetic": {Job: farm.NotificationJob{Task: task, State: state}}}
				}
				for _, suffix := range []string{"", ".browser"} {
					old := notification_state.Store{Path: filepath.Join(legacy, "notifications", filepath.Base(current.Path)+suffix)}
					for _, task := range []string{"archived-job", "live-job"} {
						if _, err := old.Observe(snapshot(task, farm.StateRunning)); err != nil {
							t.Fatal(err)
						}
						events, err := old.Observe(snapshot(task, farm.StateFinished))
						if err != nil || len(events) != 1 {
							t.Fatal(events, err)
						}
						if _, err := old.Reserve(events[0].ID); err != nil {
							t.Fatal(err)
						}
					}
				}
				owner, err := accountstore.Open(region, username)
				if err != nil {
					t.Fatal(err)
				}
				reviews, err := owner.LoadReviews()
				if err != nil || !reviews["synthetic/fault"].Acknowledged || !reviews["synthetic/fault"].Dismissed {
					t.Fatal(reviews, err)
				}
				snoozes, err := owner.LoadSnoozes()
				if err != nil || !snoozes["synthetic"].Equal(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
					t.Fatal(snoozes, err)
				}
				// Retain original lock inodes throughout monitoring, including restart.
				if lock, err := appidentity.Lock(filepath.Join(legacy, "sessions", id+".lock")); err == nil {
					_ = lock.Close()
					t.Fatal("legacy host can acquire account ownership")
				}
				for _, suffix := range []string{"", ".browser"} {
					migrated := notification_state.Store{Path: current.Path + suffix}
					for _, task := range []string{"archived-job", "live-job"} {
						for _, state := range []string{farm.StateRunning, farm.StateFinished} {
							events, err := migrated.Observe(snapshot(task, state))
							if err != nil || len(events) != 0 {
								t.Fatal("notification replay", events, err)
							}
						}
					}
				}
				if err := owner.SaveReviews(nil); err != nil {
					t.Fatal(err)
				}
				owner.Close()
				again, err := accountstore.Open(region, username)
				if err != nil {
					t.Fatal(err)
				}
				defer again.Close()
				reviews, err = again.LoadReviews()
				if err != nil || len(reviews) != 0 {
					t.Fatal("legacy decisions resurrected", reviews, err)
				}
			})
		}
	}
}

func TestUpgradeBusyConflictAndInterruptedCopy(t *testing.T) {
	root := upgradeRoot(t)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte("global\x00synthetic")))
	source := filepath.Join(root, appidentity.LegacyDirectory, "notices", id+".json")
	destination := filepath.Join(root, appidentity.Namespace, "notices", id+".json")
	upgradeWrite(t, source, `{"version":1,"reviews":{}}`)
	legacyLock := filepath.Join(root, appidentity.LegacyDirectory, "sessions", id+".lock")
	lock, err := appidentity.Lock(legacyLock)
	if err != nil {
		t.Fatal(err)
	}
	if owner, err := accountstore.Open("global", "synthetic"); !errors.Is(err, accountstore.ErrBusy) {
		if owner != nil {
			owner.Close()
		}
		t.Fatal(err)
	}
	_ = lock.Close()
	// A legacy notification worker also blocks migration independently of its
	// account lease, so no ledger is copied while old delivery is active.
	ledger, err := notification_state.New("global", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	oldOwnerPath := filepath.Join(root, appidentity.LegacyDirectory, "notifications", filepath.Base(ledger.Path)+".owner.lock")
	oldOwner, err := appidentity.Lock(oldOwnerPath)
	if err != nil {
		t.Fatal(err)
	}
	if owner, err := accountstore.Open("global", "synthetic"); !errors.Is(err, accountstore.ErrBusy) {
		if owner != nil {
			owner.Close()
		}
		t.Fatal(err)
	}
	_ = oldOwner.Close()
	// Simulate a destination saved before migration. No merge can establish which
	// notification claims are newer, so conflicting state requires recovery.
	upgradeWrite(t, destination, "newer state")
	if owner, err := accountstore.Open("global", "synthetic"); !errors.Is(err, appidentity.ErrMigration) {
		if owner != nil {
			owner.Close()
		}
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "newer state" {
		t.Fatal(string(data), err)
	}
	// An interrupted copy left identical notices but a failed snooze destination.
	upgradeWrite(t, destination, `{"version":1,"reviews":{}}`)
	blocker := filepath.Join(root, appidentity.Namespace, "alerts")
	upgradeWrite(t, filepath.Join(root, appidentity.LegacyDirectory, "alerts", id+".json"), `{"version":1,"snoozes":{}}`)
	upgradeWrite(t, blocker, "blocked")
	if owner, err := accountstore.Open("global", "synthetic"); err == nil {
		owner.Close()
		t.Fatal("failed copy reported success")
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	owner, err := accountstore.Open("global", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	owner.Close()
}

func TestFreshNamespaceAndCacheMigration(t *testing.T) {
	root := upgradeRoot(t)
	owner, err := accountstore.Open("global", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	owner.Close()
	if _, err := os.Stat(filepath.Join(root, appidentity.LegacyDirectory)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fresh installation created legacy storage", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	upgradeWrite(t, filepath.Join(cache, appidentity.LegacyDirectory, "diagnostics", "synthetic.json"), "{}")
	for range 2 {
		if err := appidentity.MigrateCache(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(cache, appidentity.Namespace, "diagnostics", "synthetic.json"))
	if err != nil || string(data) != "{}" {
		t.Fatal(string(data), err)
	}
}

package ui

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/config"
)

func TestFarmViewPersistsAndRestoresThroughConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "view.toml")
	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := overviewModel(t).WithConfiguration(saved).WithConfigPersistence(path, func(snapshot config.Config) error {
		if err := config.SaveChanges(path, saved, snapshot); err != nil {
			return err
		}
		saved = snapshot.Clone()
		return nil
	})
	reviews, snoozes := m.Farm.Reviews(), m.Snoozes
	m = persistenceKey(m, "F")
	m = persistenceKey(m, "S")
	loaded, err := config.Load(path)
	if err != nil || loaded.View != (config.ViewPreferences{Filter: config.FilterAttention, Sort: config.SortAttention}) || m.HasUnsavedPersistence() {
		t.Fatal(loaded, err)
	}
	restarted := overviewModel(t).WithConfiguration(loaded)
	if restarted.printerFilter != filterAttention || restarted.printerSort != sortAttention || restarted.current().ID != "X" {
		t.Fatal("restart did not restore view")
	}
	if !reflect.DeepEqual(reviews, m.Farm.Reviews()) || !reflect.DeepEqual(snoozes, m.Snoozes) {
		t.Fatal("view edited review/snooze state")
	}
}

func TestSavedEmptyViewResetAndDialogGuards(t *testing.T) {
	m := overviewModel(t)
	for _, item := range m.Farm.Attention() {
		m.Farm.Acknowledge(item.Key, true)
	}
	cfg := m.configuration()
	cfg.View = config.ViewPreferences{Filter: config.FilterAttention, Sort: config.SortFinish}
	m = m.WithConfiguration(cfg)
	if m.current() != nil || !strings.Contains(ansi.Strip(m.View()), "0 resets All / Name") {
		t.Fatal("empty saved view lost reset guidance")
	}
	m = key(m, "?")
	m = key(m, "0")
	if m.printerFilter != filterAttention || m.printerSort != sortFinish {
		t.Fatal("reset escaped dialog")
	}
	m = key(m, "esc")
	m = resizeModel(m, 59, 17)
	m = key(m, "0")
	if m.printerFilter != filterAttention {
		t.Fatal("hidden reset changed view")
	}
	m = resizeModel(m, 60, 18)
	m = key(m, "0")
	if m.printerFilter != filterAll || m.printerSort != sortName || m.current() == nil {
		t.Fatal("empty-view reset failed")
	}
	revision := m.ConfigRevision()
	m = key(m, "0")
	if m.ConfigRevision() != revision {
		t.Fatal("unchanged reset scheduled another save")
	}
}

func TestFarmViewWritesCoalesceAndFailureCanRetry(t *testing.T) {
	var snapshots []config.Config
	fail := true
	m := overviewModel(t).WithConfigPersistence("synthetic", func(snapshot config.Config) error {
		snapshots = append(snapshots, snapshot)
		if fail {
			return errors.New("synthetic failure")
		}
		return nil
	})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	m = next.(Model)
	// The first command owns an immutable Attention/Name snapshot. Later edits
	// coalesce with Config edits into a complete latest snapshot.
	m = key(m, "S")
	m = key(m, ",")
	m = key(m, "enter")
	m = completePersistence(m, cmd)
	if len(snapshots) != 2 || snapshots[0].View.Sort != config.SortName || snapshots[1].View.Sort != config.SortAttention || snapshots[1].Region != config.RegionChina {
		t.Fatal(snapshots)
	}
	if !m.HasUnsavedPersistence() || !strings.Contains(ansi.Strip(m.View()), "Save failed") {
		t.Fatal("write failure hidden")
	}
	// Navigation and unchanged reset do not retry a failed revision.
	m = key(m, "down")
	if len(snapshots) != 2 {
		t.Fatal("navigation retried save")
	}
	fail = false
	m = persistenceKey(m, "r")
	if m.HasUnsavedPersistence() || m.configStorageError != "" || len(snapshots) != 3 {
		t.Fatal("explicit retry did not save view")
	}
}

func TestSavedViewNormalizesBeforeRendering(t *testing.T) {
	m := overviewModel(t)
	cfg := m.configuration()
	cfg.View = config.ViewPreferences{Filter: "next-version", Sort: "unknown"}
	m = m.WithConfiguration(cfg)
	if m.printerFilter != filterAll || m.printerSort != sortName {
		t.Fatal("invalid values were restored")
	}
	m.selectPrinter("H")
	selected := m.current().ID
	m = key(m, "S")
	if m.current().ID != selected {
		t.Fatal("saved sort lost selected printer identity")
	}
}

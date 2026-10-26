package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
)

func TestConfigTogglesStartAndStopDeliveryImmediately(t *testing.T) {
	m, client, _ := notificationModel(t)
	m.alertPrefs.DesktopNotifications = false
	m = key(m, ",")
	if m.dialog.active != dialogConfig || len(client.inputs) != 0 {
		t.Fatal("disabled startup observed ledger")
	}
	m = key(m, " ")
	if !m.alertPrefs.DesktopNotifications || !client.inputs[len(client.inputs)-1].Policies["TEST"].Desktop {
		t.Fatal("toggle did not enable delivery immediately")
	}
	m = key(m, " ")
	if client.inputs[len(client.inputs)-1].Policies["TEST"].Desktop {
		t.Fatal("toggle did not cancel delivery policy")
	}
	m = key(m, "down")
	m = key(m, "enter")
	if m.alertPrefs.NotificationSound {
		t.Fatal("sound toggle missed selection")
	}
	m = key(m, "down")
	m = key(m, " ")
	if m.alertPrefs.CompletedPrints {
		t.Fatal("completion toggle missed selection")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 60, Height: 18}, {Width: 120, Height: 32}, {Width: 59, Height: 17}} {
		next, _ := m.Update(size)
		m = next.(Model)
		view := ansi.Strip(m.View())
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size.Width {
				t.Fatal("config exceeds width")
			}
		}
		if size.Width < minimumWidth {
			before := m.alertPrefs.AlertRules
			m = key(m, " ")
			if m.alertPrefs.AlertRules != before {
				t.Fatal("hidden toggle acted below minimum")
			}
		}
	}
	m = key(m, "esc")
	if m.hasOpenDialog() {
		t.Fatal("Esc did not close config below minimum")
	}
}

func TestConfigSaveCoalescesAndBlocksQuit(t *testing.T) {
	m := demoModel()
	var saved []config.AlertRules
	m = m.WithConfigPersistence("synthetic.toml", func(rules config.AlertRules) error { saved = append(saved, rules); return nil })
	m = key(m, ",")
	// Dispatch explicitly: persistence must not call the writer in the reducer.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if len(saved) != 0 || !m.configWrites.inFlight {
		t.Fatal("save ran in reducer or was not queued")
	}
	first := m.configRevision
	m = key(m, "enter") // Newer value waits behind the first snapshot.
	if m.configWrites.attempted != first {
		t.Fatal("overlapping config writes")
	}
	m = key(m, "esc")
	m = key(m, "q")
	if !m.quitting || !m.persistencePending() {
		t.Fatal("quit skipped pending config")
	}
	m = completePersistence(m, cmd)
	if len(saved) != 2 || !saved[0].DesktopNotifications || saved[1].DesktopNotifications || m.HasUnsavedPersistence() {
		t.Fatal("latest config did not follow immutable first snapshot")
	}
}

func TestConfigSaveFailureIsVisibleAndExplicitlyRetried(t *testing.T) {
	m := demoModel()
	writes := 0
	m = m.WithConfigPersistence("synthetic.toml", func(config.AlertRules) error {
		writes++
		if writes == 1 {
			return errors.New("read-only")
		}
		return nil
	})
	m = persistenceKey(m, ",")
	m = persistenceKey(m, "enter")
	if writes != 1 || !m.alertPrefs.DesktopNotifications || !strings.Contains(ansi.Strip(m.View()), "Save failed") || !m.HasUnsavedPersistence() {
		t.Fatal("failed save did not preserve session setting and warning")
	}
	m = persistenceKey(m, "down")
	if writes != 1 {
		t.Fatal("navigation retried failed save")
	}
	m = persistenceKey(m, "esc")
	m = key(m, "q")
	if !m.quitProblem || m.quitting {
		t.Fatal("quit silently discarded a failed config save")
	}
	m = key(m, "esc")
	m = persistenceKey(m, ",")
	m = persistenceKey(m, "r")
	if writes != 2 || m.HasUnsavedPersistence() || m.configStorageError != "" {
		t.Fatal("explicit retry did not save")
	}
}

func TestConfigGlobalTogglePreservesPrinterOverride(t *testing.T) {
	m, client, _ := notificationModel(t)
	desktop := true
	m.alertPrefs.Printers = map[string]config.AlertOverride{"TEST": {DesktopNotifications: &desktop}}
	m = key(m, ",")
	m = key(m, "enter")
	if m.alertPrefs.DesktopNotifications || !client.inputs[len(client.inputs)-1].Policies["TEST"].Desktop {
		t.Fatal("global toggle overwrote a printer override")
	}
	if !strings.Contains(ansi.Strip(m.View()), "printer overrides") {
		t.Fatal("override behavior is hidden")
	}
}

func TestConfigFooterUsesOnlyHelpWithStorageWarnings(t *testing.T) {
	for _, width := range []int{60, 120} {
		for _, failed := range []bool{false, true} {
			m := resizeModel(demoModel(), width, 18)
			m = key(m, ",")
			if failed {
				m.configStorageError = "Config save failed"
			}
			rows := strings.Split(ansi.Strip(m.View()), "\n")
			footer := rows[len(rows)-1]
			if strings.TrimSpace(footer) != "Show macOS alerts while this TUI runs." {
				t.Fatal("Config footer should contain only selected-option help")
			}
			if !strings.Contains(rows[1], "Esc / ,") {
				t.Fatal("header hides close keys")
			}
			if failed && !strings.Contains(strings.Join(rows[:len(rows)-1], "\n"), "Config save failed") {
				t.Fatal("body hides save failure")
			}
			if !strings.Contains(strings.Join(rows[:len(rows)-1], "\n"), "removes comments") {
				t.Fatal("TOML save note missing from body")
			}
			m = key(m, ",")
			if m.hasOpenDialog() {
				t.Fatal("advertised close key did not close Config")
			}
		}
	}
}

func TestConfigFooterExplainsSelectedOption(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m := key(resizeModel(demoModel(), size[0], size[1]), ",")
		for _, description := range []string{
			"Show macOS alerts while this TUI runs.",
			"Request sound on desktop notifications.",
			"Notify on confirmed print completion.",
			"Test delivery using the current sound.",
		} {
			rows := strings.Split(ansi.Strip(m.View()), "\n")
			last := rows[len(rows)-1]
			if strings.TrimSpace(last) != description {
				t.Fatalf("selected option helper missing at %v: %q", size, last)
			}
			if !strings.Contains(rows[1], "Space/Enter") {
				t.Fatal("header must retain activation keys")
			}
			m = key(m, "down")
		}
		m = key(m, "up")
		m.notificationTestPending = true
		rows := strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.Contains(rows[len(rows)-1], "closing cancels") {
			t.Fatal("pending test helper missing")
		}
	}
}

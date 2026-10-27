package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/muesli/termenv"
)

func TestStartupConfigurationPreservesRestoredCloudReviews(t *testing.T) {
	for _, decision := range []string{"acknowledged", "dismissed", "both"} {
		t.Run(decision, func(t *testing.T) {
			observed := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
			previous := farm.New(10)
			previous.Reconcile(observed)
			if decision != "dismissed" {
				previous.Acknowledge(farm.ConditionCloud, true)
			}
			if decision != "acknowledged" {
				previous.Dismiss(farm.ConditionCloud)
			}
			saved := previous.Reviews()

			// Match dashboard startup: install configuration before loading reviews.
			state := farm.New(10)
			m := New(state, nil, nil, false).WithConfiguration(config.Config{
				Region: config.RegionGlobal, LowFilamentPercent: 25, Alerts: config.DefaultAlerts(),
			})
			if state.LowFilament != 25 {
				t.Fatal("startup threshold was not installed")
			}
			state.RestoreReviews(saved)
			for _, at := range []time.Time{observed.Add(time.Hour), observed.Add(2 * time.Hour)} {
				next, _ := m.Update(tickMsg(at))
				m = next.(Model)
				condition, ok := state.Active[farm.ConditionCloud]
				want := saved[farm.ConditionCloud]
				if !ok || condition.Acknowledged != want.Acknowledged || condition.Dismissed != want.Dismissed || !condition.Since.Equal(want.Since) {
					t.Fatalf("restored cloud occurrence changed: got %+v, want %+v", condition, want)
				}
				if state.Unacknowledged() != 0 {
					t.Fatal("saved cloud review became unreviewed")
				}
			}
		})
	}
}

func openAlertConfig(m Model) Model {
	m = key(m, ",")
	for m.configSection != configAlerts {
		m = key(m, "tab")
	}
	return m
}

func TestConfigTogglesStartAndStopDeliveryImmediately(t *testing.T) {
	m, client, _ := notificationModel(t)
	m.alertPrefs.DesktopNotifications = false
	m = openAlertConfig(m)
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
	var saved []config.Config
	m = m.WithConfigPersistence("synthetic.toml", func(rules config.Config) error { saved = append(saved, rules); return nil })
	m = openAlertConfig(m)
	if strings.Contains(ansi.Strip(m.View()), "Saved") {
		t.Fatal("idle Config should not show Saved")
	}
	// Dispatch explicitly: persistence must not call the writer in the reducer.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if len(saved) != 0 || !m.ConfigWrites().InFlight {
		t.Fatal("save ran in reducer or was not queued")
	}
	if strings.Contains(ansi.Strip(m.View()), "Saving…") {
		t.Fatal("pending saves should remain silent")
	}
	first := m.ConfigRevision()
	m = key(m, "enter") // Newer value waits behind the first snapshot.
	if m.ConfigWrites().Attempted != first {
		t.Fatal("overlapping config writes")
	}
	m = key(m, "esc")
	m = key(m, "q")
	if !m.quitting || !m.persistencePending() {
		t.Fatal("quit skipped pending config")
	}
	m = completePersistence(m, cmd)
	if view := ansi.Strip(m.configView()); strings.Contains(view, "Saved") || strings.Contains(view, "Saving…") {
		t.Fatal("successful save should clear status")
	}
	if len(saved) != 2 || !saved[0].Alerts.DesktopNotifications || saved[1].Alerts.DesktopNotifications || m.HasUnsavedPersistence() {
		t.Fatal("latest config did not follow immutable first snapshot")
	}
}

func TestConfigSaveFailureIsVisibleAndExplicitlyRetried(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	m := demoModel()
	writes := 0
	m = m.WithConfigPersistence("synthetic.toml", func(config.Config) error {
		writes++
		if writes == 1 {
			return errors.New("read-only")
		}
		return nil
	})
	m = openAlertConfig(m)
	m = persistenceKey(m, "enter")
	if writes != 1 || !m.alertPrefs.DesktopNotifications || !strings.Contains(ansi.Strip(m.View()), "Save failed") || !m.HasUnsavedPersistence() {
		t.Fatal("failed save did not preserve session setting and warning")
	}
	if !strings.Contains(m.View(), m.styles.err.Render("Save failed · r retry; changes may be lost on exit.")) || !strings.Contains(m.View(), "38;2;255;128;135") {
		t.Fatal("save failure should use the shared red failure style")
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
	m = openAlertConfig(m)
	m = persistenceKey(m, "r")
	if writes != 2 || m.HasUnsavedPersistence() || m.configStorageError != "" {
		t.Fatal("explicit retry did not save")
	}
}

func TestConfigGlobalTogglePreservesPrinterOverride(t *testing.T) {
	m, client, _ := notificationModel(t)
	desktop := true
	m.alertPrefs.Printers = map[string]config.AlertOverride{"TEST": {DesktopNotifications: &desktop}}
	m = openAlertConfig(m)
	m = key(m, "enter")
	if m.alertPrefs.DesktopNotifications || !client.inputs[len(client.inputs)-1].Policies["TEST"].Desktop {
		t.Fatal("global toggle overwrote a printer override")
	}
	if !strings.Contains(ansi.Strip(m.View()), "Printers") {
		t.Fatal("override behavior is hidden")
	}
}

func TestConfigFooterUsesOnlyHelpWithStorageWarnings(t *testing.T) {
	for _, width := range []int{60, 120} {
		for _, failed := range []bool{false, true} {
			m := resizeModel(demoModel(), width, 18)
			m = openAlertConfig(m)
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
			if failed && !strings.Contains(strings.Join(rows[:len(rows)-1], "\n"), "Save failed") {
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
		m := openAlertConfig(resizeModel(demoModel(), size[0], size[1]))
		for _, description := range []string{
			"Show macOS alerts while this TUI runs.",
			"Request sound on desktop notifications.",
			"Notify on confirmed print completion.",
			"Open dialogs for actionable printer conditions.",
			"Alert on verified filament intervention conditions.",
			"Alert on verified extrusion intervention conditions.",
			"Alert on other verified printer faults.",
			"Alert on reported print failures.",
			"Alert on pauses without a classified cause.",
			"Test delivery using the current sound.",
		} {
			rows := strings.Split(ansi.Strip(m.View()), "\n")
			last := rows[len(rows)-1]
			if strings.TrimSpace(last) != description {
				t.Fatalf("selected option helper missing at %v: %q", size, last)
			}
			if !strings.Contains(rows[1], "Enter") {
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

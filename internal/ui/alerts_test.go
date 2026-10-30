package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/alertstore"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
)

// alertTickAt advances exact deadlines without waiting on real time.
func alertTickAt(m Model, at time.Time) Model {
	return persistenceUpdate(m, tickMsg(at))
}

// TestAutomaticAlertPreferences preserves visibility and supports explicit manual inspection.
func TestAutomaticAlertPreferences(t *testing.T) {
	prefs := config.DefaultAlerts()
	prefs.AutomaticModals = false
	m := demoModel().WithAlertPreferences(prefs)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if m.attention != nil || m.Farm.NoticeCount() != 3 || m.Farm.Unacknowledged() != 3 {
		t.Fatal("muting hid/reviewed notices or failed to prevent the modal")
	}
	if !strings.Contains(ansi.Strip(m.detailText(100)), "Automatic modals off") {
		t.Fatal("muted status not visible")
	}
	m = key(m, "!")
	if m.attention == nil || !m.attention.manual {
		t.Fatal("explicit inspection ignored muted actionable notices")
	}
	m = key(m, "esc")
	if len(m.Farm.Reviews()) != 0 {
		t.Fatal("manual inspection saved a review")
	}

	on := true
	prefs.Printers = map[string]config.AlertOverride{"X": {AutomaticModals: &on}}
	m = demoModel().WithAlertPreferences(prefs)
	m = attentionReport(t, m, "H", runoutReport)
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.attention == nil || len(m.attention.groups) != 1 || m.attention.groups[0].id != "X" {
		t.Fatal("per-printer enable override failed")
	}
}

// TestFilamentFilterIncludesRelatedPause prevents duplicate state alerts bypassing a category mute.
func TestFilamentFilterIncludesRelatedPause(t *testing.T) {
	prefs := config.DefaultAlerts()
	prefs.Filament = false
	m := demoModel().WithAlertPreferences(prefs)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if m.attention != nil || m.Farm.NoticeCount() != 3 {
		t.Fatal("muted runout still interrupted through its paused-state notice")
	}
	m = attentionTick(attentionReport(t, m, "H", `{"print":{"gcode_state":"FAILED","hms":[],"print_error":0}}`))
	if m.attention == nil || m.attention.groups[0].items[0].Key != "H/state" {
		t.Fatal("filament filter muted an independent print failure")
	}
}

// TestSnoozeKeepsNoticesAndOtherPrinterAlerts distinguishes snoozing from saved reviews.
func TestSnoozeKeepsNoticesAndOtherPrinterAlerts(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = attentionReport(t, m, "X", runoutReport)
	count := m.Farm.NoticeCount()
	m = key(m, "s")
	if m.alertDialog == nil || !strings.Contains(ansi.Strip(m.View()), "PRINTER ALERTS / SNOOZE") {
		t.Fatal("attention dialog did not open controls for its printer")
	}
	m = key(m, "1")
	if !m.snoozed("H") || m.alertDialog != nil || m.attention == nil || len(m.attention.groups) != 1 || m.attention.groups[0].id != "X" {
		t.Fatal("snooze discarded other printer's queued alerts")
	}
	if m.Farm.NoticeCount() != count || m.Farm.Unacknowledged() != count || len(m.Farm.Reviews()) != 0 {
		t.Fatal("snooze changed underlying notice decisions")
	}
	if !strings.Contains(ansi.Strip(m.detailText(100)), "Snoozed until") {
		t.Fatal("snooze missing from printer details")
	}
	m = key(m, "esc")
	m = key(m, "!")
	if m.attention == nil || m.attention.groups[0].id != "H" {
		t.Fatal("manual inspection could not see snoozed printer")
	}
}

// TestSnoozeRestartExpiryAndRecurrence persists one printer deadline across new occurrences.
func TestSnoozeRestartExpiryAndRecurrence(t *testing.T) {
	store := alertstore.Store{Path: filepath.Join(t.TempDir(), "alerts.json")}
	m := demoModel()
	m.Demo = false
	m = m.WithSnoozePersistence(nil, store.Save, nil)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	m = persistenceKey(m, "esc") // Existing session suppression must not outlast the snooze.
	m = persistenceKey(m, "s")
	m = persistenceKey(m, "2")
	until := m.Snoozes["H"]
	saved, err := store.Load()
	if err != nil || !saved["H"].Equal(until) {
		t.Fatal("snooze not saved after command completion")
	}
	state := farm.New(10)
	next := New(state, nil, nil, false).WithSnoozePersistence(saved, store.Save, nil)
	updated, _ := next.Update(monitor.Event{Kind: monitor.EventConnection, Connected: true, At: m.now})
	next = updated.(Model)
	if !next.snoozed("H") {
		t.Fatal("startup discarded snooze before discovery")
	}
	updated, _ = next.Update(monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "H", Name: "H2D", Online: true}}, At: m.now})
	next = updated.(Model)
	next = attentionTick(attentionReport(t, next, "H", runoutReport))
	if next.attention != nil || !next.snoozed("H") {
		t.Fatal("restart re-prompted while snoozed")
	}
	next = attentionReport(t, next, "H", `{"print":{"gcode_state":"RUNNING","hms":[],"print_error":0}}`)
	next = attentionTick(attentionReport(t, next, "H", runoutReport))
	if next.attention != nil {
		t.Fatal("recurrence bypassed printer snooze")
	}
	next = persistenceKey(next, "!")
	next = persistenceKey(next, "esc") // Manual viewing during a snooze must not suppress its expiry alert.
	next = alertTickAt(next, until.Add(-time.Second))
	if next.attention != nil || !next.snoozed("H") {
		t.Fatal("snooze expired early")
	}
	next = alertTickAt(next, until)
	next = alertTickAt(next, until.Add(3*time.Second))
	if next.attention == nil || len(next.attention.groups[0].items) != 3 {
		t.Fatal("expiry failed to re-arm eligible unreviewed faults")
	}
	saved, err = store.Load()
	if err != nil || len(saved) != 0 {
		t.Fatal("expired snooze was not removed from storage")
	}
}

// TestSnoozeExpiryHonorsPersistedReviews keeps acknowledgements and dismissals independent.
func TestSnoozeExpiryHonorsPersistedReviews(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		t.Run(map[bool]string{false: "acknowledged", true: "dismissed"}[dismiss], func(t *testing.T) {
			m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
			if dismiss {
				m = key(m, "c")
			} else {
				m = key(m, "a")
			}
			reviews := m.Farm.Reviews()
			state := farm.New(10)
			state.RestoreReviews(reviews)
			until := m.now.Add(time.Hour)
			next := New(state, nil, nil, false).WithSnoozePersistence(map[string]time.Time{"H": until}, nil, nil)
			for _, event := range []monitor.Event{{Kind: monitor.EventConnection, Connected: true, At: m.now}, {Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "H", Online: true}}, At: m.now}} {
				updated, _ := next.Update(event)
				next = updated.(Model)
			}
			next = attentionReport(t, next, "H", runoutReport)
			next = alertTickAt(next, until.Add(3*time.Second))
			next = attentionTick(next)
			if next.attention != nil || len(next.Farm.Reviews()) != len(reviews) {
				t.Fatal("expiry overrode persisted reviews")
			}
			for keyName := range reviews {
				c := next.Farm.Active[keyName]
				if c.Dismissed != dismiss || c.Acknowledged == dismiss {
					t.Fatal("review flags changed")
				}
			}
		})
	}
}

// TestResumeAndRemovedPrinterCleanup handles explicit resume and discovery-scoped removal.
func TestResumeAndRemovedPrinterCleanup(t *testing.T) {
	m := demoModel()
	m.Demo = false
	saves := 0
	m = m.WithSnoozePersistence(map[string]time.Time{"H": m.now.Add(time.Hour), "removed": m.now.Add(time.Hour)}, func(v map[string]time.Time) error { saves++; return nil }, nil)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if len(m.Snoozes) != 2 {
		t.Fatal("pending printer was pruned before discovery")
	}
	m = persistenceUpdate(m, monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "H", Online: true}}, At: m.now})
	if len(m.Snoozes) != 1 || saves != 1 {
		t.Fatal("confirmed removed printer's snooze was not pruned/saved")
	}
	m = persistenceKey(m, "s")
	m = persistenceKey(m, "u")
	m = attentionTick(m)
	if m.snoozed("H") || saves != 2 || m.attention == nil {
		t.Fatal("resume did not persist or re-arm alerts")
	}
}

// TestSnoozeStorageFailuresAndDemoIsolation preserves bad files and keeps warnings visible.
func TestSnoozeStorageFailuresAndDemoIsolation(t *testing.T) {
	m := demoModel()
	m.Demo = false
	writes := 0
	m = m.WithSnoozePersistence(nil, func(map[string]time.Time) error { writes++; return errors.New("disk full") }, nil)
	m = persistenceKey(m, "s")
	m = persistenceKey(m, "1")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Could not save snoozes") {
		t.Fatal("failed save was not visible")
	}
	m = persistenceKey(m, "s")
	if !strings.Contains(ansi.Strip(m.View()), "Could not save snoozes") {
		t.Fatal("controls hid storage warning")
	}
	m = persistenceKey(m, "esc")
	m = attentionTick(m)
	if writes != 1 {
		t.Fatal("save retried continuously")
	}
	m = m.WithSnoozePersistence(nil, func(map[string]time.Time) error { writes++; return nil }, errors.New("bad file"))
	m = persistenceKey(m, "s")
	m = persistenceKey(m, "2")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Saved snoozes unavailable") {
		t.Fatal("failed load overwrote original data")
	}
	demo := demoModel().WithSnoozePersistence(nil, func(map[string]time.Time) error { writes++; return nil }, nil)
	demo = persistenceKey(demo, "s")
	demo = persistenceKey(demo, "1")
	if writes != 1 || !demo.snoozed("H") {
		t.Fatal("demo wrote account snoozes or could not snooze locally")
	}
}

// TestSnoozeDialogBoundsAndLiveRemoval keeps controls safe during resize and discovery.
func TestSnoozeDialogBoundsAndLiveRemoval(t *testing.T) {
	m := demoModel()
	m = key(m, "s")
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}, {0, 0}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		view := m.View()
		if view != "" && len(strings.Split(view, "\n")) > size[1] {
			t.Fatal("height overflow")
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow")
			}
		}
	}
	next, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, At: m.now})
	m = next.(Model)
	m = key(m, "1")
	if len(m.Snoozes) != 0 {
		t.Fatal("removed printer acquired a snooze")
	}
}

// TestResetReviewsDoesNotEndSnooze keeps interruption deadlines separate from permanent decisions.
func TestResetReviewsDoesNotEndSnooze(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "a")
	m = key(m, "s")
	m = key(m, "3")
	until := m.Snoozes["H"]
	m = key(m, "n")
	m = key(m, "R")
	m = key(m, "y")
	m = key(m, "esc")
	m = attentionTick(m)
	if !m.Snoozes["H"].Equal(until) || m.attention != nil || len(m.Farm.Reviews()) != 0 || m.Farm.Unacknowledged() != 3 {
		t.Fatal("review reset changed snooze or failed to preserve notices")
	}
}

// TestAlertControlsCaptureQuitAndReturnToPrompt keeps layered dialogs from leaking global input.
func TestAlertControlsCaptureQuitAndReturnToPrompt(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "s")
	for _, input := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		next, cmd := m.Update(input)
		m = next.(Model)
		if cmd != nil || m.alertDialog == nil || m.attention == nil {
			t.Fatal("quit escaped layered alert controls")
		}
	}
	m = key(m, "esc")
	if m.alertDialog != nil || m.attention == nil {
		t.Fatal("cancel did not restore the action-needed prompt")
	}
}

// TestMutedCategoriesDoNotSilenceOtherFaults checks a mixed pause retains enabled causes.
func TestMutedCategoriesDoNotSilenceOtherFaults(t *testing.T) {
	prefs := config.DefaultAlerts()
	prefs.Filament = false
	m := demoModel().WithAlertPreferences(prefs)
	m = attentionTick(attentionReport(t, m, "H", `{"print":{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073},{"attr":50333952,"code":131073}]}}`))
	if m.attention == nil {
		t.Fatal("filament filter suppressed a separate extrusion fault")
	}
	foundState, foundExtrusion := false, false
	for _, item := range m.attention.groups[0].items {
		if item.Key == "H/state" {
			foundState = true
			continue
		}
		if len(item.Categories) != 1 || item.Categories[0] != "extrusion" {
			t.Fatalf("muted category leaked into automatic prompt: %+v", item)
		}
		foundExtrusion = true
	}
	if !foundState || !foundExtrusion {
		t.Fatal("enabled fault and its related pause were not presented")
	}
}

// TestMinimumSizeShowsAlertControls keeps the snooze entry and close actions discoverable.
func TestMinimumSizeShowsAlertControls(t *testing.T) {
	m := demoModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = next.(Model)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "s alerts") || !strings.Contains(view, "q quit") {
		t.Fatal("minimum dashboard hides alert/quit shortcuts")
	}
	m = key(m, "s")
	for _, text := range []string{"1: 15m", "2: 1h", "3: 4h", "u: resume", "s / Esc close"} {
		if !strings.Contains(ansi.Strip(m.View()), text) {
			t.Fatalf("minimum controls hide %q", text)
		}
	}
}

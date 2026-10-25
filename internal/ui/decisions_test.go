package ui

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/noticestore"
)

// decisionText reads the full reference content independently of viewport height.
func decisionText(m Model) string {
	view := m.decisions.view
	view.Height = 300
	view.GotoTop()
	return ansi.Strip(view.View())
}

// TestDecisionNavigationAndRestore checks independent flags, identity, and dashboard focus.
func TestDecisionNavigationAndRestore(t *testing.T) {
	m := noticeModel(t)
	item, _ := m.selectedNotice()
	m.Farm.Acknowledge(item.ActiveKey, true)
	m.Farm.Dismiss(item.ActiveKey)
	for keyName := range m.Farm.Active {
		if keyName != item.ActiveKey {
			m.Farm.Dismiss(keyName)
			break
		}
	}
	m.focus = 1
	m.details.SetYOffset(2)
	offset := m.details.YOffset
	m = key(m, "n")
	m.decisions.key = item.ActiveKey
	m.syncDecisions()
	for _, text := range []string{item.Printer, item.Text, item.Code, "Active", "Dismissed locally", "Acknowledged locally"} {
		if !strings.Contains(decisionText(m), text) {
			t.Fatalf("decision view lacks %q", text)
		}
	}
	m = key(m, "G")
	if m.decisions.index != len(m.decisionItems())-1 {
		t.Fatal("last-item navigation failed")
	}
	m = key(m, "g")
	m = key(m, "j")
	m = key(m, "k")
	if m.decisions.index != 0 {
		t.Fatal("item navigation failed")
	}
	m.decisions.key = item.ActiveKey
	m.syncDecisions()
	// Earlier-sorting reports must not move the operator onto another decision.
	reviews := m.Farm.Reviews()
	reviews["A/fault/pending"] = farm.Review{Dismissed: true, Since: m.now}
	m.Farm.RestoreReviews(reviews)
	m.syncDecisions()
	if m.decisions.key != item.ActiveKey {
		t.Fatal("incoming decision moved selection")
	}
	beforeRaw, _ := json.Marshal(m.Farm.Printers["H"].Raw)
	events := len(m.Farm.Events)
	m = key(m, "r")
	if c := m.Farm.Active[item.ActiveKey]; c.Dismissed || !c.Acknowledged {
		t.Fatal("restore changed acknowledgement")
	}
	if m.decisions.key != item.ActiveKey {
		t.Fatal("restore lost identity while acknowledgement remains")
	}
	m = key(m, "u")
	if m.Farm.Active[item.ActiveKey].Acknowledged {
		t.Fatal("explicit unreviewed action failed")
	}
	afterRaw, _ := json.Marshal(m.Farm.Printers["H"].Raw)
	if string(beforeRaw) != string(afterRaw) || len(m.Farm.Events) != events || len(m.Refresh) != 0 {
		t.Fatal("local edits changed telemetry/history or requested printer work")
	}
	m = key(m, "esc")
	if m.decisions != nil || m.focus != 1 || m.details.YOffset != offset {
		t.Fatal("close changed dashboard focus/scroll")
	}
}

// TestDecisionRearmsOnlyRestoredUnreviewedOccurrence checks session suppression and deferral.
func TestDecisionRearmsOnlyRestoredUnreviewedOccurrence(t *testing.T) {
	for _, acknowledged := range []bool{false, true} {
		t.Run(map[bool]string{false: "unreviewed", true: "acknowledged"}[acknowledged], func(t *testing.T) {
			m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
			m = key(m, "esc") // Session-only suppression is now populated.
			item := m.Farm.Attention()[0]
			m.Farm.Acknowledge(item.Key, acknowledged)
			m.Farm.Dismiss(item.Key)
			m = key(m, "n")
			m.decisions.key = item.Key
			m.syncDecisions()
			m = key(m, "r")
			m = attentionTick(m)
			if m.attention != nil {
				t.Fatal("alert interrupted the decisions view")
			}
			if acknowledged {
				m = key(m, "esc")
				m = attentionTick(m)
				if m.attention != nil {
					t.Fatal("acknowledged restoration alerted")
				}
				m = key(m, "n")
				m.decisions.key = item.Key
				m = key(m, "u")
			}
			m = key(m, "esc")
			m = attentionTick(m)
			if m.attention == nil || len(m.attention.groups[0].items) != 1 || m.attention.groups[0].items[0].Key != item.Key {
				t.Fatal("reversal failed to re-arm exactly the selected occurrence")
			}
		})
	}
}

// TestDecisionResolutionAndPendingStatus avoids manufacturing faults from saved metadata.
func TestDecisionResolutionAndPendingStatus(t *testing.T) {
	m := demoModel()
	const keyName = "H/fault/0700_2000_0002_0001"
	m.Farm.RestoreReviews(map[string]farm.Review{keyName: {Code: "0700_2000_0002_0001", Level: "warning", Since: m.now, Acknowledged: true, Dismissed: true}})
	m = key(m, "n")
	if !strings.Contains(decisionText(m), "Awaiting telemetry") || len(m.Farm.Active) != 0 {
		t.Fatal("pending decision looked active")
	}
	m = key(m, "r")
	m = key(m, "u")
	if len(m.decisionItems()) != 0 || len(m.Farm.Active) != 0 {
		t.Fatal("pending reversal created an active condition")
	}
	m = attentionReport(t, m, "H", runoutReport)
	m.Farm.Dismiss(keyName)
	m.syncDecisions()
	if m.decisions.key != keyName || !strings.Contains(decisionText(m), "filament runout") {
		t.Fatal("live decision was not shown")
	}
	m = attentionReport(t, m, "H", `{"print":{"hms":[],"print_error":0,"gcode_state":"RUNNING"}}`)
	m = key(m, "r")
	if len(m.decisionItems()) != 0 || len(m.Farm.Active) != 0 || !strings.Contains(decisionText(m), "No local notice decisions") {
		t.Fatal("resolution left or restored a synthetic fault")
	}
}

// TestDecisionResetAndRestart verifies asynchronous account-isolated persistence and confirmation.
func TestDecisionResetAndRestart(t *testing.T) {
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "account-a.json")}
	other := noticestore.Store{Path: filepath.Join(t.TempDir(), "account-b.json")}
	m := noticeModel(t)
	m.Demo = false
	m = m.WithNoticePersistence(store.Save, nil)
	item, _ := m.selectedNotice()
	m = persistenceKey(m, "a")
	m = persistenceKey(m, "c")
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Save(saved); err != nil {
		t.Fatal(err)
	}
	state := farm.New(10)
	state.RestoreReviews(saved)
	next := New(state, nil, nil, false).WithNoticePersistence(store.Save, nil)
	next = persistenceKey(next, "n")
	if len(next.decisionItems()) != 1 || !strings.Contains(decisionText(next), "Awaiting telemetry") {
		t.Fatal("restart did not expose pending saved decisions")
	}
	next = persistenceKey(next, "r")
	saved, err = store.Load()
	if err != nil || saved[item.ActiveKey].Dismissed || !saved[item.ActiveKey].Acknowledged {
		t.Fatal("individual restoration was not saved after command completion")
	}
	// A second restart applies the edited flags to actual matching telemetry.
	replayedState := farm.New(10)
	replayedState.RestoreReviews(saved)
	replayed := New(replayedState, nil, nil, false)
	payload, err := json.Marshal(map[string]any{"print": m.Farm.Printers["H"].Raw})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []monitor.Event{
		{Kind: monitor.EventConnection, Connected: true, At: m.now},
		{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "H", Name: "H2D", Online: true}}, At: m.now},
		{Kind: monitor.EventReport, ID: "H", Payload: payload, At: m.now},
	} {
		updated, _ := replayed.Update(event)
		replayed = updated.(Model)
	}
	if c := replayed.Farm.Active[item.ActiveKey]; c.Dismissed || !c.Acknowledged {
		t.Fatal("restored visibility/acknowledgement did not survive telemetry after restart")
	}
	next = persistenceKey(next, "R")
	next = persistenceKey(next, "n")
	if next.decisions == nil || next.decisions.confirmReset || len(next.Farm.Reviews()) != 1 {
		t.Fatal("cancel changed decisions or left the manager")
	}
	next = persistenceKey(next, "R")
	next = persistenceKey(next, "y")
	saved, err = store.Load()
	if err != nil || len(saved) != 0 || len(next.Farm.Active) != 0 {
		t.Fatal("reset failed to persist empty decisions without creating telemetry")
	}
	untouched, err := other.Load()
	if err != nil || len(untouched) != 1 {
		t.Fatal("reset changed another account")
	}
	restarted := farm.New(10)
	restarted.RestoreReviews(saved)
	if len(restarted.Reviews()) != 0 {
		t.Fatal("reset decisions returned after restart")
	}
}

// TestDecisionErrorsAndTerminalSizes checks failures remain visible in both manager screens.
func TestDecisionErrorsAndTerminalSizes(t *testing.T) {
	m := noticeModel(t)
	item, _ := m.selectedNotice()
	m.Farm.Acknowledge(item.ActiveKey, true)
	m.Farm.Dismiss(item.ActiveKey)
	m.Demo = false
	m = m.WithNoticePersistence(func(map[string]farm.Review) error { return errors.New("disk full") }, nil)
	m = persistenceKey(m, "n")
	m.decisions.key = item.ActiveKey
	m = persistenceKey(m, "r")
	if !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("restoration save failure was hidden")
	}
	m = persistenceKey(m, "R")
	if !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("reset confirmation hid storage failure")
	}
	m = persistenceKey(m, "y")
	if !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("reset save failure was hidden")
	}
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}, {0, 0}} {
		for _, confirmation := range []bool{false, true} {
			m.decisions.confirmReset = confirmation
			next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m = next.(Model)
			lines := strings.Split(m.View(), "\n")
			if m.View() != "" && len(lines) > size[1] {
				t.Fatal("height overflow")
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("width overflow")
				}
			}
		}
	}
	// A load failure continues to protect the unreadable file from all writes.
	writes := 0
	m = noticeModel(t)
	m.Demo = false
	m = m.WithNoticePersistence(func(map[string]farm.Review) error { writes++; return nil }, errors.New("unreadable file"))
	m.Farm.Dismiss(item.ActiveKey)
	m = persistenceKey(m, "n")
	m = persistenceKey(m, "R")
	m = persistenceKey(m, "y")
	if writes != 0 || !strings.Contains(ansi.Strip(m.View()), "Saved notices unavailable") {
		t.Fatal("reset overwrote unreadable saved decisions")
	}
}

// TestResetRearmsAlertsAfterClosing checks bulk reset clears session suppression too.
func TestResetRearmsAlertsAfterClosing(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "c")
	m = key(m, "n")
	m = key(m, "R")
	m = key(m, "y")
	m = attentionTick(m)
	if m.attention != nil || len(m.attentionSeen) != 0 {
		t.Fatal("reset alerted before closing or kept session suppression")
	}
	m = key(m, "esc")
	m = attentionTick(m)
	if m.attention == nil || len(m.attention.groups[0].items) != 3 {
		t.Fatal("reset did not re-arm all actionable occurrences")
	}
	m = key(m, "esc")
	m = attentionReport(t, m, "H", `{"print":{"hms":[],"print_error":0,"gcode_state":"RUNNING"}}`)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if m.attention == nil {
		t.Fatal("recurrence was suppressed")
	}
}

// TestDecisionScrollingAndSmallConfirmation keeps long descriptions readable at minimum size.
func TestDecisionScrollingAndSmallConfirmation(t *testing.T) {
	m := noticeModel(t)
	item, _ := m.selectedNotice()
	c := m.Farm.Active[item.ActiveKey]
	c.Text = strings.Repeat("Detailed printer fault description with context. ", 30)
	m.Farm.Active[item.ActiveKey] = c
	m.Farm.Dismiss(item.ActiveKey)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = key(next.(Model), "n")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = next.(Model)
	if m.decisions.view.YOffset == 0 {
		t.Fatal("long descriptions cannot be scrolled")
	}
	m = key(m, "R")
	view := ansi.Strip(m.View())
	for _, text := range []string{"RESET NOTICE DECISIONS?", "account and region", "y: reset decisions", "n / Esc cancel"} {
		if !strings.Contains(view, text) {
			t.Fatalf("minimum-size confirmation hides %q", text)
		}
	}
	m = key(m, "esc")
	if m.decisions.confirmReset || m.decisions.view.YOffset == 0 {
		t.Fatal("cancel lost the description's scroll position")
	}
}

// TestPendingFilamentDecisionsRemainDistinct checks saved slot identity before reports arrive.
func TestPendingFilamentDecisionsRemainDistinct(t *testing.T) {
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "notices.json")}
	m := demoModel()
	r := farm.Review{Dismissed: true, Level: "warning", Since: m.now}
	if err := store.Save(map[string]farm.Review{"H/low/0/0": r, "H/low/1/3": r, "H/low/128/0": r}); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	m.Farm.RestoreReviews(saved)
	m.Demo = false
	m = m.WithNoticePersistence(store.Save, nil)
	m = persistenceKey(m, "n")
	for _, want := range []struct{ key, description string }{
		{"H/low/0/0", "AMS 1 slot 1"},
		{"H/low/1/3", "AMS 2 slot 4"},
		{"H/low/128/0", "AMS HT 1 slot 1"},
	} {
		if m.decisions.key != want.key {
			t.Fatalf("navigation selected %q, want %q", m.decisions.key, want.key)
		}
		text := decisionText(m)
		if !strings.Contains(text, want.description) || !strings.Contains(text, "saved low-filament decision") || !strings.Contains(text, "Awaiting telemetry") || len(m.Farm.Active) != 0 {
			t.Fatalf("saved decision lacks distinct identity or claims live status: %s", text)
		}
		m = persistenceKey(m, "j")
	}
	m = persistenceKey(m, "k") // Select AMS 2 slot 4 rather than the neighboring AMS HT decision.
	m = persistenceKey(m, "r")
	saved, err = store.Load()
	if err != nil || len(saved) != 2 || !saved["H/low/0/0"].Dismissed || !saved["H/low/128/0"].Dismissed {
		t.Fatalf("restoration changed neighboring decisions: %v, %v", saved, err)
	}
	if _, exists := saved["H/low/1/3"]; exists || len(m.Farm.Active) != 0 {
		t.Fatal("selected pending dismissal was retained or created an active fault")
	}
}

// TestPendingDecisionFallbackRetainsIdentity distinguishes unsupported or malformed saved keys.
func TestPendingDecisionFallbackRetainsIdentity(t *testing.T) {
	m := demoModel()
	m.Farm.RestoreReviews(map[string]farm.Review{
		"H/low/bad/0": {Dismissed: true, Since: m.now},
		"H/low/0/-1":  {Dismissed: true, Since: m.now},
		"H/unknown/1": {Dismissed: true, Since: m.now},
	})
	m = key(m, "n")
	for _, family := range []string{"low/0/-1", "low/bad/0", "unknown/1"} {
		if text := decisionText(m); !strings.Contains(text, family) || !strings.Contains(text, "Awaiting telemetry") {
			t.Fatalf("fallback dropped saved identity %q: %s", family, text)
		}
		m = key(m, "j")
	}
}

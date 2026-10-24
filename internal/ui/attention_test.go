package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-labs-tui/internal/farm"
	"github.com/cliffom/bambu-labs-tui/internal/monitor"
)

const runoutReport = `{"print":{"gcode_state":"PAUSE","stg_cur":6,"print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}}`

// attentionReport sends telemetry through the real UI update and persistence path.
func attentionReport(t *testing.T, m Model, id, payload string) Model {
	t.Helper()
	next, _ := m.Update(monitor.Event{Kind: "report", ID: id, Payload: []byte(payload), At: m.now})
	return next.(Model)
}

// attentionTick advances the batching window without sleeping.
func attentionTick(m Model) Model {
	next, _ := m.Update(tickMsg(m.now.Add(3 * time.Second)))
	return next.(Model)
}

// attentionText reads all modal content without depending on scroll position.
func attentionText(m Model) string {
	view := m.attention.view
	view.Height = 200
	view.GotoTop()
	return ansi.Strip(view.View())
}

// TestAttentionBatchCloseAndRecurrence checks modal presentation and session suppression.
func TestAttentionBatchCloseAndRecurrence(t *testing.T) {
	m := demoModel()
	m.focus, m.selection = 1, 1
	m = attentionReport(t, m, "H", runoutReport)
	if m.attention != nil {
		t.Fatal("modal opened before related reports could be batched")
	}
	m = attentionTick(m)
	if m.attention == nil || len(m.attention.groups) != 1 || len(m.attention.groups[0].items) != 3 {
		t.Fatal("state and fault reports were not grouped")
	}
	view := ansi.Strip(m.View())
	for _, text := range []string{"ACTION NEEDED", "H2D", "Paused", "filament runout", "0700_2000_0002_0001", "wiki.bambulab.com", "does not resolve"} {
		if !strings.Contains(view+attentionText(m), text) {
			t.Fatalf("modal lacks %q", text)
		}
	}
	m = key(m, "esc")
	if m.attention != nil || m.focus != 1 || m.selection != 1 {
		t.Fatal("close changed focus or failed to close")
	}
	for _, c := range m.Farm.Active {
		if c.Acknowledged || c.Dismissed {
			t.Fatal("close implicitly reviewed notices")
		}
	}
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if m.attention != nil {
		t.Fatal("routine telemetry repeated the prompt")
	}
	m = key(m, "!")
	if m.attention == nil {
		t.Fatal("explicit reopen failed")
	}
	m = key(m, "esc")
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","print_error":0,"hms":[]}}`)
	m = attentionTick(attentionReport(t, m, "H", runoutReport))
	if m.attention == nil {
		t.Fatal("recurrence was suppressed")
	}
}

// TestAttentionQueueAndLocalActions keeps printers and notice identities separate.
func TestAttentionQueueAndLocalActions(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = attentionReport(t, m, "X", runoutReport)
	if len(m.attention.groups) != 2 || m.attention.groups[m.attention.index].id != "H" {
		t.Fatal("new printer replaced the incident being read")
	}
	m = key(m, "tab")
	if m.attention.groups[m.attention.index].id != "X" {
		t.Fatal("Tab did not navigate queued printers")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.attention.groups[m.attention.index].id != "H" {
		t.Fatal("Shift+Tab did not restore the first printer")
	}
	writes := 0
	m = m.WithNoticePersistence(func(reviews map[string]farm.Review) error {
		writes++
		if len(reviews) == 0 {
			t.Fatal("review snapshot empty")
		}
		return nil
	}, nil)
	m = key(m, "a")
	if writes != 1 || len(m.attention.groups) != 1 || m.attention.groups[0].id != "X" {
		t.Fatal("ack did not advance to next printer or save immediately")
	}
	for key, c := range m.Farm.Active {
		if strings.HasPrefix(key, "H/") && !c.Acknowledged || strings.HasPrefix(key, "X/") && c.Acknowledged {
			t.Fatal("acknowledgement crossed printer groups")
		}
	}
	m = key(m, "c")
	if m.attention != nil || writes != 2 || len(m.Farm.Printers["X"].Faults) != 2 {
		t.Fatal("dismissal did not save/close or edited telemetry")
	}
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.attention != nil {
		t.Fatal("reviewed incidents returned")
	}
	m = attentionTick(attentionReport(t, m, "X", `{"print":{"print_error":117473286}}`))
	if m.attention == nil || len(m.attention.groups[0].items) != 1 {
		t.Fatal("different fault was suppressed by dismissal")
	}
}

// TestAttentionStartupReviews suppresses acknowledged and dismissed restored incidents.
func TestAttentionStartupReviews(t *testing.T) {
	for _, action := range []string{"a", "c"} {
		t.Run(action, func(t *testing.T) {
			m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
			m = key(m, action)
			s := farm.New(10)
			s.RestoreReviews(m.Farm.Reviews())
			s.Cloud(true, "", m.now)
			s.Discover([]farm.Device{{ID: "H", Name: "H2D", Online: true}}, m.now)
			fresh := New(s, make(chan monitor.Event), make(chan struct{}, 1), false)
			fresh = attentionTick(attentionReport(t, fresh, "H", `{"print":{"gcode_state":"PAUSE"}}`))
			fresh = attentionTick(attentionReport(t, fresh, "H", runoutReport))
			if fresh.attention != nil {
				t.Fatal("a saved review caused another startup modal")
			}
		})
	}
}

// TestAttentionDeferralIncrementalAndResolution keeps dialogs and telemetry usable.
func TestAttentionDeferralIncrementalAndResolution(t *testing.T) {
	for _, dialog := range []string{"help", "inspection", "confirmation"} {
		t.Run(dialog, func(t *testing.T) {
			m := demoModel()
			switch dialog {
			case "help":
				m.help = true
			case "inspection":
				m.inspected = &noticeItem{Text: "Reading another notice"}
			case "confirmation":
				m.confirmClear = true
			}
			m = attentionTick(attentionReport(t, m, "H", runoutReport))
			if m.attention != nil {
				t.Fatal("attention replaced an existing dialog")
			}
			m = key(m, "esc")
			if m.attention == nil {
				t.Fatal("deferred alert never opened")
			}
		})
	}
	m := attentionReport(t, demoModel(), "H", `{"print":{"gcode_state":"PAUSE"}}`)
	m = attentionTick(m)
	if m.attention != nil {
		t.Fatal("pause arrived before fault snapshots")
	}
	m = attentionReport(t, m, "H", `{"print":{"print_error":117473297}}`)
	m = attentionTick(m)
	m = attentionReport(t, m, "H", runoutReport)
	if len(m.attention.groups) != 1 || len(m.attention.groups[0].items) != 3 {
		t.Fatal("incremental HMS report created another dialog or was lost")
	}
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","hms":[],"print_error":0}}`)
	if m.attention == nil || !strings.Contains(attentionText(m), "Resolved / no longer active") {
		t.Fatal("resolution replaced the modal or left stale action guidance")
	}
	m = key(m, "a")
	if len(m.Farm.Reviews()) != 0 {
		t.Fatal("resolved notices were acknowledged")
	}
}

// TestAttentionNavigationBoundsAndQuit exercises supported terminal dimensions.
func TestAttentionNavigationBoundsAndQuit(t *testing.T) {
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}} {
		m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		for _, line := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("modal exceeded terminal width")
			}
		}
		if len(strings.Split(m.View(), "\n")) > size[1] {
			t.Fatal("modal exceeded terminal height")
		}
		if size[0] >= 60 && size[1] >= 18 && !strings.Contains(ansi.Strip(m.View()), "Esc close") {
			t.Fatal("modal actions were clipped at a supported terminal size")
		}
		_, quit := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if quit == nil {
			t.Fatal("quit unavailable inside modal")
		}
	}
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	next, open := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = next.(Model)
	if open == nil || m.attention == nil {
		t.Fatal("troubleshooting action closed the modal or failed to return a browser command")
	}
	m = key(m, "enter")
	if m.attention != nil || m.inspected == nil || m.current().ID != "H" {
		t.Fatal("notice navigation failed")
	}
	m = attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "p")
	if m.attention != nil || m.focus != 1 || m.current().ID != "H" {
		t.Fatal("printer navigation failed")
	}
}

// TestAttentionResizeAndPauseExplanation handles deferred sizing and refined telemetry.
func TestAttentionResizeAndPauseExplanation(t *testing.T) {
	m := demoModel()
	m.width, m.height = 40, 10
	m = attentionTick(attentionReport(t, m, "H", `{"print":{"gcode_state":"PAUSE","hms":[],"print_error":0}}`))
	if m.attention != nil {
		t.Fatal("modal opened below the supported size")
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = next.(Model)
	if m.attention == nil {
		t.Fatal("resize did not open the deferred alert")
	}
	m = attentionReport(t, m, "H", `{"print":{"print_error":50364435}}`)
	if !strings.Contains(attentionText(m), "No longer requires an alert") {
		t.Fatal("explicit user-pause explanation did not update the open alert")
	}
}

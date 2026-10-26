package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

const runoutReport = `{"print":{"gcode_state":"PAUSE","stg_cur":6,"print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}}`

// attentionReport sends telemetry through the real UI update and persistence path.
func attentionReport(t *testing.T, m Model, id, payload string) Model {
	t.Helper()
	return persistenceUpdate(m, monitor.Event{Kind: monitor.EventReport, ID: id, Payload: []byte(payload), At: m.now})
}

// attentionTick advances the batching window without sleeping.
func attentionTick(m Model) Model {
	return persistenceUpdate(m, tickMsg(m.now.Add(3*time.Second)))
}

// attentionText reads all modal content without depending on scroll position.
func attentionText(m Model) string {
	view := m.attention.view
	view.Height = max(1, view.TotalLineCount())
	view.GotoTop()
	return strings.Join(strings.Fields(ansi.Strip(view.View())), " ")
}

// TestAttentionBatchCloseAndRecurrence checks modal presentation and session suppression.
func TestAttentionBatchCloseAndRecurrence(t *testing.T) {
	m := demoModel()
	m.focus, m.selection = panelDetails, 1
	m = attentionReport(t, m, "H", runoutReport)
	if m.attention != nil {
		t.Fatal("modal opened before related reports could be batched")
	}
	m = attentionTick(m)
	if m.attention == nil || len(m.attention.groups) != 1 || len(m.attention.groups[0].items) != 3 {
		t.Fatal("state and fault reports were not grouped")
	}
	view := ansi.Strip(m.View())
	for _, text := range []string{"ACTION NEEDED", "H2D", "Paused", "filament runout", "0700_2000_0002_0001", "HMS troubleshooting", "does not resolve"} {
		if !strings.Contains(view+attentionText(m), text) {
			t.Fatalf("modal lacks %q", text)
		}
	}
	m = key(m, "esc")
	if m.attention != nil || m.focus != panelDetails || m.selection != 1 {
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
	m = persistenceKey(m, "tab")
	if m.attention.groups[m.attention.index].id != "X" {
		t.Fatal("Tab did not navigate queued printers")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.attention.groups[m.attention.index].id != "H" {
		t.Fatal("Shift+Tab did not restore the first printer")
	}
	writes := 0
	m.Demo = false
	m = m.WithNoticePersistence(func(reviews map[string]farm.Review) error {
		writes++
		if len(reviews) == 0 {
			t.Fatal("review snapshot empty")
		}
		return nil
	}, nil)
	m = persistenceKey(m, "a")
	if writes != 1 || len(m.attention.groups) != 1 || m.attention.groups[0].id != "X" {
		t.Fatal("ack did not advance to next printer or save after command completion")
	}
	for key, c := range m.Farm.Active {
		if strings.HasPrefix(key, "H/") && !c.Acknowledged || strings.HasPrefix(key, "X/") && c.Acknowledged {
			t.Fatal("acknowledgement crossed printer groups")
		}
	}
	m = persistenceKey(m, "c")
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
				m.openDialog(dialogHelp)
			case "inspection":
				m.openDialog(dialogInspector)
				m.inspected = &noticeItem{Text: "Reading another notice"}
			case "confirmation":
				m.openDialog(dialogClearHistory)
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

// TestAttentionNavigationBounds exercises supported terminal dimensions.
func TestAttentionNavigationBounds(t *testing.T) {
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
		for _, quitKey := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
			next, cmd := m.Update(quitKey)
			m = next.(Model)
			if cmd != nil || m.attention == nil {
				t.Fatal("quit key escaped the modal")
			}
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
	if m.attention != nil || m.focus != panelDetails || m.current().ID != "H" {
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

func TestObservationTime(t *testing.T) {
	since := time.Date(2026, 10, 26, 21, 32, 5, 0, time.UTC)
	for _, tc := range []struct {
		elapsed time.Duration
		age     string
	}{
		{0, "just now"},
		{59 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{2 * time.Minute, "2 minutes ago"},
		{time.Hour - time.Second, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{24*time.Hour - time.Second, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{48 * time.Hour, "2 days ago"},
		{-time.Second, "clock skew; observation is in the future"},
	} {
		t.Run(tc.age, func(t *testing.T) {
			got := observationTime(since, since.Add(tc.elapsed))
			want := "First observed: " + since.Local().Format("2006-01-02 15:04:05 MST (Z07:00)") + " (" + tc.age + ")"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
	if got := observationTime(time.Time{}, since); got != "First observed: unavailable" {
		t.Fatal(got)
	}
}

func TestAttentionTimeUpdatesPreservePositionAndLifetime(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = attentionReport(t, m, "X", runoutReport)
	m.width, m.height = 60, 18
	m.syncAttentionView()
	items := append([]farm.Attention(nil), m.attention.groups[0].items...)
	m.attention.view.SetYOffset(5)
	focus, selection := m.focus, m.selection
	m = persistenceUpdate(m, tickMsg(m.now.Add(time.Minute)))
	if m.attention.index != 0 || m.attention.view.YOffset != 5 || m.focus != focus || m.selection != selection {
		t.Fatal("age update moved selection, focus, or scroll")
	}
	if !strings.Contains(attentionText(m), "1 minute ago") {
		t.Fatalf("open modal age did not advance: now=%s since=%s text=%s", m.now, items[0].Since, attentionText(m))
	}
	m = attentionReport(t, m, "H", runoutReport)
	if m.attention.view.YOffset != 5 {
		t.Fatal("telemetry moved scroll")
	}
	m = persistenceUpdate(m, tickMsg(m.now.Add(3*time.Minute)))
	if !strings.Contains(attentionText(m), "Stale") {
		t.Fatal("elapsed time lost stale connection label")
	}
	m = persistenceUpdate(m, monitor.Event{Kind: monitor.EventConnection, Connected: false, At: m.now})
	if !strings.Contains(attentionText(m), "Cloud disconnected") {
		t.Fatal("connection loss was not labelled")
	}
	m = persistenceUpdate(m, monitor.Event{Kind: monitor.EventConnection, Connected: true, At: m.now})
	m = key(m, "esc")
	m = key(m, "!")
	for i, item := range m.attention.groups[0].items {
		if !item.Since.Equal(items[i].Since) {
			t.Fatal("report/reconnect/reopen reset occurrence time")
		}
	}
	m = persistenceUpdate(m, tickMsg(m.now.Add(time.Minute)))
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","print_error":0,"hms":[]}}`)
	m = attentionReport(t, m, "H", runoutReport)
	group := m.attention.groups[0]
	if len(group.items) != 2*len(items) || !strings.Contains(attentionText(m), "Replaced by a new occurrence") {
		t.Fatal("recurrence lost previous occurrence or replacement label")
	}
	for _, item := range group.items[len(items):] {
		if !item.Since.After(items[0].Since) {
			t.Fatal("recurrence retained old observation time")
		}
	}
}

func TestAttentionHierarchyAndLongContent(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
		m.width, m.height = size[0], size[1]
		group := &m.attention.groups[0]
		for i, item := range group.items {
			c := m.Farm.Active[item.Key]
			c.Text = strings.Repeat("Long reported fault description ", 12) + " [" + item.Code + "]"
			m.Farm.Active[item.Key] = c
			group.items[i].Guidance = strings.Repeat("Inspect the filament path. ", 8)
		}
		m.syncAttentionView()
		text := attentionText(m)
		for _, item := range group.items {
			if item.Code != "" && strings.Count(text, item.Code) != 1 {
				t.Fatalf("code %s was duplicated or missing", item.Code)
			}
		}
		if strings.Count(text, "First observed:") != len(group.items) || !strings.Contains(text, "Next:") {
			t.Fatal("independent times or intervention missing")
		}
		view := strings.Split(ansi.Strip(m.View()), "\n")
		for _, label := range []string{"a acknowledge", "c hide", "s snooze", "Esc close"} {
			if !strings.Contains(view[len(view)-1], label) {
				t.Fatalf("footer lost %q at %v", label, size)
			}
		}
		m.attention.view.GotoBottom()
		if !strings.Contains(ansi.Strip(m.View()), group.name) {
			t.Fatal("scroll hid the active printer")
		}
		if !strings.Contains(ansi.Strip(m.View()), "printer controls") {
			t.Fatal("local action explanation is inaccessible")
		}
	}
}

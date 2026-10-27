package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

func TestDialogOwnerRoutesInputViewAndAttentionDeferral(t *testing.T) {
	for _, tc := range []struct {
		name, input, title string
		owner              dialogKind
	}{
		{"help", "?", "KEYBOARD", dialogHelp},
		{"about", "A", "ABOUT", dialogAbout},
		{"server status", "W", "SERVER STATUS", dialogServerStatus},
		{"inspector", "enter", "NOTICE", dialogInspector},
		{"history", "C", "CLEAR RECENT EVENTS?", dialogClearHistory},
		{"clear notices", "X", "CLEAR ALL NOTICES?", dialogClearNotices},
		{"filament", "m", "AMS / FILAMENT", dialogFilament},
		{"decisions", "n", "NOTICE DECISIONS", dialogDecisions},
		{"reset", "n", "RESET NOTICE DECISIONS?", dialogDecisions},
		{"config", ",", "CONFIG / GENERAL", dialogConfig},
		{"alerts", "s", "PRINTER ALERTS / SNOOZE", dialogAlerts},
		{"notification activity", "N", "NOTIFICATION ACTIVITY", dialogNotificationActivity},
		{"diagnostics", "D", "LOCAL DIAGNOSTIC EXPORT", dialogExport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := noticeModel(t)
			if tc.name == "reset" {
				m = key(m, "a")
			}
			m = key(m, tc.input)
			if tc.name == "reset" {
				m = key(m, "R")
			}
			if m.dialog.active != tc.owner || m.dialog.returnTo != dialogNone || !m.hasOpenDialog() {
				t.Fatalf("wrong focus owner: %+v", m.dialog)
			}
			if !strings.Contains(ansi.Strip(m.View()), tc.title) {
				t.Fatalf("owner did not render %q", tc.title)
			}
			// A global navigation key must not reach the dashboard through the dialog.
			selection, filter := m.selection, m.printerFilter
			m = key(m, "F")
			if m.dialog.active != tc.owner || m.selection != selection || m.printerFilter != filter {
				t.Fatal("input escaped the active owner")
			}
			m = attentionTick(attentionReport(t, m, "X", runoutReport))
			if m.dialog.active != tc.owner || m.attention != nil {
				t.Fatal("attention took focus from owner")
			}
			for _, size := range []tea.WindowSizeMsg{{Width: 60, Height: 18}, {Width: 40, Height: 10}, {Width: 120, Height: 35}} {
				next, _ := m.Update(size)
				m = next.(Model)
				if m.dialog.active != tc.owner {
					t.Fatal("resize changed focus owner")
				}
				for _, line := range strings.Split(ansi.Strip(m.View()), "\n") {
					if ansi.StringWidth(line) > size.Width {
						t.Fatal("dialog overflowed resized width")
					}
				}
			}
			m = key(m, "esc")
			if tc.name == "reset" {
				if m.dialog.active != dialogDecisions || m.decisions.confirmReset {
					t.Fatal("reset cancel lost its parent")
				}
				m = key(m, "esc")
			}
			if m.dialog.active != dialogAttention || m.attention == nil {
				t.Fatal("attention did not resume after owner closed")
			}
		})
	}
}

func TestNestedAlertReturnAndSuspendedAttentionSync(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		t.Run(map[bool]string{false: "one printer", true: "two printers"}[multiple], func(t *testing.T) {
			m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
			if multiple {
				m = attentionReport(t, m, "X", runoutReport)
			}
			m = key(m, "s")
			if m.dialog != (dialogState{active: dialogAlerts, returnTo: dialogAttention}) {
				t.Fatal("nested snooze did not suspend attention")
			}
			next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = next.(Model)
			if m.attention.view.Width != 80 || m.alertDialog.view.Width != 78 {
				t.Fatal("resize did not synchronize owner and suspended parent")
			}
			// Resolution while suspended must be reflected when returning to the prompt.
			m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","hms":[],"print_error":0}}`)
			m = key(m, "esc")
			if m.dialog.active != dialogAttention || m.dialog.returnTo != dialogNone || !strings.Contains(attentionText(m), "Resolved / no longer active") {
				t.Fatal("return lost parent state or live synchronization")
			}
			m = key(m, "s")
			m = key(m, "1")
			if multiple {
				if m.dialog.active != dialogAttention || m.attention.groups[0].id != "X" {
					t.Fatal("snooze lost the other printer's parent prompt")
				}
			} else if m.hasOpenDialog() || m.attention != nil || m.alertDialog != nil || m.dialog.returnTo != dialogNone {
				t.Fatal("empty suspended prompt was restored")
			}
		})
	}
}

func TestReplacingNestedOwnerClearsPayloads(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "s")
	m.openFilament()
	if m.dialog.active != dialogFilament || m.dialog.returnTo != dialogNone || m.alertDialog != nil || m.attention != nil {
		t.Fatal("replacement left orphaned dialog payloads")
	}
	m.closeDialog()
	if m.hasOpenDialog() || m.filament != nil {
		t.Fatal("closing replacement did not release focus")
	}
}

func TestLateDiagnosticResultDoesNotChangeDialogOwner(t *testing.T) {
	m := key(demoModel(), "D")
	destination := m.exportDialog.destination
	m = key(m, "esc")
	m = key(m, "?")
	next, _ := m.Update(exportResultMsg{destination: destination})
	m = next.(Model)
	if m.dialog.active != dialogHelp || !strings.Contains(m.View(), "KEYBOARD") || m.exportDialog != nil || !strings.Contains(m.toast, destination) {
		t.Fatal("late diagnostic result changed owner or reopened export")
	}
}

func TestRemovedPrinterDoesNotChangeNestedOwner(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "s")
	next, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{}, At: m.now})
	m = next.(Model)
	if m.dialog.active != dialogAlerts || !strings.Contains(m.alertDialog.view.View(), "Printer no longer") {
		t.Fatal("removal lost alert owner")
	}
	m = key(m, "esc")
	if m.dialog.active != dialogAttention || m.attention == nil || !strings.Contains(attentionText(m), "Resolved / no longer active") {
		t.Fatal("removed printer lost suspended parent")
	}
}

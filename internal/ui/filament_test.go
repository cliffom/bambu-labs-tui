package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

// filamentModel provides reordered AMS/AMS HT units, empty/unknown/zero slots, and external spools.
func filamentModel() Model {
	m := demoModel()
	remaining, empty, occupied := 0, false, true
	humidity, temperature := 31, 28
	p := m.current()
	p.AMS = []farm.AMS{
		{ID: 128, Slots: []farm.Slot{{ID: 0, Material: "PETG"}}},
		{ID: 1, Slots: []farm.Slot{{ID: 0, Material: "PLA-CF"}}},
		{ID: 0, Humidity: &humidity, Temperature: &temperature, Slots: []farm.Slot{
			{ID: 3, Material: "TPU", Nozzles: []int{1}},
			{ID: 1, Present: &empty},
			{ID: 0, Present: &occupied, Material: "PLA", Color: "918669FF", Remaining: &remaining, Nozzles: []int{0}},
			{ID: 2},
		}},
	}
	p.External = []farm.Slot{{ID: 254, Material: "PETG"}, {ID: 255, Material: "PLA"}}
	m.syncPanels()
	return m
}

// filamentText reads all browser content without affecting its scroll position.
func filamentText(m Model) string {
	view := m.filament.view
	view.Height = 300
	view.GotoTop()
	return ansi.Strip(view.View())
}

// TestFilamentSummaryAndNavigation keeps the dashboard compact and every unit reachable.
func TestFilamentSummaryAndNavigation(t *testing.T) {
	m := filamentModel()
	detail := ansi.Strip(m.detailText(100))
	if strings.Contains(detail, "Slot 1:") || !strings.Contains(detail, "2 AMS · 1 AMS HT · 2 external") || !strings.Contains(detail, "1 low (≤10%) · 4 unknown") {
		t.Fatalf("dashboard inventory is expanded or counts are incorrect: %s", detail)
	}
	m.focus = 1
	m.details.SetYOffset(4)
	m = key(m, "m")
	if m.filament == nil || m.filament.unitKey != "ams:0" || m.filament.slotID != 0 {
		t.Fatal("initial selection did not use stable numeric identities")
	}
	for _, value := range []string{"Humidity:", "31% RH", "Temperature:", "28°C", "Empty", "Unknown", "0%", "Right", "#918669"} {
		if !strings.Contains(filamentText(m), value) {
			t.Fatalf("missing filament attribute %q", value)
		}
	}
	m = key(m, "j")
	if m.filament.slotID != 1 || !strings.Contains(filamentText(m), "Filament:        Empty") {
		t.Fatal("empty slot was inferred from a zero estimate or lost its state")
	}
	m = key(m, "G")
	if m.filament.slotID != 3 {
		t.Fatal("last-slot navigation failed")
	}
	m = key(m, "tab")
	if m.filament.unitKey != "ams:1" {
		t.Fatal("second AMS unreachable")
	}
	m = key(m, "tab")
	if !strings.Contains(ansi.Strip(m.View()), "AMS HT 1") {
		t.Fatal("AMS HT label lost its wire identity")
	}
	m = key(m, "tab")
	if m.filament.unitKey != "external" || m.filament.slotID != 255 || !strings.Contains(filamentText(m), "External spool quantity is not reported") {
		t.Fatal("external spools are inaccessible or their quantity is misrepresented")
	}
	m = key(m, "j")
	if m.filament.slotID != 254 {
		t.Fatal("second external spool unreachable")
	}
	m = key(m, "esc")
	if m.filament != nil || m.focus != 1 || m.selection != 0 || m.details.YOffset != 4 {
		t.Fatal("closing the browser changed dashboard focus/scroll")
	}
}

// TestFilamentLiveIdentityAndRemoval follows the selected printer, unit, and slot through updates.
func TestFilamentLiveIdentityAndRemoval(t *testing.T) {
	m := key(filamentModel(), "m")
	m = key(m, "j")
	m = key(m, "j")
	p := m.Farm.Printers["H"]
	// Reorder underlying units and slots, then update the selected slot in place.
	p.AMS = []farm.AMS{p.AMS[2], p.AMS[0], p.AMS[1]}
	p.AMS[0].Slots = []farm.Slot{{ID: 2, Material: "Updated material"}, {ID: 0, Material: "PLA"}}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if m.filament.unitKey != "ams:0" || m.filament.slotID != 2 || !strings.Contains(filamentText(m), "Updated material") {
		t.Fatal("telemetry reordered or replaced the selected slot")
	}
	p.AMS[0].Slots = []farm.Slot{{ID: 0, Material: "PLA"}}
	m = attentionTick(m)
	if m.filament.slotID != 0 {
		t.Fatal("removed slot did not fall back safely")
	}
	p.AMS = p.AMS[1:]
	m = attentionTick(m)
	if m.filament.unitKey != "ams:1" {
		t.Fatal("removed unit did not fall back by numeric identity")
	}
	// Switching the dashboard's underlying selection cannot retarget an open browser.
	m.selection = 1
	m = attentionTick(m)
	if m.filament.printerID != "H" || !strings.Contains(filamentText(m), "H2D") {
		t.Fatal("browser jumped to a different printer")
	}
	next, _ = m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "X", Name: "X2D", Online: true}}, At: m.now})
	m = next.(Model)
	if !strings.Contains(filamentText(m), "no longer in the account") {
		t.Fatal("removed printer left stale inventory or caused a crash")
	}
	m = key(m, "m")
	if m.filament != nil {
		t.Fatal("toggle-close failed after printer removal")
	}
}

// TestFilamentDefersAttention processes telemetry without replacing an inventory dialog.
func TestFilamentDefersAttention(t *testing.T) {
	m := key(filamentModel(), "m")
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.filament == nil || m.attention != nil || len(m.Farm.Attention()) == 0 {
		t.Fatal("incoming fault replaced the browser or stopped telemetry processing")
	}
	m = key(m, "esc")
	if m.filament != nil || m.attention == nil || m.attention.groups[0].id != "X" {
		t.Fatal("queued attention was lost after the browser closed")
	}
}

// TestFilamentEmptyAndTerminalBounds keeps controls visible and selected rows reachable.
func TestFilamentEmptyAndTerminalBounds(t *testing.T) {
	m := key(demoModel(), "m")
	if !strings.Contains(filamentText(m), "No AMS or external-spool data") {
		t.Fatal("empty inventory has no explanation")
	}
	m = key(m, "j")
	m = key(m, "tab")
	m = key(m, "enter")
	if m.filament == nil {
		t.Fatal("empty inventory navigation closed the modal")
	}
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}} {
		m = key(filamentModel(), "m")
		m.Farm.Printers["H"].Name = strings.Repeat("Long printer name ", 8)
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = key(next.(Model), "G")
		view := ansi.Strip(m.View())
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("inventory modal exceeded terminal width")
			}
		}
		if len(strings.Split(view, "\n")) > size[1] {
			t.Fatal("inventory modal exceeded terminal height")
		}
		if size[0] >= 60 && size[1] >= 18 {
			if !strings.Contains(view, "m / Esc close") || !strings.Contains(view, "› Slot 4") {
				t.Fatal("close controls or selected slot clipped at supported terminal size")
			}
			m = key(m, "enter")
			if m.filament.detailsOffset >= m.filament.view.Height && m.filament.view.YOffset == 0 {
				t.Fatal("Enter did not navigate to slot details")
			}
		}
	}
}

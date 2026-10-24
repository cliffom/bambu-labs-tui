package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

func demoModel() Model {
	s := farm.New(10)
	s.Cloud(true, "", time.Now())
	s.Discover([]farm.Device{{ID: "H", Name: "H2D", Model: "H2D", Online: true}, {ID: "X", Name: "X2D", Model: "X2D", Online: true}}, time.Now())
	return New(s, make(chan monitor.Event), make(chan struct{}, 1), true)
}
func key(m Model, s string) Model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return next.(Model)
}
func TestNavigationAndHelp(t *testing.T) {
	m := demoModel()
	m = key(m, "j")
	if m.current().ID != "X" {
		t.Fatal("selection did not move")
	}
	m = key(m, "?")
	if !strings.Contains(ansi.Strip(m.View()), "KEYBOARD") {
		t.Fatal("help missing")
	}
	m = key(m, "?")
	for range 5 {
		m = key(m, "j")
	}
	if m.selection != 1 {
		t.Fatal("selection exceeded printers")
	}
	m = key(m, "r")
	if len(m.Refresh) != 1 {
		t.Fatal("refresh not sent")
	}
}
func TestResponsiveBounds(t *testing.T) {
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}, {0, 0}} {
		m := demoModel()
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := next.(Model).View()
		if view == "" {
			continue
		}
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Fatalf("height overflow %v: %d", size, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width overflow %v: %q", size, line)
			}
		}
	}
}
func TestUnknownAndEmptyFilament(t *testing.T) {
	if !strings.Contains(ansi.Strip(slotValue(farm.Slot{})), "Unknown") {
		t.Fatal("missing unknown")
	}
	empty := false
	if !strings.Contains(ansi.Strip(slotValue(farm.Slot{Present: &empty})), "Empty") {
		t.Fatal("missing empty")
	}
	zero := 0
	if !strings.Contains(ansi.Strip(slotValue(farm.Slot{Remaining: &zero})), "0%") {
		t.Fatal("zero estimate lost")
	}
}
func TestUntrustedPrinterTextCannotEscapeTerminal(t *testing.T) {
	m := demoModel()
	m.Farm.Printers["H"].Name = "H2D\x1b]52;c;YXR0YWNr\a\n"
	m.syncPanels()
	view := m.View()
	if strings.Contains(view, "\x1b]52") {
		t.Fatal("terminal control injection")
	}
}
func TestEmptyAccountAndSelectionAfterDiscovery(t *testing.T) {
	m := New(farm.New(10), make(chan monitor.Event), make(chan struct{}, 1), false)
	next, _ := m.Update(monitor.Event{Kind: "connection", Connected: true, At: time.Now()})
	m = next.(Model)
	if !strings.Contains(ansi.Strip(m.View()), "No printers found") {
		t.Fatal("empty state missing")
	}
	m = demoModel()
	m = key(m, "j")
	next, _ = m.Update(monitor.Event{Kind: "devices", Devices: []farm.Device{}, At: time.Now()})
	m = next.(Model)
	if m.current() != nil {
		t.Fatal("stale selection")
	}
}

func noticeModel(t *testing.T) Model {
	t.Helper()
	m := demoModel()
	b, err := os.ReadFile("../farm/testdata/x2d.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Farm.Report("H", b, m.now); err != nil {
		t.Fatal(err)
	}
	m.width, m.height, m.focus = 120, 30, 2
	m.syncPanels()
	return m
}

func TestInspectAndAcknowledgeFault(t *testing.T) {
	m := noticeModel(t)
	item, ok := m.selectedNotice()
	if !ok || item.Code == "" {
		t.Fatal("no selected fault")
	}
	m = key(m, "enter")
	if m.inspected == nil || !strings.Contains(ansi.Strip(m.View()), item.Code) || !strings.Contains(m.View(), "wiki.bambulab.com") {
		t.Fatal("fault details missing")
	}
	m = key(m, "a")
	if !m.Farm.Active[item.ActiveKey].Acknowledged {
		t.Fatal("acknowledge failed")
	}
	updated, _ := m.Update(monitor.Event{Kind: "report", ID: "H", Payload: []byte(`{"print":{"mc_percent":74}}`), At: m.now})
	m = updated.(Model)
	if m.inspected == nil || m.noticeKey != item.Key || !m.inspected.Acknowledged {
		t.Fatal("telemetry moved selection or acknowledgement")
	}
	m = key(m, "a")
	if m.Farm.Active[item.ActiveKey].Acknowledged {
		t.Fatal("toggle acknowledgement failed")
	}
	m = key(m, "c")
	if !m.Farm.Active[item.ActiveKey].Dismissed || m.Farm.Active[item.ActiveKey].Acknowledged || len(m.Farm.Printers["H"].Faults) != 2 {
		t.Fatal("clear must dismiss independently of acknowledgement without editing telemetry")
	}
	if m.inspected != nil || m.focus != 2 {
		t.Fatal("detail return lost focus")
	}
	for _, n := range m.noticeItems() {
		if n.ActiveKey == item.ActiveKey || n.Code == item.Code {
			t.Fatal("dismissed notice remains in list/history")
		}
	}
	if strings.Contains(m.detailText(100), item.Code) {
		t.Fatal("dismissed fault remains in printer detail")
	}
}

func TestClearFromListHidesNoticeAndUpdatesCounts(t *testing.T) {
	m := noticeModel(t)
	item, _ := m.selectedNotice()
	count := m.Farm.NoticeCount()
	m = key(m, "a")
	if m.Farm.NoticeCount() != count || m.Farm.Active[item.ActiveKey].Dismissed {
		t.Fatal("acknowledgement hid notice")
	}
	m = key(m, "c")
	if m.Farm.NoticeCount() != count-1 {
		t.Fatal("clear did not reduce visible count")
	}
	updated, _ := m.Update(monitor.Event{Kind: "report", ID: "H", Payload: []byte(`{"print":{"mc_percent":75}}`), At: m.now})
	m = updated.(Model)
	for _, n := range m.noticeItems() {
		if n.ActiveKey == item.ActiveKey {
			t.Fatal("dismissed notice returned on telemetry update")
		}
	}
}

func TestHistoricalSelectionSurvivesNewEvents(t *testing.T) {
	m := noticeModel(t)
	m = key(m, "G")
	item, _ := m.selectedNotice()
	if item.EventID == 0 {
		t.Fatal("no historical event selected")
	}
	updated, _ := m.Update(monitor.Event{Kind: "connection", At: m.now, Problem: "Disconnected"})
	m = updated.(Model)
	if m.noticeKey != item.Key {
		t.Fatal("new notice moved selection")
	}
	m = key(m, "enter")
	if !strings.Contains(ansi.Strip(m.View()), "Historical event") {
		t.Fatal("not a historical detail")
	}
	m = key(m, "c")
	if m.inspected != nil {
		t.Fatal("deleted detail remained open")
	}
	for _, e := range m.Farm.Events {
		if e.ID == item.EventID {
			t.Fatal("event not deleted")
		}
	}
}

func TestHistoryClearRequiresConfirmation(t *testing.T) {
	m := noticeModel(t)
	count, active := len(m.Farm.Events), len(m.Farm.Active)
	m = key(m, "C")
	if !m.confirmClear || !strings.Contains(m.View(), "CLEAR RECENT EVENTS?") {
		t.Fatal("confirmation missing")
	}
	m = key(m, "n")
	if m.confirmClear || len(m.Farm.Events) != count {
		t.Fatal("cancel cleared history")
	}
	m = key(m, "C")
	m = key(m, "y")
	if len(m.Farm.Events) != 0 || len(m.Farm.Active) != active {
		t.Fatal("history clear modified active conditions")
	}
	m.Farm.Reconcile(m.now)
	if len(m.Farm.Events) != 0 {
		t.Fatal("cleared history reappeared")
	}
}

func TestFaultShortcutAndResolvedDetail(t *testing.T) {
	m := noticeModel(t)
	m.focus = 1
	m = key(m, "f")
	if m.inspected == nil || m.focus != 2 {
		t.Fatal("fault shortcut")
	}
	updated, _ := m.Update(monitor.Event{Kind: "report", ID: "H", Payload: []byte(`{"print":{"hms":[],"print_error":0}}`), At: m.now})
	m = updated.(Model)
	if m.inspected.Active || !strings.Contains(ansi.Strip(m.View()), "Resolved by printer") {
		t.Fatal("open detail did not reflect resolution")
	}
}

func TestNoticeSelectionScrollsIntoViewAndBounds(t *testing.T) {
	m := noticeModel(t)
	m = key(m, "G")
	if m.notices.YOffset == 0 {
		t.Fatal("selection did not scroll")
	}
	span := m.noticeSpans[m.noticeIndex]
	if span[0] < m.notices.YOffset || span[0] >= m.notices.YOffset+m.notices.Height {
		t.Fatal("selection outside viewport")
	}
	for _, size := range [][2]int{{120, 30}, {80, 24}, {60, 18}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		m = key(m, "enter")
		for _, line := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("detail overflow")
			}
		}
		if len(strings.Split(m.View(), "\n")) > size[1] {
			t.Fatal("detail height overflow")
		}
		m = key(m, "esc")
	}
}

func TestHMSLinkRejectsMalformedCodes(t *testing.T) {
	for _, code := range []string{"0700_8006", "bad", "0500_0200_0003_000A/evil", "0500_0200_0003_00ZZ"} {
		if hmsURL(code) != "" {
			t.Fatal("invalid HMS link", code)
		}
	}
	if !strings.HasSuffix(hmsURL("0500_0200_0003_000A"), "0500_0200_0003_000a") {
		t.Fatal("valid HMS link missing")
	}
}

func TestFirstNoticeFocusStartsAtActiveConditions(t *testing.T) {
	m := demoModel()
	m.Farm.Cloud(false, "Connecting", m.now)
	m.syncPanels()
	m.Farm.Cloud(true, "", m.now)
	m.syncPanels()
	b, err := os.ReadFile("../farm/testdata/x2d.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Farm.Report("H", b, m.now); err != nil {
		t.Fatal(err)
	}
	m.syncPanels()
	m = key(m, "tab")
	m = key(m, "tab")
	item, ok := m.selectedNotice()
	if !ok || !item.Active {
		t.Fatal("first focus selected startup history instead of active notices")
	}
}

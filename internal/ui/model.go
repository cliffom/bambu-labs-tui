package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-labs-tui/internal/farm"
	"github.com/cliffom/bambu-labs-tui/internal/monitor"
)

var (
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3E4BB"))
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#929BA8"))
	white    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E4E9F0"))
	warning  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4BA72"))
	danger   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8087"))
	selected = lipgloss.NewStyle().Background(lipgloss.Color("#263B36")).Foreground(lipgloss.Color("#D9F5E4"))
)

type tickMsg time.Time
type endMsg struct{}
type Model struct {
	Farm                            *farm.State
	Events                          <-chan monitor.Event
	Refresh                         chan<- struct{}
	Demo                            bool
	width, height, selection, focus int
	now                             time.Time
	help, ended                     bool
	toast                           string
	toastAt                         time.Time
	details, notices                viewport.Model
	noticeKey                       string
	noticeIndex                     int
	noticeFocused                   bool
	noticeSpans                     [][2]int
	inspected                       *noticeItem
	inspection                      viewport.Model
	confirmClear                    bool
}

func New(state *farm.State, events <-chan monitor.Event, refresh chan<- struct{}, demo bool) Model {
	m := Model{Farm: state, Events: events, Refresh: refresh, Demo: demo, width: 100, height: 32, now: time.Now(), details: viewport.New(50, 15), notices: viewport.New(40, 15), inspection: viewport.New(90, 26)}
	m.syncPanels()
	return m
}
func tick() tea.Cmd { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func (m Model) wait() tea.Cmd {
	return func() tea.Msg {
		e, ok := <-m.Events
		if !ok {
			return endMsg{}
		}
		return e
	}
}
func (m Model) Init() tea.Cmd { return tea.Batch(m.wait(), tick()) }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case endMsg:
		m.ended = true
	case browserResultMsg:
		m.toast = "Opened Bambu troubleshooting in your browser"
		if v.err != nil {
			m.toast = "Could not open browser; use the troubleshooting URL shown"
		}
		m.toastAt = m.now
	case tickMsg:
		m.now = time.Time(v)
		m.Farm.Reconcile(m.now)
		if m.now.Sub(m.toastAt) > 8*time.Second {
			m.toast = ""
		}
		cmd = tick()
	case monitor.Event:
		m.now = time.Now()
		switch v.Kind {
		case "devices":
			id := ""
			if p := m.current(); p != nil {
				id = p.ID
			}
			m.Farm.Discover(v.Devices, v.At)
			m.selection = 0
			for i, k := range m.Farm.Order {
				if k == id {
					m.selection = i
				}
			}
		case "connection":
			m.Farm.Cloud(v.Connected, v.Problem, v.At)
		case "report":
			if err := m.Farm.Report(v.ID, v.Payload, v.At); err != nil {
				m.toast = "Ignored malformed printer status"
				m.toastAt = m.now
			}
		case "problem":
			m.toast = v.Problem
			m.toastAt = m.now
		}
		cmd = m.wait()
	case tea.KeyMsg:
		key := v.String()
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.confirmClear {
			switch key {
			case "y", "Y":
				m.Farm.ClearEvents()
				m.confirmClear = false
				m.toast = "Session event history cleared; active conditions remain"
				m.toastAt = m.now
			case "esc", "n", "N":
				m.confirmClear = false
			}
			m.syncPanels()
			return m, nil
		}
		if m.inspected != nil {
			cmd = m.inspectorKey(key, msg)
			m.syncPanels()
			return m, cmd
		}
		if m.help && key != "?" && key != "esc" {
			return m, nil
		}
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help = !m.help
		case "esc":
			m.help = false
			m.focus = 0
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
		case "f":
			m.focusFault()
		case "enter":
			if m.focus == 2 {
				m.openNotice()
			}
		case "a":
			if m.focus == 2 {
				if item, ok := m.selectedNotice(); ok {
					m.acknowledge(item, true)
				}
			}
		case "c":
			if m.focus == 2 {
				if item, ok := m.selectedNotice(); ok {
					m.clearNotice(item)
				}
			}
		case "C":
			if m.focus == 2 && len(m.Farm.Events) > 0 {
				m.confirmClear = true
			}
		case "r":
			if m.ended {
				m.toast = "Reconnect by restarting bambu-tui after signing in"
			} else {
				select {
				case m.Refresh <- struct{}{}:
				default:
				}
				m.toast = "Status refresh requested"
			}
			m.toastAt = m.now
		default:
			switch m.focus {
			case 0:
				old := m.selection
				switch v.String() {
				case "up", "k":
					m.selection = max(0, m.selection-1)
				case "down", "j":
					m.selection = min(max(0, len(m.Farm.Order)-1), m.selection+1)
				case "home", "g":
					m.selection = 0
				case "end", "G":
					m.selection = max(0, len(m.Farm.Order)-1)
				}
				if old != m.selection {
					m.details.GotoTop()
				}
			case 1:
				m.details, cmd = m.details.Update(msg)
			default:
				switch key {
				case "up", "k":
					m.moveNotice(-1, false)
				case "down", "j":
					m.moveNotice(1, false)
				case "home", "g":
					m.moveNotice(0, true)
				case "end", "G":
					m.moveNotice(len(m.noticeItems())-1, true)
				case "pgup", "ctrl+u":
					m.moveNotice(-max(1, m.notices.Height/3), false)
				case "pgdown", "ctrl+d":
					m.moveNotice(max(1, m.notices.Height/3), false)
				}
			}
		}
	}
	m.syncPanels()
	return m, cmd
}
func (m Model) current() *farm.Printer {
	if m.selection < 0 || m.selection >= len(m.Farm.Order) {
		return nil
	}
	return m.Farm.Printers[m.Farm.Order[m.selection]]
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
func cell(s string, w int) string {
	s = ansi.Truncate(safe(s), max(0, w), "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func integer(n *int) string {
	if n == nil {
		return "—"
	}
	return fmt.Sprint(*n)
}
func percent(n *int) string {
	if n == nil {
		return "Unknown"
	}
	return fmt.Sprintf("%d%%", *n)
}
func duration(n *int) string {
	if n == nil {
		return "Unknown"
	}
	if *n >= 60 {
		return fmt.Sprintf("%dh %02dm", *n/60, *n%60)
	}
	return fmt.Sprintf("%dm", *n)
}
func temp(t farm.Temperature) string {
	if t.Current == nil {
		return "Unknown"
	}
	if t.Target == nil {
		return fmt.Sprintf("%d°C", *t.Current)
	}
	return fmt.Sprintf("%d / %d°C", *t.Current, *t.Target)
}
func stateLabel(s string) string {
	switch s {
	case "RUNNING":
		return "Printing"
	case "PAUSE":
		return "Paused"
	case "FINISH":
		return "Finished"
	case "FAILED":
		return "Failed"
	case "UNKNOWN":
		return "Unknown"
	default:
		return strings.ToLower(s)
	}
}
func (m Model) tableRows() int { return min(max(1, len(m.Farm.Order)), max(1, min(6, m.height/5))) }
func (m Model) panelSizes() (int, int, int) {
	available := max(4, m.height-m.tableRows()-9)
	if m.width >= 105 {
		left := (m.width - 3) * 57 / 100
		return left, m.width - left - 3, available
	}
	return m.width, m.width, max(3, (available-2)/2)
}
func (m *Model) syncPanels() {
	left, right, h := m.panelSizes()
	m.details.Width = left
	m.details.Height = h
	m.notices.Width = right
	m.notices.Height = h
	m.details.SetContent(m.detailText(left))
	if m.focus == 2 && !m.noticeFocused {
		m.noticeKey = ""
		m.noticeIndex = 0
		m.noticeFocused = true
	}
	m.syncNoticeSelection(m.noticeItems())
	m.notices.SetContent(m.noticeText(right))
	m.keepNoticeVisible()
	m.syncInspector()
}

func (m Model) View() string {
	if m.width < 60 || m.height < 18 {
		return fit("BAMBU / FARM\n\nResize to at least 60 columns × 18 rows.\nPress q to quit.", m.width, m.height)
	}
	if m.confirmClear {
		return m.clearHistoryView()
	}
	if m.inspected != nil {
		return m.inspectorView()
	}
	mode := "CLOUD"
	if m.Demo {
		mode = "DEMO"
	}
	conn := warning.Render("CONNECTING")
	if m.Farm.Connected {
		conn = accent.Render("CONNECTED")
	}
	if m.ended && !m.Demo {
		conn = danger.Render("SIGN-IN REQUIRED")
	}
	header := accent.Bold(true).Render("BAMBU") + muted.Render(" / FARM") + "  " + muted.Render(mode) + "  " + conn
	clock := m.now.Local().Format("Mon 02 Jan · 15:04:05")
	header += strings.Repeat(" ", max(1, m.width-ansi.StringWidth(header)-ansi.StringWidth(clock))) + muted.Render(clock)
	printing, attention := 0, m.Farm.NoticeCount()
	for _, p := range m.Farm.Printers {
		if p.State == "RUNNING" {
			printing++
		}
	}
	summary := fmt.Sprintf("%d printers   %d printing   %d active notices · %d unreviewed", len(m.Farm.Order), printing, attention, m.Farm.Unacknowledged())
	lines := []string{header, muted.Render(summary), "", m.section("01  PRINTERS", m.focus == 0, m.width), m.tableHeader()}
	if len(m.Farm.Order) == 0 {
		text := "Discovering account printers…"
		if m.Farm.Connected {
			text = "No printers found. Bind your printers to this Bambu account."
		}
		lines = append(lines, muted.Render(text))
	} else {
		count := m.tableRows()
		start := max(0, m.selection-count+1)
		for i := start; i < min(len(m.Farm.Order), start+count); i++ {
			lines = append(lines, m.printerRow(i))
		}
	}
	lines = append(lines, "")
	left, right, _ := m.panelSizes()
	if m.width >= 105 {
		titles := lipgloss.JoinHorizontal(lipgloss.Top, m.section("02  PRINTER DETAILS", m.focus == 1, left), "   ", m.section("03  NOTICES", m.focus == 2, right))
		panels := lipgloss.JoinHorizontal(lipgloss.Top, m.details.View(), "   ", m.notices.View())
		lines = append(lines, titles, panels)
	} else {
		lines = append(lines, m.section("02  PRINTER DETAILS", m.focus == 1, left), m.details.View(), m.section("03  NOTICES", m.focus == 2, right), m.notices.View())
	}
	foot := muted.Render("j/k select  tab panel  ↑/↓ scroll  r refresh  ? help  q quit")
	if m.focus == 1 {
		foot = muted.Render("↑/↓ scroll  f view faults  tab panel  q quit")
	}
	if m.focus == 2 {
		foot = muted.Render("j/k select  enter view  a ack  c clear  C history  tab panel")
	}
	if m.toast != "" {
		foot = warning.Render(safe(m.toast))
	}
	if m.help {
		lines = []string{header, "", accent.Render("KEYBOARD"), "", "↑ / ↓  j / k   Select printer/notice; scroll printer details", "Tab / Shift+Tab Switch between printers, details, and notices", "g / G          First / last printer or notice", "PgUp / PgDown  Scroll details or page through notices", "Enter          Open selected notice; Esc/Enter returns", "f              View selected printer's first active fault", "a              Toggle local acknowledgement of an active notice", "c              Dismiss active notice / delete historical event", "C              Clear session event history (asks for confirmation)", "o              Open HMS troubleshooting from the detail view", "r              Request a fresh printer status snapshot", "Esc            Close help and focus printers", "q / Ctrl+C     Quit", "", muted.Render("a marks reviewed. c hides locally; printer state is unchanged."), muted.Render("? or Esc closes help")}
	}
	body := strings.Join(lines, "\n")
	rows := strings.Split(body, "\n")
	if len(rows) > m.height-1 {
		rows = rows[:m.height-1]
	}
	for len(rows) < m.height-1 {
		rows = append(rows, "")
	}
	rows = append(rows, foot)
	return fit(strings.Join(rows, "\n"), m.width, m.height)
}
func fit(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > max(0, h) {
		lines = lines[:max(0, h)]
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, max(0, w), "")
	}
	return strings.Join(lines, "\n")
}
func (m Model) section(title string, focused bool, w int) string {
	style := muted
	marker := "  "
	if focused {
		style = accent
		marker = "› "
	}
	text := marker + safe(title) + " "
	return style.Render(text + strings.Repeat("─", max(0, w-ansi.StringWidth(text))))
}
func (m Model) tableHeader() string {
	if m.width < 80 {
		return muted.Render(cell("  PRINTER", 21) + cell("STATE", 12) + cell("PROGRESS", 12) + "LEFT")
	}
	if m.width < 110 {
		return muted.Render(cell("  PRINTER", 22) + cell("LINK", 10) + cell("STATE", 12) + cell("DONE", 9) + cell("LEFT", 11) + "FINISH")
	}
	return muted.Render(cell("  PRINTER", 23) + cell("LINK", 10) + cell("STATE", 12) + cell("PROGRESS", 16) + cell("LEFT", 11) + cell("FINISH", 10) + cell("!", 4) + "JOB")
}
func (m Model) printerRow(i int) string {
	p := m.Farm.Printers[m.Farm.Order[i]]
	marker := "  "
	if i == m.selection {
		marker = "› "
	}
	name := marker + p.Name
	state := stateLabel(p.State)
	left := duration(p.Remaining)
	connection := p.Connection(m.now, m.Farm.Connected)
	if connection == "Cloud disconnected" {
		connection = "No cloud"
	}
	if connection == "Waiting for data" {
		connection = "Waiting"
	}
	eta := "—"
	if t, ok := p.ETA(); ok {
		eta = t.Local().Format("15:04")
		if t.Local().Format("2006-01-02") != m.now.Local().Format("2006-01-02") {
			eta += " +1d"
		}
	}
	if p.Stale(m.now) || !m.Farm.Connected || !p.Online {
		left = "~" + left
		eta = "~" + eta
	}
	var row string
	if m.width < 80 {
		row = cell(name, 21) + cell(state, 12) + cell(percent(p.Progress), 12) + left
	} else if m.width < 110 {
		row = cell(name, 22) + cell(connection, 10) + cell(state, 12) + cell(percent(p.Progress), 9) + cell(left, 11) + eta
	} else {
		bar := "────────"
		if p.Progress != nil {
			n := *p.Progress * 8 / 100
			bar = strings.Repeat("━", n) + strings.Repeat("─", 8-n)
		}
		notices := 0
		for _, c := range m.Farm.Active {
			if c.Printer == p.Name && !c.Dismissed {
				notices++
			}
		}
		row = cell(name, 23) + cell(connection, 10) + cell(state, 12) + cell(bar+" "+percent(p.Progress), 16) + cell(left, 11) + cell(eta, 10) + cell(fmt.Sprint(notices), 4) + safe(p.Job)
	}
	row = cell(row, m.width)
	if i == m.selection {
		return selected.Render(row)
	}
	return white.Render(row)
}
func wrapLines(lines []string, w int) string {
	return ansi.Hardwrap(strings.Join(lines, "\n"), max(1, w), true)
}
func (m Model) detailText(w int) string {
	p := m.current()
	if p == nil {
		return muted.Render("Select a printer once discovery completes.")
	}
	job := safe(p.Job)
	if job == "" {
		job = "No reported job"
	}
	lines := []string{accent.Bold(true).Render(safe(p.Name)) + muted.Render("  "+safe(p.Model)), white.Render(job), muted.Render(p.Connection(m.now, m.Farm.Connected) + " · " + safe(p.Stage)), "", fmt.Sprintf("Layer %s / %s  ·  %s complete", integer(p.Layer), integer(p.TotalLayers), percent(p.Progress)), "Remaining " + duration(p.Remaining), "Bed      " + temp(p.Bed), "Chamber  " + temp(p.Chamber)}
	for _, n := range p.Nozzles {
		name := "Right"
		if n.ID == 1 {
			name = "Left"
		}
		flag := ""
		if n.Active {
			flag = " · active"
		}
		lines = append(lines, fmt.Sprintf("%-8s %s%s", name, temp(n.Temperature), flag))
	}
	if p.LastReport.IsZero() {
		lines = append(lines, muted.Render("Waiting for first printer status…"))
	} else {
		lines = append(lines, muted.Render("Last report "+p.LastReport.Local().Format("15:04:05")))
	}
	lines = append(lines, "", accent.Render("FILAMENT"))
	if len(p.AMS) == 0 && len(p.External) == 0 {
		lines = append(lines, muted.Render("No filament data reported"))
	}
	for _, a := range p.AMS {
		summary := farm.AMSLabel(a.ID)
		if a.Humidity != nil {
			summary += fmt.Sprintf(" · %d%% RH", *a.Humidity)
		}
		if a.Temperature != nil {
			summary += fmt.Sprintf(" · %d°C", *a.Temperature)
		}
		lines = append(lines, white.Bold(true).Render(summary))
		for _, slot := range a.Slots {
			lines = append(lines, slotLine(fmt.Sprintf("%d", slot.ID+1), slot))
		}
	}
	for _, slot := range p.External {
		lines = append(lines, slotLine(fmt.Sprintf("External %d", 256-slot.ID), slot))
	}
	var visibleFaults []farm.Fault
	for _, f := range p.Faults {
		if c := m.Farm.Active[p.ID+"/fault/"+f.Code]; !c.Dismissed {
			visibleFaults = append(visibleFaults, f)
		}
	}
	if len(visibleFaults) > 0 {
		lines = append(lines, "", warning.Render("PRINTER NOTICES / FAULTS"))
		for _, f := range visibleFaults {
			style := muted
			switch f.Level {
			case "error":
				style = danger
			case "warning":
				style = warning
			}
			lines = append(lines, style.Render(safe(f.Text)), muted.Render(safe(f.Code)))
		}
	}
	return wrapLines(lines, w)
}
func slotLine(name string, s farm.Slot) string {
	if s.Present != nil && !*s.Present {
		return muted.Render("  " + name + "  Empty")
	}
	material := safe(s.Material)
	if material == "" {
		material = "Unknown material"
	}
	color := strings.ToUpper(s.Color)
	swatch := "○"
	hex := ""
	if len(color) == 8 || len(color) == 6 {
		valid := true
		for _, r := range color {
			if !strings.ContainsRune("0123456789ABCDEF", r) {
				valid = false
			}
		}
		if valid {
			hex = "#" + color[:6]
			swatch = lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("●")
		}
	}
	active := ""
	for _, id := range s.Nozzles {
		side := "R"
		if id == 1 {
			side = "L"
		}
		active += " " + side
	}
	remaining := percent(s.Remaining)
	return "  " + name + " " + swatch + " " + material + " " + muted.Render(hex) + "  " + remaining + accent.Render(active)
}

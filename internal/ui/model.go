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
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/diagnostics"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

var (
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A3E4BB"))
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#929BA8"))
	white    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E4E9F0"))
	warning  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4BA72"))
	danger   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8087"))
	selected = lipgloss.NewStyle().Background(lipgloss.Color("#263B36")).Foreground(lipgloss.Color("#D9F5E4"))
)

type (
	tickMsg time.Time

	endMsg struct{}

	Model struct {
		Farm                            *farm.State
		Events                          <-chan monitor.Event
		Refresh                         chan<- struct{}
		Demo                            bool
		width, height, selection, focus int
		now                             time.Time
		ended                           bool
		toast                           string
		toastAt                         time.Time
		details, notices                viewport.Model
		noticeKey                       string
		noticeIndex                     int
		noticeFocused                   bool
		noticeSpans                     [][2]int
		inspected                       *noticeItem
		inspection                      viewport.Model
		dialog                          dialogState
		saveReviews                     func(map[string]farm.Review) error
		reviewWrites                    persistence
		snoozeWrites                    persistence
		snoozeRevision                  uint64
		quitting                        bool
		quitProblem                     bool
		quitGeneration                  uint64
		noticeStorageError              string
		attention                       *attentionDialog
		attentionSeen                   map[attentionOccurrence]bool
		attentionPending                time.Time
		filament                        *filamentDialog
		decisions                       *decisionsDialog
		alertDialog                     *alertDialog
		alertPrefs                      config.AlertPreferences
		snoozes                         map[string]time.Time
		saveSnoozes                     func(map[string]time.Time) error
		snoozesDirty                    bool
		alertStorageError               string
		printersDiscovered              bool
		printerIDs                      []string
		printerMetrics                  map[string]printerOverview
		printerFilter                   printerFilter
		printerSort                     printerSort
		overview                        farmOverview
		diagnostics                     diagnostics.Recorder
		exportDialog                    *exportDialog
		clearNotices                    *clearNoticesDialog
	}
)

// WithNoticePersistence enables asynchronous saves from the UI reducer. A failed
// load disables writes for this session to preserve the original file.
func (m Model) WithNoticePersistence(save func(map[string]farm.Review) error, loadErr error) Model {
	m.reviewWrites = persistence{attempted: m.Farm.ReviewRevision(), saved: m.Farm.ReviewRevision()}
	m.saveReviews = nil
	if loadErr != nil {
		m.noticeStorageError = "Saved notices unavailable; decisions apply only to this session"
	} else if !m.Demo {
		m.saveReviews = save
	}
	return m
}

func New(state *farm.State, events <-chan monitor.Event, refresh chan<- struct{}, demo bool) Model {
	m := Model{alertPrefs: config.DefaultAlerts(), Farm: state, Events: events, Refresh: refresh, Demo: demo, width: 100, height: 32, now: time.Now(), details: viewport.New(50, 15), notices: viewport.New(40, 15), inspection: viewport.New(90, 26)}
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
	result, cmd := m.update(msg)
	result.pruneSnoozes()
	result.syncDialogs()
	cmd = tea.Batch(cmd, result.persist())
	if result.quitting && !result.persistencePending() {
		if result.reviewWrites.failed != 0 || result.snoozeWrites.failed != 0 {
			result.quitting, result.quitProblem = false, true
		} else {
			return result, tea.Quit
		}
	}
	return result, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch v := msg.(type) {
	case persistenceResultMsg:
		m.persistenceResult(v)
	case shutdownTimeoutMsg:
		if m.quitting && uint64(v) == m.quitGeneration {
			m.quitting, m.quitProblem = false, true
		}
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case endMsg:
		m.ended = true
	case exportResultMsg:
		m.exportResult(v)
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
		case monitor.EventDevices:
			m.Farm.Discover(v.Devices, v.At)
			m.printersDiscovered = true
		case monitor.EventConnection:
			m.Farm.Cloud(v.Connected, v.Problem, v.At)
		case monitor.EventReport:
			if m.Farm.Printers[v.ID] != nil {
				m.diagnostics.Record(v.ID, v.Payload)
			}
			if err := m.Farm.Report(v.ID, v.Payload, v.At); err != nil {
				m.toast = "Ignored malformed printer status"
				m.toastAt = m.now
			}
		case monitor.EventProblem:
			m.toast = v.Problem
			m.toastAt = m.now
		}
		cmd = m.wait()
	case tea.KeyMsg:
		key := v.String()
		if !m.acceptsKey(key) {
			return m, nil
		}
		if m.hasOpenDialog() {
			cmd = routeFor(m.dialog.active).key(&m, key, msg)
			m.syncPanels()
			return m, cmd
		}
		switch v.String() {
		case "q", "ctrl+c":
			cmd = m.beginShutdown()
			return m, cmd
		case "?":
			m.openDialog(dialogHelp)
		case "!":
			m.attentionSeen = nil
			m.syncAttention(true)
		case "esc":
			m.focus = 0
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
		case "D":
			m.openExport()
		case "F":
			m.printerFilter = (m.printerFilter + 1) % filterCount
		case "S":
			m.printerSort = (m.printerSort + 1) % sortCount
		case "f":
			m.focusFault()
		case "m":
			m.openFilament()
		case "n":
			m.openDecisions()
		case "s":
			if p := m.current(); p != nil {
				m.openAlerts(p.ID)
			}
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
				m.openDialog(dialogClearHistory)
			}
		case "X":
			if m.focus == 2 {
				m.openClearNotices()
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
					m.selection = min(max(0, len(m.printerIDs)-1), m.selection+1)
				case "home", "g":
					m.selection = 0
				case "end", "G":
					m.selection = max(0, len(m.printerIDs)-1)
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
	return m.Farm.Printers[m.selectedPrinterID()]
}

// terminalFits reports whether the dashboard and dialogs fit at their minimum size.
func (m Model) terminalFits() bool {
	return m.width >= 60 && m.height >= 18
}

// printerTelemetryCurrent reports whether displayed telemetry has a live, fresh source.
func (m Model) printerTelemetryCurrent(p *farm.Printer) bool {
	return m.Farm.Connected && p.Online && !p.Stale(m.now)
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
	case "IDLE":
		return "Idle"
	case "UNKNOWN":
		return "Unknown"
	default:
		return strings.ToLower(s)
	}
}

func (m Model) tableRows() int { return min(max(1, len(m.printerIDs)), max(1, min(6, m.height/5))) }

func (m Model) panelSizes() (int, int, int) {
	available := max(4, m.height-m.tableRows()-11)
	if m.width >= 105 {
		left := (m.width - 3) * 57 / 100
		return left, m.width - left - 3, available
	}
	return m.width, m.width, max(1, available/2)
}

func (m *Model) syncPanels() {
	m.syncPrinterList()
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
}

func (m Model) View() string {
	route := routeFor(m.dialog.active)
	if !m.terminalFits() && !route.compact {
		if m.hasOpenDialog() {
			return fit(versionedTitle("BAMBU / FARM", m.width)+"\n\nResize to at least 60 columns × 18 rows.\nPress Esc to close the dialog.", m.width, m.height)
		}
		return fit(versionedTitle("BAMBU / FARM", m.width)+"\n\nResize to at least 60 columns × 18 rows.\nPress q to quit.", m.width, m.height)
	}
	if m.hasOpenDialog() {
		return route.view(m)
	}
	return m.dashboardView(false)
}

func (m Model) dashboardView(help bool) string {
	header := m.dashboardHeader()
	lines := []string{header}
	lines = append(lines, m.overviewLines()...)
	lines = append(lines, "", m.section("01  PRINTERS", m.focus == 0, m.width), muted.Render(m.printerListStatus()), m.tableHeader())
	if len(m.printerIDs) == 0 {
		lines = append(lines, muted.Render(m.emptyPrinterList()))
	} else {
		count := m.tableRows()
		start := max(0, m.selection-count+1)
		for i := start; i < min(len(m.printerIDs), start+count); i++ {
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
	foot := muted.Render("j/k select  tab panel  m AMS  n reviews  s alerts  D export  ? help  q quit")
	if m.focus == 1 {
		foot = muted.Render("↑/↓ scroll  m filament  n decisions  s alerts  q quit")
	}
	if m.focus == 2 {
		foot = muted.Render("j/k select  enter view  a ack  c clear  X clear all  n decisions  s alerts")
	}
	if m.width < 80 {
		foot = muted.Render("j/k move  tab  m AMS  n reviews  s alerts  D diag  ?  q quit")
	}
	if m.toast != "" {
		foot = warning.Render(safe(m.toast))
	}
	if message := m.storageWarning(); message != "" {
		foot = warning.Render(message)
	}
	if help {
		lines = []string{header, "", accent.Render("KEYBOARD"), "", "↑ / ↓  j / k   Select printer/notice; scroll printer details", "Tab / Shift+Tab Switch between printers, details, and notices", "g / G          First / last printer or notice", "PgUp / PgDown  Scroll details or page through notices", "Enter          Open selected notice; Esc/Enter returns", "D              Export local allowlisted diagnostics (confirmation)", "F / S          Cycle printer filter / sort", "f              View selected printer's first active fault", "m              Browse selected printer's AMS/filament", "n              Manage local notice decisions (restore/reset)", "s              Printer alert preferences and snoozing", "!              Reopen unreviewed action-needed alerts", "a              Toggle local acknowledgement of an active notice", "c              Dismiss active notice / delete historical event", "C              Clear session event history (asks for confirmation)", "X              Clear all notices across the farm (confirmation)", "o              Open HMS troubleshooting from the detail view", "r              Request a fresh printer status snapshot", "Esc            Close help and focus printers", "q / Ctrl+C     Quit from dashboard (close help first)", "", muted.Render("a marks reviewed. c hides locally; printer state is unchanged."), muted.Render("? or Esc closes help")}
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
	p := m.Farm.Printers[m.printerIDs[i]]
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
			eta = t.Local().Format("Mon 15:04")
		}
	}
	if !m.printerTelemetryCurrent(p) {
		left = "~" + left
	}
	if eta != "—" && !m.finishTimeCurrent(p) {
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
		notices := m.printerMetrics[p.ID].notices
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

package ui

import (
	"context"
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

type (
	tickMsg time.Time

	endMsg struct{}

	Model struct {
		styles                     *styles
		Farm                       *farm.State
		Events                     <-chan monitor.Event
		Refresh                    chan<- struct{}
		Demo                       bool
		width, height, selection   int
		focus                      dashboardPanel
		now                        time.Time
		ended                      bool
		toast                      string
		toastAt                    time.Time
		toastKind                  feedbackKind
		exportError, browserError  string
		help, status               viewport.Model
		clearHistoryEvents         []uint64
		details, notices           viewport.Model
		noticeKey                  string
		noticeIndex                int
		noticeFocused              bool
		noticeSpans                []noticeSpan
		inspected                  *noticeItem
		inspection                 viewport.Model
		dialog                     dialogState
		saveReviews                func(map[string]farm.Review) error
		reviewWrites               persistence
		snoozeWrites               persistence
		snoozeRevision             uint64
		quitting                   bool
		quitProblem                bool
		quitGeneration             uint64
		noticeStorageError         string
		attention                  *attentionDialog
		attentionSeen              map[attentionOccurrence]bool
		attentionPending           time.Time
		filament                   *filamentDialog
		decisions                  *decisionsDialog
		alertDialog                *alertDialog
		alertPrefs                 config.AlertPreferences
		snoozes                    map[string]time.Time
		saveSnoozes                func(map[string]time.Time) error
		snoozesDirty               bool
		alertStorageError          string
		notificationClient         notificationClient
		notificationStarted        bool
		testNotification           func(context.Context, bool) error
		notificationTestContext    context.Context
		notificationTestCancel     context.CancelFunc
		notificationTestPending    bool
		notificationTestGeneration uint64
		notificationTestStatus     string
		notificationTestKind       feedbackKind
		notificationTestStatusAt   time.Time
		notificationTestFailed     bool
		notificationSetupError     string
		configSelection            int
		configSection              configSection
		configPrinterID            string
		configThemeFile            string
		configRegion               string
		activeRegion               string
		configEditing              bool
		configInput                string
		configReplaceInput         bool
		configResetConfirm         bool
		configInputError           string
		configPath                 string
		saveConfig                 func(config.Config) error
		configRevision             uint64
		configWrites               persistence
		configStorageError         string
		notificationError          string
		notificationRevision       uint64
		notificationActivity       *notificationActivityDialog
		printersDiscovered         bool
		printerIDs                 []string
		printerMetrics             map[string]printerOverview
		printerFilter              printerFilter
		printerSort                printerSort
		overview                   farmOverview
		diagnostics                diagnostics.Recorder
		exportDialog               *exportDialog
		clearNotices               *clearNoticesDialog
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
	m := Model{styles: newStyles(DefaultPalette()), configRegion: config.RegionGlobal, activeRegion: config.RegionGlobal, alertPrefs: config.DefaultAlerts(), Farm: state, Events: events, Refresh: refresh, Demo: demo, width: defaultWidth, height: defaultHeight, now: time.Now(), details: viewport.New(initialDetailsWidth, initialDetailsHeight), notices: viewport.New(initialNoticesWidth, initialNoticesHeight), inspection: viewport.New(initialInspectorWidth, initialInspectorHeight), help: viewport.New(1, 1)}
	m.syncPanels()
	return m
}

func tick() tea.Cmd {
	return tea.Tick(dashboardTickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

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
	previousView := m.viewPreferences()
	result, cmd := m.update(msg)
	result.pruneSnoozes()
	result.syncNotifications()
	result.syncDialogs()
	if result.viewPreferences() != previousView {
		result.configRevision++
	}
	cmd = tea.Batch(cmd, result.persist())
	if result.quitting && !result.persistencePending() {
		if result.reviewWrites.failed != 0 || result.snoozeWrites.failed != 0 || result.configWrites.failed != 0 {
			result.quitting, result.quitProblem = false, true
		} else {
			result.stopNotifications()
			return result, tea.Quit
		}
	}
	return result, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch v := msg.(type) {
	case notificationTestResultMsg:
		m.notificationTestResult(v)
	case configResultMsg:
		m.configResult(v)
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
		m.browserError = ""
		m.setToast(feedbackSuccess, "Opened Bambu troubleshooting in your browser")
		if v.err != nil {
			m.browserError = "Could not open browser; use the troubleshooting URL shown"
			m.setToast(feedbackError, m.browserError)
		}
	case tickMsg:
		m.now = time.Time(v)
		m.Farm.Reconcile(m.now)
		if m.now.Sub(m.toastAt) > toastLifetime {
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
				m.setToast(feedbackWarning, "Ignored malformed printer status")
			}
		case monitor.EventProblem:
			m.setToast(feedbackWarning, v.Problem)
		}
		cmd = m.wait()
	case tea.KeyMsg:
		key := v.String()
		if !m.acceptsKey(key) {
			return m, nil
		}
		if key == keyStatus && m.dialog.active != dialogShutdown {
			if m.dialog.active == dialogStatus {
				m.closeDialog()
			} else {
				m.openStatus()
			}
			return m, nil
		}
		if m.hasOpenDialog() {
			cmd = routeFor(m.dialog.active).key(&m, key, msg)
			m.syncPanels()
			return m, cmd
		}
		switch v.String() {
		case keyQuit, keyInterrupt:
			cmd = m.beginShutdown()
			return m, cmd
		case keyNotificationActivity:
			m.openNotificationActivity()
		case keyConfig:
			m.openDialog(dialogConfig)
		case keyHelp:
			m.openDialog(dialogHelp)
		case keyAttention:
			m.attentionSeen = nil
			m.syncAttention(true)
		case keyEscape:
			m.focus = panelPrinters
		case keyNextPanel:
			m.focus = (m.focus + 1) % panelCount
		case keyPreviousPanel:
			m.focus = (m.focus + panelCount - 1) % panelCount
		case keyDiagnostics:
			m.openExport()
		case keyFilter:
			m.printerFilter = (m.printerFilter + 1) % filterCount
		case keyResetView:
			m.printerFilter, m.printerSort = filterAll, sortName
		case keySort:
			m.printerSort = (m.printerSort + 1) % sortCount
		case keyFault:
			m.focusFault()
		case keyFilament:
			m.openFilament()
		case keyDecisionsOrNo:
			m.openDecisions()
		case keyAlerts:
			if p := m.current(); p != nil {
				m.openAlerts(p.ID)
			}
		case keyEnter:
			if m.focus == panelNotices {
				m.openNotice()
			}
		case keyAcknowledge:
			if m.focus == panelNotices {
				if item, ok := m.selectedNotice(); ok {
					m.acknowledge(item, true)
				}
			}
		case keyClear:
			if m.focus == panelNotices {
				if item, ok := m.selectedNotice(); ok {
					m.clearNotice(item)
				}
			}
		case keyClearHistory:
			if m.focus == panelNotices && len(m.Farm.Events) > 0 {
				m.openClearHistory()
			}
		case keyClearAll:
			if m.focus == panelNotices {
				m.openClearNotices()
			}
		case keyRefreshOrRestore:
			if m.ended {
				m.setToast(feedbackWarning, "Reconnect by restarting bambu-tui after signing in")
			} else {
				select {
				case m.Refresh <- struct{}{}:
				default:
				}
				m.setToast(feedbackInfo, "Status refresh requested")
			}
		default:
			switch m.focus {
			case panelPrinters:
				old := m.selection
				switch v.String() {
				case keyUp, keyVimUp:
					m.selection = max(0, m.selection-1)
				case keyDown, keyVimDown:
					m.selection = min(max(0, len(m.printerIDs)-1), m.selection+1)
				case keyHome, keyFirst:
					m.selection = 0
				case keyEnd, keyLast:
					m.selection = max(0, len(m.printerIDs)-1)
				}
				if old != m.selection {
					m.details.GotoTop()
				}
			case panelDetails:
				m.details, cmd = m.details.Update(msg)
			case panelNotices:
				switch key {
				case keyUp, keyVimUp:
					m.moveNotice(-1, false)
				case keyDown, keyVimDown:
					m.moveNotice(1, false)
				case keyHome, keyFirst:
					m.moveNotice(0, true)
				case keyEnd, keyLast:
					m.moveNotice(len(m.noticeItems())-1, true)
				case keyPageUp, keyHalfPageUp:
					m.moveNotice(-max(1, m.notices.Height/estimatedNoticeRows), false)
				case keyPageDown, keyHalfPageDown:
					m.moveNotice(max(1, m.notices.Height/estimatedNoticeRows), false)
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
	return m.width >= minimumWidth && m.height >= minimumHeight
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
	if *n >= minutesPerHour {
		return fmt.Sprintf("%dh %02dm", *n/minutesPerHour, *n%minutesPerHour)
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
	case farm.StateRunning:
		return "Printing"
	case farm.StatePaused:
		return "Paused"
	case farm.StateFinished:
		return "Finished"
	case farm.StateFailed:
		return "Failed"
	case farm.StateIdle:
		return "Idle"
	case farm.StateUnknown:
		return "Unknown"
	default:
		return strings.ToLower(s)
	}
}

func (m Model) tableRows() int {
	return min(max(1, len(m.printerIDs)), max(1, min(maximumPrinterRows, m.height/printerRowsHeightDivisor)))
}

func (m Model) panelSizes() (int, int, int) {
	available := max(minimumPanelRows, m.height-m.tableRows()-dashboardReservedRows)
	if m.width >= sideBySideWidth {
		left := (m.width - panelGap) * detailsWidthPercent / farm.MaxPercent
		return left, m.width - left - panelGap, available
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
	if m.focus == panelNotices && !m.noticeFocused {
		m.noticeKey = ""
		m.noticeIndex = 0
		m.noticeFocused = true
	}
	m.syncNoticeSelection(m.noticeItems())
	m.notices.SetContent(m.noticeText(right))
	m.keepNoticeVisible()
}

func (m Model) View() string {
	return m.styles.canvas(m.view(), m.width, m.height)
}

func (m Model) view() string {
	route := routeFor(m.dialog.active)
	if !m.terminalFits() && !route.compact {
		if m.hasOpenDialog() {
			return fit(m.versionedTitle("BAMBU / FARM", m.width)+"\n\nResize to at least 60 columns × 18 rows.\nPress Esc to close the dialog.", m.width, m.height)
		}
		return fit(m.versionedTitle("BAMBU / FARM", m.width)+"\n\nResize to at least 60 columns × 18 rows.\nPress q to quit.", m.width, m.height)
	}
	if m.hasOpenDialog() {
		return route.view(m)
	}
	return m.dashboardView()
}

func (m Model) dashboardView() string {
	header := m.dashboardHeader()
	lines := []string{header}
	lines = append(lines, m.overviewLines()...)
	lines = append(lines, "", m.section("PRINTERS", m.focus == panelPrinters, m.width), m.styles.secondary.Render(m.printerListStatus()), m.tableHeader())
	if len(m.printerIDs) == 0 {
		lines = append(lines, m.styles.secondary.Render(m.emptyPrinterList()))
	} else {
		count := m.tableRows()
		start := max(0, m.selection-count+1)
		for i := start; i < min(len(m.printerIDs), start+count); i++ {
			lines = append(lines, m.printerRow(i))
		}
	}
	lines = append(lines, "")
	left, right, _ := m.panelSizes()
	if m.width >= sideBySideWidth {
		titles := lipgloss.JoinHorizontal(lipgloss.Top, m.section("PRINTER DETAILS", m.focus == panelDetails, left), "   ", m.section("NOTICES", m.focus == panelNotices, right))
		panels := lipgloss.JoinHorizontal(lipgloss.Top, m.details.View(), "   ", m.notices.View())
		lines = append(lines, titles, panels)
	} else {
		lines = append(lines, m.section("PRINTER DETAILS", m.focus == panelDetails, left), m.details.View(), m.section("NOTICES", m.focus == panelNotices, right), m.notices.View())
	}
	foot := "↑/↓ select  Tab panel  m filament  n decisions  s alerts  N activity  , config  D export  ? help  q quit"
	if m.focus == panelDetails {
		foot = "↑/↓ scroll  Tab panel  m filament  n decisions  s alerts  N activity  , config  ? help  q quit"
	}
	if m.focus == panelNotices {
		foot = "↑/↓ select  Enter view  a ack  c dismiss  X clear all  n decisions  ? help  q quit"
	}
	if ansi.StringWidth(foot) > m.width-13 {
		switch m.focus {
		case panelPrinters:
			foot = "↑/↓ select  Tab panel  s alerts  ? help  q quit"
		case panelDetails:
			foot = "↑/↓ scroll  Tab panel  s alerts  ? help  q quit"
		case panelNotices:
			foot = "Enter view  a ack  c dismiss  Tab panel  q quit"
		}
	}
	if m.focus == panelNotices {
		if item, ok := m.selectedNotice(); !ok {
			foot = "Tab panel  ? help  q quit"
		} else if !item.Active {
			foot = "Enter view  c delete event  Tab panel  q quit"
		}
	}
	if len(m.printerIDs) == 0 && m.focus != panelNotices {
		foot = "F filter  S sort  0 reset  ? help  q quit"
	}
	foot = m.statusFooter(foot, "? help  q quit", m.width-2)
	body := strings.Join(lines, "\n")
	rows := strings.Split(body, "\n")
	if len(rows) > m.height-1 {
		rows = rows[:m.height-1]
	}
	for len(rows) < m.height-1 {
		rows = append(rows, "")
	}
	rows = append(rows, m.footerRow(foot))
	return fit(strings.Join(rows, "\n"), m.width, m.height)
}

// footerHints emphasizes the key at the start of each double-space-separated hint.
func (m Model) footerHints(text string) string {
	return renderHints(text, m.styles.footer, m.styles.footerHotkey, m.styles.footerSeparator)
}

func (m Model) headerHints(text string) string {
	return renderHints(text, m.styles.secondary, m.styles.hotkey, m.styles.secondary)
}

func renderHints(text string, base, hotkey, separator lipgloss.Style) string {
	hints := strings.Split(text, "  ")
	for i, hint := range hints {
		key, label, hasLabel := strings.Cut(hint, " ")
		hints[i] = hotkey.Render(key)
		if hasLabel {
			hints[i] += base.Render(" " + label)
		}
	}
	return strings.Join(hints, separator.Render("  "))
}

func (m Model) footerRow(text string) string {
	width := max(0, m.width)
	if width < 2 {
		return m.styles.footer.Width(width).Render("")
	}
	return m.styles.footer.Width(width).Padding(0, 1).Render(ansi.Truncate(text, width-2, ""))
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
	style := m.styles.secondary
	marker := "  "
	if focused {
		style = m.styles.focus
		marker = "› "
	}
	text := marker + safe(title) + " "
	return style.Render(text + strings.Repeat("─", max(0, w-ansi.StringWidth(text))))
}

func (m Model) tableHeader() string {
	c := columnsFor(m.width)
	if m.width < compactTableWidth {
		return m.styles.secondary.Render(cell("  PRINTER", c.name) + cell("STATE", c.state) + cell("PROGRESS", c.progress) + "LEFT")
	}
	if m.width < fullTableWidth {
		return m.styles.secondary.Render(cell("  PRINTER", c.name) + cell("LINK", c.link) + cell("STATE", c.state) + cell("DONE", c.progress) + cell("LEFT", c.remaining) + "FINISH")
	}
	return m.styles.secondary.Render(cell("  PRINTER", c.name) + cell("LINK", c.link) + cell("STATE", c.state) + cell("PROGRESS", c.progress) + cell("LEFT", c.remaining) + cell("FINISH", c.finish) + cell("!", c.notices) + "JOB")
}

func (m Model) printerRow(i int) string {
	c := columnsFor(m.width)
	p := m.Farm.Printers[m.printerIDs[i]]
	marker := "  "
	if i == m.selection {
		marker = "› "
	}
	name := marker + p.Name
	state := stateLabel(p.State)
	left := duration(p.Remaining)
	connection := p.Connection(m.now, m.Farm.Connected)
	if connection == farm.ConnectionCloudDisconnected {
		connection = "No cloud"
	}
	if connection == farm.ConnectionWaiting {
		connection = "Waiting"
	}
	eta := "—"
	if t, ok := p.ETA(); ok {
		eta = t.Local().Format("15:04")
		if t.Local().Format(time.DateOnly) != m.now.Local().Format(time.DateOnly) {
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
	if !p.Online {
		row = cell(name, c.name) + farm.ConnectionOffline
	} else if m.width < compactTableWidth {
		row = cell(name, c.name) + cell(state, c.state) + cell(percent(p.Progress), c.progress) + left
	} else if m.width < fullTableWidth {
		row = cell(name, c.name) + cell(connection, c.link) + cell(state, c.state) + cell(percent(p.Progress), c.progress) + cell(left, c.remaining) + eta
	} else {
		bar := strings.Repeat("─", progressBarWidth)
		if p.Progress != nil {
			n := *p.Progress * progressBarWidth / farm.MaxPercent
			bar = strings.Repeat("━", n) + strings.Repeat("─", progressBarWidth-n)
		}
		notices := m.printerMetrics[p.ID].notices
		row = cell(name, c.name) + cell(connection, c.link) + cell(state, c.state) + cell(bar+" "+percent(p.Progress), c.progress) + cell(left, c.remaining) + cell(eta, c.finish) + cell(fmt.Sprint(notices), c.notices) + safe(p.Job)
	}
	row = cell(row, m.width)
	style := m.styles.text
	if i == m.selection {
		style = m.styles.selection
	}
	if !p.Online {
		style = style.Foreground(m.styles.warning.GetForeground())
	}
	return style.Render(row)
}

func wrapLines(lines []string, w int) string {
	return ansi.Hardwrap(strings.Join(lines, "\n"), max(1, w), true)
}

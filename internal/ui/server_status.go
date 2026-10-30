package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/monitor"
)

type serverDiagnostic struct {
	text string
	kind feedbackKind
	at   time.Time
}

// Lifecycle work stays in syncWebServer's serialized command. Both Config and
// inspection describe its current attempt, independently of monitoring state.
func (m Model) webLifecycle() (string, feedbackKind) {
	if m.webBusy {
		if !m.webAttempted {
			return "Stopping", feedbackWarning
		}
		return "Starting", feedbackInfo
	}
	if m.webStatus.Error != "" {
		return "Failed", feedbackError
	}
	if m.webStatus.URL != "" {
		return "Running", feedbackSuccess
	}
	if !m.webEnabled {
		return "Stopped (disabled)", feedbackInfo
	}
	return "Stopped", feedbackWarning
}

func (m *Model) noteServer(text string, kind feedbackKind) {
	if len(m.serverDiagnostics) > 0 && m.serverDiagnostics[len(m.serverDiagnostics)-1].text == text {
		return
	}
	// Copy before appending: model values and palette changes retain independent
	// presentation caches, without any persistent or browser-visible metadata.
	notes := append([]serverDiagnostic(nil), m.serverDiagnostics...)
	notes = append(notes, serverDiagnostic{text: text, kind: kind, at: m.now})
	if len(notes) > serverDiagnosticLimit {
		notes = notes[len(notes)-serverDiagnosticLimit:]
	}
	m.serverDiagnostics = notes
}

func (m *Model) observeServerMonitor(event monitor.Event) {
	switch {
	case event.StopReason == monitor.StopAuthenticationRejected:
		m.monitorState = "Authentication rejected"
		m.noteServer("Authentication rejected; log in and restart monitoring", feedbackError)
	case event.Kind == monitor.EventConnection:
		state, kind := "Reconnecting", feedbackWarning
		if event.Connected {
			state, kind = "Connected", feedbackSuccess
		}
		if m.monitorState != state {
			m.noteServer("Monitoring: "+state, kind)
		}
		m.monitorState = state
	case event.Kind == monitor.EventProblem:
		// Backend diagnostics can contain identifiers or secrets. Keep fixed prose.
		m.noteServer("Monitor warning; monitoring continues", feedbackWarning)
	}
}

func (m *Model) openServerStatus() {
	m.openDialog(dialogServerStatus)
	m.serverStatus = viewport.New(1, 1)
	m.syncServerStatus()
}

func (m *Model) serverStatusKey(key string, msg tea.Msg) tea.Cmd {
	switch key {
	case keyEscape, keyServerStatus:
		m.closeDialog()
	case keyHome, keyFirst:
		m.serverStatus.GotoTop()
	case keyEnd, keyLast:
		m.serverStatus.GotoBottom()
	default:
		var cmd tea.Cmd
		m.serverStatus, cmd = m.serverStatus.Update(msg)
		return cmd
	}
	return nil
}

func elapsed(now, started time.Time) string {
	if started.IsZero() {
		return "Unknown"
	}
	return max(time.Duration(0), now.Sub(started)).Truncate(time.Second).String()
}

func (m *Model) syncServerStatus() {
	// Do not clamp away a scroll position while the viewport is hidden behind
	// the minimum-size guard. Observations still update in the normal reducer.
	if !m.terminalFits() {
		return
	}
	m.serverStatus.Width, m.serverStatus.Height = max(1, m.width-2), max(1, m.height-4)
	state, kind := m.webLifecycle()
	mode := "cloud"
	if m.Demo {
		mode = "demo (synthetic)"
	}
	monitoring, monitorKind := m.monitorState, feedbackWarning
	if m.Farm.Connected {
		monitorKind = feedbackSuccess
	}
	if m.ended {
		monitoring, monitorKind = "Stopped (event stream ended)", feedbackWarning
	}
	if m.monitorState == "Authentication rejected" {
		monitoring, monitorKind = m.monitorState, feedbackError
	}
	lines := []string{
		m.styles.message(kind).Render("Server: " + state),
		"Mode: " + mode,
		m.styles.message(monitorKind).Render("Monitoring: " + monitoring),
		"Session uptime: " + elapsed(m.now, m.sessionStarted),
	}
	if m.webStatus.URL != "" {
		lines = append(lines, "Bound URL: "+m.webStatus.URL, "Listener uptime: "+elapsed(m.now, m.webStatus.Started))
	} else {
		lines = append(lines, "Bound URL: unavailable (not listening)")
	}
	lines = append(lines, "Connected monitoring does not establish fresh printer telemetry.", "Controls: Config (,) > Web Server; inspection changes nothing.")
	if m.webStatus.Error != "" {
		lines = append(lines, m.styles.err.Render(safe(m.webStatus.Error)), "Choose a port, retry (r), or disable in Config > Web Server.")
	}
	for _, problem := range m.statusProblems() {
		if problem.name == "Config" || problem.name == "Notices" || problem.name == "snoozes" {
			lines = append(lines, m.styles.err.Render(problem.name+" storage: "+safe(problem.message)), problem.recovery)
		}
	}
	lines = append(lines, "", m.styles.accent.Render(fmt.Sprintf("HTTP clients: %d active", m.webClients.Active)))
	if m.webClients.Omitted > 0 {
		lines = append(lines, m.styles.warning.Render(fmt.Sprintf("%d client details omitted (tracking limit)", m.webClients.Omitted)))
	}
	lines = append(lines, "Best-effort groups: socket IP + first-request User-Agent.", "No-request sockets ignored; disconnect grace: 5 seconds.")
	if len(m.webClients.Events) == 0 {
		lines = append(lines, "Recent client activity: none")
	}
	if len(m.webClients.Events) > 0 && m.webClients.Events[0].Sequence > 1 {
		lines = append(lines, "Earlier client events no longer retained (bounded history).")
	}
	for _, event := range m.webClients.Events {
		style := m.styles.secondary
		if event.Connected {
			style = m.styles.success
		}
		lines = append(lines, style.Render(event.DisplayLine()))
	}
	lines = append(lines, "", m.styles.accent.Render("Recent diagnostics"))
	if len(m.serverDiagnostics) == 0 {
		lines = append(lines, "None")
	}
	for _, note := range m.serverDiagnostics {
		lines = append(lines, m.styles.message(note.kind).Render(note.at.Local().Format("15:04:05")+" "+note.text))
	}
	offset := m.serverStatus.YOffset
	m.serverStatus.SetContent(ansi.Wrap(strings.Join(lines, "\n"), m.serverStatus.Width, ""))
	m.serverStatus.SetYOffset(offset)
}

func (m Model) serverStatusView() string {
	return m.frame([]string{
		m.styles.accent.Bold(true).Render("SERVER STATUS"),
		m.headerHints("↑/↓ scroll  PgUp/PgDn page  Home/End  Esc back"),
		"", m.serverStatus.View(),
	}, m.footerHints("Esc back  ^E feedback"))
}

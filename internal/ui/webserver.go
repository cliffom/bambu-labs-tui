package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type (
	webServerClient interface {
		PublishDisplay(session.DisplayState)
		SetEnabled(bool, int) webserver.Status
		Status() webserver.Status
	}
	webCommandSource interface {
		Commands() <-chan webserver.Command
		Done() <-chan struct{}
	}
	webServerResultMsg struct{}
	webServerInitMsg   struct{}
)

// WithWebServer installs session-owned listener and notification workers.
// Independent display values and normalized notification inputs leave the reducer;
// HTTP clients receive only allowlisted display values, never live farm state.
func (m Model) WithWebServer(server webServerClient) Model {
	m.webServer = server
	return m
}

func (m *Model) syncWebServer() tea.Cmd {
	if m.webServer == nil {
		return nil
	}
	m.webStatus = m.webServer.Status()
	if source, ok := m.webServer.(interface{ Clients() webserver.ClientStatus }); ok {
		m.webClients = source.Clients()
	}
	m.Publish(m.webServer, m.now, session.BrowserPolicy{Alerts: m.alertPrefs, Active: !m.quitting && !m.quitProblem && !m.ended && m.dialog.active != dialogShutdown})
	if m.webBusy || (m.webInitialized && m.webAttempted == m.webEnabled && (!m.webEnabled || m.webAttemptedPort == m.webPort)) {
		return nil
	}
	enabled, port, server := m.webEnabled, m.webPort, m.webServer
	m.webBusy, m.webInitialized, m.webAttempted, m.webAttemptedPort = true, true, enabled, port
	return func() tea.Msg { server.SetEnabled(enabled, port); return webServerResultMsg{} }
}

func (m Model) waitWebCommand() tea.Cmd {
	source, ok := m.webServer.(webCommandSource)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		select {
		case c := <-source.Commands():
			return c
		case <-source.Done():
			return nil
		}
	}
}

func (m *Model) webDecisions() session.Decisions { return m.WebDecisions() }

func (m *Model) applyWebCommand(c webserver.Command) {
	c.Reply <- string(m.ApplyDecision(session.DecisionCommand{Target: c.Target, Retry: c.Retry}, m.quitting || m.dialog.active == dialogShutdown, m.webEnabled))
}

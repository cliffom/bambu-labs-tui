package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type (
	webServerClient interface {
		Publish([]byte)
		SetEnabled(bool, int) webserver.Status
		Status() webserver.Status
	}
	webServerResultMsg struct{}
	webServerInitMsg   struct{}
)

// WithWebServer installs a session-owned lifecycle worker. Only immutable,
// allowlisted JSON leaves the reducer; HTTP clients never touch live farm state.
func (m Model) WithWebServer(server webServerClient) Model {
	m.webServer = server
	return m
}

func (m *Model) syncWebServer() tea.Cmd {
	if m.webServer == nil {
		return nil
	}
	m.webStatus = m.webServer.Status()
	if m.webEnabled || m.webStatus.URL != "" {
		m.webServer.Publish(webserver.SnapshotJSON(m.Farm, m.now, m.Demo))
	}
	if m.webBusy || (m.webInitialized && m.webAttempted == m.webEnabled && (!m.webEnabled || m.webAttemptedPort == m.webPort)) {
		return nil
	}
	enabled, port, server := m.webEnabled, m.webPort, m.webServer
	m.webBusy, m.webInitialized, m.webAttempted, m.webAttemptedPort = true, true, enabled, port
	return func() tea.Msg { server.SetEnabled(enabled, port); return webServerResultMsg{} }
}

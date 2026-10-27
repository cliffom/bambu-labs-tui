package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type (
	webServerClient interface {
		Publish([]byte)
		SetEnabled(bool) webserver.Status
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
	if m.webBusy || (m.webInitialized && m.webAttempted == m.webEnabled) {
		return nil
	}
	enabled, server := m.webEnabled, m.webServer
	m.webBusy, m.webInitialized, m.webAttempted = true, true, enabled
	return func() tea.Msg { server.SetEnabled(enabled); return webServerResultMsg{} }
}

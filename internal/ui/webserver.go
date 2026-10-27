package ui

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type (
	webServerClient interface {
		Publish([]byte)
		SetEnabled(bool, int) webserver.Status
		Status() webserver.Status
	}
	webTarget struct {
		ID        string
		Condition farm.Condition
	}
	webCommandSource interface {
		Commands() <-chan webserver.Command
		Done() <-chan struct{}
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
	var decisions webserver.Decisions
	if m.webTargets != nil || m.webEnabled || m.webStatus.URL != "" {
		decisions = m.webDecisions()
	}
	if m.webEnabled || m.webStatus.URL != "" {
		m.webServer.Publish(webserver.SnapshotJSON(m.Farm, m.now, m.Demo, decisions))
	}
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

func sameWebOccurrence(key string, a, b farm.Condition) bool {
	// Match farm.Reconcile: only cloud and print-state text identifies a
	// replacement. Filament estimates can change during an ongoing occurrence.
	textMatches := a.Text == b.Text || (key != farm.ConditionCloud && !strings.HasSuffix(key, "/"+farm.ConditionState))
	return a.Since.Equal(b.Since) && a.Level == b.Level && a.Code == b.Code && textMatches
}

func (m *Model) webDecisions() webserver.Decisions {
	if m.webTargets == nil {
		m.webTargets = map[string]webTarget{}
	}
	for key, target := range m.webTargets {
		c, ok := m.Farm.Active[key]
		if !ok || !sameWebOccurrence(key, target.Condition, c) {
			delete(m.webTargets, key)
		}
	}
	ids := map[string]string{}
	for key, c := range m.Farm.Active {
		target, ok := m.webTargets[key]
		if !ok {
			var id [16]byte
			if _, err := rand.Read(id[:]); err != nil {
				panic(err)
			}
			target = webTarget{ID: hex.EncodeToString(id[:]), Condition: c}
			m.webTargets[key] = target
		}
		ids[key] = target.ID
	}
	state := "saved"
	switch {
	case m.saveReviews == nil && m.noticeStorageError != "":
		state = "session-only (saved data unavailable)"
	case m.reviewWrites.inFlight:
		state = "saving"
	case m.noticeStorageError != "":
		state = "failed"
	case m.saveReviews == nil:
		state = "session-only"
	case m.reviewWrites.saved != m.Farm.ReviewRevision():
		state = "saving"
	}
	return webserver.Decisions{Targets: ids, State: state}
}

func (m *Model) applyWebCommand(c webserver.Command) {
	result := "Stale or unknown notice; refresh before retrying"
	if m.quitting || m.dialog.active == dialogShutdown || !m.webEnabled {
		c.Reply <- "Session is stopping or web server disabled"
		return
	}
	for key, target := range m.webTargets {
		current, ok := m.Farm.Active[key]
		if target.ID != c.Target || !ok || !sameWebOccurrence(key, target.Condition, current) {
			continue
		}
		if c.Retry {
			if !current.Dismissed || m.saveReviews == nil || m.reviewWrites.failed == 0 || m.reviewWrites.inFlight {
				break
			}
			m.reviewWrites.attempted = 0
		} else if !current.Dismissed {
			m.Farm.Dismiss(key)
		}
		result = "applied"
		break
	}
	c.Reply <- result
}

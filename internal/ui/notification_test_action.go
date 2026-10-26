package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type notificationTestResultMsg struct {
	generation uint64
	err        error
}

// WithNotificationTest gives the explicit Config action a session-bound probe.
// The command captures only the context, function, and global sound preference.
func (m Model) WithNotificationTest(ctx context.Context, test func(context.Context, bool) error) Model {
	m.notificationTestContext, m.testNotification = ctx, test
	return m
}

func (m *Model) startNotificationTest() tea.Cmd {
	if m.notificationTestPending {
		return nil
	}
	if m.testNotification == nil {
		m.notificationTestStatus = "Test unavailable; run bambu-tui doctor --notify"
		return nil
	}
	ctx, cancel := context.WithCancel(m.notificationTestContext)
	m.notificationTestCancel = cancel
	m.notificationTestPending = true
	m.notificationTestGeneration++
	m.notificationTestStatus = "Test: Sending… Allow macOS permission if prompted."
	generation, test, sound := m.notificationTestGeneration, m.testNotification, m.alertPrefs.NotificationSound
	return func() tea.Msg {
		defer cancel()
		return notificationTestResultMsg{generation: generation, err: test(ctx, sound)}
	}
}

func (m *Model) notificationTestResult(msg notificationTestResultMsg) {
	if !m.notificationTestPending || msg.generation != m.notificationTestGeneration {
		return
	}
	m.notificationTestPending = false
	m.notificationTestCancel = nil
	m.notificationTestStatus = "Test accepted by macOS; check banner and sound."
	if msg.err != nil {
		m.notificationTestStatus = "Test failed; run bambu-tui doctor --notify for details."
	}
}

func (m *Model) cancelNotificationTest() {
	if m.notificationTestCancel != nil {
		m.notificationTestCancel()
		m.notificationTestCancel = nil
		m.notificationTestPending = false
		m.notificationTestGeneration++
		m.notificationTestStatus = "Test canceled; delivery unconfirmed."
	}
}

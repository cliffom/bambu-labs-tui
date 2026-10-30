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
		m.notificationTestFailed = true
		m.notificationTestKind = feedbackError
		m.notificationTestStatus = "Test unavailable; run bambustand doctor --notify"
		return nil
	}
	ctx, cancel := context.WithCancel(m.notificationTestContext)
	m.notificationTestCancel = cancel
	m.notificationTestPending = true
	m.notificationTestFailed = false
	m.notificationTestGeneration++
	m.notificationTestKind = feedbackInfo
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
	m.notificationTestStatusAt = m.now
	m.notificationTestFailed = msg.err != nil
	m.notificationTestKind = feedbackSuccess
	m.notificationTestStatus = "Test accepted by macOS; check banner and sound."
	if msg.err != nil {
		m.notificationTestKind = feedbackError
		m.notificationTestStatus = "Test failed; run bambustand doctor --notify."
	}
}

func (m *Model) cancelNotificationTest() {
	if m.notificationTestCancel != nil {
		m.notificationTestCancel()
		m.notificationTestCancel = nil
		m.notificationTestPending = false
		m.notificationTestGeneration++
		m.notificationTestStatusAt = m.now
		m.notificationTestFailed = false
		m.notificationTestKind = feedbackWarning
		m.notificationTestStatus = "Test canceled; delivery unconfirmed."
	}
}

package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

type feedbackKind uint8

const (
	feedbackInfo feedbackKind = iota
	feedbackSuccess
	feedbackWarning
	feedbackError
	feedbackMuted // Explicitly dimmed Config results; never the fresh default.
)

// Kind describes meaning, independently of source lifetime or persistence.
func (m *Model) setToast(kind feedbackKind, text string) {
	m.toast, m.toastKind, m.toastAt = text, kind, m.now
}

func (s *styles) feedback(kind feedbackKind) lipgloss.Style {
	switch kind {
	case feedbackSuccess:
		return s.footerSuccess
	case feedbackWarning:
		return s.footerWarning
	case feedbackError:
		return s.footerError
	case feedbackMuted:
		return s.footerMuted
	default:
		return s.footerInfo
	}
}

func (s *styles) message(kind feedbackKind) lipgloss.Style {
	switch kind {
	case feedbackSuccess:
		return s.success.Bold(true)
	case feedbackWarning:
		return s.warning.Bold(true)
	case feedbackError:
		return s.err.Bold(true)
	case feedbackMuted:
		return s.muted
	default:
		return s.info
	}
}

func (m Model) notificationTestFeedbackKind() feedbackKind {
	if m.notificationTestPending {
		return feedbackInfo
	}
	if m.notificationTestFailed {
		return feedbackError
	}
	if m.now.Sub(m.notificationTestStatusAt) >= notificationTestDimAfter {
		return feedbackMuted
	}
	return m.notificationTestKind
}

func notificationFeedbackKind(kind notifications.MessageKind) feedbackKind {
	switch kind {
	case notifications.MessageSuccess:
		return feedbackSuccess
	case notifications.MessageWarning:
		return feedbackWarning
	case notifications.MessageError:
		return feedbackError
	default:
		return feedbackInfo
	}
}

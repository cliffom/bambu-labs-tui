package ui

import (
	"time"

	"github.com/cliffom/bambu-tui/internal/session"

	tea "github.com/charmbracelet/bubbletea"
)

type (
	persistenceResultMsg = session.Result
	shutdownTimeoutMsg   uint64
)

func sessionCommand(job session.Job) tea.Cmd {
	if job == nil {
		return nil
	}
	return func() tea.Msg { return job() }
}

func (m *Model) persist() tea.Cmd {
	reviews, snoozes := m.Persistence()
	return tea.Batch(sessionCommand(reviews), sessionCommand(snoozes), m.persistConfig())
}

func (m *Model) persistenceResult(msg persistenceResultMsg) { m.Complete(msg) }

func (m Model) persistencePending() bool {
	return m.PersistencePending() || (m.notificationClient != nil && m.notificationClient.Status().Pending)
}

// beginShutdown bounds the wait without pretending a synchronous filesystem
// operation can be canceled. Esc cancels the shutdown, not the write. On timeout
// or error the user must explicitly choose whether to exit with unsaved changes.
func (m *Model) beginShutdown() tea.Cmd {
	m.openDialog(dialogShutdown)
	m.quitting = true
	m.quitGeneration++
	generation := m.quitGeneration
	return tea.Tick(shutdownSaveTimeout, func(time.Time) tea.Msg { return shutdownTimeoutMsg(generation) })
}

// HasUnsavedPersistence lets the caller report interrupted/forced shutdowns after
// the terminal is restored. Session-only stores have no outstanding disk writes.
func (m Model) HasUnsavedPersistence() bool {
	return m.Session.HasUnsavedPersistence()
}

package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// persistence is owned by the reducer. A command owns only an immutable snapshot.
// attempted advances when queued, saved only on success, and failed on error.
// Keeping one command in flight per store also serializes delayed completions.
type persistence struct {
	attempted, saved, failed uint64
	inFlight                 bool
}

type persistenceResultMsg struct {
	snoozes  bool
	revision uint64
	err      error
}
type shutdownTimeoutMsg uint64

func (p *persistence) start(revision uint64, save func() error, snoozes bool) tea.Cmd {
	if p.inFlight || p.attempted == revision {
		return nil
	}
	p.inFlight, p.attempted = true, revision
	return func() tea.Msg { return persistenceResultMsg{snoozes: snoozes, revision: revision, err: save()} }
}

// persist coalesces changes made during a write into the newest complete snapshot.
// A failed revision is attempted again only after another local change.
func (m *Model) persist() tea.Cmd {
	var reviews, snoozes tea.Cmd
	if m.saveReviews != nil && !m.reviewWrites.inFlight && m.reviewWrites.attempted != m.Farm.ReviewRevision() {
		snapshot, save := m.Farm.Reviews(), m.saveReviews
		reviews = m.reviewWrites.start(m.Farm.ReviewRevision(), func() error { return save(snapshot) }, false)
	}
	if m.snoozesDirty {
		m.snoozeRevision++
		m.snoozesDirty = false
	}
	if m.saveSnoozes != nil && !m.snoozeWrites.inFlight && m.snoozeWrites.attempted != m.snoozeRevision {
		snapshot := make(map[string]time.Time, len(m.snoozes))
		for id, until := range m.snoozes {
			snapshot[id] = until
		}
		save := m.saveSnoozes
		snoozes = m.snoozeWrites.start(m.snoozeRevision, func() error { return save(snapshot) }, true)
	}
	return tea.Batch(reviews, snoozes)
}

func (m *Model) persistenceResult(msg persistenceResultMsg) {
	p, warning := &m.reviewWrites, &m.noticeStorageError
	failure := "Could not save notices; decisions may be lost on restart"
	if msg.snoozes {
		p, warning = &m.snoozeWrites, &m.alertStorageError
		failure = "Could not save snoozes; changes may be lost on restart"
	}
	if !p.inFlight || p.attempted != msg.revision {
		return
	}
	p.inFlight = false
	if msg.err != nil {
		p.failed = msg.revision
		*warning = failure
	} else {
		p.saved, p.failed = msg.revision, 0
		*warning = ""
	}
}

func (m Model) persistencePending() bool {
	return m.reviewWrites.inFlight || m.snoozeWrites.inFlight
}

// beginShutdown bounds the wait without pretending a synchronous filesystem
// operation can be canceled. Esc cancels the shutdown, not the write. On timeout
// or error the user must explicitly choose whether to exit with unsaved changes.
func (m *Model) beginShutdown() tea.Cmd {
	m.openDialog(dialogShutdown)
	m.quitting = true
	m.quitGeneration++
	generation := m.quitGeneration
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return shutdownTimeoutMsg(generation) })
}

// HasUnsavedPersistence lets the caller report interrupted/forced shutdowns after
// the terminal is restored. Session-only stores have no outstanding disk writes.
func (m Model) HasUnsavedPersistence() bool {
	return (m.saveReviews != nil && m.reviewWrites.saved != m.Farm.ReviewRevision()) ||
		(m.saveSnoozes != nil && (m.snoozesDirty || m.snoozeWrites.saved != m.snoozeRevision))
}

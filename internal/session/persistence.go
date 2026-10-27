package session

import (
	"time"

	"github.com/cliffom/bambu-tui/internal/config"
)

type (
	ConfigResult struct {
		Revision uint64
		Err      error
	}
	// WriteState tracks a single serialized store; revisions are not interchangeable.
	WriteState struct {
		Attempted, Saved, Failed uint64
		InFlight                 bool
		attempt                  uint64
		retry                    bool
	}
	// Job owns an immutable snapshot. Run it outside the serial state owner and
	// deliver its Result back to Complete before asking for the next jobs.
	Job    func() Result
	Result struct {
		Snoozes  bool
		Revision uint64
		Attempt  uint64
		Err      error
	}
)

func (p *WriteState) start(revision uint64, save func() error, snoozes bool) Job {
	if p.InFlight || (p.Attempted == revision && !p.retry) {
		return nil
	}
	p.InFlight, p.Attempted = true, revision
	p.retry = false
	p.attempt++
	attempt := p.attempt
	return func() Result { return Result{Snoozes: snoozes, Revision: revision, Attempt: attempt, Err: save()} }
}

func (m *Session) Persistence() (reviews, snoozes Job) {
	if m.saveReviews != nil && !m.reviewWrites.InFlight && (m.reviewWrites.Attempted != m.Farm.ReviewRevision() || m.reviewWrites.retry) {
		snapshot, save := m.Farm.Reviews(), m.saveReviews
		reviews = m.reviewWrites.start(m.Farm.ReviewRevision(), func() error { return save(snapshot) }, false)
	}
	if m.snoozesDirty {
		m.snoozeRevision++
		m.snoozesDirty = false
	}
	if m.saveSnoozes != nil && !m.snoozeWrites.InFlight && (m.snoozeWrites.Attempted != m.snoozeRevision || m.snoozeWrites.retry) {
		snapshot := make(map[string]time.Time, len(m.Snoozes))
		for id, until := range m.Snoozes {
			snapshot[id] = until
		}
		save := m.saveSnoozes
		snoozes = m.snoozeWrites.start(m.snoozeRevision, func() error { return save(snapshot) }, true)
	}
	return reviews, snoozes
}

func (m *Session) Complete(msg Result) {
	p, warning := &m.reviewWrites, &m.NoticeStorageError
	failure := "Could not save notices; decisions may be lost on restart"
	if msg.Snoozes {
		p, warning = &m.snoozeWrites, &m.AlertStorageError
		failure = "Could not save snoozes; changes may be lost on restart"
	}
	if !p.InFlight || p.Attempted != msg.Revision || p.attempt != msg.Attempt {
		return
	}
	p.InFlight = false
	if msg.Err != nil {
		p.Failed = msg.Revision
		*warning = failure
	} else {
		p.Saved, p.Failed = msg.Revision, 0
		*warning = ""
	}
}

// CanRetryPersistence reports failed writable stores with no queued or running
// attempt. Loading failures and demo stores never grant write permission.
func (m Session) CanRetryPersistence() (reviews, snoozes bool) {
	eligible := func(p WriteState) bool { return p.Failed != 0 && !p.InFlight && !p.retry }
	return m.saveReviews != nil && eligible(m.reviewWrites), m.saveSnoozes != nil && eligible(m.snoozeWrites)
}

// RetryPersistence queues the selected eligible stores without changing domain
// revisions or local decisions. Persistence captures the latest complete values.
// Repeated requests while queued/in flight are ignored, not deferred retries.
func (m *Session) RetryPersistence(reviews, snoozes bool) bool {
	r, s := m.CanRetryPersistence()
	r, s = reviews && r, snoozes && s
	if r {
		m.reviewWrites.retry = true
	}
	if s {
		m.snoozeWrites.retry = true
	}
	return r || s
}

func saveState(writable bool, warning string, p WriteState, revision uint64, dirty bool) string {
	switch {
	case !writable && warning != "":
		return "session-only (saved data unavailable)"
	case !writable:
		return "session-only"
	case p.InFlight || p.retry:
		return "saving"
	case warning != "":
		return "failed"
	case dirty || p.Saved != revision:
		return "saving"
	default:
		return "saved"
	}
}

func (m Session) ReviewSaveState() string {
	return saveState(m.saveReviews != nil, m.NoticeStorageError, m.reviewWrites, m.Farm.ReviewRevision(), false)
}

func (m Session) SnoozeSaveState() string {
	return saveState(m.saveSnoozes != nil, m.AlertStorageError, m.snoozeWrites, m.snoozeRevision, m.snoozesDirty)
}

func (m Session) PersistencePending() bool {
	return m.reviewWrites.InFlight || m.reviewWrites.retry || m.snoozeWrites.InFlight || m.snoozeWrites.retry || m.configWrites.InFlight
}

func (m Session) HasUnsavedPersistence() bool {
	return (m.saveConfig != nil && m.configWrites.Saved != m.configRevision) || (m.saveReviews != nil && m.reviewWrites.Saved != m.Farm.ReviewRevision()) ||
		(m.saveSnoozes != nil && (m.snoozesDirty || m.snoozeWrites.Saved != m.snoozeRevision))
}

// PersistConfig captures the host's complete configuration, including any
// terminal view choices. It shares the same serialization/coalescing contract.
func (m *Session) PersistConfig(cfg config.Config) func() ConfigResult {
	if !m.ConfigSaveNeeded() {
		return nil
	}
	revision, snapshot, save := m.configRevision, cfg.Clone(), m.saveConfig
	m.configWrites.InFlight, m.configWrites.Attempted = true, revision
	return func() ConfigResult { return ConfigResult{Revision: revision, Err: save(snapshot)} }
}

// CompleteConfig rejects delayed completions from an older write.
func (m *Session) CompleteConfig(msg ConfigResult) bool {
	if !m.configWrites.InFlight || m.configWrites.Attempted != msg.Revision {
		return false
	}
	m.configWrites.InFlight = false
	if msg.Err != nil {
		m.configWrites.Failed = msg.Revision
	} else {
		m.configWrites.Saved, m.configWrites.Failed = msg.Revision, 0
	}
	return true
}

// ConfigSaveNeeded lets adapters avoid building configuration snapshots on
// telemetry ticks when there is no new revision to persist.
func (m Session) ConfigSaveNeeded() bool {
	return m.saveConfig != nil && !m.configWrites.InFlight && m.configWrites.Attempted != m.configRevision
}

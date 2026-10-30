// Package session owns monitoring state for serial terminal and future headless
// hosts. Call methods on one event loop; only immutable snapshots and Jobs may
// leave it. Exported state is a read view for adapters, not a concurrent API.
package session

import (
	"time"

	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
)

type Session struct {
	saveConfig                            func(config.Config) error
	configWrites                          WriteState
	configRevision                        uint64
	Farm                                  *farm.State
	Demo                                  bool
	saveReviews                           func(map[string]farm.Review) error
	reviewWrites, snoozeWrites            WriteState
	snoozeRevision                        uint64
	NoticeStorageError, AlertStorageError string
	Snoozes                               map[string]time.Time
	saveSnoozes                           func(map[string]time.Time) error
	snoozesDirty, PrintersDiscovered      bool
	webTargets                            map[string]webTarget
}

func New(state *farm.State, demo bool) Session { return Session{Farm: state, Demo: demo} }

func (m *Session) WithNoticePersistence(save func(map[string]farm.Review) error, loadErr error) {
	m.reviewWrites = WriteState{Attempted: m.Farm.ReviewRevision(), Saved: m.Farm.ReviewRevision()}
	m.saveReviews = nil
	if loadErr != nil {
		m.NoticeStorageError = "Saved notices unavailable; decisions apply only to this session"
	} else if !m.Demo {
		m.saveReviews = save
	}
}

func (m *Session) WithSnoozePersistence(snoozes map[string]time.Time, save func(map[string]time.Time) error, loadErr error) {
	m.Snoozes = make(map[string]time.Time, len(snoozes))
	for id, until := range snoozes {
		m.Snoozes[id] = until
	}
	m.saveSnoozes = nil
	if loadErr != nil {
		m.AlertStorageError = "Saved snoozes unavailable; changes apply only to this session"
	} else if !m.Demo {
		m.saveSnoozes = save
	}
}

// ApplyEvent preserves event timestamps and normalizes through farm.State.
func (m *Session) ApplyEvent(v monitor.Event) error {
	var err error
	switch v.Kind {
	case monitor.EventDevices:
		m.Farm.Discover(v.Devices, v.At)
		m.PrintersDiscovered = true
	case monitor.EventConnection:
		m.Farm.Cloud(v.Connected, v.Problem, v.At)
	case monitor.EventReport:
		err = m.Farm.Report(v.ID, v.Payload, v.At)
	}
	m.refreshTargets()
	return err
}

func (m *Session) Reconcile(at time.Time) { m.Farm.Reconcile(at); m.refreshTargets() }

func (m *Session) refreshTargets() {
	if m.webTargets != nil {
		m.WebDecisions()
	}
}

// PruneSnoozes returns printers whose terminal prompt suppression may be rearmed.
func (m *Session) PruneSnoozes(at time.Time) []string {
	var expired []string
	for id, until := range m.Snoozes {
		_, present := m.Farm.Printers[id]
		if !at.Before(until) || (m.PrintersDiscovered && !present) {
			delete(m.Snoozes, id)
			expired = append(expired, id)
			m.snoozesDirty = true
		}
	}
	return expired
}

func (m *Session) SetSnooze(id string, duration time.Duration, at time.Time) bool {
	if _, exists := m.Farm.Printers[id]; !exists {
		return false
	}
	if m.Snoozes == nil {
		m.Snoozes = map[string]time.Time{}
	}
	if duration > 0 {
		m.Snoozes[id] = at.Add(duration)
	} else {
		delete(m.Snoozes, id)
	}
	m.snoozesDirty = true

	return true
}

func (m *Session) Acknowledge(key string, value bool) bool { return m.Farm.Acknowledge(key, value) }

func (m *Session) Dismiss(key string) { m.Farm.Dismiss(key) }

func (m *Session) DeleteEvent(id uint64) { m.Farm.DeleteEvent(id) }

func (m *Session) RestoreDismissal(key string) bool { return m.Farm.RestoreDismissal(key) }

func (m *Session) ResetReviews() { m.Farm.ResetReviews() }

func (m *Session) MarkUnreviewed(key string) bool { return m.Farm.MarkUnreviewed(key) }

func (m *Session) SetLowFilament(value int, at time.Time) {
	m.Farm.LowFilament = value
	m.Reconcile(at)
}

// Persistence status is returned by value; only the session advances revisions.
func (m Session) ReviewWrites() WriteState { return m.reviewWrites }

func (m Session) SnoozeWrites() WriteState { return m.snoozeWrites }

func (m Session) ConfigWrites() WriteState { return m.configWrites }

func (m Session) SnoozeRevision() uint64 { return m.snoozeRevision }

func (m Session) ConfigRevision() uint64 { return m.configRevision }

func (m *Session) ConfigChanged() { m.configRevision++ }

func (m Session) NoticesWritable() bool { return m.saveReviews != nil }

func (m Session) SnoozesWritable() bool { return m.saveSnoozes != nil }

func (m *Session) WithConfigPersistence(save func(config.Config) error) { m.saveConfig = save }

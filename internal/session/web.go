package session

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type webTarget struct {
	ID        string
	Condition farm.Condition
}

// Publish sends independent allowlisted bytes, never live farm pointers.
func (m *Session) Publish(sink interface{ Publish([]byte) }, at time.Time) {
	sink.Publish(webserver.SnapshotJSON(m.Farm, at, m.Demo, m.WebDecisions()))
}

func sameWebOccurrence(key string, a, b farm.Condition) bool {
	// Match farm.Reconcile: only cloud and print-state text identifies a
	// replacement. Filament estimates can change during an ongoing occurrence.
	textMatches := a.Text == b.Text || (key != farm.ConditionCloud && !strings.HasSuffix(key, "/"+farm.ConditionState))
	return a.Since.Equal(b.Since) && a.Level == b.Level && a.Code == b.Code && textMatches
}

func (m *Session) WebDecisions() webserver.Decisions {
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
	case m.saveReviews == nil && m.NoticeStorageError != "":
		state = "session-only (saved data unavailable)"
	case m.reviewWrites.InFlight:
		state = "saving"
	case m.NoticeStorageError != "":
		state = "failed"
	case m.saveReviews == nil:
		state = "session-only"
	case m.reviewWrites.Saved != m.Farm.ReviewRevision():
		state = "saving"
	}
	return webserver.Decisions{Targets: ids, State: state}
}

func (m *Session) ApplyWebCommand(c webserver.Command, stopping, enabled bool) {
	result := "Stale or unknown notice; refresh before retrying"
	if stopping || !enabled {
		c.Reply <- "Session is stopping or web server disabled"
		return
	}
	for key, target := range m.webTargets {
		current, ok := m.Farm.Active[key]
		if target.ID != c.Target || !ok || !sameWebOccurrence(key, target.Condition, current) {
			continue
		}
		if c.Retry {
			if !current.Dismissed || m.saveReviews == nil || m.reviewWrites.Failed == 0 || m.reviewWrites.InFlight {
				break
			}
			m.reviewWrites.Attempted = 0
		} else if !current.Dismissed {
			m.Farm.Dismiss(key)
		}
		result = "applied"
		break
	}
	c.Reply <- result
}

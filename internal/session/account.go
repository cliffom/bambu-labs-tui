package session

import (
	"time"

	"github.com/cliffom/bambustand/internal/farm"
)

// AccountPersistence is attached only after the host acquires exclusive account
// ownership. Hosts retain lease and shutdown lifecycle; failed loads disable the
// corresponding writer in both terminal and foreground modes.
type AccountPersistence interface {
	LoadReviews() (map[string]farm.Review, error)
	LoadSnoozes() (map[string]time.Time, error)
	SaveReviews(map[string]farm.Review) error
	SaveSnoozes(map[string]time.Time) error
}

func (m *Session) WithAccountPersistence(stores AccountPersistence) []string {
	if m.Demo {
		return nil
	}
	var warnings []string
	reviews, err := stores.LoadReviews()
	m.Farm.RestoreReviews(reviews)
	m.WithNoticePersistence(stores.SaveReviews, err)
	if err != nil {
		warnings = append(warnings, "Notice persistence: saved data unavailable; decisions apply only to this session; repair the notice store and restart")
	}
	snoozes, err := stores.LoadSnoozes()
	m.WithSnoozePersistence(snoozes, stores.SaveSnoozes, err)
	if err != nil {
		warnings = append(warnings, "Alert persistence: saved data unavailable; changes apply only to this session; repair the snooze store and restart")
	}
	return warnings
}

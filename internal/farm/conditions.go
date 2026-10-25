package farm

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Acknowledge marks an ongoing condition as reviewed locally. It never
// edits printer telemetry or sends a printer command. Resolution and recurrence
// create a new, unacknowledged condition.
func (s *State) Acknowledge(key string, acknowledged bool) bool {
	c, ok := s.Active[key]
	if !ok {
		return false
	}
	c.Acknowledged = acknowledged
	s.Active[key] = c
	s.rememberReview(key, c)
	return true
}

// Dismiss hides this occurrence locally, including its matching history entry.
// The underlying telemetry stays intact so recurrence can be detected.
func (s *State) Dismiss(key string) bool {
	c, ok := s.Active[key]
	if !ok {
		return false
	}
	c.Dismissed = true
	s.Active[key] = c
	s.rememberReview(key, c)
	events := s.Events[:0]
	for _, e := range s.Events {
		if e.ConditionKey != key || e.At.Before(c.Since) {
			events = append(events, e)
		}
	}
	s.Events = events
	return true
}

func (s *State) NoticeCount() int {
	n := 0
	for _, c := range s.Active {
		if !c.Dismissed {
			n++
		}
	}
	return n
}

func (s *State) Unacknowledged() int {
	n := 0
	for _, c := range s.Active {
		if !c.Acknowledged && !c.Dismissed {
			n++
		}
	}
	return n
}

func (s *State) Reconcile(now time.Time) {
	next := map[string]Condition{}
	if !s.Connected {
		text := "Cloud disconnected; reconnecting"
		if s.CloudProblem != "" {
			text = s.CloudProblem
		}
		next[ConditionCloud] = Condition{Printer: "Farm", Level: LevelWarning, Text: text}
	}
	for id, p := range s.Printers {
		if !p.Online {
			next[id+"/"+ConditionOffline] = Condition{Printer: p.Name, Level: LevelWarning, Text: "Printer offline"}
		} else if s.Connected && !p.LastReport.IsZero() && p.Stale(now) {
			next[id+"/"+ConditionStale] = Condition{Printer: p.Name, Level: LevelWarning, Text: "Telemetry stale; last report " + p.LastReport.Local().Format(time.TimeOnly)}
		}
		for _, f := range p.Faults {
			level := f.Level
			if level == "" {
				level = LevelWarning
			}
			next[id+"/"+ConditionFaultPrefix+f.Code] = Condition{Printer: p.Name, Level: level, Text: f.Text + " [" + f.Code + "]", Code: f.Code}
		}
		for _, a := range p.AMS {
			for _, slot := range a.Slots {
				if slot.Present != nil && !*slot.Present {
					continue
				}
				if slot.Remaining != nil && *slot.Remaining <= s.LowFilament {
					key := fmt.Sprintf(lowFilamentKeyFormat, id, a.ID, slot.ID)
					next[key] = Condition{Printer: p.Name, Level: LevelWarning, Text: fmt.Sprintf("%s slot %d: estimated filament %d%%", AMSLabel(a.ID), slot.ID+1, *slot.Remaining)}
				}
			}
		}
		if p.State == StatePaused || p.State == StateFailed {
			next[id+"/"+ConditionState] = Condition{Printer: p.Name, Level: LevelWarning, Text: "Print " + label(p.State)}
		}
	}
	keys := make([]string, 0, len(next))
	for k := range next {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Restored reviews must survive discovery, ticks, and partial reports until
	// telemetry has actually supplied the relevant condition family.
	for k := range s.reviews {
		if _, active := next[k]; !active && s.reviewKnown(k) {
			s.forgetReview(k)
		}
	}
	for _, k := range keys {
		c := next[k]
		old, ongoing := s.Active[k]
		// An expired session or failed print is a new condition even if a
		// previous connection problem or pause used the same list identity.
		changed := old.Level != c.Level || old.Code != c.Code || ((k == ConditionCloud || strings.HasSuffix(k, "/"+ConditionState)) && old.Text != c.Text)
		if ongoing && !changed {
			c.Since, c.Acknowledged = old.Since, old.Acknowledged
			c.Dismissed = old.Dismissed
		} else {
			c.Since = now
			if review, ok := s.reviews[k]; ok {
				if review.matches(k, c) {
					c.Since, c.Acknowledged, c.Dismissed = review.Since, review.Acknowledged, review.Dismissed
				} else {
					s.forgetReview(k)
				}
			}
			if !c.Dismissed {
				s.add(Event{At: now, Printer: c.Printer, Level: c.Level, Text: c.Text, Code: c.Code, ConditionKey: k})
			}
		}
		next[k] = c
	}
	keys = keys[:0]
	for k := range s.Active {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := next[k]; !ok {
			c := s.Active[k]
			if !c.Dismissed {
				s.add(Event{At: now, Printer: c.Printer, Level: LevelInfo, Text: "Resolved: " + c.Text, Code: c.Code, ConditionKey: k})
			}
		}
	}
	s.Active = next
}

func label(s string) string {
	switch s {
	case StateRunning:
		return "printing"
	case StatePaused:
		return "paused"
	case StateFinished:
		return "finished"
	case StateFailed:
		return "failed"
	default:
		return strings.ToLower(s)
	}
}

package farm

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const maxProtocolJobIDBytes = 128

type (
	// NotificationSnapshot contains only fresh normalized observations. An omitted
	// printer/family means unknown, never a confirmed incident resolution.
	NotificationSnapshot map[string]NotificationPrinter

	NotificationPrinter struct {
		Job                              NotificationJob
		StateKnown, PrintKnown, HMSKnown bool
		Conditions                       map[string]NotificationCondition
	}

	NotificationJob struct{ Task, Subtask, State string }

	NotificationCondition struct {
		Level, Code, State string
		Since              time.Time
		Family             string
		Eligible, Reviewed bool
	}
)

const (
	NotificationStateFamily = "state"
	NotificationPrintFamily = "print"
	NotificationHMSFamily   = "hms"
)

// NotificationSnapshot reuses Attention classification and reviews without
// changing any notice lifetime or exporting printer/job names.
func (s *State) NotificationSnapshot(now time.Time) NotificationSnapshot {
	out := NotificationSnapshot{}
	eligible := map[string]bool{}
	for _, item := range s.Attention() {
		eligible[item.Key] = true
	}
	for id, p := range s.Printers {
		if p.Connection(now, s.Connected) != ConnectionLive {
			continue
		}
		hms, hmsKnown := p.Raw[fieldHMS].([]any)
		for _, item := range hms {
			m := object(item)
			hmsKnown = hmsKnown && notificationFaultNumber(m[fieldAttr]) && notificationFaultNumber(m[fieldCode])
		}
		n := NotificationPrinter{Job: p.notificationJob, StateKnown: notificationStateKnown(p.State), PrintKnown: notificationFaultNumber(p.Raw[fieldPrintError]), HMSKnown: hmsKnown, Conditions: map[string]NotificationCondition{}}
		for key, c := range s.Active {
			printer, family, ok := strings.Cut(key, "/")
			if !ok || printer != id {
				continue
			}
			nc := NotificationCondition{Level: c.Level, Code: c.Code, Since: c.Since, Eligible: eligible[key], Reviewed: c.Acknowledged || c.Dismissed}
			switch {
			case family == ConditionState:
				nc.Family, nc.State = NotificationStateFamily, p.State
			case strings.HasPrefix(family, ConditionFaultPrefix):
				nc.Family = NotificationPrintFamily
				if strings.Count(c.Code, "_") == HMSCodeParts-1 {
					nc.Family = NotificationHMSFamily
				}
			default:
				continue
			}
			if (nc.Family == NotificationPrintFamily && !n.PrintKnown) || (nc.Family == NotificationHMSFamily && !n.HMSKnown) || (nc.Family == NotificationStateFamily && !n.StateKnown) {
				continue
			}
			n.Conditions[key] = nc
		}
		out[id] = n
	}
	return out
}

func notificationStateKnown(state string) bool {
	switch state {
	case StateRunning, StatePaused, StateFinished, StateFailed, StateIdle, StatePreparing, StateSlicing:
		return true
	}
	return false
}

// A job state is tied to the identity present when that state was explicitly
// reported. An ID-only delta must not inherit the previous job's FINISH state.
func (p *Printer) observeNotificationJob(delta map[string]any) {
	next := p.notificationJob
	if value, supplied := delta[fieldTaskID]; supplied {
		task := protocolJobID(value)
		if task != next.Task {
			next.Subtask = ""
		}
		next.Task = task
	}
	if value, supplied := delta[fieldSubtaskID]; supplied {
		next.Subtask = protocolJobID(value)
	}
	if next.Task != p.notificationJob.Task || next.Subtask != p.notificationJob.Subtask {
		next.State = StateUnknown
	}
	if value, supplied := delta[fieldGcodeState]; supplied {
		state, _ := value.(string)
		next.State = StateUnknown
		if notificationStateKnown(state) {
			next.State = state
		}
	}
	p.notificationJob = next
}

// protocolJobID gives telemetry boundaries and notification evidence the same
// identity interpretation. Zero/empty/invalid values are unknown, not a job ID.
func protocolJobID(value any) string {
	var id string
	switch v := value.(type) {
	case string:
		id = strings.TrimSpace(v)
	case float64:
		if v < 1 || v > 1<<53-1 || math.Trunc(v) != v {
			return ""
		}
		id = strconv.FormatFloat(v, 'f', 0, 64)
	default:
		return ""
	}
	if id == "" || id == "0" || len(id) > maxProtocolJobIDBytes || strings.ContainsFunc(id, unicode.IsControl) {
		return ""
	}
	return id
}

func notificationFaultNumber(value any) bool {
	switch v := value.(type) {
	case float64:
		return v >= 0 && v <= math.MaxUint32 && math.Trunc(v) == v
	case string:
		_, err := strconv.ParseUint(v, 10, 32)
		return err == nil
	}
	return false
}

package notifications

import (
	"errors"
	"io/fs"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cliffom/bambustand/internal/notification_state"
)

// ActivityLimit bounds session-only diagnostic metadata; it never changes the
// persistent delivery ledger or its at-most-once policy.
const (
	ActivityLimit         = 100
	activityDecisionLimit = 4096
)

const (
	OutcomeAccepted     = "Accepted"
	OutcomeFailed       = "Failed"
	OutcomeSuppressed   = "Suppressed"
	OutcomeUnconfirmed  = "Unconfirmed"
	OutcomePaused       = "Paused"
	OutcomeStatus       = "Status"
	ReasonDesktopOff    = "desktop_off"
	ReasonCategoryOff   = "category_off"
	ReasonCompletionOff = "completion_off"
	ReasonQuiet         = "quiet_suppressed"
	ReasonSnoozed       = "snoozed"
	ReasonClosing       = "session_closing"
	ReasonUnavailable   = "telemetry_unavailable"
	ReasonChanged       = "state_changed"
	ReasonAccepted      = "macos_accepted"
	ReasonCanceled      = "delivery_interrupted"
	ReasonHelper        = "helper_unavailable"
	ReasonPermission    = "permission_denied"
	ReasonTimeout       = "helper_timeout"
	ReasonBackend       = "backend_failed"
	ReasonQueue         = "queue_full"
	ReasonStorageBusy   = "storage_busy"
	ReasonStorageFull   = "storage_full"
	ReasonStorage       = "storage_unavailable"
	ReasonReservation   = "reservation_claimed"
)

type Activity struct {
	ID                                                    uint64
	FirstAt, At                                           time.Time
	PrinterID, Printer, Subject, Channel, Outcome, Reason string
	Repeats                                               uint64
	Request                                               uint64
	GroupSize                                             int
	key                                                   string
}

// ReasonText uses allowlisted explanations, never backend stderr, paths, raw
// telemetry, credentials, or protocol job identifiers.
func ReasonText(reason string) string {
	switch reason {
	case ReasonQuiet:
		return "Quiet hours: desktop banner and sound suppressed permanently for this occurrence; no replay or summary."
	case ReasonOwner:
		return "This TUI owns automatic delivery for this account and region."
	case ReasonStandby:
		return "Another TUI owns delivery. Monitoring continues; takeover follows owner exit."
	case ReasonObserver:
		return "Desktop delivery is off; this TUI does not retain delivery ownership."
	case ReasonRetry:
		return "Checking existing state again; prior attempts will not be resent."
	case ReasonPaused:
		return "Worker paused before submission; reservation kept. No automatic retry."
	case ReasonDesktopOff:
		return "Desktop notifications are off for this printer."
	case ReasonCategoryOff:
		return "This incident's alert category is off."
	case ReasonCompletionOff:
		return "Completed-print notifications are off."
	case ReasonSnoozed:
		return "This printer is snoozed."
	case ReasonClosing:
		return "The TUI session is closing or paused."
	case ReasonUnavailable:
		return "Printer telemetry is unavailable or stale; no current delivery."
	case ReasonChanged:
		return "The reported condition, job, or delivery policy changed before delivery."
	case ReasonAccepted:
		return "macOS accepted the request; banner and sound are unconfirmed."
	case ReasonCanceled:
		return "Delivery was interrupted; outcome unknown. No automatic retry."
	case ReasonHelper:
		return "Notification helper missing or unavailable; run doctor --notify."
	case ReasonPermission:
		return "macOS denied permission; enable BambuStand notifications."
	case ReasonTimeout:
		return "The notification helper timed out. No automatic retry."
	case ReasonBackend:
		return "macOS submission failed; run doctor --notify. No automatic retry."
	case ReasonQueue:
		return "Update queue full; delivery paused. Press r in activity to retry after the queue drains."
	case ReasonStorageBusy:
		return "Identity storage busy; delivery paused. Press r after competing access ends."
	case ReasonStorageFull:
		return "Live identity limit. Stop hosts; see alerts guide for explicit job compaction."
	case ReasonStorage:
		return "Identity storage unavailable; delivery paused. Fix storage, then press r in activity."
	case ReasonReservation:
		return "Already claimed by this or another session; no duplicate request."
	case notification_state.SuppressedReviewed:
		return "This condition is acknowledged or dismissed locally."
	case notification_state.SuppressedNotActionable:
		return "This reported condition is not eligible for intervention alerts."
	case notification_state.SuppressedAccepted:
		return "This occurrence was already accepted; no duplicate request."
	case notification_state.SuppressedFailed:
		return "This occurrence already failed; no automatic retry."
	case notification_state.SuppressedReserved:
		return "This occurrence was already reserved; prior outcome is unconfirmed."
	case notification_state.SuppressedBaseline:
		return "Completion baseline or reused/closed job; no new confirmed transition."
	case notification_state.SuppressedJobIdentity:
		return "Completion has no trustworthy job identity; no guessed notification."
	default:
		return "No delivery decision is available."
	}
}

func storageReason(err error) string {
	switch {
	case errors.Is(err, notification_state.ErrBusy):
		return ReasonStorageBusy
	case errors.Is(err, notification_state.ErrCapacity):
		return ReasonStorageFull
	default:
		return ReasonStorage
	}
}

func deliveryReason(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return ReasonHelper
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitCode() {
		case helperPermissionDeniedExit:
			return ReasonPermission
		case helperTimeoutExit:
			return ReasonTimeout
		}
	}
	return ReasonBackend
}

// appendActivityLocked returns true only for a new retained decision. Repeated
// reports update its count/time in place without moving selection or toasting.
func (w *Worker) appendActivityLocked(entry Activity, key string) bool {
	now := w.now()
	identity, _, _ := strings.Cut(key, "/")
	if w.activitySeen == nil {
		w.activitySeen = map[string]string{}
	}
	if previousKey, seen := w.activitySeen[identity]; seen && previousKey == key {
		for i := range w.status.History {
			previous := &w.status.History[i]
			if previous.key == key {
				previous.At, previous.Repeats = now, previous.Repeats+1
				break
			}
		}
		return false
	}
	if _, seen := w.activitySeen[identity]; !seen {
		if len(w.activitySeenOrder) == activityDecisionLimit {
			delete(w.activitySeen, w.activitySeenOrder[0])
			copy(w.activitySeenOrder, w.activitySeenOrder[1:])
			w.activitySeenOrder = w.activitySeenOrder[:activityDecisionLimit-1]
		}
		w.activitySeenOrder = append(w.activitySeenOrder, identity)
	}
	w.activitySeen[identity] = key
	w.activitySequence++
	entry.ID, entry.FirstAt, entry.At, entry.Repeats, entry.key = w.activitySequence, now, now, 1, key
	entry.Channel = "macOS desktop"
	entry.PrinterID = cleanText(entry.PrinterID, maxNotificationTitleRunes)
	entry.Printer = cleanTitle(entry.Printer)
	entry.Subject = cleanText(entry.Subject, maxNotificationTitleRunes)
	if len(w.status.History) == ActivityLimit {
		copy(w.status.History, w.status.History[1:])
		w.status.History = w.status.History[:ActivityLimit-1]
	}
	w.status.History = append(w.status.History, entry)
	w.notifyChangedLocked()
	return true
}

func activityFor(candidate notification_state.Candidate, source Input, outcome, reason string) Activity {
	policy := source.Policies[candidate.PrinterID]
	printer := policy.PrinterName
	if printer == "" {
		printer = candidate.PrinterID
	}
	subject := "Print completion"
	if candidate.Kind == notification_state.Intervention {
		condition := source.Snapshot[candidate.PrinterID].Conditions[candidate.ConditionKey]
		subject = "Printer condition"
		if condition.Code != "" {
			subject = "Fault " + condition.Code
		} else if condition.State != "" {
			subject = "Print " + strings.ToLower(condition.State)
		}
	}
	return Activity{PrinterID: candidate.PrinterID, Printer: printer, Subject: subject, Outcome: outcome, Reason: reason}
}

func (w *Worker) recordSuppression(candidate notification_state.Candidate, source Input, reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entry := activityFor(candidate, source, OutcomeSuppressed, reason)
	if w.appendActivityLocked(entry, candidate.ID+"/"+OutcomeSuppressed+"/"+reason) {
		w.status.Suppressed++
		if !w.status.Disabled {
			w.status.Revision++
			w.status.Last = "Desktop notification suppressed: " + ReasonText(reason)
			w.status.LastKind = MessageInfo
		}
	}
}

func (w *Worker) recordGroup(group []notification_state.Candidate, source Input, outcome, reason string, request uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, candidate := range group {
		entry := activityFor(candidate, source, outcome, reason)
		if request != 0 {
			entry.Request, entry.GroupSize = request, len(group)
		}
		w.appendActivityLocked(entry, candidate.ID+"/"+outcome+"/"+reason+"/"+strconv.FormatUint(request, 10))
	}
}

// Quiet consumption is reported only after its ledger result is saved. This
// policy-only explanation cannot establish that an occurrence was consumed.
func suppressionReason(candidate notification_state.Candidate, current Input) string {
	if !current.Active {
		return ReasonClosing
	}
	if current.ObserveOnly {
		return ReasonDesktopOff
	}
	printer, live := current.Snapshot[candidate.PrinterID]
	if !live {
		return ReasonUnavailable
	}
	policy := current.Policies[candidate.PrinterID]
	if policy.Snoozed {
		return ReasonSnoozed
	}
	if !policy.Desktop {
		return ReasonDesktopOff
	}
	if candidate.Kind == notification_state.Completion {
		if !policy.Completion {
			return ReasonCompletionOff
		}
		return ReasonChanged
	}
	condition, present := printer.Conditions[candidate.ConditionKey]
	if !present {
		return ReasonChanged
	}
	if condition.Reviewed {
		return notification_state.SuppressedReviewed
	}
	if !condition.Eligible {
		return notification_state.SuppressedNotActionable
	}
	if !policy.Allowed[candidate.ConditionKey] {
		return ReasonCategoryOff
	}
	return ReasonChanged
}

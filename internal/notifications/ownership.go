package notifications

import (
	"errors"
	"time"

	"github.com/cliffom/bambu-tui/internal/notificationstate"
)

const (
	OwnershipIdle         = "idle"
	OwnershipOwner        = "owner"
	OwnershipStandby      = "standby"
	OwnershipObserver     = "observer"
	ownershipPollInterval = time.Second
	ReasonOwner           = "delivery_owner"
	ReasonStandby         = "delivery_standby"
	ReasonObserver        = "identity_observer"
	ReasonRetry           = "recovery_requested"
	ReasonPaused          = "worker_paused"
)

func OwnershipText(status Status) string {
	if status.Disabled {
		if status.Pending {
			return "Paused; stopping current work"
		}
		return "Paused; r retries after fixing the cause"
	}
	if status.Recovering {
		return "Checking existing identity state"
	}
	if status.Ownership == "" && status.Pending {
		return "Preparing notification ownership"
	}
	switch status.Ownership {
	case OwnershipOwner:
		return "This TUI owns desktop delivery"
	case OwnershipStandby:
		return "Another TUI owns desktop delivery"
	case OwnershipObserver:
		return "Desktop off; no delivery ownership"
	default:
		return "Idle; ownership begins after desktop opt-in"
	}
}

// Retry reopens a paused worker or requests an immediate standby check. The next
// Submit must provide a fresh UI snapshot; cached frames are never replayed.
// Accepted, failed, and reserved ledger attempts remain untouched.
func (w *Worker) Retry() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.ctx.Err() != nil || w.working || w.latest.sequence == 0 || (!w.status.Disabled && w.status.Ownership != OwnershipStandby) {
		return false
	}
	w.status.Disabled, w.status.Error = false, ""
	w.status.Recovering = w.status.Ownership != OwnershipStandby
	w.forceFrame, w.awaitFresh = true, true
	// Discard only paused/standby frames. They are not authoritative owner
	// lifecycle observations, and a resumed owner must use fresh state.
	draining := true
	for draining {
		select {
		case <-w.queue:
		default:
			draining = false
		}
	}
	w.status.Revision++
	w.status.Last = "Notification recovery requested; checking existing identities"
	w.appendActivityLocked(Activity{Printer: "Session", Subject: "Delivery worker", Outcome: OutcomeStatus, Reason: ReasonRetry}, "worker/"+ReasonRetry)
	return true
}

// requestOwnershipCheck schedules a new snapshot through the live UI. It does
// not read disk, take a lease, or deliver from an old cached telemetry frame.
func (w *Worker) requestOwnershipCheck() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.status.Disabled && w.status.Ownership == OwnershipStandby && !w.latest.ObserveOnly && w.latest.Active {
		w.forceFrame = true
	}
}

func (w *Worker) acquireOwner(input frame) bool {
	w.mu.Lock()
	if w.owner != nil {
		w.mu.Unlock()
		return true
	}
	current := w.latest
	w.mu.Unlock()
	// A takeover starts only from the newest incoming snapshot. The established
	// owner still records every queued lifecycle frame while rechecking delivery.
	if input.sequence != current.sequence || !input.Active {
		return false
	}
	owner, err := w.ledger.AcquireOwner()
	if err != nil {
		if errors.Is(err, notificationstate.ErrBusy) {
			w.mu.Lock()
			role, reason := OwnershipStandby, ReasonStandby
			if input.ObserveOnly {
				role, reason = OwnershipObserver, ReasonObserver
			}
			changed := w.status.Ownership != role
			w.status.Ownership, w.status.Recovering = role, false
			if changed {
				w.status.Revision++
				w.status.Last = OwnershipText(w.status)
				w.appendActivityLocked(Activity{Printer: "Session", Subject: "Notification ownership", Outcome: OutcomeStatus, Reason: reason}, "ownership/"+reason)
			}
			w.mu.Unlock()
		} else {
			w.disable("Desktop notifications paused: "+ReasonText(storageReason(err)), storageReason(err))
		}
		return false
	}
	w.mu.Lock()
	w.owner = owner
	role, reason := OwnershipOwner, ReasonOwner
	if input.ObserveOnly {
		role, reason = OwnershipObserver, ReasonObserver
	}
	changed := w.status.Ownership != role
	w.status.Ownership = role
	if changed {
		w.status.Revision++
		w.status.Last = OwnershipText(w.status)
		w.appendActivityLocked(Activity{Printer: "Session", Subject: "Notification ownership", Outcome: OutcomeStatus, Reason: reason}, "ownership/"+reason)
	}
	w.mu.Unlock()
	return true
}

func (w *Worker) releaseOwner() {
	w.mu.Lock()
	owner := w.owner
	w.owner = nil
	if w.status.Ownership == OwnershipOwner {
		w.status.Ownership = OwnershipIdle
	}
	w.mu.Unlock()
	if owner != nil {
		_ = owner.Close()
	}
}

func (w *Worker) releaseInactiveOwner() {
	w.mu.Lock()
	release := w.status.Disabled || w.latest.ObserveOnly
	if w.latest.ObserveOnly && !w.status.Disabled {
		w.status.Ownership = OwnershipObserver
	}
	w.mu.Unlock()
	if release {
		w.releaseOwner()
	}
}

// State transitions wake bounded observers without blocking worker or reducer.
func (w *Worker) notifyChangedLocked() {
	select {
	case w.changed <- struct{}{}:
	default:
	}
}

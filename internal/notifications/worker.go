package notifications

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notification_state"
)

const workerQueueCapacity = 256

const (
	deliveryRateWindow = time.Minute
	deliveryRateLimit  = 12
)

type (
	Ledger interface {
		AcquireOwner() (*notification_state.Owner, error)
		ObserveActivity(farm.NotificationSnapshot) (notification_state.Observation, error)
		ReserveBatch([]string) ([]string, error)
		RecordResult(string, notification_state.Attempt) error
	}
	Policy struct {
		PrinterName                         string
		Desktop, Completion, Sound, Snoozed bool
		Allowed                             map[string]bool
	}
	Input struct {
		Snapshot     farm.NotificationSnapshot
		Policies     map[string]Policy
		Active, Demo bool
		Quiet        bool
		BurstWindow  time.Duration
		ObserveOnly  bool
	}
	Status struct {
		Revision, Accepted, Failed, Suppressed uint64
		Last, Error                            string
		LastKind                               MessageKind
		Disabled, Pending                      bool
		History                                []Activity
		Ownership                              string
		Recovering                             bool
	}
	frame struct {
		sequence uint64
		Input
	}
	Message struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Sound bool   `json:"sound"`
	}
	Worker struct {
		mu                sync.Mutex
		latest            frame
		status            Status
		stopped           bool
		working           bool
		flight            []notification_state.Candidate
		flightSource      Input
		cancelFlight      context.CancelFunc
		queue             chan frame
		done              chan struct{}
		ctx               context.Context
		cancel            context.CancelFunc
		ledger            Ledger
		send              func(context.Context, Message) error
		now               func() time.Time
		activitySequence  uint64
		requestSequence   uint64
		activitySeen      map[string]string
		activitySeenOrder []string
		owner             *notification_state.Owner
		previousOwner     *notification_state.Owner
		observationGap    bool
		forceFrame        bool
		awaitFresh        bool
		changed           chan struct{}
		batchDue          time.Time
		nextDelivery      time.Time
		deliveryTimes     []time.Time
		scheduled         bool
	}
)

func NewWorker(ctx context.Context, ledger Ledger, send func(context.Context, Message) error) *Worker {
	return newWorker(ctx, ledger, send, time.Now)
}

func newWorker(ctx context.Context, ledger Ledger, send func(context.Context, Message) error, now func() time.Time) *Worker {
	ctx, cancel := context.WithCancel(ctx)
	w := &Worker{ctx: ctx, cancel: cancel, ledger: ledger, send: send, queue: make(chan frame, workerQueueCapacity), done: make(chan struct{}), now: now, changed: make(chan struct{}, 1)}
	go w.run()
	return w
}

// Submit never waits on disk, delivery, or queue space. Overflow disables the
// worker rather than dropping lifecycle transitions and risking duplicate IDs.
func (w *Worker) Submit(input Input) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.ctx.Err() != nil {
		return
	}
	if !w.forceFrame && reflect.DeepEqual(w.latest.Input, input) {
		return
	}
	w.forceFrame = false
	w.awaitFresh = false
	w.latest = frame{sequence: w.latest.sequence + 1, Input: input}
	if w.status.Disabled {
		return
	}
	for _, candidate := range w.flight {
		if !eligible(candidate, w.flightSource, input) {
			w.cancelFlight()
			break
		}
	}
	select {
	case w.queue <- w.latest:
	default:
		w.status.Disabled = true
		w.status.Recovering = false
		w.status.Error = "Desktop notifications paused: " + ReasonText(ReasonQueue)
		w.status.Last = w.status.Error
		w.status.LastKind = MessageError
		w.status.Revision++
		w.appendActivityLocked(Activity{Printer: "Session", Subject: "Delivery worker", Outcome: OutcomePaused, Reason: ReasonQueue}, "worker/queue")
		if w.cancelFlight != nil {
			w.cancelFlight()
		}
	}
}

func (w *Worker) Stop()                 { w.mu.Lock(); w.stopped = true; w.cancel(); w.mu.Unlock() }
func (w *Worker) Done() <-chan struct{} { return w.done }
func (w *Worker) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	status := w.status
	status.History = slices.Clone(w.status.History)
	status.Pending = w.working || (w.ctx.Err() == nil && !w.status.Disabled && len(w.queue) > 0)
	return status
}

func (w *Worker) report(kind MessageKind, message, problem string, accepted, failed, suppressed bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.status.Disabled {
		w.status.Revision++
		w.status.Last = message
		w.status.LastKind = kind
		if problem != "" || accepted {
			w.status.Error = problem
		}
	}
	if accepted {
		w.status.Accepted++
	}
	if failed {
		w.status.Failed++
	}
	if suppressed {
		w.status.Suppressed++
	}
}

func (w *Worker) disable(message, reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status.Disabled, w.status.Last, w.status.Error = true, message, message
	w.status.LastKind = MessageError
	w.status.Recovering = false
	w.status.Revision++
	w.appendActivityLocked(Activity{Printer: "Session", Subject: "Delivery worker", Outcome: OutcomePaused, Reason: reason}, "worker/"+reason)
	if w.cancelFlight != nil {
		w.cancelFlight()
	}
}

func (w *Worker) run() {
	defer close(w.done)
	defer w.releaseOwner()
	poll := time.NewTicker(ownershipPollInterval)
	defer poll.Stop()
	defer func() { w.mu.Lock(); w.working = false; w.mu.Unlock() }()
	defer w.cancel()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-poll.C:
			w.requestOwnershipCheck()
		case input := <-w.queue:
			if w.ctx.Err() != nil {
				return
			}
			w.mu.Lock()
			if w.status.Disabled || w.awaitFresh {
				w.mu.Unlock()
				// Overflow can pause an idle owner between process calls. The
				// serialized loop releases its lease even when every queued frame
				// is skipped; any in-flight sender has already returned here.
				w.releaseInactiveOwner()
				w.mu.Lock()
				w.notifyChangedLocked()
				w.mu.Unlock()
				continue
			}
			w.working = true
			w.mu.Unlock()
			w.process(input)
			w.mu.Lock()
			w.working = false
			w.notifyChangedLocked()
			w.mu.Unlock()
		}
	}
}

func (w *Worker) process(input frame) bool {
	defer w.releaseInactiveOwner()
	if !w.acquireOwner(input) {
		w.mu.Lock()
		paused := w.status.Disabled
		w.mu.Unlock()
		return !paused
	}
	observation, err := w.ledger.ObserveActivity(input.Snapshot)
	if err != nil {
		w.disable("Desktop notifications paused: "+ReasonText(storageReason(err)), storageReason(err))
		return false
	}
	w.mu.Lock()
	paused := w.status.Disabled
	w.status.Recovering = false
	w.mu.Unlock()
	if paused {
		return false
	}
	for _, decision := range observation.Suppressed {
		w.recordSuppression(decision.Candidate, input.Input, decision.Reason)
	}
	// Every lifecycle frame above is retained; only the newest policy can deliver.
	w.mu.Lock()
	currentFrame := w.latest
	w.mu.Unlock()
	var candidates []notification_state.Candidate
	for _, candidate := range observation.Candidates {
		policy := currentFrame.Input
		if input.Quiet {
			policy = input.Input
		}
		policy.Quiet = false
		if (input.Quiet || currentFrame.Quiet) && eligible(candidate, input.Input, policy) {
			// Persist consumption before leaving quiet hours, including across restart.
			reserved, reserveErr := w.ledger.ReserveBatch([]string{candidate.ID})
			if reserveErr != nil {
				w.disable("Desktop notifications paused: "+ReasonText(storageReason(reserveErr)), storageReason(reserveErr))
				return false
			}
			for _, id := range reserved {
				if saveErr := w.ledger.RecordResult(id, notification_state.QuietSuppressed); saveErr != nil {
					w.disable("Desktop notifications paused: "+ReasonText(storageReason(saveErr)), storageReason(saveErr))
					return false
				}
			}
			w.recordSuppression(candidate, input.Input, ReasonQuiet)
		} else if eligible(candidate, input.Input, currentFrame.Input) {
			candidates = append(candidates, candidate)
		} else {
			w.recordSuppression(candidate, input.Input, suppressionReason(candidate, currentFrame.Input))
		}
	}
	if currentFrame.sequence != input.sequence {
		return true
	}
	w.mu.Lock()
	w.scheduled = len(candidates) > 0
	w.mu.Unlock()
	if len(candidates) == 0 {
		w.batchDue = time.Time{}
		return true
	}
	now := w.now()
	if w.batchDue.IsZero() {
		w.batchDue = now.Add(input.BurstWindow)
	}
	if now.Before(w.batchDue) || now.Before(w.nextDelivery) {
		return true
	}
	w.batchDue = time.Time{}
	groups := deliveryGroups(candidates)
	if input.BurstWindow > 0 {
		groups = [][]notification_state.Candidate{candidates}
	}
	for _, group := range groups {
		if w.ctx.Err() != nil {
			return false
		}
		w.mu.Lock()
		current := w.latest
		w.mu.Unlock()
		if current.sequence != input.sequence {
			return true
		}
		var allowed []notification_state.Candidate
		for _, candidate := range group {
			if eligible(candidate, input.Input, current.Input) {
				allowed = append(allowed, candidate)
			} else {
				reason := suppressionReason(candidate, current.Input)
				w.recordSuppression(candidate, input.Input, reason)
			}
		}
		if len(allowed) == 0 {
			continue
		}
		ids := make([]string, 0, len(allowed))
		for _, candidate := range allowed {
			ids = append(ids, candidate.ID)
		}
		reserved, err := w.ledger.ReserveBatch(ids)
		if err != nil {
			w.disable("Desktop notifications paused: "+ReasonText(storageReason(err)), storageReason(err))
			return false
		}
		claimed := map[string]bool{}
		for _, id := range reserved {
			claimed[id] = true
		}
		group = nil
		for _, candidate := range allowed {
			if claimed[candidate.ID] {
				group = append(group, candidate)
			} else {
				w.recordSuppression(candidate, input.Input, ReasonReservation)
			}
		}
		if len(group) == 0 {
			continue
		}
		w.mu.Lock()
		current = w.latest
		closing := w.stopped || w.ctx.Err() != nil
		paused = w.status.Disabled
		valid := !closing && !paused
		for _, candidate := range group {
			valid = valid && eligible(candidate, input.Input, current.Input)
		}
		if !valid {
			w.mu.Unlock()
			for _, candidate := range group {
				reason := suppressionReason(candidate, current.Input)
				if closing {
					reason = ReasonClosing
				} else if paused {
					reason = ReasonPaused
				}
				w.recordSuppression(candidate, input.Input, reason)
			}
			continue
		}
		delivery, cancel := context.WithCancel(w.ctx)
		w.flight, w.flightSource, w.cancelFlight = group, input.Input, cancel
		w.requestSequence++
		request := w.requestSequence
		w.mu.Unlock()
		first := group[0]
		policy := current.Policies[first.PrinterID]
		title := policy.PrinterName
		body := policy.PrinterName + " needs attention. Open Bambu TUI to inspect the reported condition."
		if len(group) > 1 {
			body = fmt.Sprintf("%s needs attention (%d notices). Open Bambu TUI to inspect them.", policy.PrinterName, len(group))
		}
		if first.Kind == notification_state.Completion {
			body = policy.PrinterName + " reports that the print finished."
		}
		if input.BurstWindow > 0 && len(group) > 1 {
			printers := map[string]bool{}
			policy.Sound = false
			for _, member := range group {
				printers[member.PrinterID] = true
				policy.Sound = policy.Sound || current.Policies[member.PrinterID].Sound
			}
			title = "Bambu TUI"
			body = fmt.Sprintf("%d events across %d printers. Open Notification activity in Bambu TUI for individual conditions and completions.", len(group), len(printers))
		}
		// At most twelve OS submissions in any rolling minute, including
		// failures and ambiguous outcomes. The bounded timestamps are session-only.
		sentAt := w.now()
		w.deliveryTimes = slices.DeleteFunc(w.deliveryTimes, func(at time.Time) bool { return !sentAt.Before(at.Add(deliveryRateWindow)) })
		w.deliveryTimes = append(w.deliveryTimes, sentAt)
		w.nextDelivery = time.Time{}
		if len(w.deliveryTimes) >= deliveryRateLimit {
			w.nextDelivery = w.deliveryTimes[0].Add(deliveryRateWindow)
		}
		if current.Demo {
			title = "Demo: " + title
			body = "Demo: " + body
		}
		err = w.send(delivery, Message{Title: title, Body: body, Sound: policy.Sound})
		cancel()
		w.mu.Lock()
		w.flight, w.cancelFlight = nil, nil
		w.mu.Unlock()
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || w.ctx.Err() != nil) {
			reason := ReasonCanceled
			if errors.Is(err, context.DeadlineExceeded) {
				reason = ReasonTimeout
			}
			w.recordGroup(group, input.Input, OutcomeUnconfirmed, reason, request)
			w.report(MessageWarning, "Desktop notification outcome unconfirmed", "Desktop notification outcome unconfirmed; attempt will not be retried", false, false, false)
			return true
		}
		outcome := notification_state.Accepted
		if err != nil {
			outcome = notification_state.Failed
		}
		if err != nil {
			w.recordGroup(group, input.Input, OutcomeFailed, deliveryReason(err), request)
			w.report(MessageError, "Desktop notification failed", "Desktop notification failed; run doctor --notify", false, true, false)
		} else {
			w.recordGroup(group, input.Input, OutcomeAccepted, ReasonAccepted, request)
			w.report(MessageSuccess, "Desktop notification accepted by macOS (display unconfirmed)", "", true, false, false)
		}
		for _, candidate := range group {
			if saveErr := w.ledger.RecordResult(candidate.ID, outcome); saveErr != nil {
				w.disable("Desktop notifications paused: outcome storage failed. "+ReasonText(storageReason(saveErr)), storageReason(saveErr))
				return false
			}
		}
		return true
	}
	return true
}

// Group only co-reported intervention occurrences on the same printer. Keep
// notice identities independent, and never merge printers or job completion.
func deliveryGroups(candidates []notification_state.Candidate) [][]notification_state.Candidate {
	var groups [][]notification_state.Candidate
	printers := map[string]int{}
	for _, candidate := range candidates {
		if candidate.Kind == notification_state.Intervention {
			if index, present := printers[candidate.PrinterID]; present {
				groups[index] = append(groups[index], candidate)
				continue
			}
			printers[candidate.PrinterID] = len(groups)
		}
		groups = append(groups, []notification_state.Candidate{candidate})
	}
	return groups
}

func eligible(candidate notification_state.Candidate, source, current Input) bool {
	if !current.Active || current.ObserveOnly || current.Quiet {
		return false
	}
	p, live := current.Snapshot[candidate.PrinterID]
	before := source.Snapshot[candidate.PrinterID]
	policy := current.Policies[candidate.PrinterID]
	if !live || !policy.Desktop || policy.Snoozed {
		return false
	}
	if candidate.Kind == notification_state.Completion {
		return policy.Completion && p.Job == before.Job && p.Job.State == farm.StateFinished
	}
	condition, present := p.Conditions[candidate.ConditionKey]
	return present && condition.Eligible && condition == before.Conditions[candidate.ConditionKey] && policy.Allowed[candidate.ConditionKey]
}

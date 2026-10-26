package notifications

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notificationstate"
)

const workerQueueCapacity = 256

type (
	Ledger interface {
		Observe(farm.NotificationSnapshot) ([]notificationstate.Candidate, error)
		ReserveBatch([]string) ([]string, error)
		RecordResult(string, notificationstate.Attempt) error
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
	}
	Status struct {
		Revision, Accepted, Failed, Suppressed uint64
		Last, Error                            string
		Disabled, Pending                      bool
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
		mu           sync.Mutex
		latest       frame
		status       Status
		stopped      bool
		working      bool
		flight       []notificationstate.Candidate
		flightSource Input
		cancelFlight context.CancelFunc
		queue        chan frame
		done         chan struct{}
		ctx          context.Context
		cancel       context.CancelFunc
		ledger       Ledger
		send         func(context.Context, Message) error
		suppressed   map[string]bool
	}
)

func NewWorker(ctx context.Context, ledger Ledger, send func(context.Context, Message) error) *Worker {
	ctx, cancel := context.WithCancel(ctx)
	w := &Worker{ctx: ctx, cancel: cancel, ledger: ledger, send: send, queue: make(chan frame, workerQueueCapacity), done: make(chan struct{}), suppressed: map[string]bool{}}
	go w.run()
	return w
}

// Submit never waits on disk, delivery, or queue space. Overflow disables the
// worker rather than dropping lifecycle transitions and risking duplicate IDs.
func (w *Worker) Submit(input Input) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.status.Disabled || reflect.DeepEqual(w.latest.Input, input) {
		return
	}
	w.latest = frame{sequence: w.latest.sequence + 1, Input: input}
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
		w.status.Error = "Desktop notifications paused: update queue full; restart to retry"
		w.status.Last = w.status.Error
		w.status.Revision++
		w.cancel()
	}
}

func (w *Worker) Stop()                 { w.mu.Lock(); w.stopped = true; w.cancel(); w.mu.Unlock() }
func (w *Worker) Done() <-chan struct{} { return w.done }
func (w *Worker) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	status := w.status
	status.Pending = w.working || (w.ctx.Err() == nil && len(w.queue) > 0)
	return status
}

func (w *Worker) report(message, problem string, accepted, failed, suppressed bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status.Disabled {
		return
	}
	w.status.Revision++
	w.status.Last = message
	if problem != "" || accepted {
		w.status.Error = problem
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

func (w *Worker) disable(message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status.Disabled, w.status.Last, w.status.Error = true, message, message
	w.status.Revision++
	w.cancel()
}

func (w *Worker) run() {
	defer close(w.done)
	defer func() { w.mu.Lock(); w.working = false; w.mu.Unlock() }()
	defer w.cancel()
	for {
		select {
		case <-w.ctx.Done():
			return
		case input := <-w.queue:
			if w.ctx.Err() != nil {
				return
			}
			w.mu.Lock()
			w.working = true
			w.mu.Unlock()
			keepRunning := w.process(input)
			w.mu.Lock()
			w.working = false
			w.mu.Unlock()
			if !keepRunning {
				return
			}
		}
	}
}

func (w *Worker) process(input frame) bool {
	candidates, err := w.ledger.Observe(input.Snapshot)
	if err != nil {
		w.disable("Desktop notifications paused: identity storage unavailable; restart to retry")
		return false
	}
	for _, group := range deliveryGroups(candidates) {
		if w.ctx.Err() != nil {
			return false
		}
		w.mu.Lock()
		current := w.latest
		w.mu.Unlock()
		if current.sequence != input.sequence {
			return true
		}
		var allowed []notificationstate.Candidate
		for _, candidate := range group {
			if eligible(candidate, input.Input, current.Input) {
				allowed = append(allowed, candidate)
			} else {
				w.reportSuppressed(candidate, current.Input)
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
			w.disable("Desktop notifications paused: reservation failed; restart to retry")
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
			}
		}
		if len(group) == 0 {
			continue
		}
		w.mu.Lock()
		current = w.latest
		valid := !w.stopped && w.ctx.Err() == nil
		for _, candidate := range group {
			valid = valid && eligible(candidate, input.Input, current.Input)
		}
		if !valid {
			w.mu.Unlock()
			w.report("Desktop notification suppressed: state changed before delivery", "", false, false, true)
			continue
		}
		delivery, cancel := context.WithCancel(w.ctx)
		w.flight, w.flightSource, w.cancelFlight = group, input.Input, cancel
		w.mu.Unlock()
		first := group[0]
		policy := current.Policies[first.PrinterID]
		title := policy.PrinterName
		body := policy.PrinterName + " needs attention. Open Bambu TUI to inspect the reported condition."
		if len(group) > 1 {
			body = fmt.Sprintf("%s needs attention (%d notices). Open Bambu TUI to inspect them.", policy.PrinterName, len(group))
		}
		if first.Kind == notificationstate.Completion {
			body = policy.PrinterName + " reports that the print finished."
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
			w.report("Desktop notification outcome unconfirmed", "Desktop notification outcome unconfirmed; attempt will not be retried", false, false, false)
			continue
		}
		outcome := notificationstate.Accepted
		if err != nil {
			outcome = notificationstate.Failed
		}
		for _, candidate := range group {
			if saveErr := w.ledger.RecordResult(candidate.ID, outcome); saveErr != nil {
				w.disable("Desktop notifications paused: outcome storage failed; restart to retry")
				return false
			}
		}
		if err != nil {
			w.report("Desktop notification failed", "Desktop notification failed; run doctor --notify", false, true, false)
		} else {
			w.report("Desktop notification accepted by macOS (display unconfirmed)", "", true, false, false)
		}
	}
	return true
}

func (w *Worker) reportSuppressed(candidate notificationstate.Candidate, current Input) {
	if w.suppressed[candidate.ID] {
		return
	}
	if len(w.suppressed) >= workerQueueCapacity {
		clear(w.suppressed)
	}
	w.suppressed[candidate.ID] = true
	reason := "Desktop notification suppressed: preferences"
	if current.Policies[candidate.PrinterID].Snoozed {
		reason = "Desktop notification suppressed: snoozed"
	}
	if !current.Active {
		reason = "Desktop notification suppressed: session closing"
	}
	w.report(reason, "", false, false, true)
}

// Group only co-reported intervention occurrences on the same printer. Keep
// notice identities independent, and never merge printers or job completion.
func deliveryGroups(candidates []notificationstate.Candidate) [][]notificationstate.Candidate {
	var groups [][]notificationstate.Candidate
	printers := map[string]int{}
	for _, candidate := range candidates {
		if candidate.Kind == notificationstate.Intervention {
			if index, present := printers[candidate.PrinterID]; present {
				groups[index] = append(groups[index], candidate)
				continue
			}
			printers[candidate.PrinterID] = len(groups)
		}
		groups = append(groups, []notificationstate.Candidate{candidate})
	}
	return groups
}

func eligible(candidate notificationstate.Candidate, source, current Input) bool {
	if !current.Active {
		return false
	}
	p, live := current.Snapshot[candidate.PrinterID]
	before := source.Snapshot[candidate.PrinterID]
	policy := current.Policies[candidate.PrinterID]
	if !live || !policy.Desktop || policy.Snoozed {
		return false
	}
	if candidate.Kind == notificationstate.Completion {
		return policy.Completion && p.Job == before.Job && p.Job.State == farm.StateFinished
	}
	condition, present := p.Conditions[candidate.ConditionKey]
	return present && condition.Eligible && condition == before.Conditions[candidate.ConditionKey] && policy.Allowed[candidate.ConditionKey]
}

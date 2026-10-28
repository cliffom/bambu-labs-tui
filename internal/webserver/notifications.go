package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notification_state"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

const (
	browserQueueCapacity = 256
	browserIdentityLimit = 4096
	browserEventLifetime = 15 * time.Second
	browserBatchLimit    = 12
	browserRateWindow    = time.Minute
	browserLedgerError   = "Browser notifications paused: identity storage unavailable, full, or observation queue overflowed. Repair storage; for capacity, stop hosts and run compact-notifications --consume-unfinished, then restart."
)

type (
	// BrowserLedger is separate from the native channel's ledger. Its transactions
	// run only on the browser worker, never the session reducer or HTTP handler.
	BrowserLedger interface {
		ObserveActivity(farm.NotificationSnapshot) (notification_state.Observation, error)
		ReserveBatch([]string) ([]string, error)
	}
	browserEvent struct {
		ID      string `json:"id"`
		Printer string `json:"printer"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		Sound   bool   `json:"sound"`
	}
	browserResult struct {
		Cursor  uint64         `json:"cursor"`
		Events  []browserEvent `json:"events"`
		Error   string         `json:"error,omitempty"`
		Warning string         `json:"warning,omitempty"`
	}
	browserRequest struct {
		Baseline bool   `json:"baseline"`
		After    uint64 `json:"after"`
	}
	browserWork struct {
		generation uint64
		input      *notifications.Input
		request    browserRequest
		reply      chan browserResult
	}
	browserOccurrence struct {
		sequence uint64
		at       time.Time
	}
	browserWorker struct {
		ctx                 context.Context
		cancel              context.CancelFunc
		done                chan struct{}
		queue               chan browserWork
		failed              atomic.Bool
		mu                  sync.Mutex
		latest              notifications.Input
		ledger              BrowserLedger
		now                 func() time.Time
		sequence            uint64
		submitted, observed uint64
		attempts            []time.Time
		seen                map[string]browserOccurrence
		candidates          []notification_state.Candidate
		capacity            notification_state.Capacity
	}
)

func newBrowserWorker(ctx context.Context, ledger BrowserLedger) *browserWorker {
	ctx, cancel := context.WithCancel(ctx)
	b := &browserWorker{ctx: ctx, cancel: cancel, done: make(chan struct{}), queue: make(chan browserWork, browserQueueCapacity), ledger: ledger, now: time.Now, seen: map[string]browserOccurrence{}}
	if ledger == nil {
		b.failed.Store(true)
	}
	go b.run()
	return b
}

// SubmitNotifications accepts independently owned normalized values. Overflow
// fails closed rather than losing a clear/recur transition and reusing identity.
func (s *Server) SubmitNotifications(input notifications.Input) { s.browser.submit(input) }

func (b *browserWorker) submit(input notifications.Input) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failed.Load() || b.ctx.Err() != nil || reflect.DeepEqual(b.latest, input) {
		return
	}
	b.latest = input
	b.submitted++
	select {
	case b.queue <- browserWork{input: &input, generation: b.submitted}:
	default:
		b.failed.Store(true)
	}
}

func (b *browserWorker) run() {
	defer close(b.done)
	for {
		select {
		case <-b.ctx.Done():
			return
		case work := <-b.queue:
			if work.input != nil {
				if !b.failed.Load() {
					b.observe(*work.input)
					b.observed = work.generation
				}
			} else {
				work.reply <- b.claim(work.request)
			}
		}
	}
}

func (b *browserWorker) observe(input notifications.Input) {
	observation, err := b.ledger.ObserveActivity(input.Snapshot)
	if err != nil {
		b.failed.Store(true)
		return
	}
	b.capacity = observation.Capacity
	// Retire old identities even while a family is unknown. A confirmed clear
	// can have removed the ledger entry already; that is safe to forget locally.
	retired := map[string]bool{}
	for id, occurrence := range b.seen {
		if !b.now().Before(occurrence.at.Add(browserEventLifetime)) {
			if _, err := b.ledger.ReserveBatch([]string{id}); err != nil && !errors.Is(err, notification_state.ErrUnknown) {
				b.failed.Store(true)
				return
			}
			delete(b.seen, id)
			retired[id] = true
		}
	}
	for _, decision := range observation.Suppressed {
		if decision.Kind == notification_state.Intervention && (decision.Reason == notification_state.SuppressedReviewed || decision.Reason == notification_state.SuppressedNotActionable) {
			if _, err := b.ledger.ReserveBatch([]string{decision.ID}); err != nil {
				b.failed.Store(true)
				return
			}
		}
		delete(b.seen, decision.ID)
	}
	b.candidates = observation.Candidates
	for _, c := range observation.Candidates {
		if retired[c.ID] {
			continue
		}
		occurrence, present := b.seen[c.ID]
		if !present {
			b.sequence++
			occurrence = browserOccurrence{sequence: b.sequence, at: b.now()}
			b.seen[c.ID] = occurrence
		}
		// Consume muted/quiet/expired events so policy edits, restoration, and
		// reconnection cannot replay them. Unknown families still retain identity.
		if !browserEligible(c, input) || !b.now().Before(occurrence.at.Add(browserEventLifetime)) {
			if _, err := b.ledger.ReserveBatch([]string{c.ID}); err != nil {
				b.failed.Store(true)
				return
			}
			delete(b.seen, c.ID)
		}
	}
	// Retain first-seen times across unknown/stale snapshots, but bound memory.
	if len(b.seen) > browserIdentityLimit {
		b.failed.Store(true)
	}
}

func browserEligible(c notification_state.Candidate, input notifications.Input) bool {
	p, present := input.Snapshot[c.PrinterID]
	policy := input.Policies[c.PrinterID]
	if !present || !input.Active || input.Quiet || policy.Snoozed {
		return false
	}
	if c.Kind == notification_state.Completion {
		return policy.Completion && p.Job.State == farm.StateFinished
	}
	condition, present := p.Conditions[c.ConditionKey]
	return present && condition.Eligible && policy.Allowed[c.ConditionKey]
}

func (b *browserWorker) claim(request browserRequest) browserResult {
	result := browserResult{Cursor: b.sequence, Events: []browserEvent{}}
	if b.capacity.NearLimit() {
		result.Warning = fmt.Sprintf("Live notification IDs %d/%d: near limit. Stop hosts; compact-notifications --consume-unfinished permanently suppresses retained completions.", b.capacity.Entries, b.capacity.Limit)
	}
	if b.failed.Load() {
		result.Error = browserLedgerError
		return result
	}
	if request.Baseline {
		return result
	}
	// Wait for queued lifecycle observations before claiming; a future snapshot
	// may look identical despite containing a clear/recur transition. Never hold
	// the reducer-facing mutex during filesystem work.
	b.mu.Lock()
	latest, generation := b.latest, b.submitted
	b.mu.Unlock()
	if generation != b.observed {
		result.Cursor = request.After
		return result
	}
	if b.failed.Load() || b.ctx.Err() != nil {
		result.Error = browserLedgerError
		return result
	}
	now := b.now()
	var recent []time.Time
	for _, at := range b.attempts {
		if now.Before(at.Add(browserRateWindow)) {
			recent = append(recent, at)
		}
	}
	b.attempts = recent
	limit := browserBatchLimit - len(recent)
	var ids []string
	events := map[string]browserEvent{}
	for _, c := range b.candidates {
		occurrence, present := b.seen[c.ID]
		if !present || occurrence.sequence <= request.After || !b.now().Before(occurrence.at.Add(browserEventLifetime)) || !browserEligible(c, latest) {
			continue
		}
		// Reserve excess events too: advancing one tab's cursor cannot
		// suppress them for another tab or a restarted host.
		ids = append(ids, c.ID)
		if len(events) >= limit {
			continue
		}
		policy := latest.Policies[c.PrinterID]
		body := "Needs attention. Open the dashboard to inspect reported conditions. Physical resolution is unconfirmed."
		if c.Kind == notification_state.Completion {
			body = "Reports that the print finished."
		}
		title := policy.PrinterName
		if latest.Demo {
			title = "Demo: " + title
		}
		events[c.ID] = browserEvent{ID: c.ID, Printer: opaque(c.PrinterID), Title: title, Body: body, Sound: policy.Sound}
	}
	if len(ids) == 0 {
		return result
	}
	reserved, err := b.ledger.ReserveBatch(ids)
	if err != nil {
		b.failed.Store(true)
		result.Error = browserLedgerError
		return result
	}
	b.mu.Lock()
	changed := generation != b.submitted || b.failed.Load() || b.ctx.Err() != nil
	b.mu.Unlock()
	for _, id := range reserved {
		if event, attempted := events[id]; attempted {
			b.attempts = append(b.attempts, now)
			if !changed {
				result.Events = append(result.Events, event)
			}
		}
		delete(b.seen, id)
	}
	// All attempts, including lost responses, constructor errors and crashes,
	// remain reserved permanently. Acceptance never proves visible delivery.
	return result
}

func (s *Server) browserNotifications(w http.ResponseWriter, r *http.Request, host string) {
	if !s.allowCommand(w, r, host) {
		return
	}
	var request *browserRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCommandBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request == nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid notification request", http.StatusBadRequest)
		return
	}
	reply := make(chan browserResult, 1)
	select {
	case s.browser.queue <- browserWork{request: *request, reply: reply}:
	default:
		http.Error(w, "Session busy", http.StatusServiceUnavailable)
		return
	}
	timer := time.NewTimer(commandTimeout)
	defer timer.Stop()
	select {
	case result := <-reply:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	case <-timer.C:
		http.Error(w, "Outcome unknown; attempts will not be retried", http.StatusGatewayTimeout)
	case <-s.ctx.Done():
		http.Error(w, "Session ended", http.StatusServiceUnavailable)
	case <-r.Context().Done():
	}
}

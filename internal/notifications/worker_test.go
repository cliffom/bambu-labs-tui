package notifications

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notification_state"
)

type workerLedger struct {
	notification_state.Store
	observed        chan struct{}
	recorded        chan notification_state.Attempt
	observeGate     func()
	reserveGate     func()
	observeErr      error
	reserveErr      error
	reserveAfterErr error
	recordErr       error
}

func (l *workerLedger) ObserveActivity(s farm.NotificationSnapshot) (notification_state.Observation, error) {
	if l.observeGate != nil {
		l.observeGate()
	}
	if l.observeErr != nil {
		return notification_state.Observation{}, l.observeErr
	}
	result, err := l.Store.ObserveActivity(s)
	l.observed <- struct{}{}
	return result, err
}

func (l *workerLedger) ReserveBatch(ids []string) ([]string, error) {
	if l.reserveErr != nil {
		return nil, l.reserveErr
	}
	if l.reserveGate != nil {
		l.reserveGate()
	}
	claimed, err := l.Store.ReserveBatch(ids)
	if err == nil && l.reserveAfterErr != nil {
		return claimed, l.reserveAfterErr
	}
	return claimed, err
}

func (l *workerLedger) RecordResult(id string, a notification_state.Attempt) error {
	if l.recordErr != nil {
		return l.recordErr
	}
	err := l.Store.RecordResult(id, a)
	l.recorded <- a
	return err
}

func testLedger(t *testing.T) *workerLedger {
	t.Helper()
	return &workerLedger{Store: notification_state.Store{Path: filepath.Join(t.TempDir(), "ledger.json")}, observed: make(chan struct{}, workerQueueCapacity+4), recorded: make(chan notification_state.Attempt, 16)}
}

func TestWorkerDoesNotSendAfterAmbiguousReservationCommit(t *testing.T) {
	ledger := testLedger(t)
	// Model a post-rename directory-sync failure: a reservation can be visible
	// on reload even though its durability could not be acknowledged.
	ledger.reserveAfterErr = errors.New("commit directory: sync failed")
	var calls atomic.Int32
	send := func(context.Context, Message) error { calls.Add(1); return nil }
	w := NewWorker(context.Background(), ledger, send)
	w.Submit(incidentFrame())
	awaitPaused(t, w)
	stopWorker(t, w)
	if calls.Load() != 0 || w.Status().Accepted != 0 {
		t.Fatal("sender ran after unconfirmed reservation durability")
	}
	ledger.reserveAfterErr = nil
	// A fresh worker must honor the visible reservation, not replay it.
	w = NewWorker(context.Background(), ledger, send)
	w.mu.Lock()
	w.latest = frame{Input: incidentFrame()}
	w.mu.Unlock()
	if !w.process(frame{Input: incidentFrame()}) {
		t.Fatal("restart could not read the reserved occurrence")
	}
	stopWorker(t, w)
	if calls.Load() != 0 {
		t.Fatal("restart replayed the ambiguous attempt")
	}
}

func incidentFrame() Input {
	return Input{Active: true, Snapshot: farm.NotificationSnapshot{"TEST": {StateKnown: true, PrintKnown: true, HMSKnown: true, Conditions: map[string]farm.NotificationCondition{"TEST/fault/0700_8011": {Family: farm.NotificationPrintFamily, Code: "0700_8011", Level: farm.LevelError, Eligible: true, Since: time.Unix(1, 0)}}}}, Policies: map[string]Policy{"TEST": {PrinterName: "Synthetic printer", Desktop: true, Sound: true, Completion: true, Allowed: map[string]bool{"TEST/fault/0700_8011": true}}}}
}

func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach expected synchronization point")
		var zero T
		return zero
	}
}
func stopWorker(t *testing.T, w *Worker) { t.Helper(); w.Stop(); await(t, w.Done()) }

func awaitPaused(t *testing.T, w *Worker) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		status := w.Status()
		if status.Disabled && !status.Pending {
			return
		}
		select {
		case <-w.changed:
		case <-deadline.C:
			t.Fatal("worker did not pause")
		}
	}
}

func TestWorkerDeliversAndDeduplicatesAcrossRestart(t *testing.T) {
	ledger := testLedger(t)
	messages := make(chan Message, 4)
	send := func(_ context.Context, m Message) error { messages <- m; return nil }
	w := NewWorker(context.Background(), ledger, send)
	w.Submit(incidentFrame())
	message := await(t, messages)
	if message.Title != "Synthetic printer" || !message.Sound || !strings.Contains(message.Body, "Synthetic printer needs attention") {
		t.Fatal("wrong live message", message)
	}
	if result := await(t, ledger.recorded); result != notification_state.Accepted {
		t.Fatal(result)
	}
	stopWorker(t, w)
	if status := w.Status(); status.Accepted != 1 || status.Error != "" {
		t.Fatal(status)
	}
	restarted := NewWorker(context.Background(), ledger, send)
	restarted.Submit(incidentFrame())
	await(t, ledger.observed) // The first worker's receipt.
	await(t, ledger.observed)
	stopWorker(t, restarted)
	select {
	case m := <-messages:
		t.Fatal("duplicate delivery", m)
	default:
	}
}

func TestWorkerCancelsDeliveryOnSnoozeAndShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprint(shutdown), func(t *testing.T) {
			ledger := testLedger(t)
			started, canceled := make(chan struct{}), make(chan struct{})
			w := NewWorker(context.Background(), ledger, func(ctx context.Context, _ Message) error {
				close(started)
				<-ctx.Done()
				close(canceled)
				return ctx.Err()
			})
			w.Submit(incidentFrame())
			await(t, started)
			if shutdown {
				w.Stop()
			} else {
				muted := incidentFrame()
				p := muted.Policies["TEST"]
				p.Snoozed = true
				muted.Policies["TEST"] = p
				w.Submit(muted)
			}
			await(t, canceled)
			stopWorker(t, w)
			if w.Status().Accepted != 0 {
				t.Fatal("canceled delivery claimed acceptance")
			}
			select {
			case result := <-ledger.recorded:
				t.Fatal("ambiguous attempt recorded as definite result", result)
			default:
			}
		})
	}
}

func TestPolicyChangedDuringReservationPreventsOSCall(t *testing.T) {
	ledger := testLedger(t)
	entered, release := make(chan struct{}), make(chan struct{})
	ledger.reserveGate = func() { close(entered); <-release }
	var calls atomic.Int32
	w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls.Add(1); return nil })
	w.Submit(incidentFrame())
	await(t, entered)
	muted := incidentFrame()
	muted.Active = false
	w.Submit(muted)
	close(release)
	stopWorker(t, w)
	if calls.Load() != 0 {
		t.Fatal("stale reservation sent a notification")
	}
}

func TestQueueOverflowDisablesWithoutBlockingUI(t *testing.T) {
	ledger := testLedger(t)
	entered, release := make(chan struct{}), make(chan struct{})
	ledger.observeGate = func() { close(entered); <-release }
	var calls atomic.Int32
	w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls.Add(1); return nil })
	w.Submit(incidentFrame())
	await(t, entered)
	for i := range workerQueueCapacity + 1 {
		input := incidentFrame()
		p := input.Policies["TEST"]
		p.PrinterName = fmt.Sprint(i)
		input.Policies["TEST"] = p
		w.Submit(input)
	}
	if !w.Status().Disabled || !strings.Contains(w.Status().Error, "queue full") || len(withoutOwnership(w.Status()).History) != 1 || withoutOwnership(w.Status()).History[0].Reason != ReasonQueue {
		t.Fatal(w.Status())
	}
	close(release)
	stopWorker(t, w)
	if calls.Load() != 0 {
		t.Fatal("overflowed worker delivered stale data")
	}
}

func TestWorkerFailureIsVisibleAndNotAccepted(t *testing.T) {
	ledger := testLedger(t)
	w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { return errors.New("synthetic denial") })
	w.Submit(incidentFrame())
	if outcome := await(t, ledger.recorded); outcome != notification_state.Failed {
		t.Fatal(outcome)
	}
	stopWorker(t, w)
	status := w.Status()
	if status.Failed != 1 || status.Accepted != 0 || !strings.Contains(status.Error, "doctor --notify") {
		t.Fatal(status)
	}
	broken := testLedger(t)
	broken.observeErr = errors.New("corrupt store")
	failed := NewWorker(context.Background(), broken, func(context.Context, Message) error { t.Error("sent with failed storage"); return nil })
	failed.Submit(incidentFrame())
	awaitPaused(t, failed)
	stopWorker(t, failed)
	if !failed.Status().Disabled {
		t.Fatal("storage failure did not disable worker")
	}
}

func TestCompletionDeliveryRequiresActivityAndCurrentPolicy(t *testing.T) {
	ledger := testLedger(t)
	sent := make(chan Message, 1)
	w := NewWorker(context.Background(), ledger, func(_ context.Context, m Message) error { sent <- m; return nil })
	active := incidentFrame()
	p := active.Snapshot["TEST"]
	p.Conditions = nil
	p.Job = farm.NotificationJob{Task: "synthetic-job", State: farm.StateRunning}
	active.Snapshot["TEST"] = p
	w.Submit(active)
	await(t, ledger.observed)
	finished := incidentFrame()
	p.Conditions = nil
	p.Job.State = farm.StateFinished
	finished.Snapshot["TEST"] = p
	w.Submit(finished)
	message := await(t, sent)
	if message.Title != "Synthetic printer" || !strings.Contains(message.Body, "reports that the print finished") {
		t.Fatal(message)
	}
	await(t, ledger.recorded)
	stopWorker(t, w)
}

func TestEligibilityRejectsChangedReviewedMutedAndStaleSources(t *testing.T) {
	source := incidentFrame()
	candidate := notification_state.Candidate{PrinterID: "TEST", ConditionKey: "TEST/fault/0700_8011", Kind: notification_state.Intervention}
	if !eligible(candidate, source, source) {
		t.Fatal("valid source suppressed")
	}
	for _, kind := range []string{"closing", "snoozed", "disabled", "category", "reviewed", "recurrence", "stale"} {
		t.Run(kind, func(t *testing.T) {
			input := incidentFrame()
			p := input.Policies["TEST"]
			printer := input.Snapshot["TEST"]
			c := printer.Conditions[candidate.ConditionKey]
			switch kind {
			case "closing":
				input.Active = false
			case "snoozed":
				p.Snoozed = true
			case "disabled":
				p.Desktop = false
			case "category":
				p.Allowed[candidate.ConditionKey] = false
			case "reviewed":
				c.Eligible = false
			case "recurrence":
				c.Since = c.Since.Add(time.Second)
			case "stale":
				delete(input.Snapshot, "TEST")
			}
			if kind != "stale" {
				printer.Conditions[candidate.ConditionKey] = c
				input.Snapshot["TEST"] = printer
			}
			input.Policies["TEST"] = p
			if eligible(candidate, source, input) {
				t.Fatal("ineligible input allowed")
			}
		})
	}
}

func groupedIncidentFrame() Input {
	input := incidentFrame()
	p := input.Snapshot["TEST"]
	p.Conditions["TEST/fault/0700_2100_0002_0001"] = farm.NotificationCondition{Family: farm.NotificationHMSFamily, Code: "0700_2100_0002_0001", Level: farm.LevelWarning, Eligible: true, Since: time.Unix(1, 0)}
	p.Conditions["TEST/state"] = farm.NotificationCondition{Family: farm.NotificationStateFamily, State: farm.StatePaused, Level: farm.LevelWarning, Eligible: true, Since: time.Unix(1, 0)}
	input.Snapshot["TEST"] = p
	policy := input.Policies["TEST"]
	for key := range p.Conditions {
		policy.Allowed[key] = true
	}
	input.Policies["TEST"] = policy
	return input
}

func TestCoReportedIncidentsProduceOneOSSubmission(t *testing.T) {
	ledger := testLedger(t)
	messages := make(chan Message, 4)
	w := NewWorker(context.Background(), ledger, func(_ context.Context, message Message) error { messages <- message; return nil })
	input := groupedIncidentFrame()
	w.Submit(input)
	message := await(t, messages)
	if !strings.Contains(message.Body, "3 notices") {
		t.Fatal("group count missing", message)
	}
	for range 3 {
		if result := await(t, ledger.recorded); result != notification_state.Accepted {
			t.Fatal(result)
		}
	}
	stopWorker(t, w)
	if w.Status().Accepted != 1 {
		t.Fatal("accepted counter should count OS submissions", w.Status())
	}
	select {
	case extra := <-messages:
		t.Fatal("incident burst produced another submission", extra)
	default:
	}
	if pending, err := ledger.Observe(input.Snapshot); err != nil || len(pending) != 0 {
		t.Fatal("group did not preserve individual deduplication", pending, err)
	}
}

func TestGroupedDeliveryKeepsMutedNoticePending(t *testing.T) {
	ledger := testLedger(t)
	messages := make(chan Message, 4)
	w := NewWorker(context.Background(), ledger, func(_ context.Context, message Message) error { messages <- message; return nil })
	input := groupedIncidentFrame()
	input.Policies["TEST"].Allowed["TEST/state"] = false
	w.Submit(input)
	if message := await(t, messages); !strings.Contains(message.Body, "2 notices") {
		t.Fatal(message)
	}
	for range 2 {
		await(t, ledger.recorded)
	}
	stopWorker(t, w)
	pending, err := ledger.Observe(input.Snapshot)
	if err != nil || len(pending) != 1 || pending[0].ConditionKey != "TEST/state" {
		t.Fatal("muted notice identity was consumed", pending, err)
	}
}

func TestDeliveryGroupingSeparatesPrintersAndCompletion(t *testing.T) {
	items := []notification_state.Candidate{{ID: "a1", PrinterID: "A", Kind: notification_state.Intervention}, {ID: "b1", PrinterID: "B", Kind: notification_state.Intervention}, {ID: "a2", PrinterID: "A", Kind: notification_state.Intervention}, {ID: "a3", PrinterID: "A", Kind: notification_state.Completion}}
	groups := deliveryGroups(items)
	if len(groups) != 3 || len(groups[0]) != 2 || groups[1][0].PrinterID != "B" || groups[2][0].Kind != notification_state.Completion {
		t.Fatal("grouping merged unrelated deliveries", groups)
	}
}

func TestDemoNotificationTitleIdentifiesThePrinter(t *testing.T) {
	ledger := testLedger(t)
	messages := make(chan Message, 1)
	w := NewWorker(context.Background(), ledger, func(_ context.Context, message Message) error { messages <- message; return nil })
	input := incidentFrame()
	input.Demo = true
	w.Submit(input)
	message := await(t, messages)
	if message.Title != "Demo: Synthetic printer" {
		t.Fatal("demo title does not identify the source", message)
	}
	await(t, ledger.recorded)
	stopWorker(t, w)
}

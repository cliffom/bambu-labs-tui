package notifications

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/notification_state"
)

func awaitWorkerStatus(t *testing.T, w *Worker, ready func(Status) bool) Status {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		status := w.Status()
		if ready(status) {
			return status
		}
		select {
		case <-w.changed:
		case <-timeout.C:
			t.Fatalf("worker did not reach expected status: %+v", w.Status())
		}
	}
}

func sameLedger(t *testing.T, original *workerLedger) *workerLedger {
	t.Helper()
	other := testLedger(t)
	other.Store = original.Store
	return other
}

func TestStandbyTakesOverWithoutReplayingAcceptedIncident(t *testing.T) {
	ledger := testLedger(t)
	first := NewWorker(context.Background(), ledger, func(context.Context, Message) error { return nil })
	defer stopWorker(t, first)
	first.Submit(incidentFrame())
	awaitWorkerStatus(t, first, func(s Status) bool { return s.Ownership == OwnershipOwner && s.Accepted == 1 && !s.Pending })
	var secondCalls atomic.Int32
	other := sameLedger(t, ledger)
	second := NewWorker(context.Background(), other, func(context.Context, Message) error { secondCalls.Add(1); return nil })
	defer stopWorker(t, second)
	second.Submit(incidentFrame())
	standby := awaitWorkerStatus(t, second, func(s Status) bool { return s.Ownership == OwnershipStandby && !s.Pending })
	if standby.Disabled || standby.Error != "" {
		t.Fatal("standby was treated as a failure", standby)
	}
	select {
	case <-other.observed:
		t.Fatal("standby wrote lifecycle state")
	default:
	}
	stopWorker(t, first)
	second.requestOwnershipCheck()
	// Timer polling must not acquire or deliver from the old cached snapshot.
	probe, err := ledger.AcquireOwner()
	if err != nil {
		t.Fatal("standby polled disk before fresh Submit", err)
	}
	_ = probe.Close()
	second.Submit(incidentFrame())
	status := awaitWorkerStatus(t, second, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Pending })
	if secondCalls.Load() != 0 {
		t.Fatal("takeover duplicated an accepted request")
	}
	found := false
	for _, entry := range status.History {
		found = found || entry.Reason == notification_state.SuppressedAccepted
	}
	if !found {
		t.Fatal("takeover lost persisted acceptance", status)
	}
}

func TestTakeoverUsesFreshSnapshotAndMutedOwnerReleasesLease(t *testing.T) {
	ledger := testLedger(t)
	var calls atomic.Int32
	first := NewWorker(context.Background(), ledger, func(context.Context, Message) error { t.Error("muted owner sent"); return nil })
	defer stopWorker(t, first)
	snoozed := incidentFrame()
	p := snoozed.Policies["TEST"]
	p.Snoozed = true
	snoozed.Policies["TEST"] = p
	first.Submit(snoozed)
	awaitWorkerStatus(t, first, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Pending })
	second := NewWorker(context.Background(), sameLedger(t, ledger), func(context.Context, Message) error { calls.Add(1); return nil })
	defer stopWorker(t, second)
	second.Submit(incidentFrame())
	awaitWorkerStatus(t, second, func(s Status) bool { return s.Ownership == OwnershipStandby && !s.Pending })
	muted := incidentFrame()
	muted.ObserveOnly = true
	p = muted.Policies["TEST"]
	p.Desktop = false
	muted.Policies["TEST"] = p
	first.Submit(muted)
	awaitWorkerStatus(t, first, func(s Status) bool { return s.Ownership == OwnershipObserver && !s.Pending })
	second.requestOwnershipCheck()
	fresh := incidentFrame()
	printer := fresh.Snapshot["TEST"]
	printer.Conditions = nil
	fresh.Snapshot["TEST"] = printer
	second.Submit(fresh)
	awaitWorkerStatus(t, second, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Pending })
	if calls.Load() != 0 {
		t.Fatal("takeover delivered cached cleared incident")
	}
	second.Submit(incidentFrame())
	awaitWorkerStatus(t, second, func(s Status) bool { return s.Accepted == 1 && !s.Pending })
	if calls.Load() != 1 {
		t.Fatal("new confirmed recurrence was not delivered")
	}
}

func TestRecoveryDoesNotRetryUnconfirmedAttempt(t *testing.T) {
	ledger := testLedger(t)
	ledger.recordErr = errors.New("synthetic write failure")
	var calls atomic.Int32
	worker := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls.Add(1); return nil })
	defer stopWorker(t, worker)
	worker.Submit(incidentFrame())
	awaitPaused(t, worker)
	before, err := os.ReadFile(ledger.Path)
	if err != nil {
		t.Fatal(err)
	}
	ledger.recordErr = nil
	if !worker.Retry() {
		t.Fatal("paused worker rejected retry")
	}
	if calls.Load() != 1 {
		t.Fatal("retry sent from cached state")
	}
	worker.Submit(incidentFrame())
	awaitWorkerStatus(t, worker, func(s Status) bool {
		return !s.Disabled && !s.Recovering && !s.Pending && s.Ownership == OwnershipOwner
	})
	after, err := os.ReadFile(ledger.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("recovery rewrote reserved identity", err)
	}
	if calls.Load() != 1 {
		t.Fatal("recovery replayed an unconfirmed attempt")
	}
}

func TestPausedRecoveryDropsOldQueuedStateAndProtectsCorruptFile(t *testing.T) {
	ledger := testLedger(t)
	ledger.observeErr = errors.New("synthetic storage failure")
	var calls atomic.Int32
	worker := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls.Add(1); return nil })
	defer stopWorker(t, worker)
	worker.Submit(incidentFrame())
	awaitPaused(t, worker)
	fresh := incidentFrame()
	printer := fresh.Snapshot["TEST"]
	printer.Conditions = nil
	fresh.Snapshot["TEST"] = printer
	worker.Submit(fresh)
	ledger.observeErr = nil
	if !worker.Retry() {
		t.Fatal("recovery rejected")
	}
	worker.Submit(fresh)
	awaitWorkerStatus(t, worker, func(s Status) bool { return !s.Disabled && !s.Pending && !s.Recovering })
	if calls.Load() != 0 {
		t.Fatal("recovery replayed stale queued incident")
	}
	// Preserve a valid accepted backup, then corrupt the live file.
	worker.Submit(incidentFrame())
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Accepted == 1 && !s.Pending })
	backup, err := os.ReadFile(ledger.Path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{synthetic unreadable state")
	if err := os.WriteFile(ledger.Path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	changed := incidentFrame()
	p := changed.Policies["TEST"]
	p.Sound = false
	changed.Policies["TEST"] = p
	worker.Submit(changed)
	awaitPaused(t, worker)
	if !worker.Retry() {
		t.Fatal("corrupt-state recheck rejected")
	}
	worker.Submit(changed)
	awaitPaused(t, worker)
	data, err := os.ReadFile(ledger.Path)
	if err != nil || !bytes.Equal(data, corrupt) {
		t.Fatal("recheck overwrote unreadable state", err)
	}
	if err := os.WriteFile(ledger.Path, backup, 0o600); err != nil {
		t.Fatal(err)
	}
	if !worker.Retry() {
		t.Fatal("restored backup recheck rejected")
	}
	worker.Submit(changed)
	awaitWorkerStatus(t, worker, func(s Status) bool { return !s.Disabled && !s.Recovering && !s.Pending })
	if calls.Load() != 1 {
		t.Fatal("restoring ledger replayed prior acceptance")
	}
}

func TestLeaseIsHeldUntilCanceledSenderReturns(t *testing.T) {
	ledger := testLedger(t)
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	first := NewWorker(context.Background(), ledger, func(ctx context.Context, _ Message) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	})
	var releaseOnce sync.Once
	allowReturn := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() { allowReturn(); stopWorker(t, first) })
	first.Submit(incidentFrame())
	await(t, started)
	other := NewWorker(context.Background(), sameLedger(t, ledger), func(context.Context, Message) error { t.Error("unconfirmed request replayed"); return nil })
	defer stopWorker(t, other)
	other.Submit(incidentFrame())
	awaitWorkerStatus(t, other, func(s Status) bool { return s.Ownership == OwnershipStandby && !s.Pending })
	first.Stop()
	await(t, canceled)
	if owner, err := ledger.AcquireOwner(); !errors.Is(err, notification_state.ErrBusy) {
		if owner != nil {
			_ = owner.Close()
		}
		t.Fatal("lease released while sender still running", err)
	}
	allowReturn()
	await(t, first.Done())
	other.requestOwnershipCheck()
	other.Submit(incidentFrame())
	awaitWorkerStatus(t, other, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Pending })
}

func TestRetryDrainsPausedFramesAndRequiresNewSubmit(t *testing.T) {
	ledger := testLedger(t)
	worker := NewWorker(context.Background(), ledger, func(context.Context, Message) error { t.Error("old queued frame was delivered"); return nil })
	defer stopWorker(t, worker)
	worker.mu.Lock()
	worker.status.Disabled = true
	worker.latest = frame{sequence: 5, Input: incidentFrame()}
	worker.queue <- worker.latest
	worker.mu.Unlock()
	if !worker.Retry() {
		t.Fatal("paused worker refused retry")
	}
	worker.mu.Lock()
	waiting, queued := worker.awaitFresh, len(worker.queue)
	worker.mu.Unlock()
	if !waiting || queued != 0 {
		t.Fatal("recovery kept cached frames", waiting, queued)
	}
	probe, err := ledger.AcquireOwner()
	if err != nil {
		t.Fatal("recovery took ownership before fresh state", err)
	}
	_ = probe.Close()
	fresh := incidentFrame()
	printer := fresh.Snapshot["TEST"]
	printer.Conditions = nil
	fresh.Snapshot["TEST"] = printer
	worker.Submit(fresh)
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Recovering && !s.Pending })
}

func TestObserveOnlyCannotSendOrRetainOwnership(t *testing.T) {
	ledger := testLedger(t)
	worker := NewWorker(context.Background(), ledger, func(context.Context, Message) error { t.Error("observer submitted a notification"); return nil })
	defer stopWorker(t, worker)
	input := incidentFrame()
	input.ObserveOnly = true
	worker.Submit(input)
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Ownership == OwnershipObserver && !s.Pending })
	owner, err := ledger.AcquireOwner()
	if err != nil {
		t.Fatal("observer retained ownership", err)
	}
	_ = owner.Close()
}

// Delay starting the serialized loop to reproduce a burst entirely between
// processing calls, after the worker has already established idle ownership.
func TestIdleOwnerQueueOverflowReleasesLeaseForStandby(t *testing.T) {
	ledger := testLedger(t)
	ctx, cancel := context.WithCancel(context.Background())
	worker := &Worker{
		ctx: ctx, cancel: cancel, ledger: ledger,
		send:  func(context.Context, Message) error { t.Error("overflowed worker sent"); return nil },
		queue: make(chan frame, workerQueueCapacity), done: make(chan struct{}),
		now: time.Now, changed: make(chan struct{}, 1),
	}
	initial := incidentFrame()
	policy := initial.Policies["TEST"]
	policy.Snoozed = true
	initial.Policies["TEST"] = policy
	worker.latest = frame{sequence: 1, Input: initial}
	worker.process(worker.latest)
	if worker.Status().Ownership != OwnershipOwner {
		t.Fatal("initial processing did not establish ownership")
	}
	standby := NewWorker(context.Background(), sameLedger(t, ledger), func(context.Context, Message) error { return nil })
	defer stopWorker(t, standby)
	standby.Submit(initial)
	awaitWorkerStatus(t, standby, func(s Status) bool { return s.Ownership == OwnershipStandby && !s.Pending })
	for i := range workerQueueCapacity + 1 {
		input := incidentFrame()
		p := input.Policies["TEST"]
		p.PrinterName = fmt.Sprint(i)
		input.Policies["TEST"] = p
		worker.Submit(input)
	}
	if !worker.Status().Disabled {
		t.Fatal("burst did not overflow queue")
	}
	go worker.run()
	defer stopWorker(t, worker)
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Disabled && s.Ownership == OwnershipIdle })
	standby.requestOwnershipCheck()
	standby.Submit(initial)
	awaitWorkerStatus(t, standby, func(s Status) bool { return s.Ownership == OwnershipOwner && !s.Pending })
}

func TestOverflowKeepsLeaseUntilCanceledSenderReturns(t *testing.T) {
	ledger := testLedger(t)
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	worker := NewWorker(context.Background(), ledger, func(ctx context.Context, _ Message) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	})
	var once sync.Once
	allowReturn := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { allowReturn(); stopWorker(t, worker) })
	worker.Submit(incidentFrame())
	await(t, started)
	for i := range workerQueueCapacity + 1 {
		input := incidentFrame()
		p := input.Policies["TEST"]
		p.PrinterName = fmt.Sprint(i)
		input.Policies["TEST"] = p
		worker.Submit(input)
	}
	await(t, canceled)
	if !worker.Status().Disabled {
		t.Fatal("overflow did not pause worker")
	}
	if owner, err := ledger.AcquireOwner(); !errors.Is(err, notification_state.ErrBusy) {
		if owner != nil {
			_ = owner.Close()
		}
		t.Fatal("overflow released ownership before sender returned", err)
	}
	allowReturn()
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Ownership == OwnershipIdle && !s.Pending })
	owner, err := ledger.AcquireOwner()
	if err != nil {
		t.Fatal("paused worker retained ownership after sender returned", err)
	}
	_ = owner.Close()
}

func TestOptOutDrainsQueuedClearAndRecurrenceBeforeReleasingOwner(t *testing.T) {
	ledger := testLedger(t)
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	worker := NewWorker(context.Background(), ledger, func(ctx context.Context, _ Message) error {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
			return ctx.Err()
		}
		return nil
	})
	var once sync.Once
	allowReturn := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { allowReturn(); stopWorker(t, worker) })
	worker.Submit(incidentFrame())
	await(t, started)
	clear := incidentFrame()
	printer := clear.Snapshot["TEST"]
	printer.Conditions = nil
	clear.Snapshot["TEST"] = printer
	worker.Submit(clear)
	worker.Submit(incidentFrame())
	muted := incidentFrame()
	muted.ObserveOnly = true
	policy := muted.Policies["TEST"]
	policy.Desktop = false
	muted.Policies["TEST"] = policy
	worker.Submit(muted)
	await(t, canceled)
	allowReturn()
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Ownership == OwnershipObserver && !s.Pending })
	if calls.Load() != 1 {
		t.Fatal("muted recurrence was delivered")
	}
	owner, err := ledger.AcquireOwner()
	if err != nil {
		t.Fatal("opt-out retained ownership after draining", err)
	}
	_ = owner.Close()
	worker.Submit(incidentFrame())
	awaitWorkerStatus(t, worker, func(s Status) bool { return s.Accepted == 1 && !s.Pending })
	if calls.Load() != 2 {
		t.Fatal("confirmed recurrence was not delivered after re-enabling")
	}
}

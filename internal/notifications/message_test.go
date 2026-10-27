package notifications

import (
	"context"
	"testing"
)

func TestOwnershipAndAcceptanceHaveIndependentMessageKinds(t *testing.T) {
	ledger := testLedger(t)
	started, release := make(chan struct{}), make(chan struct{})
	w := NewWorker(context.Background(), ledger, func(ctx context.Context, _ Message) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	defer stopWorker(t, w)
	w.Submit(incidentFrame())
	await(t, started)
	status := w.Status()
	if status.Ownership != OwnershipOwner || status.LastKind != MessageInfo || status.Accepted != 0 {
		t.Fatal("ownership incorrectly implies successful delivery", status)
	}
	standby := NewWorker(context.Background(), sameLedger(t, ledger), func(context.Context, Message) error {
		t.Error("standby delivered a notification")
		return nil
	})
	defer stopWorker(t, standby)
	standby.Submit(incidentFrame())
	status = awaitWorkerStatus(t, standby, func(s Status) bool { return s.Ownership == OwnershipStandby && !s.Pending })
	if status.LastKind != MessageInfo || status.Error != "" {
		t.Fatal("standby coordination treated as an error", status)
	}
	close(release)
	status = awaitWorkerStatus(t, w, func(s Status) bool { return s.Accepted == 1 && !s.Pending })
	if status.LastKind != MessageSuccess {
		t.Fatal("native acceptance lost its success meaning", status)
	}
	muted := incidentFrame()
	muted.ObserveOnly = true
	w.Submit(muted)
	status = awaitWorkerStatus(t, w, func(s Status) bool { return s.Ownership == OwnershipObserver && !s.Pending })
	if status.LastKind != MessageInfo {
		t.Fatal("muted observer transition lost its informational meaning", status)
	}
}

package notifications

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/notification_state"
)

func withoutOwnership(status Status) Status {
	history := status.History[:0:0]
	for _, entry := range status.History {
		if entry.Subject != "Notification ownership" {
			history = append(history, entry)
		}
	}
	status.History = history
	return status
}

func TestActivityRetentionCoalescingTransitionsAndCopyOwnership(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	w := &Worker{ctx: context.Background(), now: func() time.Time { return now }}
	entry := Activity{Printer: "Synthetic", Subject: "Fault 0700_8011", Outcome: OutcomeSuppressed, Reason: ReasonSnoozed}
	w.appendActivityLocked(entry, "occurrence/snoozed")
	id := w.status.History[0].ID
	now = now.Add(time.Minute)
	for range 5 {
		if w.appendActivityLocked(entry, "occurrence/snoozed") {
			t.Fatal("repeated decision appended")
		}
	}
	if got := w.status.History[0]; got.ID != id || got.Repeats != 6 || got.FirstAt.Equal(got.At) {
		t.Fatal(got)
	}
	for i := range ActivityLimit + 5 {
		w.appendActivityLocked(entry, fmt.Sprintf("%d/snoozed", i))
	}
	if len(w.status.History) != ActivityLimit {
		t.Fatal("history is not bounded")
	}
	if w.appendActivityLocked(entry, "occurrence/snoozed") {
		t.Fatal("evicted repeated decision flooded history")
	}
	if !w.appendActivityLocked(entry, "occurrence/desktop_off") {
		t.Fatal("reason transition not recorded")
	}
	if !w.appendActivityLocked(entry, "occurrence/snoozed") {
		t.Fatal("return to prior reason should be a new transition")
	}
	status := withoutOwnership(w.Status())
	status.History[0].Printer = "changed"
	if w.Status().History[0].Printer == "changed" {
		t.Fatal("status shares worker-owned history")
	}
	for i := range activityDecisionLimit + 1 {
		w.appendActivityLocked(entry, fmt.Sprintf("bounded-%d/reason", i))
	}
	if len(w.activitySeen) > activityDecisionLimit || len(w.activitySeenOrder) > activityDecisionLimit {
		t.Fatal("coalescing metadata is unbounded")
	}
	restarted := &Worker{ctx: context.Background(), now: func() time.Time { return now }}
	if len(restarted.Status().History) != 0 {
		t.Fatal("history survived a new session")
	}
}

func TestWorkerActivityExplainsPoliciesAndCoalescesRepeatedObservations(t *testing.T) {
	for _, tc := range []struct{ kind, reason string }{
		{"desktop", ReasonDesktopOff}, {"category", ReasonCategoryOff}, {"snooze", ReasonSnoozed}, {"closing", ReasonClosing},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			ledger := testLedger(t)
			w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { t.Error("suppressed request sent"); return nil })
			defer stopWorker(t, w)
			input := incidentFrame()
			policy := input.Policies["TEST"]
			switch tc.kind {
			case "desktop":
				policy.Desktop = false
			case "category":
				policy.Allowed["TEST/fault/0700_8011"] = false
			case "snooze":
				policy.Snoozed = true
			case "closing":
				input.Active = false
			}
			input.Policies["TEST"] = policy
			var owner *notification_state.Owner
			if tc.kind == "closing" {
				held, err := ledger.AcquireOwner()
				if err != nil {
					t.Fatal(err)
				}
				owner = held
			}
			w.mu.Lock()
			w.latest = frame{Input: input}
			w.owner = owner
			w.mu.Unlock()
			// Drive deterministic observations directly while the worker's queue is idle.
			for i := range 5 {
				if !w.process(frame{Input: input}) {
					t.Fatal("worker stopped", i)
				}
			}
			status := withoutOwnership(w.Status())
			if len(status.History) != 1 || status.History[0].Reason != tc.reason || status.History[0].Repeats != 5 || status.Suppressed != 1 {
				t.Fatal(status)
			}
		})
	}
}

func TestGroupedOutcomesRetainMembersAndRestartExplainsDedup(t *testing.T) {
	ledger := testLedger(t)
	w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { return nil })
	w.Submit(groupedIncidentFrame())
	for range 3 {
		await(t, ledger.recorded)
	}
	stopWorker(t, w)
	history := withoutOwnership(w.Status()).History
	if len(history) != 3 {
		t.Fatal(history)
	}
	for _, entry := range history {
		if entry.Outcome != OutcomeAccepted || entry.GroupSize != 3 || entry.Request != history[0].Request || entry.Request == 0 || entry.Subject == "" {
			t.Fatal(entry)
		}
	}
	restarted := NewWorker(context.Background(), ledger, func(context.Context, Message) error { t.Error("duplicate OS call"); return nil })
	if len(restarted.Status().History) != 0 {
		t.Fatal("old session activity restored")
	}
	// An observed persisted acceptance explains suppression in the new session.
	if !restarted.process(frame{Input: groupedIncidentFrame()}) {
		t.Fatal("restarted worker stopped")
	}
	stopWorker(t, restarted)
	for _, entry := range withoutOwnership(restarted.Status()).History {
		if entry.Reason != notification_state.SuppressedAccepted {
			t.Fatal(entry)
		}
	}
}

func TestFailedAndUnconfirmedActivityDoNotLeakBackendData(t *testing.T) {
	for _, tc := range []struct {
		err             error
		outcome, reason string
		kind            MessageKind
	}{
		{errors.New("secret-token /private/account/path raw-telemetry"), OutcomeFailed, ReasonBackend, MessageError},
		{fmt.Errorf("private helper: %w", fs.ErrNotExist), OutcomeFailed, ReasonHelper, MessageError},
		{context.DeadlineExceeded, OutcomeUnconfirmed, ReasonTimeout, MessageWarning},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			ledger := testLedger(t)
			input := incidentFrame()
			w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { return tc.err })
			w.mu.Lock()
			w.latest = frame{Input: input}
			w.mu.Unlock()
			if !w.process(frame{Input: input}) {
				t.Fatal("unexpected worker pause")
			}
			stopWorker(t, w)
			if got := w.Status().LastKind; got != tc.kind {
				t.Fatalf("outcome %s has message kind %v, want %v", tc.outcome, got, tc.kind)
			}
			history := withoutOwnership(w.Status()).History
			if len(history) != 1 || history[0].Outcome != tc.outcome || history[0].Reason != tc.reason {
				t.Fatal(history)
			}
			if strings.Contains(fmt.Sprint(history), "secret-token") || strings.Contains(ReasonText(history[0].Reason), "private/account") {
				t.Fatal("backend data leaked")
			}
		})
	}
}

func TestActivityReportsStorageFailuresAndPreservesAcceptedOutcome(t *testing.T) {
	for _, tc := range []struct {
		stage  string
		err    error
		reason string
	}{
		{"observe", notification_state.ErrBusy, ReasonStorageBusy},
		{"observe", notification_state.ErrCapacity, ReasonStorageFull},
		{"reserve", errors.New("private path secret"), ReasonStorage},
		{"record", errors.New("private token"), ReasonStorage},
	} {
		t.Run(tc.stage+tc.reason, func(t *testing.T) {
			ledger := testLedger(t)
			switch tc.stage {
			case "observe":
				ledger.observeErr = tc.err
			case "reserve":
				ledger.reserveErr = tc.err
			case "record":
				ledger.recordErr = tc.err
			}
			calls := 0
			w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls++; return nil })
			input := incidentFrame()
			w.mu.Lock()
			w.latest = frame{Input: input}
			w.mu.Unlock()
			if w.process(frame{Input: input}) {
				t.Fatal("storage failure did not pause")
			}
			stopWorker(t, w)
			status := withoutOwnership(w.Status())
			if !status.Disabled || status.History[len(status.History)-1].Reason != tc.reason || strings.Contains(status.Error, "private") {
				t.Fatal(status)
			}
			if tc.stage == "record" {
				if calls != 1 || status.Accepted != 1 || len(status.History) != 2 || status.History[0].Outcome != OutcomeAccepted || status.History[1].Outcome != OutcomePaused {
					t.Fatal("accepted native request was hidden by ledger failure", status)
				}
			} else if calls != 0 {
				t.Fatal("called sender despite failed identity storage")
			}
		})
	}
}

func TestBackendExitCodesHaveAllowlistedReasons(t *testing.T) {
	for _, tc := range []struct {
		code   int
		reason string
	}{{3, ReasonPermission}, {5, ReasonTimeout}, {4, ReasonBackend}} {
		err := exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", tc.code)).Run()
		if got := deliveryReason(fmt.Errorf("private stderr is not history: %w", err)); got != tc.reason {
			t.Fatal(got, tc.reason)
		}
	}
}

func TestFailedAttemptExplainsNoAutomaticRetry(t *testing.T) {
	ledger := testLedger(t)
	calls := 0
	w := NewWorker(context.Background(), ledger, func(context.Context, Message) error { calls++; return errors.New("synthetic native failure") })
	input := incidentFrame()
	w.mu.Lock()
	w.latest = frame{Input: input}
	w.mu.Unlock()
	for range 3 {
		if !w.process(frame{Input: input}) {
			t.Fatal("native failure stopped monitoring")
		}
	}
	stopWorker(t, w)
	status := withoutOwnership(w.Status())
	if calls != 1 || status.Failed != 1 || len(status.History) != 2 || status.History[1].Reason != notification_state.SuppressedFailed {
		t.Fatal("failed attempt replayed or was not explained", status)
	}
}

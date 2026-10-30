package notifications

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/notification_state"
)

// Drive the serialized worker directly so boundaries need neither sleeps nor
// scheduler timing. Production polls request fresh UI snapshots before delivery.
func scheduledWorker(t *testing.T, ledger Ledger, now *time.Time, messages *[]Message) *Worker {
	t.Helper()
	w := &Worker{ctx: context.Background(), ledger: ledger, now: func() time.Time { return *now }, changed: make(chan struct{}, 1), send: func(_ context.Context, m Message) error { *messages = append(*messages, m); return nil }}
	t.Cleanup(w.releaseOwner)
	return w
}

func step(t *testing.T, w *Worker, input Input) {
	t.Helper()
	w.latest = frame{sequence: w.latest.sequence + 1, Input: input}
	if !w.process(w.latest) {
		t.Fatal(w.Status())
	}
}

func TestQuietExitRestartAndRecurrence(t *testing.T) {
	ledger := testLedger(t)
	now := time.Unix(100, 0)
	var messages []Message
	w := scheduledWorker(t, ledger, &now, &messages)
	quiet := incidentFrame()
	quiet.Quiet = true
	step(t, w, quiet)
	if len(messages) != 0 || w.Status().Suppressed != 1 {
		t.Fatal("quiet hours delivered", w.Status())
	}
	step(t, w, quiet)
	w.releaseOwner()
	restarted := scheduledWorker(t, ledger, &now, &messages)
	step(t, restarted, incidentFrame())
	if len(messages) != 0 {
		t.Fatal("replayed quiet-hours backlog")
	}
	clear := incidentFrame()
	p := clear.Snapshot["TEST"]
	p.Conditions = nil
	clear.Snapshot["TEST"] = p
	step(t, restarted, clear)
	recurrence := incidentFrame()
	c := recurrence.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"]
	c.Since = now
	recurrence.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"] = c
	step(t, restarted, recurrence)
	if len(messages) != 1 {
		t.Fatal("recurrence was consumed with unrelated occurrence")
	}
}

func TestQueuedQuietFrameConsumedAfterExit(t *testing.T) {
	now := time.Unix(100, 0)
	var messages []Message
	w := scheduledWorker(t, testLedger(t), &now, &messages)
	// An established owner drains lifecycle frames even when the UI is newer.
	step(t, w, Input{Active: true, Snapshot: farm.NotificationSnapshot{}, Policies: map[string]Policy{}})
	quiet := incidentFrame()
	quiet.Quiet = true
	w.latest = frame{sequence: 3, Input: incidentFrame()}
	if !w.process(frame{sequence: 2, Input: quiet}) {
		t.Fatal(w.Status())
	}
	step(t, w, incidentFrame())
	if len(messages) != 0 {
		t.Fatal("queued quiet event replayed after exit")
	}
}

func TestBurstWindowGroupsReportsAndKeepsMemberIdentities(t *testing.T) {
	now := time.Unix(100, 0)
	var messages []Message
	ledger := testLedger(t)
	w := scheduledWorker(t, ledger, &now, &messages)
	first := incidentFrame()
	first.BurstWindow = 2 * time.Second
	step(t, w, first)
	now = now.Add(time.Second)
	burst := groupedIncidentFrame()
	burst.BurstWindow = first.BurstWindow
	// Include a second printer and a completion in the same window.
	other := burst.Snapshot["TEST"]
	other.Conditions = nil
	other.Job = farm.NotificationJob{Task: "synthetic", State: farm.StateRunning}
	burst.Snapshot["OTHER"] = other
	burst.Policies["OTHER"] = burst.Policies["TEST"]
	step(t, w, burst)
	other.Job.State = farm.StateFinished
	burst.Snapshot["OTHER"] = other
	step(t, w, burst)
	if len(messages) != 0 {
		t.Fatal("sent before window boundary")
	}
	now = now.Add(time.Second)
	step(t, w, burst)
	if len(messages) != 1 || !strings.Contains(messages[0].Body, "4 events across 2 printers") {
		t.Fatal(messages)
	}
	var request uint64
	count := 0
	for _, entry := range w.Status().History {
		if entry.Outcome == OutcomeAccepted {
			count++
			if entry.GroupSize != 4 || (request != 0 && entry.Request != request) {
				t.Fatal(entry)
			}
			request = entry.Request
		}
	}
	if count != 4 {
		t.Fatal("lost member identity", w.Status())
	}
	if pending, err := ledger.Observe(burst.Snapshot); err != nil || len(pending) != 0 {
		t.Fatal("unconsumed members", pending, err)
	}
	// Grouping must not acknowledge any underlying condition.
	for _, condition := range burst.Snapshot["TEST"].Conditions {
		if condition.Reviewed {
			t.Fatal("group reviewed a condition")
		}
	}
}

func TestBurstRevalidatesReviewAndQuietPolicy(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(map[bool]string{false: "review", true: "quiet"}[quiet], func(t *testing.T) {
			now := time.Unix(100, 0)
			var messages []Message
			ledger := testLedger(t)
			w := scheduledWorker(t, ledger, &now, &messages)
			input := incidentFrame()
			input.BurstWindow = 2 * time.Second
			step(t, w, input)
			now = now.Add(2 * time.Second)
			if quiet {
				input.Quiet = true
			} else {
				c := input.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"]
				c.Eligible = false
				c.Reviewed = true
				input.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"] = c
			}
			step(t, w, input)
			if len(messages) != 0 {
				t.Fatal("delivered obsolete batch")
			}
		})
	}
}

func TestRollingRateLimitWithControllableClock(t *testing.T) {
	now := time.Unix(100, 0)
	var messages []Message
	w := scheduledWorker(t, testLedger(t), &now, &messages)
	for i := range deliveryRateLimit + 1 {
		clear := incidentFrame()
		p := clear.Snapshot["TEST"]
		p.Conditions = nil
		clear.Snapshot["TEST"] = p
		step(t, w, clear)
		input := incidentFrame()
		c := input.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"]
		c.Since = now.Add(time.Duration(i) * time.Second)
		input.Snapshot["TEST"].Conditions["TEST/fault/0700_8011"] = c
		step(t, w, input)
	}
	if len(messages) != deliveryRateLimit {
		t.Fatal("rate limit", len(messages))
	}
	latest := w.latest.Input
	now = now.Add(time.Minute - time.Nanosecond)
	step(t, w, latest)
	if len(messages) != deliveryRateLimit {
		t.Fatal("sent before rate boundary")
	}
	now = now.Add(time.Nanosecond)
	step(t, w, latest)
	if len(messages) != deliveryRateLimit+1 {
		t.Fatal("did not resume at rate boundary")
	}
}

func TestQuietResultIsDurablyClassified(t *testing.T) {
	ledger := testLedger(t)
	items, err := ledger.Observe(incidentFrame().Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ledger.ReserveBatch([]string{items[0].ID}); err != nil {
		t.Fatal(err)
	}
	if err = ledger.RecordResult(items[0].ID, notification_state.QuietSuppressed); err != nil {
		t.Fatal(err)
	}
	activity, err := ledger.ObserveActivity(incidentFrame().Snapshot)
	if err != nil || len(activity.Candidates) != 0 || len(activity.Suppressed) != 1 || activity.Suppressed[0].Reason != ReasonQuiet {
		t.Fatal(activity, err)
	}
}

func TestCompletionDuringQuietHoursDoesNotReplay(t *testing.T) {
	now := time.Unix(100, 0)
	var messages []Message
	w := scheduledWorker(t, testLedger(t), &now, &messages)
	input := incidentFrame()
	input.Quiet = true
	p := input.Snapshot["TEST"]
	p.Conditions = nil
	p.Job = farm.NotificationJob{Task: "synthetic-completion", State: farm.StateRunning}
	input.Snapshot["TEST"] = p
	step(t, w, input)
	p.Job.State = farm.StateFinished
	input.Snapshot["TEST"] = p
	step(t, w, input)
	input.Quiet = false
	step(t, w, input)
	if len(messages) != 0 {
		t.Fatal("quiet completion replayed")
	}
}

func TestScheduledPollRequiresFreshSubmit(t *testing.T) {
	now := time.Unix(100, 0)
	var messages []Message
	w := scheduledWorker(t, testLedger(t), &now, &messages)
	input := incidentFrame()
	input.BurstWindow = time.Second
	step(t, w, input)
	now = now.Add(time.Second)
	w.requestOwnershipCheck()
	if !w.forceFrame || len(messages) != 0 {
		t.Fatal("poll must request fresh UI state, not send cached telemetry")
	}
	step(t, w, input)
	if len(messages) != 1 {
		t.Fatal("fresh snapshot did not deliver due batch")
	}
}

func TestQuietOverlapPreservesExclusionReasonAndPendingOccurrence(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		completion   bool
		exclude      func(*Input)
	}{
		{name: "snooze", reason: ReasonSnoozed, exclude: func(input *Input) { p := input.Policies["TEST"]; p.Snoozed = true; input.Policies["TEST"] = p }},
		{name: "category", reason: ReasonCategoryOff, exclude: func(input *Input) { input.Policies["TEST"].Allowed["TEST/fault/0700_8011"] = false }},
		{name: "desktop", reason: ReasonDesktopOff, exclude: func(input *Input) { p := input.Policies["TEST"]; p.Desktop = false; input.Policies["TEST"] = p }},
		{name: "observe only", reason: ReasonDesktopOff, exclude: func(input *Input) { input.ObserveOnly = true }},
		{name: "completion", reason: ReasonCompletionOff, completion: true, exclude: func(input *Input) { p := input.Policies["TEST"]; p.Completion = false; input.Policies["TEST"] = p }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(100, 0)
			var messages []Message
			ledger := testLedger(t)
			w := scheduledWorker(t, ledger, &now, &messages)
			input := incidentFrame()
			if tc.completion {
				printer := input.Snapshot["TEST"]
				printer.Conditions = nil
				printer.Job = farm.NotificationJob{Task: "synthetic-overlap", State: farm.StateRunning}
				input.Snapshot["TEST"] = printer
				step(t, w, input)
				printer.Job.State = farm.StateFinished
				input.Snapshot["TEST"] = printer
			}
			input.Quiet = true
			tc.exclude(&input)
			step(t, w, input)
			history := withoutOwnership(w.Status()).History
			if len(messages) != 0 || len(history) != 1 || history[0].Reason != tc.reason {
				t.Fatal("wrong overlapping-policy decision", messages, history)
			}
			pending, err := ledger.Observe(input.Snapshot)
			if err != nil || len(pending) != 1 {
				t.Fatal("excluded occurrence should remain pending", pending, err)
			}
			// Quiet hours exit and the independent exclusion ends. The same pending
			// occurrence can deliver, matching the activity's non-permanent reason.
			resumed := incidentFrame()
			resumed.Snapshot = input.Snapshot
			step(t, w, resumed)
			if len(messages) != 1 {
				t.Fatal("pending occurrence did not deliver after exclusions ended", messages)
			}
		})
	}
}

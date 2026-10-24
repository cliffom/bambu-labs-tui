package farm

import (
	"os"
	"testing"
	"time"
)

func restart(t *testing.T, reviews map[string]Review, now time.Time) *State {
	t.Helper()
	s := New(10)
	s.RestoreReviews(reviews)
	s.Reconcile(now) // Initial UI ticks precede discovery and telemetry.
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Name: "Renamed printer", Online: true}}, now)
	report(t, s, `{"mc_percent":75}`, now)
	return s
}

func TestReviewsSurviveRestartAndPartialStartup(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		t.Run(map[bool]string{false: "acknowledged", true: "dismissed"}[dismiss], func(t *testing.T) {
			s, _, now := setup(t, "x2d")
			key := "TEST/fault/0300_0100_0001_0007"
			if dismiss {
				s.Dismiss(key)
			} else {
				s.Acknowledge(key, true)
			}
			copy := s.Reviews()
			next := restart(t, copy, now.Add(time.Hour))
			if len(next.Reviews()) != 1 {
				t.Fatal("startup discarded saved decisions before fault telemetry")
			}
			b, err := os.ReadFile("testdata/x2d.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := next.Report("TEST", b, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			c := next.Active[key]
			if c.Dismissed != dismiss || c.Acknowledged == dismiss || !c.Since.Equal(now) {
				t.Fatalf("review not restored: %+v", c)
			}
			for _, e := range next.Events {
				if dismiss && e.ConditionKey == key {
					t.Fatal("dismissed fault reappeared in startup history")
				}
			}
			if !dismiss {
				next.Acknowledge(key, false)
				if len(next.Reviews()) != 0 {
					t.Fatal("undo acknowledgement left persistent decision")
				}
			}
		})
	}
}

func TestSavedResolutionAndRecurrenceAcrossRestarts(t *testing.T) {
	s, _, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	s.Dismiss(key)
	next := restart(t, s.Reviews(), now.Add(time.Hour))
	report(t, next, `{"print_error":0}`, now.Add(time.Hour))
	if len(next.Reviews()) != 1 {
		t.Fatal("print_error report incorrectly resolved an HMS notice")
	}
	report(t, next, `{"hms":[]}`, now.Add(time.Hour))
	if len(next.Reviews()) != 0 || next.ReviewRevision() == 0 {
		t.Fatal("confirmed resolution did not remove and dirty saved decisions")
	}
	third := restart(t, next.Reviews(), now.Add(2*time.Hour))
	report(t, third, `{"hms":[{"attr":50331904,"code":65543}]}`, now.Add(2*time.Hour))
	if c := third.Active[key]; c.Dismissed || c.Acknowledged || c.Since.IsZero() {
		t.Fatal("recurring fault inherited old review")
	}
}

func TestSavedLowFilamentReviewWaitsForItsSlot(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	report(t, s, `{"ams":{"ams":[{"id":"0","tray":[{"id":"0","remain":8}]}]}}`, now)
	key := "TEST/low/0/0"
	s.Dismiss(key)
	next := restart(t, s.Reviews(), now)
	report(t, next, `{"ams":{"ams":[{"id":"0","humidity_raw":30,"tray":[{"id":"1","remain":90}]}]}}`, now)
	if len(next.Reviews()) != 1 {
		t.Fatal("unrelated slot telemetry discarded saved review")
	}
	report(t, next, `{"ams":{"ams":[{"id":"0","tray":[{"id":"0","remain":7}]}]}}`, now)
	if !next.Active[key].Dismissed {
		t.Fatal("changed remaining percentage reset dismissal")
	}
	report(t, next, `{"ams":{"ams":[{"id":"0","tray":[{"id":"0","remain":80}]}]}}`, now)
	if len(next.Reviews()) != 0 {
		t.Fatal("refilled slot retained dismissal")
	}
}

func TestChangedSavedPrintStateAndRemovedPrinter(t *testing.T) {
	s, _, now := setup(t, "x2d")
	s.Dismiss("TEST/state")
	next := restart(t, s.Reviews(), now)
	report(t, next, `{"gcode_state":"FAILED"}`, now)
	if next.Active["TEST/state"].Dismissed {
		t.Fatal("new failure inherited paused-state dismissal")
	}
	s.Dismiss("TEST/fault/0300_0100_0001_0007")
	next.RestoreReviews(s.Reviews())
	next.Discover(nil, now)
	if len(next.Reviews()) != 0 {
		t.Fatal("removed printer retained decisions")
	}
}

// TestReverseReviewsPreservesTelemetry checks independent review flags and recurrence.
func TestReverseReviewsPreservesTelemetry(t *testing.T) {
	s, _, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	s.Acknowledge(key, true)
	s.Dismiss(key)
	before, events := s.Active[key], len(s.Events)
	if !s.RestoreDismissal(key) {
		t.Fatal("dismissal was not restored")
	}
	c := s.Active[key]
	if c.Dismissed || !c.Acknowledged || c.Text != before.Text || c.Since != before.Since || len(s.Events) != events {
		t.Fatal("restore changed acknowledgement, telemetry, or history")
	}
	s.Reconcile(now)
	if !s.Active[key].Acknowledged || s.Active[key].Dismissed {
		t.Fatal("routine reconcile undid restoration")
	}
	s.Dismiss(key)
	if !s.MarkUnreviewed(key) || !s.Active[key].Dismissed || s.Active[key].Acknowledged {
		t.Fatal("unreviewed did not preserve dismissal")
	}
	s.RestoreDismissal(key)
	if _, ok := s.Reviews()[key]; ok {
		t.Fatal("fully reversed decision was kept")
	}
	report(t, s, `{"hms":[]}`, now.Add(time.Second))
	if s.RestoreDismissal(key) || s.MarkUnreviewed(key) {
		t.Fatal("resolved notice was synthesized")
	}
	report(t, s, `{"hms":[{"attr":50331904,"code":65543}]}`, now.Add(2*time.Second))
	if c := s.Active[key]; c.Since.Equal(before.Since) || c.Dismissed || c.Acknowledged {
		t.Fatal("recurrence reused a saved decision")
	}
}

// TestEditPendingReviews checks startup decisions can be reversed before fault reports.
func TestEditPendingReviews(t *testing.T) {
	s := New(10)
	r := Review{Code: "0300_0100_0001_0007", Level: "warning", Since: time.Now(), Acknowledged: true, Dismissed: true}
	s.RestoreReviews(map[string]Review{"TEST/fault/" + r.Code: r})
	key := "TEST/fault/" + r.Code
	if !s.RestoreDismissal(key) || !s.Reviews()[key].Acknowledged || s.Reviews()[key].Dismissed {
		t.Fatal("pending restoration changed acknowledgement")
	}
	if !s.MarkUnreviewed(key) || len(s.Reviews()) != 0 || len(s.Active) != 0 || len(s.Events) != 0 || s.ReviewRevision() != 2 {
		t.Fatal("pending edits created telemetry or failed to dirty storage")
	}
	if s.RestoreDismissal(key) || s.MarkUnreviewed(key) {
		t.Fatal("missing decisions were recreated")
	}
}

// TestResetReviewsPreservesOccurrences checks a bulk reset includes pending startup reviews.
func TestResetReviewsPreservesOccurrences(t *testing.T) {
	s, _, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	s.Acknowledge(key, true)
	s.Dismiss(key)
	reviews := s.Reviews()
	reviews["OTHER/fault/unknown"] = Review{Dismissed: true, Since: now}
	s.RestoreReviews(reviews)
	before, events, revision := s.Active[key], len(s.Events), s.ReviewRevision()
	s.ResetReviews()
	if len(s.Reviews()) != 0 || s.Active[key].Acknowledged || s.Active[key].Dismissed || s.Active[key].Since != before.Since || len(s.Events) != events || s.ReviewRevision() != revision+1 {
		t.Fatal("reset changed occurrence/history or omitted pending decisions")
	}
	s.Reconcile(now)
	if s.Active[key].Acknowledged || s.Active[key].Dismissed {
		t.Fatal("reconcile undid reset")
	}
	revision = s.ReviewRevision()
	s.ResetReviews()
	if s.ReviewRevision() != revision {
		t.Fatal("empty reset dirtied storage")
	}
}

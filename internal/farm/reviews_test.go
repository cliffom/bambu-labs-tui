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

package notificationstate

import (
	"os"
	"strings"
	"testing"

	"github.com/cliffom/bambu-tui/internal/farm"
)

func TestObservationExplainsReviewsAndAttemptsWithoutConsumingIdentity(t *testing.T) {
	store, state, now := fixture(t)
	report(t, state, `{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}`, now)
	first, err := store.ObserveActivity(state.NotificationSnapshot(now))
	if err != nil || len(first.Candidates) != 1 {
		t.Fatal(first, err)
	}
	candidate := first.Candidates[0]
	state.Acknowledge(candidate.ConditionKey, true)
	reviewed, err := store.ObserveActivity(state.NotificationSnapshot(now))
	if err != nil || len(reviewed.Candidates) != 0 || len(reviewed.Suppressed) != 1 || reviewed.Suppressed[0].Reason != SuppressedReviewed {
		t.Fatal(reviewed, err)
	}
	state.Acknowledge(candidate.ConditionKey, false)
	unreviewed, err := store.ObserveActivity(state.NotificationSnapshot(now))
	if err != nil || len(unreviewed.Candidates) != 1 || unreviewed.Candidates[0].ID != candidate.ID {
		t.Fatal("activity changed delivery identity", unreviewed, err)
	}
	reserve(t, store, candidate)
	for _, expected := range []string{SuppressedReserved, SuppressedAccepted} {
		if expected == SuppressedAccepted {
			if err := store.RecordResult(candidate.ID, Accepted); err != nil {
				t.Fatal(err)
			}
		}
		result, err := store.ObserveActivity(state.NotificationSnapshot(now))
		if err != nil || len(result.Candidates) != 0 || len(result.Suppressed) != 1 || result.Suppressed[0].Reason != expected {
			t.Fatal(result, err)
		}
	}
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"Private printer name", "SYNTHETIC-PRINTER-PRIVATE", SuppressedReviewed, SuppressedAccepted} {
		if strings.Contains(string(data), private) {
			t.Fatal("activity metadata persisted", private)
		}
	}
}

func TestObservationExplainsBaselineUnknownAndNonIntervention(t *testing.T) {
	for _, tc := range []struct{ body, reason string }{
		{`{"gcode_state":"FINISH","task_id":"synthetic-private-job","subtask_id":"synthetic-subtask"}`, SuppressedBaseline},
		{`{"gcode_state":"FINISH"}`, SuppressedJobIdentity},
		{`{"gcode_state":"PAUSE","print_error":0,"hms":[]}`, SuppressedNotActionable},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			store, state, now := fixture(t)
			report(t, state, tc.body, now)
			// Supply a confirmed non-intervention condition without guessing from text.
			snapshot := state.NotificationSnapshot(now)
			if tc.reason == SuppressedNotActionable {
				p := snapshot["SYNTHETIC-PRINTER-PRIVATE"]
				p.Conditions = map[string]farm.NotificationCondition{"synthetic/state": {Family: farm.NotificationStateFamily, State: farm.StatePaused}}
				snapshot["SYNTHETIC-PRINTER-PRIVATE"] = p
			}
			result, err := store.ObserveActivity(snapshot)
			if err != nil || len(result.Candidates) != 0 || len(result.Suppressed) != 1 || result.Suppressed[0].Reason != tc.reason {
				t.Fatal(result, err)
			}
			for _, decision := range result.Suppressed {
				if strings.Contains(decision.ID, "private-job") {
					t.Fatal("raw job identity in activity")
				}
			}
		})
	}
}

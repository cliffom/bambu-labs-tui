package session_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/session"
)

func TestExplicitRetryPreservesIdentityAndRejectsOldAttempt(t *testing.T) {
	s, at := fixture(t)
	fail := true
	var reviews map[string]farm.Review
	var snoozes map[string]time.Time
	s.WithNoticePersistence(func(v map[string]farm.Review) error {
		reviews = v
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	s.WithSnoozePersistence(nil, func(v map[string]time.Time) error {
		snoozes = v
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	key, target := firstTarget(t, &s)
	s.Dismiss(key)
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	r, z := s.Persistence()
	oldReview, oldSnooze := r(), z()
	s.Complete(oldReview)
	s.Complete(oldSnooze)
	revision, snoozeRevision := s.Farm.ReviewRevision(), s.SnoozeRevision()
	expectedReviews, expectedSnoozes := s.Farm.Reviews(), s.Snoozes["SYNTHETIC_SERIAL"]
	for range 3 {
		s.Reconcile(at)
		if r, z := s.Persistence(); r != nil || z != nil {
			t.Fatal("failure automatically retried")
		}
	}
	if s.ReviewSaveState() != "failed" || s.SnoozeSaveState() != "failed" {
		t.Fatal("missing failed status")
	}
	if !s.RetryPersistence(true, true) || s.RetryPersistence(true, true) {
		t.Fatal("queued retry not idempotent")
	}
	if !s.PersistencePending() || s.WebDecisions().State != "saving" {
		t.Fatal("queued retry not pending")
	}
	r, z = s.Persistence()
	if r == nil || z == nil || s.RetryPersistence(true, true) {
		t.Fatal("missing jobs or accepted in-flight retry")
	}
	// The same domain revision has a new attempt: old success OR failure cannot
	// finish it, even when a duplicate completion arrives during the retry.
	for _, old := range []session.Result{oldReview, oldSnooze} {
		s.Complete(old)
		old.Err = nil
		s.Complete(old)
	}
	if !s.ReviewWrites().InFlight || !s.SnoozeWrites().InFlight {
		t.Fatal("old attempt completed retry")
	}
	if s.Farm.ReviewRevision() != revision || s.SnoozeRevision() != snoozeRevision || s.WebDecisions().Targets[key] != target {
		t.Fatal("retry changed domain identity")
	}
	if !reflect.DeepEqual(s.Farm.Reviews(), expectedReviews) || s.Snoozes["SYNTHETIC_SERIAL"] != expectedSnoozes {
		t.Fatal("retry changed local decisions")
	}
	fail = false
	s.Complete(r())
	s.Complete(z())
	if s.HasUnsavedPersistence() || s.PersistencePending() || s.WebDecisions().State != "saved" || s.SnoozeSaveState() != "saved" {
		t.Fatal("successful retry not saved")
	}
	if !reflect.DeepEqual(reviews, expectedReviews) || snoozes["SYNTHETIC_SERIAL"] != expectedSnoozes {
		t.Fatal("retry saved wrong snapshot")
	}
	if s.RetryPersistence(true, true) {
		t.Fatal("saved stores eligible for retry")
	}
}

func TestRetryCapturesLatestAndCoalescesEditsDuringAttempt(t *testing.T) {
	s, at := fixture(t)
	fail := true
	var reviews []map[string]farm.Review
	var snoozes []map[string]time.Time
	s.WithNoticePersistence(func(v map[string]farm.Review) error {
		reviews = append(reviews, v)
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	s.WithSnoozePersistence(nil, func(v map[string]time.Time) error {
		snoozes = append(snoozes, v)
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	key, _ := firstTarget(t, &s)
	s.Acknowledge(key, true)
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	r, z := s.Persistence()
	s.Complete(r())
	s.Complete(z())
	s.RetryPersistence(true, true)
	// Edits before job capture belong to the retry snapshot.
	s.Dismiss(key)
	s.SetSnooze("SYNTHETIC_SERIAL", 2*time.Hour, at)
	r, z = s.Persistence()
	// Later edits coalesce without mutating those immutable snapshots.
	s.RestoreDismissal(key)
	s.SetSnooze("SYNTHETIC_SERIAL", 3*time.Hour, at)
	if r, z := s.Persistence(); r != nil || z != nil {
		t.Fatal("overlapping retry jobs")
	}
	// A failed retry is not itself permission for another retry, but the newer
	// edits still require one complete latest snapshot.
	s.Complete(r())
	s.Complete(z())
	r, z = s.Persistence()
	if !reviews[1][key].Dismissed || !snoozes[1]["SYNTHETIC_SERIAL"].Equal(at.Add(2*time.Hour)) {
		t.Fatal("retry snapshot aliased later edits")
	}
	fail = false
	s.Complete(r())
	s.Complete(z())
	if len(reviews) != 3 || len(snoozes) != 3 || !reflect.DeepEqual(reviews[2], s.Farm.Reviews()) || !reflect.DeepEqual(snoozes[2], s.Snoozes) || s.HasUnsavedPersistence() {
		t.Fatal("latest edits not saved")
	}
}

func TestRetryFailureNeedsAnotherExplicitRequest(t *testing.T) {
	s, at := fixture(t)
	s.WithSnoozePersistence(nil, func(map[string]time.Time) error { return errors.New("full") }, nil)
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	_, z := s.Persistence()
	s.Complete(z())
	if !s.RetryPersistence(false, true) {
		t.Fatal("failed snoozes not eligible")
	}
	_, z = s.Persistence()
	s.Complete(z())
	if r, z := s.Persistence(); r != nil || z != nil {
		t.Fatal("retry loop after another failure")
	}
	if s.SnoozeSaveState() != "failed" || !s.RetryPersistence(false, true) {
		t.Fatal("explicit second retry unavailable")
	}
}

func TestRetryProtectsEachUnavailableStoreIndependently(t *testing.T) {
	for _, protectedReviews := range []bool{true, false} {
		for _, demo := range []bool{true, false} {
			s, at := fixture(t)
			s.Demo = demo
			var reviewLoad, snoozeLoad error
			if protectedReviews {
				reviewLoad = errors.New("unsupported")
			} else {
				snoozeLoad = errors.New("unreadable")
			}
			s.WithNoticePersistence(func(map[string]farm.Review) error {
				if demo || protectedReviews {
					t.Fatal("protected review write")
				}
				return errors.New("full")
			}, reviewLoad)
			s.WithSnoozePersistence(nil, func(map[string]time.Time) error {
				if demo || !protectedReviews {
					t.Fatal("protected snooze write")
				}
				return errors.New("full")
			}, snoozeLoad)
			key, _ := firstTarget(t, &s)
			s.Dismiss(key)
			s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
			r, z := s.Persistence()
			if r != nil {
				s.Complete(r())
			}
			if z != nil {
				s.Complete(z())
			}
			canReviews, canSnoozes := s.CanRetryPersistence()
			if canReviews != (!demo && !protectedReviews) || canSnoozes != (!demo && protectedReviews) {
				t.Fatal("incorrect independent eligibility")
			}
			if s.RetryPersistence(true, true) == demo {
				t.Fatal("incorrect retry result")
			}
			r, z = s.Persistence()
			if (r != nil) != canReviews || (z != nil) != canSnoozes {
				t.Fatal("retry bypassed protection")
			}
		}
	}
}

func TestBrowserRetryUsesSharedPolicyWithoutRetryingSnoozes(t *testing.T) {
	s, at := fixture(t)
	fail := true
	s.WithNoticePersistence(func(map[string]farm.Review) error {
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	s.WithSnoozePersistence(nil, func(map[string]time.Time) error { return errors.New("full") }, nil)
	_, target := firstTarget(t, &s)
	decision(&s, target, false)
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	r, z := s.Persistence()
	old := r()
	s.Complete(old)
	s.Complete(z())
	if decision(&s, target, true) != "applied" || decision(&s, target, true) == "applied" {
		t.Fatal("browser retry not idempotent")
	}
	r, z = s.Persistence()
	if r == nil || z != nil || decision(&s, target, true) == "applied" {
		t.Fatal("browser retried snoozes or queued overlapping attempt")
	}
	s.Complete(old)
	if !s.ReviewWrites().InFlight {
		t.Fatal("old result completed browser retry")
	}
	fail = false
	s.Complete(r())
	if s.WebDecisions().State != "saved" || s.SnoozeSaveState() != "failed" {
		t.Fatal("browser save status diverged")
	}
}

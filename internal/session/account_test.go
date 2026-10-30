package session_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/session"
)

type accountFixture struct {
	noticeErr, snoozeErr error
	reads, writes        int
}

func (a *accountFixture) LoadReviews() (map[string]farm.Review, error) {
	a.reads++
	return map[string]farm.Review{"SYNTHETIC/state": {Level: farm.LevelWarning, Text: "Print paused", Acknowledged: true}}, a.noticeErr
}

func (a *accountFixture) LoadSnoozes() (map[string]time.Time, error) {
	a.reads++
	return map[string]time.Time{"SYNTHETIC": time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)}, a.snoozeErr
}
func (a *accountFixture) SaveReviews(map[string]farm.Review) error { a.writes++; return nil }
func (a *accountFixture) SaveSnoozes(map[string]time.Time) error   { a.writes++; return nil }

func TestAccountInitializationIndependentFailedLoadProtection(t *testing.T) {
	for _, tc := range []struct{ notice, snooze bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		store := &accountFixture{}
		if tc.notice {
			store.noticeErr = errors.New("private raw notice error")
		}
		if tc.snooze {
			store.snoozeErr = errors.New("private raw snooze error")
		}
		state := session.New(farm.New(10), false)
		warnings := state.WithAccountPersistence(store)
		if state.NoticesWritable() == tc.notice || state.SnoozesWritable() == tc.snooze {
			t.Fatal("failed load writer enabled", tc)
		}
		want := 0
		if tc.notice {
			want++
		}
		if tc.snooze {
			want++
		}
		if len(warnings) != want {
			t.Fatal(warnings)
		}
		if strings.Contains(strings.Join(warnings, " "), "private raw") {
			t.Fatal("private error exposed")
		}
		if len(state.Snoozes) != 1 || len(state.Farm.Reviews()) != 1 {
			t.Fatal("startup restore lost")
		}
		if store.reads != 2 || store.writes != 0 {
			t.Fatal("initialization wrote state")
		}
	}
	demo := session.New(farm.New(10), true)
	store := &accountFixture{}
	demo.WithAccountPersistence(store)
	if store.reads != 0 {
		t.Fatal("demo read account stores")
	}
}

func TestDisplayContractIsIndependentBeforeEncoding(t *testing.T) {
	s, at := fixture(t)
	report(t, &s, `{"print":{"mc_percent":25,"bed_temper":40,"ams":{"ams":[{"id":"0","humidity":3,"temp":21,"tray":[{"id":"0","remain":5,"tray_type":"PLA"}]}]}}}`, at)
	display := session.BuildDisplay(s.Farm, at, false, s.WebDecisions())
	before := *display.Printers[0].Progress
	s.Farm.Printers["SYNTHETIC_SERIAL"].Name = "Changed"
	*s.Farm.Printers["SYNTHETIC_SERIAL"].Progress = 99
	*s.Farm.Printers["SYNTHETIC_SERIAL"].Bed.Current = 70
	if *display.Printers[0].Progress != before || display.Printers[0].Name == "Changed" || *display.Printers[0].Bed.Current != 40 {
		t.Fatal("display aliases mutable state")
	}
	current := session.BuildDisplay(s.Farm, at, false, s.WebDecisions())
	if reflect.DeepEqual(display, current) {
		t.Fatal("display did not observe owner changes")
	}
}

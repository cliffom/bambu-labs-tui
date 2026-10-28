package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/accountstore"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notification_state"
)

func TestCompactNotificationsProtectsLiveHostAndBothChannels(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stores, err := accountstore.Open("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	defer stores.Close()
	ledger, err := notification_state.New("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	reviews := map[string]farm.Review{"SYNTHETIC/state": {Level: farm.LevelWarning, Acknowledged: true}}
	snoozes := map[string]time.Time{"SYNTHETIC": time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)}
	if err := stores.SaveReviews(reviews); err != nil {
		t.Fatal(err)
	}
	if err := stores.SaveSnoozes(snoozes); err != nil {
		t.Fatal(err)
	}
	native := ledger
	for _, suffix := range []string{"", ".browser"} {
		ledger.Path = native.Path + suffix
		if events, err := ledger.Observe(farm.NotificationSnapshot{"SYNTHETIC": {Job: farm.NotificationJob{Task: "unfinished", State: farm.StateRunning}, PrintKnown: true, Conditions: map[string]farm.NotificationCondition{"SYNTHETIC/fault": {Family: farm.NotificationPrintFamily, Level: farm.LevelError, Code: "SYNTHETIC", Eligible: true}}}}); err != nil {
			t.Fatal(err)
		} else {
			if len(events) != 1 || events[0].Kind != notification_state.Intervention {
				t.Fatal(events)
			}
			if ok, err := ledger.Reserve(events[0].ID); err != nil || !ok {
				t.Fatal(ok, err)
			}
		}
	}
	var output bytes.Buffer
	if err := compactAccountNotifications("global", "synthetic-account", &output); !errors.Is(err, accountstore.ErrBusy) {
		t.Fatal("maintenance ran during live host", err)
	}
	stores.Close()
	if err := compactAccountNotifications("global", "synthetic-account", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "unfinished jobs will not notify completion") {
		t.Fatal("maintenance consequence omitted", output.String())
	}
	for _, suffix := range []string{"", ".browser"} {
		ledger.Path = native.Path + suffix
		if events, err := ledger.Observe(farm.NotificationSnapshot{"SYNTHETIC": {Job: farm.NotificationJob{Task: "unfinished", State: farm.StateFinished}, PrintKnown: true, Conditions: map[string]farm.NotificationCondition{"SYNTHETIC/fault": {Family: farm.NotificationPrintFamily, Level: farm.LevelError, Code: "SYNTHETIC", Eligible: true}}}}); err != nil || len(events) != 0 {
			t.Fatal("maintenance replayed", suffix, events, err)
		}
	}
	if err := compactAccountNotifications("global", "synthetic-account", &output); err != nil {
		t.Fatal("repeat maintenance failed", err)
	}
	restored, err := accountstore.Open("global", "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	gotReviews, err := restored.LoadReviews()
	if err != nil || !reflect.DeepEqual(gotReviews, reviews) {
		t.Fatal("maintenance changed local reviews", gotReviews, err)
	}
	gotSnoozes, err := restored.LoadSnoozes()
	if err != nil || !reflect.DeepEqual(gotSnoozes, snoozes) {
		t.Fatal("maintenance changed snoozes", gotSnoozes, err)
	}
}

func TestCompactRequiresExplicitUnfinishedConsumption(t *testing.T) {
	if err := run([]string{"compact-notifications"}); err == nil || !strings.Contains(err.Error(), "--consume-unfinished") {
		t.Fatal("maintenance did not require explicit decision", err)
	}
	if err := run([]string{"--consume-unfinished"}); err == nil || !strings.Contains(err.Error(), "only for compact-notifications") {
		t.Fatal("maintenance consent accepted in monitoring", err)
	}
}

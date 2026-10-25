package farm

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFreshnessBoundariesByState(t *testing.T) {
	for _, state := range []string{"IDLE", "FINISH", "RUNNING", "PAUSE", "FAILED", "PREPARE", "SLICING", "UNKNOWN", ""} {
		t.Run(state, func(t *testing.T) {
			now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
			s := New(10)
			s.Cloud(true, "", now)
			s.Discover([]Device{{ID: "TEST", Online: true}}, now)
			report(t, s, fmt.Sprintf(`{"gcode_state":%q}`, state), now)
			p := s.Printers["TEST"]
			limit := 2 * time.Minute
			if state == "IDLE" || state == "FINISH" {
				limit = 10 * time.Minute
			}
			for _, age := range []time.Duration{0, limit - time.Nanosecond, limit, limit + time.Nanosecond} {
				at := now.Add(age)
				s.Reconcile(at)
				_, notice := s.Active["TEST/stale"]
				stale := age > limit
				link := "Live"
				if stale {
					link = "Stale"
				}
				if p.Stale(at) != stale || notice != stale || p.Connection(at, true) != link {
					t.Fatalf("state %q age %v: stale=%v notice=%v link=%s", state, age, p.Stale(at), notice, p.Connection(at, true))
				}
			}
			count := len(s.Events)
			s.Reconcile(now.Add(limit + time.Second))
			if len(s.Events) != count {
				t.Fatal("ongoing stale condition duplicated history")
			}
			report(t, s, `{"nozzle_temper":20}`, now.Add(limit+2*time.Second))
			if _, stale := s.Active["TEST/stale"]; stale || len(s.Events) != count+1 || !strings.HasPrefix(s.Events[len(s.Events)-1].Text, "Resolved: ") {
				t.Fatal("fresh partial telemetry did not resolve stale condition once")
			}
		})
	}
}

func TestIdleToPrintingRestoresShortFreshnessWindow(t *testing.T) {
	for _, state := range []string{"IDLE", "FINISH"} {
		t.Run(state, func(t *testing.T) {
			s, p, now := setup(t, "h2d")
			report(t, s, fmt.Sprintf(`{"gcode_state":%q,"hms":[],"print_error":0}`, state), now)
			started := now.Add(5 * time.Minute)
			report(t, s, `{"gcode_state":"RUNNING"}`, started)
			s.Reconcile(started.Add(2*time.Minute + time.Nanosecond))
			if _, stale := s.Active["TEST/stale"]; !stale || p.Connection(started.Add(121*time.Second), true) != "Stale" {
				t.Fatal("printing retained idle grace period")
			}
			report(t, s, `{"gcode_state":"FINISH"}`, started.Add(3*time.Minute))
			s.Reconcile(started.Add(8 * time.Minute))
			if _, stale := s.Active["TEST/stale"]; stale || p.Connection(started.Add(8*time.Minute), true) != "Live" {
				t.Fatal("completion did not restore idle grace period")
			}
		})
	}
}

func TestIdleGraceDoesNotHideMissingReportsOrOutages(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	s := New(10)
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	p := s.Printers["TEST"]
	p.State = "IDLE"
	if !p.Stale(now) || p.Connection(now, true) != "Waiting for data" {
		t.Fatal("idle printer without telemetry appeared live")
	}
	report(t, s, `{"gcode_state":"IDLE"}`, now)
	s.Discover([]Device{{ID: "TEST", Online: false}}, now.Add(time.Second))
	if _, offline := s.Active["TEST/offline"]; !offline || p.Connection(now.Add(time.Second), true) != "Offline" {
		t.Fatal("idle grace hid offline status")
	}
	s.Cloud(false, "", now.Add(2*time.Second))
	if _, disconnected := s.Active["cloud"]; !disconnected || p.Connection(now.Add(2*time.Second), false) != "Cloud disconnected" {
		t.Fatal("idle grace hid cloud disconnection")
	}
}

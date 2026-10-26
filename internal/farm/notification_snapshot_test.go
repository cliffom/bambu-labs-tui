package farm

import (
	"reflect"
	"testing"
	"time"
)

func notificationFarm(t *testing.T) (*State, time.Time) {
	t.Helper()
	now := time.Now()
	s := New(10)
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Name: "Synthetic name", Online: true}}, now)
	return s, now
}

func TestNotificationJobStateStaysAttachedToExplicitIdentity(t *testing.T) {
	s, now := notificationFarm(t)
	for _, tc := range []struct {
		body string
		want NotificationJob
	}{
		{`{"task_id":"task-a","subtask_id":"copy-a","gcode_state":"FINISH"}`, NotificationJob{"task-a", "copy-a", StateFinished}},
		{`{"task_id":"task-b"}`, NotificationJob{"task-b", "", StateUnknown}},
		{`{"mc_percent":100}`, NotificationJob{"task-b", "", StateUnknown}},
		{`{"gcode_state":"RUNNING"}`, NotificationJob{"task-b", "", StateRunning}},
		{`{"subtask_id":"copy-b"}`, NotificationJob{"task-b", "copy-b", StateUnknown}},
		{`{"gcode_state":"RUNNING"}`, NotificationJob{"task-b", "copy-b", StateRunning}},
		{`{"gcode_state":null}`, NotificationJob{"task-b", "copy-b", StateUnknown}},
		{`{"task_id":0,"subtask_id":0,"gcode_state":"FINISH"}`, NotificationJob{"", "", StateFinished}},
	} {
		report(t, s, tc.body, now)
		got := s.NotificationSnapshot(now)["TEST"].Job
		if got != tc.want {
			t.Fatalf("%s: got %+v, want %+v", tc.body, got, tc.want)
		}
	}
}

func TestNotificationSnapshotKnownAbsenceAndEligibility(t *testing.T) {
	s, now := notificationFarm(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}`, now)
	p := s.NotificationSnapshot(now)["TEST"]
	if !p.StateKnown || !p.PrintKnown || !p.HMSKnown || len(p.Conditions) != 1 {
		t.Fatalf("missing known families: %+v", p)
	}
	for key, condition := range p.Conditions {
		if !condition.Eligible || condition.Family != NotificationPrintFamily {
			t.Fatalf("classification missing: %+v", condition)
		}
		s.Acknowledge(key, true)
	}
	before := s.Reviews()
	for _, condition := range s.NotificationSnapshot(now)["TEST"].Conditions {
		if condition.Eligible {
			t.Fatal("reviewed condition eligible")
		}
	}
	if !reflect.DeepEqual(before, s.Reviews()) {
		t.Fatal("snapshot mutated local reviews")
	}
	for _, body := range []string{`{"print_error":null,"hms":null}`, `{"print_error":false,"hms":[{"attr":"unknown","code":0}]}`} {
		report(t, s, body, now)
		p = s.NotificationSnapshot(now)["TEST"]
		if p.PrintKnown || p.HMSKnown {
			t.Fatal("malformed family became confirmed absence", body)
		}
	}
	report(t, s, `{"print_error":0,"hms":[]}`, now)
	p = s.NotificationSnapshot(now)["TEST"]
	if !p.PrintKnown || !p.HMSKnown || len(p.Conditions) != 0 {
		t.Fatal("explicit clear was not recognized")
	}
	if got := s.NotificationSnapshot(now.Add(3 * time.Minute)); len(got) != 0 {
		t.Fatal("stale snapshot supplied current evidence")
	}
	s.Cloud(false, "", now)
	if got := s.NotificationSnapshot(now); len(got) != 0 {
		t.Fatal("disconnected snapshot supplied current evidence")
	}
}

func TestNotificationJobMetadataRejectsInvalidReportsAtomically(t *testing.T) {
	s, now := notificationFarm(t)
	report(t, s, `{"task_id":"old","gcode_state":"RUNNING"}`, now)
	before := s.NotificationSnapshot(now)["TEST"].Job
	if err := s.Report("TEST", []byte(`{"print":{"task_id":"new","gcode_state":"FINISH"},`), now); err == nil {
		t.Fatal("malformed report accepted")
	}
	if got := s.NotificationSnapshot(now)["TEST"].Job; got != before {
		t.Fatal("rejected telemetry changed job evidence")
	}
}

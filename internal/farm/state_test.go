package farm

import (
	"os"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T, fixture string) (*State, *Printer, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	s := New(10)
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Name: "Printer", Online: true}}, now)
	data, err := os.ReadFile("testdata/" + fixture + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Report("TEST", data, now); err != nil {
		t.Fatal(err)
	}
	return s, s.Printers["TEST"], now
}
func report(t *testing.T, s *State, body string, at time.Time) {
	t.Helper()
	if err := s.Report("TEST", []byte(`{"print":`+body+`}`), at); err != nil {
		t.Fatal(err)
	}
}

func TestDualNozzleFixtures(t *testing.T) {
	t.Run("H2D", func(t *testing.T) {
		s, p, now := setup(t, "h2d")
		if len(p.Nozzles) != 2 || *p.Nozzles[0].Temperature.Current != 219 || *p.Nozzles[1].Temperature.Target != 240 {
			t.Fatalf("nozzles: %+v", p.Nozzles)
		}
		if !p.Nozzles[1].Active || *p.Nozzles[1].AMS != 128 {
			t.Fatal("left nozzle path not decoded")
		}
		if *p.Bed.Current != 60 || *p.Bed.Target != 60 || *p.Chamber.Current != 35 {
			t.Fatal("packed temperatures")
		}
		if len(p.AMS) != 2 || len(p.AMS[0].Slots) != 4 {
			t.Fatal("AMS inventory")
		}
		slots := p.AMS[0].Slots
		if slots[1].Remaining == nil || *slots[1].Remaining != 0 || slots[2].Remaining != nil || *slots[3].Present || slots[3].Remaining != nil {
			t.Fatal("zero, unknown, and empty must differ")
		}
		if p.External[0].Remaining != nil {
			t.Fatal("external estimate is a placeholder")
		}
		if len(s.Active) != 1 {
			t.Fatalf("active notices: %+v", s.Active)
		}
		eta, ok := p.ETA()
		if !ok || !eta.Equal(now.Add(90*time.Minute)) {
			t.Fatal("ETA")
		}
	})
	t.Run("X2D", func(t *testing.T) {
		s, p, _ := setup(t, "x2d")
		if len(p.Nozzles) != 2 || *p.Nozzles[0].AMS != 1 || len(p.External[0].Nozzles) != 1 {
			t.Fatal("dual filament path assignments")
		}
		if len(p.Faults) != 2 || p.Faults[0].Code != "0700_8006" || p.Faults[1].Code != "0300_0100_0001_0007" {
			t.Fatalf("faults: %+v", p.Faults)
		}
		if _, ok := p.ETA(); ok {
			t.Fatal("paused ETA must be unavailable")
		}
		if len(s.Active) != 4 {
			t.Fatalf("active notices: %+v", s.Active)
		}
	})
}
func TestPartialReportsPreserveIdentityAndETA(t *testing.T) {
	s, p, now := setup(t, "h2d")
	eta, _ := p.ETA()
	report(t, s, `{"device":{"extruder":{"info":[{"id":1,"temp":15728878}]}},"ams":{"ams":[{"id":"0","tray":[{"id":"2","remain":15}]}]}}`, now.Add(time.Minute))
	if len(p.Nozzles) != 2 || *p.Nozzles[0].Temperature.Current != 219 || *p.Nozzles[1].Temperature.Current != 238 {
		t.Fatal("partial nozzle merge")
	}
	if len(p.AMS) != 2 || len(p.AMS[0].Slots) != 4 || p.AMS[0].Slots[2].Material != "PLA-CF" {
		t.Fatal("partial AMS merge")
	}
	got, _ := p.ETA()
	if !eta.Equal(got) {
		t.Fatal("unrelated updates drifted ETA")
	}
}
func TestSpoolRemovalAndReplacement(t *testing.T) {
	s, p, now := setup(t, "h2d")
	report(t, s, `{"ams":{"ams":[{"id":"0","tray":[{"id":"0","tray_uuid":"replacement"}]}]}}`, now)
	if p.AMS[0].Slots[0].Remaining != nil {
		t.Fatal("replacement retained previous quantity")
	}
	report(t, s, `{"ams":{"ams":[{"id":"0","tray":[{"id":"1","state":0}]}]}}`, now)
	report(t, s, `{"ams":{"ams":[{"id":"0","tray":[{"id":"1","state":3}]}]}}`, now)
	slot := p.AMS[0].Slots[1]
	if slot.Remaining != nil || slot.Material != "" {
		t.Fatal("reloaded unknown spool inherited old metadata")
	}
	report(t, s, `{"ams":{"ams":[]}}`, now)
	if len(p.AMS) != 0 {
		t.Fatal("explicit empty collection did not clear AMS")
	}
}
func TestFaultClearAndNoticeDedup(t *testing.T) {
	s, p, now := setup(t, "x2d")
	count := len(s.Events)
	for range 5 {
		report(t, s, `{"mc_percent":72}`, now)
	}
	if len(s.Events) != count {
		t.Fatal("duplicate notices")
	}
	report(t, s, `{"hms":[],"print_error":0,"gcode_state":"RUNNING","ams":{"ams":[{"id":1,"tray":[{"id":0,"remain":50}]}]}}`, now)
	if len(p.Faults) != 0 || len(s.Active) != 0 {
		t.Fatalf("conditions did not resolve: %+v", s.Active)
	}
}
func TestStaleAndCloudOfflineAreDistinct(t *testing.T) {
	s, p, now := setup(t, "h2d")
	later := now.Add(121 * time.Second)
	s.Reconcile(later)
	if p.Connection(later, true) != "Stale" {
		t.Fatal("expected stale")
	}
	count := len(s.Events)
	s.Reconcile(later.Add(time.Second))
	if len(s.Events) != count {
		t.Fatal("stale repeated")
	}
	s.Cloud(false, "Internet unavailable", later)
	if p.Connection(later, false) != "Cloud disconnected" {
		t.Fatal("cloud outage called printer offline")
	}
	if p.State != "RUNNING" {
		t.Fatal("time inferred completion")
	}
}
func TestNewJobClearsOldETA(t *testing.T) {
	s, p, now := setup(t, "h2d")
	report(t, s, `{"task_id":"new-job","gcode_state":"RUNNING"}`, now)
	if p.Remaining != nil || p.Progress != nil || p.Job != "" {
		t.Fatal("new job inherited old fields")
	}
	if _, ok := p.ETA(); ok {
		t.Fatal("new job inherited ETA")
	}
}

func TestLegacyEmptySlotAndExternalPath(t *testing.T) {
	s, p, now := setup(t, "h2d")
	report(t, s, `{"ams":{"ams":[{"id":"0","tray":[{"id":"0"}]}]}}`, now)
	if slot := p.AMS[0].Slots[0]; slot.Present == nil || *slot.Present || slot.Remaining != nil {
		t.Fatal("identity-only empty slot retained previous spool")
	}
	legacy := New(10)
	legacy.Discover([]Device{{ID: "TEST"}}, now)
	report(t, legacy, `{"ams":{"tray_now":"254"},"vt_tray":{"id":"254","tray_type":"TPU","remain":0}}`, now)
	p = legacy.Printers["TEST"]
	if len(p.External) != 1 || len(p.External[0].Nozzles) != 1 || p.External[0].Remaining != nil {
		t.Fatal("legacy external spool assignment")
	}
}
func TestRingBufferAndMalformedReports(t *testing.T) {
	s, p, now := setup(t, "h2d")
	for i := range 600 {
		s.add(Event{At: now, Text: strings.Repeat("x", i%3)})
	}
	if len(s.Events) != 500 {
		t.Fatal("unbounded event history")
	}
	if err := s.Report("TEST", []byte(`{"print":`), now); err == nil {
		t.Fatal("malformed JSON accepted")
	}
	if p.State != "RUNNING" {
		t.Fatal("malformed report changed state")
	}
	report(t, s, `{"print_error":4294967295}`, now)
	if p.Faults[0].Code != "FFFF_FFFF" {
		t.Fatal("unknown unsigned error code lost")
	}
}
func FuzzReport(f *testing.F) {
	f.Add([]byte(`{"print":{"mc_percent":20}}`))
	f.Add([]byte(`{"print":{"ams":{"ams":[null,{"id":"0","tray":[{},null]}]}}}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		s := New(10)
		s.Discover([]Device{{ID: "fuzz"}}, time.Now())
		_ = s.Report("fuzz", b, time.Now())
	})
}

func TestAcknowledgementTracksConditionLifetime(t *testing.T) {
	s, p, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	beforeEvents := len(s.Events)
	if !s.Acknowledge(key, true) || !s.Active[key].Acknowledged {
		t.Fatal("acknowledgement not stored")
	}
	report(t, s, `{"mc_percent":73}`, now.Add(time.Minute))
	if !s.Active[key].Acknowledged || !s.Active[key].Since.Equal(now) || len(s.Events) != beforeEvents {
		t.Fatal("routine telemetry reset acknowledgement or duplicated event")
	}
	if len(p.Faults) != 2 {
		t.Fatal("acknowledgement edited printer faults")
	}
	report(t, s, `{"hms":[]}`, now.Add(2*time.Minute))
	if s.Acknowledge(key, true) {
		t.Fatal("acknowledged resolved condition")
	}
	report(t, s, `{"hms":[{"attr":50331904,"code":65543}]}`, now.Add(3*time.Minute))
	if s.Active[key].Acknowledged || !s.Active[key].Since.Equal(now.Add(3*time.Minute)) {
		t.Fatal("recurrence inherited old acknowledgement")
	}
}

func TestClearHistoryPreservesActiveConditionsAndIdentity(t *testing.T) {
	s, _, now := setup(t, "x2d")
	active := len(s.Active)
	lastID := s.Events[len(s.Events)-1].ID
	id := s.Events[0].ID
	if !s.DeleteEvent(id) || s.DeleteEvent(id) {
		t.Fatal("individual event deletion")
	}
	s.ClearEvents()
	s.Reconcile(now)
	if len(s.Events) != 0 || len(s.Active) != active {
		t.Fatal("clear deleted active conditions or regenerated cleared events")
	}
	report(t, s, `{"gcode_state":"RUNNING"}`, now.Add(time.Minute))
	if len(s.Events) == 0 || s.Events[0].ID <= lastID {
		t.Fatal("event IDs reused after history clear")
	}
}

func TestChangedConnectionProblemNeedsReview(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(false, "Reconnecting", now)
	s.Acknowledge("cloud", true)
	s.Cloud(false, "Sign in again", now.Add(time.Second))
	if s.Active["cloud"].Acknowledged || !s.Active["cloud"].Since.Equal(now.Add(time.Second)) {
		t.Fatal("new connection problem inherited acknowledgement")
	}
}

func TestDismissalPersistsUntilResolutionAndRecurrence(t *testing.T) {
	s, p, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	count := s.NoticeCount()
	if !s.Dismiss(key) || !s.Active[key].Dismissed || s.Active[key].Acknowledged {
		t.Fatal("dismissal is not independent of acknowledgement")
	}
	if s.NoticeCount() != count-1 || s.Unacknowledged() != count-1 {
		t.Fatal("dismissed condition still counted")
	}
	for _, e := range s.Events {
		if e.ConditionKey == key {
			t.Fatal("dismissal left matching history entry")
		}
	}
	report(t, s, `{"mc_percent":75}`, now.Add(time.Minute))
	if !s.Active[key].Dismissed || len(p.Faults) != 2 {
		t.Fatal("dismissal was reset or edited telemetry")
	}
	report(t, s, `{"hms":[]}`, now.Add(2*time.Minute))
	for _, e := range s.Events {
		if e.ConditionKey == key {
			t.Fatal("dismissed condition reappeared in history at resolution")
		}
	}
	report(t, s, `{"hms":[{"attr":50331904,"code":65543}]}`, now.Add(3*time.Minute))
	if s.Active[key].Dismissed || s.Active[key].Acknowledged {
		t.Fatal("new occurrence inherited dismissal")
	}
	if s.Dismiss("missing") {
		t.Fatal("dismissed nonexistent condition")
	}
}

func TestDismissedConnectionProblemResurfacesWhenChanged(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(false, "Reconnecting", now)
	s.Dismiss("cloud")
	s.Cloud(false, "Session expired; sign in again", now.Add(time.Second))
	if s.Active["cloud"].Dismissed || s.NoticeCount() != 1 {
		t.Fatal("new connection problem hidden by old dismissal")
	}
}

package farm

import (
	"testing"
	"time"
)

func TestRestoredReviewConditionKnowledge(t *testing.T) {
	const state = "TEST/state"
	const printerFault = "TEST/fault/0700_8006"
	const hmsFault = "TEST/fault/0300_0100_0001_0007"
	const slot = "TEST/low/0/0"
	const neighbor = "TEST/low/0/1"
	const otherUnit = "TEST/low/128/0"
	for _, tc := range []struct {
		name, delta string
		resolved    []string
	}{
		{"omitted families", `{"mc_percent":80}`, nil},
		{"unknown state", `{"gcode_state":null}`, []string{state}},
		{"unknown printer fault", `{"print_error":"unknown"}`, []string{printerFault}},
		{"zero printer fault", `{"print_error":0}`, []string{printerFault}},
		{"empty HMS", `{"hms":[]}`, []string{hmsFault}},
		{"null HMS", `{"hms":null}`, []string{hmsFault}},
		{"missing inventory", `{"ams":{"tray_now":0}}`, nil},
		{"null inventory", `{"ams":{"ams":null}}`, nil},
		{"non-array inventory", `{"ams":{"ams":42}}`, nil},
		{"invalid entries filtered to empty", `{"ams":{"ams":[null,{}]}}`, []string{slot, neighbor, otherUnit}},
		{"explicit empty inventory", `{"ams":{"ams":[]}}`, []string{slot, neighbor, otherUnit}},
		{"other unit", `{"ams":{"ams":[{"id":1,"tray":[]}]}}`, nil},
		{"unit metadata", `{"ams":{"ams":[{"id":0,"humidity_raw":30}]}}`, nil},
		{"null slots", `{"ams":{"ams":[{"id":0,"tray":null}]}}`, nil},
		{"empty slots", `{"ams":{"ams":[{"id":0,"tray":[]}]}}`, []string{slot, neighbor}},
		{"other slot", `{"ams":{"ams":[{"id":0,"tray":[{"id":2,"remain":80}]}]}}`, nil},
		{"slot metadata", `{"ams":{"ams":[{"id":"0","tray":[{"id":"0","state":3,"tray_type":"PLA"}]}]}}`, nil},
		{"scanning spool", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"state":1}]}]}}`, nil},
		{"legacy empty slot", `{"ams":{"ams":[{"id":0,"tray":[{"id":0}]}]}}`, []string{slot}},
		{"empty physical slot", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"state":0}]}]}}`, []string{slot}},
		{"empty material", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"tray_type":"Empty"}]}]}}`, []string{slot}},
		{"null estimate", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":null}]}]}}`, []string{slot}},
		{"unknown estimate", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":-1}]}]}}`, []string{slot}},
		{"refilled slot", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":80}]}]}}`, []string{slot}},
		{"zero estimate remains reviewed", `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":0}]}]}}`, nil},
		{"AMS HT empty", `{"ams":{"ams":[{"id":"128","tray":[]}]}}`, []string{otherUnit}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			saved := map[string]Review{}
			for _, key := range []string{state, printerFault, hmsFault, slot, neighbor, otherUnit} {
				saved[key] = Review{Level: "warning", Since: now, Dismissed: true}
			}
			s := restart(t, saved, now)
			report(t, s, tc.delta, now)
			resolved := map[string]bool{}
			for _, key := range tc.resolved {
				resolved[key] = true
			}
			got := s.Reviews()
			for key, review := range saved {
				value, present := got[key]
				if present == resolved[key] || (present && value != review) {
					t.Errorf("review %s: present=%v, want present=%v; value=%+v", key, present, !resolved[key], value)
				}
			}
			if c, active := s.Active[slot]; active && !c.Dismissed {
				t.Fatal("known ongoing low estimate lost restored dismissal")
			}
		})
	}
}

func TestConditionKnowledgeTracksMergedInventoryReplacement(t *testing.T) {
	now := time.Now()
	key := "TEST/low/0/0"
	saved := map[string]Review{key: {Level: "warning", Since: now, Dismissed: true}}
	s := restart(t, saved, now)
	p := s.Printers["TEST"]
	report(t, s, `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":8,"state":3,"tray_uuid":"first"}]}]}}`, now)
	if !s.Active[key].Dismissed || !p.knowledge.lowFilamentKnown(0, 0) {
		t.Fatal("initial estimate not known/reviewed")
	}
	report(t, s, `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"tray_uuid":"replacement"}]}]}}`, now)
	if p.AMS[0].Slots[0].Remaining != nil || p.knowledge.lowFilamentKnown(0, 0) || len(s.Reviews()) != 1 {
		t.Fatal("replacement inherited estimate or stale knowledge")
	}
	report(t, s, `{"ams":{"ams":[{"id":128,"tray":[{"id":0,"remain":90}]}]}}`, now)
	if p.knowledge.lowFilamentKnown(0, 0) || len(s.Reviews()) != 1 {
		t.Fatal("unrelated identity delta established target estimate")
	}
	report(t, s, `{"ams":{"ams":[]}}`, now)
	if len(s.Reviews()) != 0 || !p.knowledge.lowFilamentKnown(0, 0) {
		t.Fatal("explicit inventory removal not known")
	}
	// A newly reported, incomplete inventory must replace previous empty knowledge.
	s.RestoreReviews(saved)
	report(t, s, `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"state":3}]}]}}`, now)
	if len(s.Reviews()) != 1 || p.knowledge.lowFilamentKnown(0, 0) {
		t.Fatal("new incomplete inventory inherited empty knowledge")
	}
	report(t, s, `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":80}]}]}}`, now)
	if len(s.Reviews()) != 0 {
		t.Fatal("confirmed refill did not resolve pending review")
	}
	report(t, s, `{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":7}]}]}}`, now.Add(time.Minute))
	if s.Active[key].Dismissed {
		t.Fatal("recurring low estimate inherited resolved review")
	}
}

func TestIgnoredReportsDoNotEstablishConditionKnowledge(t *testing.T) {
	now := time.Now()
	for _, payload := range []string{
		`{"info":{"command":"get_version","module":[{"name":"ota","sw_ver":"01.02.03.04"}]}}`,
		`{"print":{"command":"other","gcode_state":"RUNNING","print_error":0,"hms":[],"ams":{"ams":[]}}}`,
		`{"print":null}`,
	} {
		s := New(10)
		s.RestoreReviews(map[string]Review{"TEST/state": {Dismissed: true}, "TEST/fault/0700_8006": {Dismissed: true}, "TEST/fault/0300_0100_0001_0007": {Dismissed: true}, "TEST/low/0/0": {Dismissed: true}})
		s.Cloud(true, "", now)
		s.Discover([]Device{{ID: "TEST", Online: true}}, now)
		if err := s.Report("TEST", []byte(payload), now); err != nil {
			t.Fatal(err)
		}
		s.Reconcile(now.Add(time.Second))
		if len(s.Reviews()) != 4 || !s.Printers["TEST"].LastReport.IsZero() {
			t.Fatalf("ignored report established condition knowledge or freshness: %s", payload)
		}
	}
}

func TestJobRestartNormalizationDoesNotReuseEstimates(t *testing.T) {
	for _, terminal := range []string{"IDLE", "FINISH", "FAILED"} {
		t.Run(terminal, func(t *testing.T) {
			s, p, now := setup(t, "h2d")
			report(t, s, `{"gcode_state":"`+terminal+`"}`, now)
			report(t, s, `{"gcode_state":"RUNNING"}`, now.Add(time.Minute))
			if p.Remaining != nil || p.Progress != nil || p.Layer != nil || p.TotalLayers != nil || p.Job != "" || !p.RemainingReported.IsZero() {
				t.Fatal("restart inherited previous job estimates")
			}
			report(t, s, `{"mc_remaining_time":null,"mc_percent":-1}`, now.Add(2*time.Minute))
			if _, known := p.ETA(); known || p.Remaining != nil || p.Progress != nil {
				t.Fatal("unknown job estimates became known")
			}
		})
	}
}

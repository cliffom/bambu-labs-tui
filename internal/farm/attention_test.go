package farm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAttentionFaultFixtures checks the verified intervention scenarios on both models.
func TestAttentionFaultFixtures(t *testing.T) {
	var fixtures []struct {
		Name, Model string
		Payload     json.RawMessage
	}
	b, err := os.ReadFile("testdata/filament-faults.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			s := New(10)
			now := time.Now()
			s.Cloud(true, "", now)
			s.Discover([]Device{{ID: "TEST", Model: fixture.Model, Online: true}}, now)
			if err := s.Report("TEST", fixture.Payload, now); err != nil {
				t.Fatal(err)
			}
			items := s.Attention()
			if len(items) < 2 {
				t.Fatalf("missing fault and related pause: %+v", items)
			}
			for _, item := range items {
				if item.Guidance == "" || item.Key == "" || item.Since.IsZero() {
					t.Fatal("alert lacks guidance or occurrence identity")
				}
			}
		})
	}
}

// TestAttentionClassification distinguishes intervention from severity or pause alone.
func TestAttentionClassification(t *testing.T) {
	for _, tc := range []struct {
		name, report string
		want         int
	}{
		{"maintenance and low estimate", `{"gcode_state":"RUNNING","print_error":0,"hms":[{"attr":83952640,"code":196610}],"ams":{"ams":[{"id":"0","tray":[{"id":"0","remain":8}]}]}}`, 0},
		{"manual pause", `{"gcode_state":"PAUSE","print_error":50364435,"hms":[]}`, 0},
		{"automatic purge", `{"gcode_state":"PAUSE","print_error":0,"hms":[{"attr":117448704,"code":196609}]}`, 0},
		{"backup switch", `{"gcode_state":"PAUSE","print_error":0,"hms":[{"attr":117448704,"code":196610}]}`, 0},
		{"wire recovery overrides wording", `{"gcode_state":"PAUSE","print_error":0,"hms":[{"attr":117448704,"code":196610,"message":"Replace filament now"}]}`, 0},
		{"wire runout overrides wording", `{"gcode_state":"PAUSE","print_error":0,"hms":[{"attr":117448704,"code":131073,"message":"Purging automatically"}]}`, 2},
		{"failed print", `{"gcode_state":"FAILED"}`, 1},
		{"unknown running error", `{"gcode_state":"RUNNING","print_error":4294967295,"hms":[]}`, 0},
		{"unknown stopped error", `{"gcode_state":"PAUSE","print_error":4294967295,"hms":[]}`, 2},
		{"unknown serious HMS", `{"gcode_state":"PAUSE","print_error":0,"hms":[{"attr":117448704,"code":131071}]}`, 2},
		{"heatbed sensor fault", `{"gcode_state":"RUNNING","print_error":0,"hms":[{"attr":50331904,"code":65543}]}`, 1},
		{"ambiguous pause", `{"gcode_state":"PAUSE","print_error":0,"hms":[]}`, 1},
		{"partial pause awaiting reason", `{"gcode_state":"PAUSE"}`, 0},
		{"runout while running", `{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}`, 1},
		{"retract failure", `{"gcode_state":"PAUSE","print_error":402685956,"hms":[]}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(10)
			now := time.Now()
			s.Cloud(true, "", now)
			s.Discover([]Device{{ID: "TEST", Online: true}}, now)
			report(t, s, tc.report, now)
			if got := len(s.Attention()); got != tc.want {
				t.Fatalf("got %d attention items, want %d: %+v", got, tc.want, s.Attention())
			}
			if strings.HasPrefix(tc.name, "unknown stopped") && s.Attention()[0].Guidance != reviewPrinter {
				t.Fatal("unknown fault guidance guessed its cause")
			}
			if tc.name == "retract failure" && s.Attention()[0].Guidance != checkFilamentPath {
				t.Fatal("verified retraction fault did not use intervention metadata")
			}
		})
	}
}

// TestAttentionRestoredReviewsAndRecurrence preserves independent review lifetimes.
func TestAttentionRestoredReviewsAndRecurrence(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	body := `{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}`
	report(t, s, body, now)
	for _, item := range s.Attention() {
		s.Acknowledge(item.Key, true)
	}
	next := restart(t, s.Reviews(), now.Add(time.Hour))
	report(t, next, `{"gcode_state":"PAUSE"}`, now.Add(time.Hour))
	if len(next.Attention()) != 0 {
		t.Fatal("partial startup report created a premature alert")
	}
	report(t, next, body, now.Add(time.Hour))
	if len(next.Attention()) != 0 {
		t.Fatal("restored decisions reopened the same alert")
	}
	report(t, next, `{"print_error":117473286}`, now.Add(2*time.Hour))
	if len(next.Attention()) != 1 || !strings.HasSuffix(next.Attention()[0].Key, "0700_8006") {
		t.Fatal("a different fault was hidden by old acknowledgement")
	}
	report(t, next, `{"gcode_state":"RUNNING","print_error":0,"hms":[]}`, now.Add(3*time.Hour))
	report(t, next, body, now.Add(4*time.Hour))
	if len(next.Attention()) != 3 {
		t.Fatal("resolved and recurring incident did not need attention again")
	}
}

// TestAttentionCategoriesFollowVerifiedMetadata keeps filters independent of printer wording.
func TestAttentionCategoriesFollowVerifiedMetadata(t *testing.T) {
	cases := []struct{ name, payload, category string }{
		{"runout", `{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}`, "filament"},
		{"unknown pause", `{"gcode_state":"PAUSE","print_error":0,"hms":[]}`, "unknown_pauses"},
		{"failed", `{"gcode_state":"FAILED","print_error":0,"hms":[]}`, "failed_prints"},
		{"extrusion", `{"gcode_state":"RUNNING","print_error":0,"hms":[{"attr":50333952,"code":131073}]}`, "extrusion"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(10)
			now := time.Now()
			s.Cloud(true, "", now)
			s.Discover([]Device{{ID: "TEST", Online: true}}, now)
			report(t, s, tc.payload, now)
			items := s.Attention()
			if len(items) == 0 {
				t.Fatal("expected classified alert")
			}
			for _, item := range items {
				if len(item.Categories) != 1 || item.Categories[0] != tc.category {
					t.Fatalf("wrong category: %+v", item)
				}
			}
		})
	}
}

// Operation-level rules must not expand into slot variants or unknown codes.
func TestFaultActionCatalogueBoundaries(t *testing.T) {
	for _, tc := range []struct {
		code, guidance          string
		known, recovery, manual bool
	}{
		{code: "0700_7000_0002_0007", guidance: replaceFilament, known: true},
		{code: "0700_7100_0002_0007"},
		{code: "1807_7000_0002_0006", guidance: checkExtrusion, known: true},
		{code: "1807_7300_0002_0006"},
		{code: "1807_2300_0002_0009", guidance: checkExtrusion, known: true},
		{code: "0700_2300_0003_0002", known: true, recovery: true},
		{code: "07FF_2000_0002_0008", guidance: checkFilamentPath, known: true},
		{code: "0300_8013", known: true, manual: true},
		{code: "FFFFFFFF"},
		{code: "0700_2000_0002_FFFF"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			got := classifyFaultAction(Fault{Code: tc.code})
			if got.known != tc.known || got.guidance != tc.guidance || got.recovering != tc.recovery || got.userPause != tc.manual {
				t.Fatalf("action = %+v; want guidance %q, known %v, recovery %v, manual %v", got, tc.guidance, tc.known, tc.recovery, tc.manual)
			}
		})
	}
}

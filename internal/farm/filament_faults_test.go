package farm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFilamentFaultFixtures(t *testing.T) {
	var cases []struct {
		Name, Model    string
		Payload        json.RawMessage
		HMSText        string `json:"hms_text"`
		PrintErrorText string `json:"print_error_text"`
	}
	b, err := os.ReadFile("testdata/filament-faults.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			now := time.Now()
			s := New(10)
			s.Cloud(true, "", now)
			s.Discover([]Device{{ID: "TEST", Name: tc.Model, Model: tc.Model, Online: true}}, now)
			if err := s.Report("TEST", tc.Payload, now); err != nil {
				t.Fatal(err)
			}
			p := s.Printers["TEST"]
			foundHMS, foundPrintError := false, tc.PrintErrorText == ""
			for _, f := range p.Faults {
				if strings.Count(f.Code, "_") == 3 {
					foundHMS = strings.HasPrefix(f.Text, tc.HMSText) && f.Level == "error"
				} else {
					foundPrintError = strings.HasPrefix(f.Text, tc.PrintErrorText) && f.Level == "error"
				}
			}
			if !foundHMS || !foundPrintError {
				t.Fatalf("faults did not match expected meanings: %+v", p.Faults)
			}
			if s.Active["TEST/state"].Text != "Print paused" {
				t.Fatal("paused state must remain independently visible")
			}
			if strings.Contains(tc.Name, "runout") && p.Stage != "Filament runout" {
				t.Fatal("runout stage not recognized")
			}
			if tc.Name == "x2d-empty-slot" {
				if !strings.Contains(s.Active["TEST/low/0/0"].Text, "estimated filament 0%") {
					t.Fatal("reported zero must remain an estimate notice")
				}
				if _, ok := s.Active["TEST/low/0/1"]; ok {
					t.Fatal("empty slot must not become a low-estimate notice")
				}
			}
			if tc.Name == "h2d-external-runout" && len(s.Active) != 3 {
				t.Fatal("external placeholder quantity must not generate a low-estimate notice")
			}
		})
	}
}

func TestFilamentMessagesAndFallbacks(t *testing.T) {
	s, _, now := setup(t, "x2d")
	// Unknown units, nonexistent slots, unverified operation/slot combinations,
	// and unknown subcodes must never borrow a nearby known description.
	for _, code := range []string{"0708_2000_0002_0001", "0700_2400_0002_0001", "0700_7100_0002_0003", "1800_7100_0002_0007", "0700_2000_0002_FFFF"} {
		if _, ok := hmsMessages[code]; ok {
			t.Fatalf("unverified code acquired a meaning: %s", code)
		}
	}
	for _, code := range []string{"07088011", "18088006", "07FF8010", "FFFFFFFF"} {
		if _, ok := printErrors[code]; ok {
			t.Fatalf("unverified print error acquired a meaning: %s", code)
		}
	}
	report(t, s, `{"hms":[{"attr":117448704,"code":131073,"message":"Printer supplied runout detail"},{"attr":117448704,"code":196609},{"attr":117448704,"code":196610},{"attr":117448704,"code":131071}],"print_error":4294967295}`, now)
	p := s.Printers["TEST"]
	if p.Faults[0].Text != "Printer error" || p.Faults[0].Code != "FFFF_FFFF" {
		t.Fatal("unknown print-error fallback lost its code")
	}
	if p.Faults[1].Text != "Printer supplied runout detail" {
		t.Fatal("catalogue replaced an explicit printer message")
	}
	if !strings.Contains(p.Faults[2].Text, "purging") || p.Faults[2].Level != "warning" || !strings.Contains(p.Faults[3].Text, "backup-spool") || p.Faults[3].Level != "warning" {
		t.Fatal("automatic runout recovery stages lost their meanings or severity")
	}
	if p.Faults[4].Text != "Unrecognized HMS notice" || p.Faults[4].Code != "0700_2000_0001_FFFF" || p.Faults[4].Level != "error" {
		t.Fatal("unknown HMS fallback lost code or severity")
	}
}

func TestFilamentReviewIdentityAcrossRestart(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	runout := `{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}`
	report(t, s, runout, now)
	hmsKey, errorKey := "TEST/fault/0700_2000_0002_0001", "TEST/fault/0700_8011"
	s.Dismiss(hmsKey)
	s.Acknowledge(errorKey, true)
	next := restart(t, s.Reviews(), now.Add(time.Hour))
	report(t, next, runout, now.Add(time.Hour))
	if !next.Active[hmsKey].Dismissed || !next.Active[errorKey].Acknowledged || next.Active["TEST/state"].Dismissed || next.Active["TEST/state"].Acknowledged {
		t.Fatal("fault decisions were lost or leaked into the independent paused state")
	}
	report(t, next, `{"print_error":117473286,"hms":[{"attr":117448704,"code":131073},{"attr":117510144,"code":131074}]}`, now.Add(2*time.Hour))
	for _, key := range []string{"TEST/fault/0700_8006", "TEST/fault/0701_1000_0002_0002"} {
		c, ok := next.Active[key]
		if !ok || c.Acknowledged || c.Dismissed {
			t.Fatalf("a different fault is absent or inherited a saved decision: %s", key)
		}
	}
	report(t, next, `{"hms":[],"print_error":0,"gcode_state":"RUNNING"}`, now.Add(3*time.Hour))
	report(t, next, runout, now.Add(4*time.Hour))
	if next.Active[hmsKey].Dismissed || next.Active[errorKey].Acknowledged {
		t.Fatal("observed recurrence inherited old decisions")
	}
}

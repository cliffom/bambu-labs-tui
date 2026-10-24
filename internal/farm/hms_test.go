package farm

import (
	"strings"
	"testing"
	"time"
)

func TestHMSSeverityDescriptionAndInternalSuppression(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Cloud(true, "", now)
	s.Discover([]Device{{ID: "TEST", Name: "H2D", Online: true}}, now)
	report(t, s, `{"hms":[{"attr":83886592,"code":196618},{"attr":83952640,"code":196610},{"attr":50331904,"code":65543},{"attr":83886848,"code":131157},{"attr":83886080,"code":262145}]}`, now)
	p := s.Printers["TEST"]
	if len(p.Faults) != 4 {
		t.Fatalf("internal notice not suppressed: %+v", p.Faults)
	}
	levels := map[string]bool{}
	maintenance := false
	for _, f := range p.Faults {
		levels[f.Level] = true
		if strings.Contains(f.Text, "lubricate") {
			maintenance = true
		}
	}
	if !levels["info"] || !levels["warning"] || !levels["error"] {
		t.Fatal("severity classification", levels)
	}
	if !maintenance {
		t.Fatal("maintenance description missing")
	}
}
func TestHMSLevelFallback(t *testing.T) {
	for _, tc := range []struct {
		raw  int
		want string
	}{{1, "error"}, {2, "error"}, {3, "warning"}, {4, "info"}, {9, "warning"}} {
		if got := hmsLevel(tc.raw<<16 | 1); got != tc.want {
			t.Fatalf("%d: %s", tc.raw, got)
		}
	}
}
func TestAMSLabelsPreserveWireIdentity(t *testing.T) {
	for _, tc := range []struct {
		id   int
		want string
	}{{0, "AMS 1"}, {1, "AMS 2"}, {128, "AMS HT 1"}, {135, "AMS HT 8"}} {
		if got := AMSLabel(tc.id); got != tc.want {
			t.Fatal(got)
		}
	}
	_, p, _ := setup(t, "h2d")
	if p.AMS[1].ID != 128 || *p.Nozzles[1].AMS != 128 {
		t.Fatal("display labels changed protocol identities")
	}
}

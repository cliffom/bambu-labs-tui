package farm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCumulativeTelemetryRetentionIsBounded(t *testing.T) {
	for _, kind := range []string{"keys", "identities", "nodes", "bytes"} {
		t.Run(kind, func(t *testing.T) {
			s := New(10)
			now := time.Now()
			s.Discover([]Device{{ID: "TEST", Online: true}}, now)
			rejected := 0
			count := 2000
			if kind == "bytes" {
				count = 4
			}
			for i := 0; i < count; i++ {
				var delta map[string]any
				switch kind {
				case "keys":
					delta = map[string]any{fmt.Sprintf("extra_%d", i): "synthetic"}
				case "identities":
					delta = map[string]any{"ams": map[string]any{"ams": []any{map[string]any{"id": i}}}}
				case "nodes":
					delta = map[string]any{fmt.Sprintf("extra_%d", i): make([]any, 128)}
				case "bytes":
					delta = map[string]any{fmt.Sprintf("extra_%d", i): strings.Repeat("x", maxTelemetryBytes/2)}
				}
				body, err := json.Marshal(map[string]any{"print": delta})
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Report("TEST", body, now); err != nil {
					rejected++
				}
			}
			if rejected == 0 {
				t.Fatal("unbounded cumulative telemetry was accepted")
			}
			p := s.Printers["TEST"]
			if !telemetryWithinLimits(p.Raw) {
				t.Fatal("retained telemetry exceeds limits")
			}
			if kind == "identities" && len(array(object(p.Raw["ams"])["ams"])) != maxTelemetryCollection {
				t.Fatal("identity collection did not fill to its defined limit")
			}
			// Existing values can still change when a collection is full. Explicit
			// empty collections release capacity; rejected additions do not poison it.
			switch kind {
			case "keys":
				report(t, s, `{"extra_0":"updated"}`, now)
			case "identities":
				report(t, s, `{"ams":{"ams":[{"id":0,"humidity_raw":30}]}}`, now)
				report(t, s, `{"ams":{"ams":[]}}`, now)
				report(t, s, `{"ams":{"ams":[{"id":1}]}}`, now)
				if len(p.AMS) != 1 {
					t.Fatal("explicit empty array did not reset retained inventory")
				}
			case "nodes", "bytes":
				report(t, s, `{"extra_0":null}`, now)
			}
		})
	}
}

func TestOversizedTelemetryReportsAreRejected(t *testing.T) {
	manyKeys := map[string]any{}
	for i := 0; i <= maxTelemetryCollection; i++ {
		manyKeys[fmt.Sprint(i)] = nil
	}
	cases := map[string]any{
		"map":   manyKeys,
		"array": make([]any, maxTelemetryCollection+1),
		"nodes": make([]any, 128),
		"depth": nil,
	}
	for i := 0; i < 128; i++ {
		cases["nodes"].([]any)[i] = make([]any, 128)
	}
	for i := 0; i <= maxTelemetryDepth; i++ {
		cases["depth"] = map[string]any{"nested": cases["depth"]}
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			s, p, now := setup(t, "x2d")
			body, err := json.Marshal(map[string]any{"print": map[string]any{"hms": []any{}, "extra": value}})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Report("TEST", body, now.Add(time.Minute)); err == nil {
				t.Fatal("oversized report accepted")
			}
			if !p.LastReport.Equal(now) || len(p.Faults) == 0 {
				t.Fatal("rejected report changed freshness or faults")
			}
		})
	}
	t.Run("wire bytes", func(t *testing.T) {
		s, _, now := setup(t, "x2d")
		payload := []byte(`{"print":{}}` + strings.Repeat(" ", maxTelemetryBytes))
		if err := s.Report("TEST", payload, now); err == nil {
			t.Fatal("oversized JSON payload accepted")
		}
	})
}

func TestRejectedMergePreservesTelemetryAndReviews(t *testing.T) {
	s, p, now := setup(t, "x2d")
	key := "TEST/fault/0300_0100_0001_0007"
	s.Dismiss(key)
	for i := 0; i < maxTelemetryCollection; i++ {
		report(t, s, fmt.Sprintf(`{"extension":{"key_%d":0}}`, i), now)
	}
	before, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	reviews, revision := s.Reviews(), s.ReviewRevision()
	// This envelope is small and valid, but the merged extension exceeds the
	// collection limit. Everything in the envelope must remain unapplied.
	body := []byte(`{"info":{"command":"get_version","module":[{"name":"ota","sw_ver":"99.00.00.00"}]},"print":{"extension":{"new_key":0},"task_id":"new-task","gcode_state":"RUNNING","mc_remaining_time":1,"hms":[],"ams":{"ams":[{"id":1,"tray":[{"id":0,"state":0}]}]}}}`)
	if err := s.Report("TEST", body, now.Add(time.Minute)); err == nil {
		t.Fatal("cumulative limit not enforced")
	}
	after, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) || !reflect.DeepEqual(reviews, s.Reviews()) || revision != s.ReviewRevision() {
		t.Fatal("rejected candidate partially changed telemetry, firmware, history, or reviews")
	}
	// Rejection must not refresh stale telemetry or imply physical resolution.
	if !p.LastReport.Equal(now) || !s.Active[key].Dismissed {
		t.Fatal("rejection refreshed telemetry or cleared a local decision")
	}
	report(t, s, `{"mc_percent":42}`, now.Add(time.Minute))
	if p.Progress == nil || *p.Progress != 42 || !s.Active[key].Dismissed {
		t.Fatal("partial report lost existing telemetry or review")
	}
	report(t, s, `{"hms":[]}`, now.Add(time.Minute))
	if _, exists := s.Active[key]; exists {
		t.Fatal("accepted explicit resolution did not clear fault")
	}
}

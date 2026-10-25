package farm

import (
	"testing"
	"time"
)

func TestStageRespectsJobState(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"idle with printing code", `{"gcode_state":"IDLE","stg_cur":0}`, "Idle"},
		{"idle with negative code", `{"gcode_state":"IDLE","stg_cur":-1}`, "Idle"},
		{"finished with negative code", `{"gcode_state":"FINISH","stg_cur":-1}`, "Finished"},
		{"finished with retained stage", `{"gcode_state":"FINISH"}`, "Finished"},
		{"failed with retained stage", `{"gcode_state":"FAILED"}`, "Failed"},
		{"running", `{"gcode_state":"RUNNING","stg_cur":0}`, "Printing"},
		{"preparing", `{"gcode_state":"PREPARE","stg_cur":2}`, "Preheating bed"},
		{"paused", `{"gcode_state":"PAUSE","stg_cur":6}`, "Filament runout"},
		{"negative active stage", `{"gcode_state":"RUNNING","stg_cur":-1}`, "Unknown"},
		{"unrecognized stage", `{"gcode_state":"RUNNING","stg_cur":999}`, "Stage 999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, now := setup(t, "h2d")
			report(t, s, tc.body, now)
			if p.Stage != tc.want {
				t.Fatalf("stage = %q, want %q", p.Stage, tc.want)
			}
			// Temperature-only deltas must not bring back the stale stage.
			report(t, s, `{"bed_temper":22}`, now.Add(time.Second))
			if p.Stage != tc.want {
				t.Fatalf("stage after partial report = %q, want %q", p.Stage, tc.want)
			}
		})
	}
}

func TestNewJobClearsPreviousStage(t *testing.T) {
	for _, body := range []string{
		`{"task_id":"new-job","gcode_state":"RUNNING"}`,
		`{"gcode_state":"IDLE"}`,
		`{"gcode_state":"FINISH"}`,
		`{"gcode_state":"FAILED"}`,
	} {
		t.Run(body, func(t *testing.T) {
			s, p, now := setup(t, "h2d")
			report(t, s, `{"stg_cur":6}`, now)
			report(t, s, body, now)
			report(t, s, `{"gcode_state":"RUNNING"}`, now)
			if p.Stage != "Unknown" {
				t.Fatalf("new job inherited stage %q", p.Stage)
			}
			report(t, s, `{"stg_cur":7}`, now)
			if p.Stage != "Heating nozzle" {
				t.Fatalf("new job stage = %q", p.Stage)
			}
		})
	}
}

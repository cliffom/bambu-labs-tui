package farm

import "testing"

func TestLiveJobMetadataAcrossPartialStartReports(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reports []string
		jobs    []string
	}{
		{
			name:    "identity and name before running",
			reports: []string{`{"task_id":"next","subtask_name":"Fixture next"}`, `{"gcode_state":"RUNNING","gcode_file":"/data/Metadata/plate_2.gcode"}`, `{"mc_percent":1}`},
			jobs:    []string{"Fixture next", "Fixture next", "Fixture next"},
		},
		{
			name:    "prepare before running",
			reports: []string{`{"gcode_state":"PREPARE","subtask_name":"Fixture next"}`, `{"gcode_state":"RUNNING","gcode_file":"/data/Metadata/plate_2.gcode"}`},
			jobs:    []string{"Fixture next", "Fixture next"},
		},
		{
			name:    "slicing before prepare",
			reports: []string{`{"gcode_state":"SLICING","subtask_name":"Fixture next"}`, `{"gcode_state":"PREPARE"}`, `{"gcode_state":"RUNNING"}`},
			jobs:    []string{"Fixture next", "Fixture next", "Fixture next"},
		},
		{
			name:    "late name replaces file fallback",
			reports: []string{`{"gcode_state":"PREPARE","gcode_file":"/data/Metadata/plate_2.gcode"}`, `{"gcode_state":"RUNNING"}`, `{"subtask_name":"Fixture next"}`, `{"mc_percent":1}`},
			jobs:    []string{"/data/Metadata/plate_2.gcode", "/data/Metadata/plate_2.gcode", "Fixture next", "Fixture next"},
		},
		{
			name:    "new subtask under same task",
			reports: []string{`{"subtask_id":"next","subtask_name":"Fixture next"}`, `{"gcode_state":"RUNNING","gcode_file":"/data/Metadata/plate_2.gcode"}`},
			jobs:    []string{"Fixture next", "Fixture next"},
		},
	} {
		for _, terminal := range []string{"IDLE", "FINISH", "FAILED"} {
			t.Run(tc.name+"/"+terminal, func(t *testing.T) {
				s, p, now := setup(t, "h2d")
				report(t, s, `{"subtask_id":"previous","gcode_file":"previous.gcode","gcode_state":"`+terminal+`"}`, now)
				for i, delta := range tc.reports {
					report(t, s, delta, now)
					if p.Job != tc.jobs[i] {
						t.Fatalf("report %d: job = %q, want %q", i, p.Job, tc.jobs[i])
					}
					if p.Remaining != nil || p.Layer != nil || p.TotalLayers != nil || !p.RemainingReported.IsZero() {
						t.Fatalf("report %d retained previous job estimates", i)
					}
				}
			})
		}
	}
}

func TestNewJobIdentityDoesNotInheritStateOrMetadata(t *testing.T) {
	for _, delta := range []string{`{"task_id":"next"}`, `{"subtask_id":"next"}`} {
		t.Run(delta, func(t *testing.T) {
			s, p, now := setup(t, "h2d")
			report(t, s, `{"subtask_id":"previous"}`, now)
			report(t, s, delta, now)
			if p.Job != "" || p.State != StateUnknown || p.Progress != nil || p.Stage != "Unknown" {
				t.Fatalf("new identity inherited metadata: job=%q state=%q stage=%q progress=%v", p.Job, p.State, p.Stage, p.Progress)
			}
		})
	}
}

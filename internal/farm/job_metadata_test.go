package farm

import (
	"testing"
	"time"
)

func TestJobStartPreservesMetadataThroughDelayedIdentities(t *testing.T) {
	for _, terminal := range []string{"IDLE", "FINISH", "FAILED"} {
		for _, start := range []string{"SLICING", "PREPARE", "RUNNING"} {
			for _, identities := range []struct {
				name    string
				reports []string
			}{
				{"subtask only", []string{`{"subtask_id":"next"}`}},
				{"task then subtask", []string{`{"task_id":"next"}`, `{"subtask_id":"next"}`}},
				{"subtask then task", []string{`{"subtask_id":"next"}`, `{"task_id":"next"}`}},
				{"together", []string{`{"task_id":"next","subtask_id":"next"}`}},
			} {
				t.Run(terminal+"/"+start+"/"+identities.name, func(t *testing.T) {
					s, p, now := setup(t, "h2d")
					report(t, s, `{"subtask_id":"previous","gcode_state":"`+terminal+`"}`, now)
					started := now.Add(time.Minute)
					report(t, s, `{"gcode_state":"`+start+`","subtask_name":"Fixture next","mc_percent":2,"mc_remaining_time":30,"stg_cur":13}`, started)
					for _, delta := range append(identities.reports, `{"gcode_state":"RUNNING","gcode_file":"/data/Metadata/plate_2.gcode"}`) {
						report(t, s, delta, started.Add(time.Second))
						if p.Job != "Fixture next" || p.Remaining == nil || *p.Remaining != 30 || p.Progress == nil || *p.Progress != 2 || p.Stage != "Homing" || !p.RemainingReported.Equal(started) {
							t.Fatalf("delayed identity reset new job metadata after %s: job=%q remaining=%v progress=%v stage=%q reported=%v", delta, p.Job, p.Remaining, p.Progress, p.Stage, p.RemainingReported)
						}
						if p.State == StateUnknown {
							t.Fatal("delayed identity invalidated the observed start state")
						}
					}
					// Completing this job must allow the next start to reset it.
					report(t, s, `{"gcode_state":"FINISH"}`, started.Add(2*time.Second))
					report(t, s, `{"gcode_state":"PREPARE"}`, started.Add(3*time.Second))
					if p.Job != "" || p.Progress != nil || p.Remaining != nil {
						t.Fatal("subsequent job inherited metadata")
					}
				})
			}
		}
	}
}

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
		{
			name:    "subtask and name before task",
			reports: []string{`{"subtask_id":"next","subtask_name":"Fixture next"}`, `{"task_id":"next"}`, `{"gcode_state":"RUNNING"}`},
			jobs:    []string{"Fixture next", "Fixture next", "Fixture next"},
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

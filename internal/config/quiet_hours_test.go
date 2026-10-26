package config

import (
	"testing"
	"time"
)

func TestQuietHoursWallClockBoundaries(t *testing.T) {
	q := QuietHours{Enabled: true, Timezone: "America/New_York", Start: "22:00", End: "07:00", Days: "fri"}
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		at    string
		quiet bool
	}{
		{"2026-10-23T21:59:59-04:00", false},
		{"2026-10-23T22:00:00-04:00", true},
		{"2026-10-24T06:59:59-04:00", true},
		{"2026-10-24T07:00:00-04:00", false},
		{"2026-10-24T22:00:00-04:00", false},
	} {
		t.Run(tc.at, func(t *testing.T) {
			at, err := time.Parse(time.RFC3339, tc.at)
			if err != nil {
				t.Fatal(err)
			}
			if q.Active(at) != tc.quiet {
				t.Fatal("wrong schedule decision")
			}
		})
	}
	q.Start, q.End, q.Days = "01:00", "02:00", "sun"
	for _, at := range []string{"2026-11-01T01:30:00-04:00", "2026-11-01T01:30:00-05:00", "2026-03-08T01:59:59-05:00"} {
		now, err := time.Parse(time.RFC3339, at)
		if err != nil {
			t.Fatal(err)
		}
		if !q.Active(now) {
			t.Fatal("DST repeated/pre-skip minute", at)
		}
	}
	now, _ := time.Parse(time.RFC3339, "2026-03-08T03:00:00-04:00")
	if q.Active(now) {
		t.Fatal("DST exit")
	}
	q.Enabled = false
	if q.Active(now) {
		t.Fatal("opt-in required")
	}
}

func TestQuietHoursValidation(t *testing.T) {
	valid := QuietHours{Enabled: true, Timezone: "UTC", Start: "09:00", End: "17:00", Days: "mon,tue,wed,thu,fri"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*QuietHours){func(q *QuietHours) { q.Timezone = "Local" }, func(q *QuietHours) { q.Timezone = "Mars/Test" }, func(q *QuietHours) { q.Start = "9:00" }, func(q *QuietHours) { q.End = q.Start }, func(q *QuietHours) { q.Days = "monday" }, func(q *QuietHours) { q.Days = "" }} {
		q := valid
		mutate(&q)
		if q.Validate() == nil {
			t.Fatalf("accepted invalid schedule: %+v", q)
		}
	}
}

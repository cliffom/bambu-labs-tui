package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
)

// TestDetailFieldsAlignAndWrap preserves complete values in narrow detail panels.
func TestDetailFieldsAlignAndWrap(t *testing.T) {
	m := noticeModel(t)
	job := strings.Repeat("棚🚀", 12) + "shelf_job.gcode"
	m.current().Job = job
	for _, width := range []int{40, 60, 100} {
		text := ansi.Strip(m.detailText(width))
		lines := strings.Split(text, "\n")
		var jobParts []string
		collect := false
		fields := 0
		for _, line := range lines {
			if ansi.StringWidth(line) > width {
				t.Fatalf("detail line exceeds %d columns: %q", width, line)
			}
			if colon := strings.IndexByte(line, ':'); colon >= 0 && colon < 15 {
				fields++
				if len(line) <= 17 || strings.TrimSpace(line[colon+1:17]) != "" || line[17] == ' ' {
					t.Fatalf("value is not aligned at column 18: %q", line)
				}
				collect = strings.HasPrefix(line, "Job Name:")
			} else if line == "" {
				collect = false
			}
			if collect {
				if len(line) < 17 {
					t.Fatalf("wrapped value lost its indentation: %q", line)
				}
				jobParts = append(jobParts, line[17:])
			}
		}
		if fields < 15 || strings.Join(jobParts, "") != job {
			t.Fatal("missing labelled fields or truncated wrapped job name")
		}
	}
}

// TestFirmwareFieldsDisplayReportedCurrentUpdateUnknownAndStale states the source of the assessment.
func TestFirmwareFieldsDisplayReportedCurrentUpdateUnknownAndStale(t *testing.T) {
	m := demoModel()
	p := m.current()
	p.LastReport = m.now
	available := false
	p.Firmware = farm.Firmware{Version: "01.00.00.00", UpdateAvailable: &available, VersionReported: m.now, StatusReported: m.now}
	if text := ansi.Strip(m.detailText(100)); !strings.Contains(text, "Firmware:        01.00.00.00") || !strings.Contains(text, "Current (printer reported)") {
		t.Fatal("installed firmware or reported assessment missing")
	}
	available = true
	p.Firmware.Available = "01.01.00.00"
	if !strings.Contains(ansi.Strip(m.detailText(100)), "Update available: 01.01.00.00") {
		t.Fatal("target version missing")
	}
	m.Farm.Connected = false
	if !strings.HasPrefix(m.firmwareStatus(p), "Last known:") {
		t.Fatal("disconnected printer displayed an unqualified current assessment")
	}
	m.Farm.Connected = true
	p.Firmware.VersionReported = m.now.Add(-3 * time.Minute)
	if !strings.HasPrefix(m.firmwareStatus(p), "Last known:") {
		t.Fatal("old installed-version report concealed by fresh status telemetry")
	}
	p.Firmware = farm.Firmware{}
	if firmwareVersionText(p) != "Unknown" || m.firmwareStatus(p) != "Unknown (not confirmed)" {
		t.Fatal("missing metadata was treated as current")
	}
}

// TestDetailFinishTime distinguishes a next-day ETA from a paused printer.
func TestDetailFinishTime(t *testing.T) {
	m := demoModel()
	m.now = time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)
	p := m.current()
	p.State, p.LastReport, p.RemainingReported = "RUNNING", m.now, m.now
	remaining := 120
	p.Remaining = &remaining
	text := ansi.Strip(m.detailText(100))
	if !strings.Contains(text, "Sun 25 Oct · 01:30") {
		t.Fatal("next-day finish time lacks its date")
	}
	p.State = "PAUSE"
	text = ansi.Strip(m.detailText(100))
	if strings.Contains(text, "Sun 25 Oct · 01:30") || !strings.Contains(text, "Printer Status:  Paused") {
		t.Fatal("paused printer still shows a finish ETA or lacks its explicit status")
	}
}

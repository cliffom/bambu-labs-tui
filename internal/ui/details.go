package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
)

const detailLabelWidth = 15

// detailField aligns values and indents wrapped continuations to the same column.
func detailField(label, value string, width int, style lipgloss.Style) string {
	prefix := fmt.Sprintf("%-*s  ", detailLabelWidth, label+":")
	indent := ansi.StringWidth(prefix)
	if width <= indent {
		return wrapLines([]string{muted.Render(label + ":"), style.Render(value)}, width)
	}
	rows := strings.Split(ansi.Wrap(value, width-indent, ""), "\n")
	for i, row := range rows {
		lead := strings.Repeat(" ", indent)
		if i == 0 {
			lead = muted.Render(prefix)
		}
		rows[i] = lead + style.Render(row)
	}
	return strings.Join(rows, "\n")
}

// detailText renders labelled printer and filament fields in a shared value column.
func (m Model) detailText(w int) string {
	p := m.current()
	if p == nil {
		return muted.Render("Select a printer once discovery completes.")
	}
	field := func(label, value string) string { return detailField(label, value, w, white) }
	job := safe(p.Job)
	if job == "" {
		job = "No reported job"
	}
	model := safe(p.Model)
	if model == "" {
		model = safe(p.ModelCode)
	}
	if model == "" {
		model = "Unknown"
	}
	stage := safe(p.Stage)
	if stage == "" {
		stage = "Unknown"
	}
	finish := "—"
	if eta, ok := p.ETA(); ok {
		finish = eta.Local().Format("15:04")
		if eta.Local().Format("2006-01-02") != m.now.Local().Format("2006-01-02") {
			finish = eta.Local().Format("Mon 02 Jan · 15:04")
		}
		if p.Stale(m.now) || !p.Online || !m.Farm.Connected {
			finish = "~" + finish
		}
	}
	lines := []string{
		detailField("Printer Name", safe(p.Name), w, accent.Bold(true)),
		field("Printer Model", model),
		field("Firmware", firmwareVersionText(p)),
		field("FW Status", m.firmwareStatus(p)),
		field("Job Name", job),
		field("Printer Status", stateLabel(p.State)),
		field("Connection", p.Connection(m.now, m.Farm.Connected)),
		field("Stage", stage),
		"",
		field("Progress", percent(p.Progress)),
		field("Layer", integer(p.Layer)+" / "+integer(p.TotalLayers)),
		field("Remaining", duration(p.Remaining)),
		field("Finish Time", finish),
		"",
		field("Bed", temp(p.Bed)),
		field("Chamber", temp(p.Chamber)),
	}
	for _, n := range p.Nozzles {
		name := "Right Nozzle"
		if n.ID == 1 {
			name = "Left Nozzle"
		}
		value := temp(n.Temperature)
		if n.Active {
			value += " · active"
		}
		lines = append(lines, field(name, value))
	}
	lastReport := "Waiting for first status"
	if !p.LastReport.IsZero() {
		lastReport = p.LastReport.Local().Format("15:04:05")
	}
	lines = append(lines, detailField("Last Report", lastReport, w, muted), "", accent.Render("FILAMENT · m details"))
	lines = append(lines, m.filamentSummary(p, w)...)
	var visibleFaults []farm.Fault
	for _, f := range p.Faults {
		if c := m.Farm.Active[p.ID+"/fault/"+f.Code]; !c.Dismissed {
			visibleFaults = append(visibleFaults, f)
		}
	}
	if len(visibleFaults) > 0 {
		lines = append(lines, "", warning.Render("PRINTER NOTICES / FAULTS"))
		for _, f := range visibleFaults {
			style := muted
			switch f.Level {
			case "error":
				style = danger
			case "warning":
				style = warning
			}
			lines = append(lines, detailField("Notice", safe(f.Text), w, style), detailField("Fault Code", safe(f.Code), w, muted))
		}
	}
	return wrapLines(lines, w)
}

// firmwareVersionText distinguishes an installed OTA version from missing data.
func firmwareVersionText(p *farm.Printer) string {
	if p.Firmware.Version == "" {
		return "Unknown"
	}
	return safe(p.Firmware.Version)
}

// firmwareStatus describes the printer's assessment rather than a global latest release.
func (m Model) firmwareStatus(p *farm.Printer) string {
	f := p.Firmware
	if f.Version == "" || f.UpdateAvailable == nil {
		return "Unknown (not confirmed)"
	}
	status := "Current (printer reported)"
	if *f.UpdateAvailable {
		status = "Update available: " + safe(f.Available)
	}
	if !m.Farm.Connected || !p.Online || p.Stale(m.now) || m.now.Sub(f.StatusReported) > 120*time.Second || m.now.Sub(f.VersionReported) > 120*time.Second {
		status = "Last known: " + status
	}
	return status
}

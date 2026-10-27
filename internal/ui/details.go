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
func (m Model) detailField(label, value string, width int, style lipgloss.Style) string {
	prefix := fmt.Sprintf("%-*s  ", detailLabelWidth, label+":")
	indent := ansi.StringWidth(prefix)
	if width <= indent {
		return wrapLines([]string{m.styles.secondary.Render(label + ":"), style.Render(value)}, width)
	}
	rows := strings.Split(ansi.Wrap(value, width-indent, ""), "\n")
	for i, row := range rows {
		lead := strings.Repeat(" ", indent)
		if i == 0 {
			lead = m.styles.secondary.Render(prefix)
		}
		rows[i] = lead + style.Render(row)
	}
	return strings.Join(rows, "\n")
}

// detailText renders labelled printer and filament fields in a shared value column.
func (m Model) detailText(w int) string {
	p := m.current()
	if p == nil {
		return m.styles.secondary.Render(m.emptyPrinterList())
	}
	field := func(label, value string) string { return m.detailField(label, value, w, m.styles.text) }
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
		if eta.Local().Format(time.DateOnly) != m.now.Local().Format(time.DateOnly) {
			finish = eta.Local().Format("Mon 02 Jan · 15:04")
		}
		if !m.finishTimeCurrent(p) {
			finish = "~" + finish
		}
	}
	lines := []string{
		m.detailField("Printer Name", safe(p.Name), w, m.styles.accent.Bold(true)),
		field("Printer Model", model),
		field("Firmware", firmwareVersionText(p)),
		field("FW Status", m.firmwareStatus(p)),
		field("Job Name", job),
		field("Printer Status", stateLabel(p.State)),
		field("Connection", p.Connection(m.now, m.Farm.Connected)),
		field("Alerts", m.alertStatus(p.ID)),
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
		lastReport = p.LastReport.Local().Format(time.TimeOnly)
	}
	lines = append(lines, m.detailField("Last Report", lastReport, w, m.styles.secondary), "", m.styles.accent.Render("FILAMENT · m details"))
	lines = append(lines, m.filamentSummary(p, w)...)
	var visibleFaults []farm.Fault
	for _, f := range p.Faults {
		if c := m.Farm.Active[p.ID+"/"+farm.ConditionFaultPrefix+f.Code]; !c.Dismissed {
			visibleFaults = append(visibleFaults, f)
		}
	}
	if len(visibleFaults) > 0 {
		lines = append(lines, "", m.styles.warning.Render("PRINTER NOTICES / FAULTS"))
		for _, f := range visibleFaults {
			style := m.styles.secondary
			switch f.Level {
			case farm.LevelError:
				style = m.styles.err
			case farm.LevelWarning:
				style = m.styles.warning
			}
			lines = append(lines, m.detailField("Notice", safe(f.Text), w, style), m.detailField("Fault Code", safe(f.Code), w, m.styles.secondary))
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
	if !m.firmwareAssessmentCurrent(p) {
		status = "Last known: " + status
	}
	return status
}

// firmwareAssessmentCurrent requires fresh OTA metadata as well as live printer telemetry.
func (m Model) firmwareAssessmentCurrent(p *farm.Printer) bool {
	return m.printerTelemetryCurrent(p) &&
		m.now.Sub(p.Firmware.StatusReported) <= firmwareMaxAge &&
		m.now.Sub(p.Firmware.VersionReported) <= firmwareMaxAge
}

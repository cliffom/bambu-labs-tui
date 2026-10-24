package ui

import (
	"fmt"
	"strings"

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
	lines = append(lines, detailField("Last Report", lastReport, w, muted), "", accent.Render("FILAMENT"))
	if len(p.AMS) == 0 && len(p.External) == 0 {
		lines = append(lines, detailField("Filament", "No data reported", w, muted))
	}
	for _, a := range p.AMS {
		humidity, temperature := "Unknown", "Unknown"
		if a.Humidity != nil {
			humidity = fmt.Sprintf("%d%% RH", *a.Humidity)
		}
		if a.Temperature != nil {
			temperature = fmt.Sprintf("%d°C", *a.Temperature)
		}
		lines = append(lines, detailField("AMS", farm.AMSLabel(a.ID), w, white.Bold(true)), field("Humidity", humidity), field("Temperature", temperature))
		for _, slot := range a.Slots {
			lines = append(lines, field(fmt.Sprintf("Slot %d", slot.ID+1), slotValue(slot)))
		}
	}
	for _, slot := range p.External {
		lines = append(lines, field(fmt.Sprintf("External %d", 256-slot.ID), slotValue(slot)))
	}
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

// slotValue labels filament attributes while preserving empty and unknown states.
func slotValue(s farm.Slot) string {
	if s.Present != nil && !*s.Present {
		return muted.Render("Empty")
	}
	material := safe(s.Material)
	if material == "" {
		material = "Unknown material"
	}
	color := strings.ToUpper(s.Color)
	swatch, hex := "○", "Unknown"
	if len(color) == 8 || len(color) == 6 {
		valid := true
		for _, r := range color {
			if !strings.ContainsRune("0123456789ABCDEF", r) {
				valid = false
			}
		}
		if valid {
			hex = "#" + color[:6]
			swatch = lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("●")
		}
	}
	parts := []string{swatch + " " + material, "Color: " + hex, "Remaining: " + percent(s.Remaining)}
	var nozzles []string
	for _, id := range s.Nozzles {
		side := "Right"
		if id == 1 {
			side = "Left"
		}
		nozzles = append(nozzles, side)
	}
	if len(nozzles) > 0 {
		parts = append(parts, "Nozzle: "+strings.Join(nozzles, "/"))
	}
	return strings.Join(parts, " · ")
}

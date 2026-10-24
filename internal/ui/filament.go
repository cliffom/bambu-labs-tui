package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type filamentUnit struct {
	key, name             string
	id                    int
	humidity, temperature *int
	slots                 []farm.Slot
	external              bool
}

type filamentDialog struct {
	printerID, unitKey string
	slotID             int
	view               viewport.Model
	ensureSelection    bool
	detailsOffset      int
}

// filamentUnits returns stable unit identities and ordered copies of slot lists.
func filamentUnits(p *farm.Printer) []filamentUnit {
	var units []filamentUnit
	if p == nil {
		return units
	}
	for _, unit := range p.AMS {
		slots := append([]farm.Slot(nil), unit.Slots...)
		sort.Slice(slots, func(i, j int) bool { return slots[i].ID < slots[j].ID })
		units = append(units, filamentUnit{key: fmt.Sprintf("ams:%d", unit.ID), name: farm.AMSLabel(unit.ID), id: unit.ID, humidity: unit.Humidity, temperature: unit.Temperature, slots: slots})
	}
	// Sort by numeric wire identity rather than display label (AMS 2 before AMS 10).
	sort.Slice(units, func(i, j int) bool { return units[i].id < units[j].id })
	if len(p.External) > 0 {
		slots := append([]farm.Slot(nil), p.External...)
		sort.Slice(slots, func(i, j int) bool { return slots[i].ID > slots[j].ID })
		units = append(units, filamentUnit{key: "external", name: "External spools", external: true, slots: slots})
	}
	return units
}

// filamentSummary keeps the dashboard independent of the number of reported slots.
func (m Model) filamentSummary(p *farm.Printer, width int) []string {
	if len(p.AMS) == 0 && len(p.External) == 0 {
		return []string{detailField("Inventory", "No data reported", width, muted)}
	}
	regular, ht, low, unknown := 0, 0, 0, 0
	for _, unit := range p.AMS {
		if unit.ID >= 128 && unit.ID <= 135 {
			ht++
		} else {
			regular++
		}
		for _, slot := range unit.Slots {
			if slot.Present != nil && !*slot.Present {
				continue
			}
			if slot.Remaining == nil {
				unknown++
			} else if *slot.Remaining <= m.Farm.LowFilament {
				low++
			}
		}
	}
	var paths []string
	for _, nozzle := range p.Nozzles {
		if !nozzle.Active || nozzle.AMS == nil || nozzle.Slot == nil {
			continue
		}
		side := "Right"
		if nozzle.ID == 1 {
			side = "Left"
		}
		path := fmt.Sprintf("%s slot %d", farm.AMSLabel(*nozzle.AMS), *nozzle.Slot+1)
		if *nozzle.AMS == 254 || *nozzle.AMS == 255 {
			path = fmt.Sprintf("External %d", 256-*nozzle.AMS)
		}
		paths = append(paths, side+": "+path)
	}
	active := "Not reported"
	if len(paths) > 0 {
		active = strings.Join(paths, " · ")
	}
	return []string{
		detailField("Inventory", fmt.Sprintf("%d AMS · %d AMS HT · %d external", regular, ht, len(p.External)), width, white),
		detailField("AMS Estimates", fmt.Sprintf("%d low (≤%d%%) · %d unknown", low, m.Farm.LowFilament, unknown), width, white),
		detailField("Active Path", active, width, white),
	}
}

// openFilament opens the selected printer's inventory without changing dashboard focus.
func (m *Model) openFilament() {
	if p := m.current(); p != nil {
		m.filament = &filamentDialog{printerID: p.ID, slotID: -1, view: viewport.New(1, 1)}
		m.syncFilament()
	}
}

// filamentSelection follows wire identities through reordering, removal, and new reports.
func (d *filamentDialog) filamentSelection(units []filamentUnit) (int, int) {
	if len(units) == 0 {
		d.unitKey, d.slotID = "", -1
		return -1, -1
	}
	unitIndex := 0
	for i, unit := range units {
		if unit.key == d.unitKey {
			unitIndex = i
			break
		}
	}
	unit := units[unitIndex]
	if d.unitKey != unit.key {
		d.unitKey, d.slotID = unit.key, -1
		d.ensureSelection = true
		d.view.GotoTop()
	}
	if len(unit.slots) == 0 {
		d.slotID = -1
		return unitIndex, -1
	}
	for i, slot := range unit.slots {
		if slot.ID == d.slotID {
			return unitIndex, i
		}
	}
	d.slotID = unit.slots[0].ID
	d.ensureSelection = true
	d.view.GotoTop()
	return unitIndex, 0
}

// filamentKey navigates units and slots while keeping quit keys local to the modal.
func (m *Model) filamentKey(key string, msg tea.Msg) tea.Cmd {
	d := m.filament
	if key == "esc" || key == "m" {
		m.filament = nil
		return nil
	}
	if key == "q" || key == "ctrl+c" {
		return nil
	}
	units := filamentUnits(m.Farm.Printers[d.printerID])
	u, s := d.filamentSelection(units)
	if u < 0 {
		return nil
	}
	switch key {
	case "tab", "right", "l", "shift+tab", "left", "h":
		delta := 1
		if key == "shift+tab" || key == "left" || key == "h" {
			delta = len(units) - 1
		}
		d.unitKey, d.slotID = units[(u+delta)%len(units)].key, -1
		d.ensureSelection = true
		d.view.GotoTop()
	case "j", "down", "k", "up", "g", "home", "G", "end":
		if s < 0 {
			return nil
		}
		switch key {
		case "j", "down":
			s++
		case "k", "up":
			s--
		case "g", "home":
			s = 0
		case "G", "end":
			s = len(units[u].slots) - 1
		}
		s = max(0, min(len(units[u].slots)-1, s))
		d.slotID = units[u].slots[s].ID
		d.ensureSelection = true
		d.view.GotoTop()
	case "enter":
		d.view.SetYOffset(d.detailsOffset)
	default:
		var cmd tea.Cmd
		d.view, cmd = d.view.Update(msg)
		return cmd
	}
	return nil
}

// slotName preserves reported external-spool identities alongside ordinary slot numbers.
func slotName(slot farm.Slot, external bool) string {
	if external {
		return fmt.Sprintf("External %d", 256-slot.ID)
	}
	return fmt.Sprintf("Slot %d", slot.ID+1)
}

// syncFilament renders one unit and one selected slot while telemetry keeps flowing.
func (m *Model) syncFilament() {
	d := m.filament
	if d == nil {
		return
	}
	p := m.Farm.Printers[d.printerID]
	units := filamentUnits(p)
	u, s := d.filamentSelection(units)
	d.view.Width, d.view.Height = max(1, m.width-8), max(1, m.height-8)
	var lines []string
	selectedRow := -1
	d.detailsOffset = 0
	if p == nil {
		lines = []string{"This printer is no longer in the account.", "Close the dialog to select another printer."}
	} else {
		field := func(label, value string) string { return detailField(label, value, d.view.Width, white) }
		lines = []string{field("Printer Name", safe(p.Name)), field("Connection", p.Connection(m.now, m.Farm.Connected))}
		if u < 0 {
			lines = append(lines, "", "No AMS or external-spool data reported yet.")
		} else {
			unit := units[u]
			if !unit.external {
				humidity, temperature := "Unknown", "Unknown"
				if unit.humidity != nil {
					humidity = fmt.Sprintf("%d%% RH", *unit.humidity)
				}
				if unit.temperature != nil {
					temperature = fmt.Sprintf("%d°C", *unit.temperature)
				}
				lines = append(lines, field("Humidity", humidity), field("Temperature", temperature))
			}
			materialWidth := max(1, d.view.Width-34)
			lines = append(lines, "", muted.Render(cell("  SLOT", 14)+cell("MATERIAL", materialWidth)+cell("REMAINING", 10)+cell("NOZZLE", 10)))
			for i, slot := range unit.slots {
				marker, material, remaining := "  ", safe(slot.Material), percent(slot.Remaining)
				if material == "" {
					material = "Unknown"
				}
				if slot.Present != nil && !*slot.Present {
					material, remaining = "Empty", "—"
				} else {
					swatch, _ := filamentColor(slot.Color)
					material = swatch + " " + material
				}
				if i == s {
					marker = "› "
					selectedRow = strings.Count(strings.Join(lines, "\n"), "\n") + 1
				}
				material = ansi.Truncate(material, materialWidth, "…")
				material += strings.Repeat(" ", max(0, materialWidth-ansi.StringWidth(material)))
				row := cell(marker+slotName(slot, unit.external), 14) + material + cell(remaining, 10) + cell(slotNozzlePath(slot), 10)
				if i == s {
					row = selected.Render(row)
				}
				lines = append(lines, row)
			}
			if s < 0 {
				lines = append(lines, "No slots reported for this unit.")
			} else {
				slot := unit.slots[s]
				d.detailsOffset = strings.Count(strings.Join(lines, "\n"), "\n") + 2
				lines = append(lines, "", accent.Render(slotName(slot, unit.external)+" DETAILS"), field("Filament", slotValue(slot)))
				if unit.external {
					lines = append(lines, muted.Render("External spool quantity is not reported by the printer."))
				}
			}
		}
	}
	d.view.SetContent(wrapLines(lines, d.view.Width))
	if d.ensureSelection && selectedRow >= 0 {
		if selectedRow >= d.view.YOffset+d.view.Height {
			d.view.SetYOffset(selectedRow - d.view.Height + 1)
		}
		if selectedRow < d.view.YOffset {
			d.view.SetYOffset(selectedRow)
		}
	}
	d.ensureSelection = false
}

// slotNozzlePath shows assignment without implying that an unreported path is empty.
func slotNozzlePath(slot farm.Slot) string {
	var names []string
	for _, id := range slot.Nozzles {
		switch id {
		case 0:
			names = append(names, "Right")
		case 1:
			names = append(names, "Left")
		}
	}
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, "/")
}

// filamentView presents stable unit/slot navigation without expanding the dashboard.
func (m Model) filamentView() string {
	d := m.filament
	units := filamentUnits(m.Farm.Printers[d.printerID])
	unit := "Waiting for inventory"
	for i, candidate := range units {
		if candidate.key == d.unitKey {
			unit = fmt.Sprintf("Unit %d / %d · %s", i+1, len(units), candidate.name)
			break
		}
	}
	footer := "PgUp/PgDn scroll · m / Esc close"
	if m.noticeStorageError != "" {
		footer = ansi.Truncate(m.noticeStorageError, max(1, m.width-8), "…")
	}
	lines := []string{accent.Bold(true).Render("AMS / FILAMENT"), white.Render(unit), muted.Render("Tab/←/→ unit · ↑/↓ slot · Enter details"), "", d.view.View(), "", footer}
	box := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#A3E4BB")).Padding(0, 1).Width(max(1, m.width-4)).Render(strings.Join(lines, "\n"))
	return fit(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box), m.width, m.height)
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
	swatch, hex := filamentColor(s.Color)
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

// filamentColor returns a color chip and label, or an outline for unknown colors.
func filamentColor(value string) (string, string) {
	color := strings.ToUpper(value)
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
	return swatch, hex
}

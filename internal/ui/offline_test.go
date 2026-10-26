package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestOfflineRowsRemainDistinctAcrossLayoutsAndSelection(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	warningSGR := strings.SplitN(warning.Render("Offline"), "m", 2)[0]
	m := overviewModel(t)
	m = attentionReport(t, m, "O", runoutReport)
	m.Farm.Printers["O"].Job = "Previous job"
	for _, width := range []int{60, 80, 120} {
		m.width = width
		for _, connected := range []bool{true, false} {
			m.Farm.Connected = connected
			for _, online := range []bool{false, true} {
				m.Farm.Printers["O"].Online = online
				m.Farm.Printers["O"].Name = "Device"
				m.syncPanels()
				index := slices.Index(m.printerIDs, "O")
				for _, selected := range []bool{true, false} {
					m.selection = -1
					if selected {
						m.selection = index
					}
					row := m.printerRow(index)
					if strings.Contains(row, warningSGR) == online {
						t.Fatalf("width=%d connected=%v online=%v selected=%v: wrong offline color: %q", width, connected, online, selected, row)
					}
					if strings.Contains(row, "48;2;38;59;54") != selected {
						t.Fatalf("selection background lost: %q", row)
					}
					if strings.Contains(ansi.Strip(row), "Offline") == online {
						t.Fatalf("wrong offline label: %q", row)
					}
					if !online {
						want := []string{"Device", "Offline"}
						if selected {
							want = append([]string{"›"}, want...)
						}
						if !slices.Equal(strings.Fields(ansi.Strip(row)), want) {
							t.Fatalf("offline row contains telemetry: %q", row)
						}
					}
					if ansi.StringWidth(row) != width {
						t.Fatalf("row width=%d, want %d", ansi.StringWidth(row), width)
					}
				}
			}
		}
	}
}

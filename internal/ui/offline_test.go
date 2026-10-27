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
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	names, err := BundledThemeNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		theme, err := LoadThemeOverride(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
			lipgloss.SetColorProfile(profile)
			m := overviewModel(t)
			m = m.WithPalette(theme.Palette)
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
							fg, bg := theme.Palette.Text, lipgloss.Color("")
							if !online {
								fg = theme.Palette.Warning
							}
							if selected {
								fg, bg = theme.Palette.SelectedText, theme.Palette.SelectedBackground
							}
							wantPaint := expectedPaint(fg, bg, false)
							for column, cell := range paintedCells(row) {
								if cell != wantPaint {
									t.Fatalf("%s profile=%v width=%d connected=%v online=%v selected=%v column=%d: paint %+v, want %+v", name, profile, width, connected, online, selected, column, cell, wantPaint)
								}
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
								if selected {
									found := false
									for _, line := range strings.Split(m.View(), "\n") {
										if strings.Contains(ansi.Strip(line), "Device") && strings.HasPrefix(ansi.Strip(line), "› ") {
											found = true
											assertPaint(t, line, "Device", fg, bg, false)
											assertPaint(t, line, "Offline", fg, bg, false)
											assertPaint(t, line, "›", fg, bg, false)
										}
									}
									if !found {
										t.Fatal("selected offline row hidden in full view", name, width)
									}
								}
							}
							if profile == termenv.Ascii && strings.Contains(row, "\x1b") {
								t.Fatal("ANSI in no-color printer row")
							}
							if ansi.StringWidth(row) != width {
								t.Fatalf("row width=%d, want %d", ansi.StringWidth(row), width)
							}
						}
					}
				}
			}
		}
	}
}

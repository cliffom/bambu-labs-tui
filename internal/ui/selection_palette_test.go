package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/notifications"
	"github.com/muesli/termenv"
)

func TestSelectedActivityOutcomesUseSelectionPalette(t *testing.T) {
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
		p := theme.Palette
		for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
			lipgloss.SetColorProfile(profile)
			for _, size := range [][2]int{{60, 18}, {120, 32}} {
				for _, outcome := range []struct {
					label string
					color lipgloss.Color
				}{
					{notifications.OutcomeAccepted, p.Success},
					{notifications.OutcomeFailed, p.Error},
					{notifications.OutcomePaused, p.Error},
					{notifications.OutcomeUnconfirmed, p.Warning},
					{notifications.OutcomeSuppressed, p.SecondaryText},
					{notifications.OutcomeStatus, p.SecondaryText},
				} {
					m, client := activityModel(2)
					for i := range client.status.History {
						client.status.History[i].Outcome = outcome.label
					}
					m = resizeModel(m.WithPalette(p), size[0], size[1])
					m = key(m, "N")
					// Move selection to exercise both the newest and an older entry.
					for _, move := range []string{"", "down", "home"} {
						if move != "" {
							m = key(m, move)
						}
						selectedRows, otherRows := 0, 0
						for _, line := range strings.Split(m.View(), "\n") {
							plain := ansi.Strip(line)
							if !strings.Contains(plain, " · Fault 0700_8011 · ") {
								continue
							}
							if strings.HasPrefix(plain, "> ") {
								selectedRows++
								want := expectedPaint(p.SelectedText, p.SelectedBackground, false)
								for column, cell := range paintedCells(line) {
									// Full-view padding is outside the selected row.
									if column >= ansi.StringWidth(strings.TrimRight(plain, " ")) {
										break
									}
									if cell != want {
										t.Fatalf("%s profile=%v size=%v %s column=%d: paint %+v, want %+v", name, profile, size, outcome.label, column, cell, want)
									}
								}
							} else {
								otherRows++
								assertPaint(t, line, outcome.label, outcome.color, p.Background, false)
							}
							if profile == termenv.Ascii && strings.Contains(line, "\x1b") {
								t.Fatal("ANSI in no-color activity row")
							}
						}
						if selectedRows != 1 || otherRows != 1 {
							t.Fatal("populated activity selection was hidden", name, size, outcome.label, selectedRows, otherRows)
						}
					}
				}
			}
		}
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestFooterStaysOneFullWidthRow(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, color := range []termenv.Profile{termenv.TrueColor, termenv.Ascii} {
		lipgloss.SetColorProfile(color)
		for _, size := range [][2]int{{60, 18}, {80, 24}, {120, 32}} {
			m := demoModel()
			m.width, m.height = size[0], size[1]
			m.syncPanels()
			for _, message := range []string{"", "Export saved", strings.Repeat("Long warning ", 20)} {
				m.toast = message
				for _, view := range []string{m.View(), m.fullView([]string{"Notice details"}, "Esc back")} {
					rows := strings.Split(view, "\n")
					if len(rows) != m.height {
						t.Fatalf("size=%v: got %d rows", size, len(rows))
					}
					row := rows[len(rows)-1]
					if ansi.StringWidth(row) != m.width || !strings.HasPrefix(ansi.Strip(row), " ") || !strings.HasSuffix(ansi.Strip(row), " ") {
						t.Fatalf("size=%v: footer bounds/inset: %q", size, row)
					}
					if color == termenv.TrueColor && !strings.Contains(row, "48;2;58;70;81") {
						t.Fatalf("footer background missing: %q", row)
					}
					if color == termenv.Ascii && strings.Contains(row, "\x1b") {
						t.Fatalf("no-color footer contains ANSI: %q", row)
					}
				}
			}
		}
	}
}

func TestFooterHintsEmphasizeKeysAndPreserveLabels(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	m := demoModel()
	for _, focus := range []dashboardPanel{panelPrinters, panelDetails, panelNotices} {
		m.focus = focus
		m.syncPanels()
		rows := strings.Split(m.View(), "\n")
		row := rows[len(rows)-1]
		if !strings.Contains(row, "\x1b[1;") {
			t.Fatalf("focus=%v: footer keys lack bold emphasis: %q", focus, row)
		}
	}
	const hints = "↑/↓ select  tab panel  ?  q quit"
	if got := footerHints(hints); ansi.Strip(got) != hints || !strings.Contains(got, footer.Render(" select")) || !strings.Contains(got, footer.Render(" panel")) {
		t.Fatalf("footer changed labels or emphasized them with keys: %q", got)
	}
}

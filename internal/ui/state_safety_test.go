package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestTelemetryStateCannotInjectTerminalControls(t *testing.T) {
	original := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(original) })
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		for _, tc := range []struct{ state, label string }{
			{"RUNNING", "Printing"},
			{"PAUSE", "Paused"},
			{"FINISH", "Finished"},
			{"FAILED", "Failed"},
			{"IDLE", "Idle"},
			{"", "Unknown"},
			{"FUTURE_STATE", "future_state"},
			{"\x00\r\n", "Unknown"},
			{"\x1b]52;c;aaaa\x07", "]52;c;aaaa"},
			{"\x1b[2J\x1b[HSTATE", "[2j[hstate"},
			{"FUTURE\r\n\t\x00\u009b_STATE", "future_state"},
		} {
			t.Run(fmt.Sprintf("%v/%q", profile, tc.state), func(t *testing.T) {
				m := resizeModel(demoModel(), 120, 32)
				payload, err := json.Marshal(map[string]any{"print": map[string]any{"gcode_state": tc.state}})
				if err != nil {
					t.Fatal(err)
				}
				m = attentionReport(t, m, "H", string(payload))
				// Keep the dashboard visible even for actionable known states.
				m.closeDialog()
				if tc.state != "" && m.current().State != tc.state {
					t.Fatal("display sanitization changed telemetry")
				}
				if got := stateLabel(m.current().State); got != tc.label || strings.ContainsFunc(got, unicode.IsControl) {
					t.Fatalf("unsafe or incorrect label %q, want %q", got, tc.label)
				}
				if !strings.Contains(ansi.Strip(m.detailText(60)), "Printer Status:  "+tc.label) {
					t.Fatal("details lost the reported state label")
				}
				for _, size := range [][2]int{{120, 32}, {60, 18}, {59, 17}} {
					m = resizeModel(m, size[0], size[1])
					view := m.View()
					for _, sequence := range []string{"\x1b]52;", "\x1b[2J", "\x1b[H", "\u009b", "\x00", "\x07", "\r", "\t"} {
						if strings.Contains(view, sequence) {
							t.Fatalf("telemetry controls survived rendering: %q", sequence)
						}
					}
					if profile == termenv.Ascii && strings.Contains(view, "\x1b") {
						t.Fatal("no-color view contains an escape")
					}
					lines := strings.Split(view, "\n")
					if len(lines) != size[1] {
						t.Fatal("height overflow")
					}
					for _, line := range lines {
						if ansi.StringWidth(line) != size[0] {
							t.Fatal("width overflow")
						}
					}
					if tc.state == "FUTURE_STATE" || strings.HasPrefix(tc.state, "\x1b]52;") {
						t.Logf("profile=%v size=%dx%d:\n%s", profile, size[0], size[1], ansi.Strip(view))
					}
				}
			})
		}
	}
}

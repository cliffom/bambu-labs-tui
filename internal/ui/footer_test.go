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
	if got := m.footerHints(hints); ansi.Strip(got) != hints || !strings.Contains(got, newStyles(DefaultPalette()).footer.Render(" select")) || !strings.Contains(got, newStyles(DefaultPalette()).footer.Render(" panel")) {
		t.Fatalf("footer changed labels or emphasized them with keys: %q", got)
	}
}

func TestFooterBackgroundIsContinuousThroughFeedbackSeparators(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, color := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(color)
		want := footerCellBackgrounds(newStyles(DefaultPalette()).footer.Render("x"))[0]
		for _, size := range [][2]int{{60, 18}, {120, 32}} {
			for _, state := range []string{"hints", "notice feedback", "error", "overlapping errors", "Config pending", "Config accepted", "Config dimmed", "Config failed"} {
				m := resizeModel(noticeModel(t), size[0], size[1])
				switch state {
				case "notice feedback":
					m = key(m, "enter")
					m.toast = "Opened Bambu troubleshooting in your browser"
				case "error":
					m.NoticeStorageError = "Could not save notices; decisions may be lost on restart"
				case "overlapping errors":
					m.NoticeStorageError, m.configStorageError = "Could not save notices", "Config save failed"
				case "Config pending", "Config accepted", "Config dimmed", "Config failed":
					m = key(m, ",")
					m.notificationTestStatusAt = m.now
					m.notificationTestStatus = "Test accepted by macOS; check banner and sound."
					m.notificationTestKind = feedbackSuccess
					switch state {
					case "Config pending":
						m.notificationTestPending = true
						m.notificationTestStatus = "Test: Sending… Allow macOS permission if prompted."
					case "Config dimmed":
						m.now = m.now.Add(notificationTestDimAfter)
					case "Config failed":
						m.notificationTestFailed = true
						m.notificationTestStatus = "Test failed; run bambustand doctor --notify."
					}
				}
				rows := strings.Split(m.View(), "\n")
				row := rows[len(rows)-1]
				backgrounds := footerCellBackgrounds(row)
				if len(backgrounds) != size[0] {
					t.Fatalf("%s at %v: got %d footer cells", state, size, len(backgrounds))
				}
				for column, got := range backgrounds {
					if got != want {
						t.Fatalf("%s at %v: column %d background %q, want %q; row %q", state, size, column, got, want, row)
					}
				}
			}
		}
	}
}

// Track the SGR resets and extended colors emitted by Lip Gloss, including
// spaces and punctuation. Merely finding one background escape misses gaps.
func footerCellBackgrounds(row string) []string {
	var cells []string
	var state byte
	background := ""
	for len(row) > 0 {
		seq, width, n, next := ansi.DecodeSequence(row, state, nil)
		row, state = row[n:], next
		if width > 0 {
			for range width {
				cells = append(cells, background)
			}
			continue
		}
		if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
			continue
		}
		params := strings.Split(seq[2:len(seq)-1], ";")
		for i := 0; i < len(params); i++ {
			switch params[i] {
			case "", "0", "49":
				background = ""
			case "38", "48", "58":
				count := 0
				if i+1 < len(params) {
					switch params[i+1] {
					case "2":
						count = 5
					case "5":
						count = 3
					}
				}
				if count > 0 && i+count <= len(params) {
					if params[i] == "48" {
						background = strings.Join(params[i:i+count], ";")
					}
					i += count - 1
				}
			}
		}
	}
	return cells
}

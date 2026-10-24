package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/buildinfo"
)

// TestVersionVisibleInDashboardAndDialogs checks release metadata in every primary view.
func TestVersionVisibleInDashboardAndDialogs(t *testing.T) {
	original := buildinfo.Version
	defer func() { buildinfo.Version = original }()
	buildinfo.Version = "0.1.0-rc1"
	for _, view := range []string{"dashboard", "help", "attention", "filament", "alerts", "decisions", "reset", "inspector", "history", "diagnostics"} {
		t.Run(view, func(t *testing.T) {
			m := noticeModel(t)
			switch view {
			case "help":
				m = key(m, "?")
			case "attention":
				m = attentionTick(attentionReport(t, m, "H", runoutReport))
			case "filament":
				m = key(m, "m")
			case "alerts":
				m = key(m, "s")
			case "decisions":
				m = key(m, "n")
			case "reset":
				m = key(m, "n")
				m.decisions.confirmReset = true
			case "inspector":
				m = key(m, "enter")
			case "history":
				m.confirmClear = true
			case "diagnostics":
				m = key(m, "D")
			}
			for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}} {
				next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				m = next.(Model)
				text := ansi.Strip(m.View())
				if !strings.Contains(text, "v0.1.0-rc1") {
					t.Fatalf("version missing at %v", size)
				}
				for _, line := range strings.Split(text, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("width overflow")
					}
				}
			}
		})
	}
}

// TestCompactHeaderPreservesVersionAndConnection covers disconnected and sign-in-required states.
func TestCompactHeaderPreservesVersionAndConnection(t *testing.T) {
	original := buildinfo.Version
	defer func() { buildinfo.Version = original }()
	buildinfo.Version = "0.0.10"
	m := demoModel()
	m.width = 60
	if header := ansi.Strip(m.dashboardHeader()); !strings.HasPrefix(header, "BAMBU TUI v0.0.10 / DEMO CONNECTED / FARM") || !strings.Contains(header, m.now.Local().Format("15:04:05")) || ansi.StringWidth(header) > 60 {
		t.Fatal("compact header loses version, connection, or clock")
	}
	m.Demo = false
	m.ended = true
	if header := ansi.Strip(m.dashboardHeader()); !strings.HasPrefix(header, "BAMBU TUI v0.0.10 / CLOUD SIGN-IN REQUIRED / FARM") || ansi.StringWidth(header) > 60 {
		t.Fatal("sign-in state displaced version")
	}
	buildinfo.Version = "0.1.0\x1b]52;c;PRIVATE\a\n"
	if strings.Contains(versionLabel(), "\x1b") || strings.Contains(versionLabel(), "\n") {
		t.Fatal("version label allowed terminal controls")
	}
}

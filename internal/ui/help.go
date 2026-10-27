package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) syncHelp() {
	m.help.Width, m.help.Height = max(1, m.width-2), max(1, m.height-4)
	lines := []string{
		"↑ / ↓  j / k   Select printer/notice; scroll printer details",
		"Tab / Shift+Tab Switch between printers, details, and notices",
		"g / G          First / last printer or notice",
		"PgUp / PgDown  Scroll details or page through notices",
		"Enter          Open selected notice; Esc/Enter returns",
		"D              Export local allowlisted diagnostics (confirmation)",
		"F / S          Cycle printer filter / sort",
		"0              Reset view to All printers / Name sorting",
		"f              View selected printer's first active fault",
		"m              Browse selected printer's AMS/filament",
		"n              Manage local notice decisions (restore/reset)",
		"s              Printer alert preferences and snoozing",
		",              Configure general settings, alerts, printers, web server",
		"N              Recent notification outcomes and suppression reasons",
		"A              About Bambu TUI, platform, and theme colors",
		"Ctrl+E         Inspect full status/errors; Esc returns to the same screen",
		"!              Reopen unreviewed action-needed alerts",
		"a              Toggle local acknowledgement of an active notice",
		"c              Dismiss active notice / delete historical event",
		"C              Clear session event history (asks for confirmation)",
		"X              Clear all notices across the farm (confirmation)",
		"o              Open HMS troubleshooting from the detail view",
		"r              Request a fresh printer status snapshot",
		"Esc            Close help and focus printers",
		"q / Ctrl+C     Quit from dashboard (close help first)",
		"", "a marks reviewed. c hides locally; printer state is unchanged.",
		"? or Esc closes help. Scroll to read all controls.",
	}
	m.help.SetContent(ansi.Wrap(strings.Join(lines, "\n"), m.help.Width, ""))
}

func (m Model) helpView() string {
	return m.fullView([]string{m.styles.accent.Bold(true).Render("KEYBOARD"), m.styles.secondary.Render("Dashboard controls · scroll for more"), "", m.help.View()}, "↑/↓ scroll  PgUp/PgDn page  ?/Esc close")
}

func (m *Model) helpKey(key string, msg tea.Msg) tea.Cmd {
	switch key {
	case keyHelp:
		m.closeDialog()
	case keyEscape:
		m.closeDialog()
		m.focus = panelPrinters
	case keyQuit, keyInterrupt:
		return nil
	case keyHome, keyFirst:
		m.help.GotoTop()
	case keyEnd, keyLast:
		m.help.GotoBottom()
	default:
		var cmd tea.Cmd
		m.help, cmd = m.help.Update(msg)
		return cmd
	}
	return nil
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type statusProblem struct {
	name, message, recovery string
}

// Problems keep their source lifetime. Rendering never consumes or clears them.
func (m Model) statusProblems() []statusProblem {
	var problems []statusProblem
	for _, problem := range []statusProblem{
		{"Config", m.configStorageError, "Return to the dashboard, open Config (,), then press r to retry. During editing, finish or cancel the edit first."},
		{"Notices", m.noticeStorageError, "Failed writes retry after another local notice change. Unavailable saved data is preserved; check local storage before restarting."},
		{"Snoozes", m.alertStorageError, "Failed writes retry after another snooze change. Unavailable saved data is preserved; check local storage before restarting."},
		{"Desktop", m.notificationError, "Inspect notification activity (N from the dashboard). An idle paused or standby worker offers r to recheck; consumed attempts are never retried."},
		{"Export", m.exportError, "Return to the dashboard and press D to preview another local export."},
		{"Browser", m.browserError, "Use the full troubleshooting URL in the notice inspector, or press o there to try again."},
	} {
		if problem.message != "" {
			problems = append(problems, problem)
		}
	}
	if m.notificationTestFailed {
		problems = append(problems, statusProblem{"Notification test", m.notificationTestStatus, "Run bambu-tui doctor --notify, or retry Test Notification in Config > Alerts."})
	}
	return problems
}

func (m Model) problemSummary(width int) string {
	problems := m.statusProblems()
	if len(problems) == 0 {
		return ""
	}
	if len(problems) > 1 {
		return fmt.Sprintf("%d errors", len(problems))
	}
	problem := problems[0]
	if ansi.StringWidth(problem.message) <= width {
		return safe(problem.message)
	}
	brief, _, _ := strings.Cut(safe(problem.message), ";")
	if ansi.StringWidth(brief) <= width {
		return brief
	}
	if label := problem.name + " error"; ansi.StringWidth(label) <= width {
		return label
	}
	return "1 error"
}

// Keep essential controls and access to complete details visible, even when a
// long result or several independent failures compete for the single row.
func (m Model) statusFooter(hints, essential string, width int) string {
	const inspect = "^E status"
	width = max(1, width)
	if len(m.statusProblems()) == 0 && m.toast == "" {
		available := max(1, width-ansi.StringWidth(inspect)-2)
		if ansi.StringWidth(hints) > available {
			hints = essential
		}
		return footerHints(hints + "  " + inspect)
	}
	suffix := essential + "  " + inspect
	available := max(1, width-ansi.StringWidth(suffix)-3)
	message := safe(m.toast)
	style := muted
	if len(m.statusProblems()) > 0 {
		message = m.problemSummary(available)
		style = danger
		if !m.hasOpenDialog() {
			style = warning
		}
	}
	return style.Render(ansi.Truncate(message, available, "…")) + " · " + footerHints(suffix)
}

func (m *Model) openStatus() {
	m.openDialog(dialogStatus)
	m.status = viewport.New(1, 1)
	m.syncStatus()
}

func (m *Model) statusKey(key string, msg tea.Msg) tea.Cmd {
	if key == keyEscape || key == keyStatus {
		m.closeDialog()
		return nil
	}
	if key == keyHome || key == keyFirst {
		m.status.GotoTop()
		return nil
	}
	if key == keyEnd || key == keyLast {
		m.status.GotoBottom()
		return nil
	}
	var cmd tea.Cmd
	m.status, cmd = m.status.Update(msg)
	return cmd
}

func (m *Model) syncStatus() {
	m.status.Width, m.status.Height = max(1, m.width-2), max(1, m.height-4)
	lines := []string{"Local feedback only. Printer faults remain in Notices.", ""}
	problems := m.statusProblems()
	if len(problems) == 0 {
		lines = append(lines, "No current storage, export, browser, or delivery errors.")
	}
	for _, problem := range problems {
		lines = append(lines, danger.Bold(true).Render(problem.name+" error"), safe(problem.message), problem.recovery, "")
	}
	if m.notificationTestStatus != "" && !m.notificationTestFailed {
		lines = append(lines, "Last notification test: "+m.notificationTestStatus, "Acceptance does not confirm a banner or sound.", "")
	}
	if m.toast != "" {
		lines = append(lines, "Recent feedback: "+safe(m.toast))
	}
	m.status.SetContent(ansi.Wrap(strings.Join(lines, "\n"), m.status.Width, ""))
}

func (m Model) statusView() string {
	return m.frame([]string{accent.Bold(true).Render("STATUS / FEEDBACK"), muted.Render("↑/↓ or PgUp/PgDn scroll · Esc / Ctrl+E back"), "", m.status.View()}, footerHints("Esc back  ^E close"))
}

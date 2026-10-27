package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

// Five header rows, one spacer, four detail rows, and the footer.
const activityReservedRows = 11

type notificationActivityDialog struct {
	entries  []notifications.Activity
	selected uint64
	index    int
	follow   bool
}

func (m *Model) openNotificationActivity() {
	m.openDialog(dialogNotificationActivity)
	m.notificationActivity = &notificationActivityDialog{follow: true}
	m.syncNotificationActivity()
}

func (m *Model) syncNotificationActivity() {
	d := m.notificationActivity
	if d == nil {
		return
	}
	d.entries = nil
	if m.notificationClient != nil {
		d.entries = slices.Clone(m.notificationClient.Status().History)
		slices.Reverse(d.entries)
	}
	if len(d.entries) == 0 {
		d.index, d.selected = 0, 0
		return
	}
	if d.follow {
		d.index = 0
	} else {
		d.index = len(d.entries) - 1 // Oldest retained entry if selection was evicted.
		for i, entry := range d.entries {
			if entry.ID == d.selected {
				d.index = i
				break
			}
		}
	}
	d.selected = d.entries[d.index].ID
}

func (m *Model) notificationActivityKey(key string, _ tea.Msg) tea.Cmd {
	d := m.notificationActivity
	page := max(1, m.height-activityReservedRows)
	switch key {
	case keyEscape, keyNotificationActivity:
		m.closeDialog()
		return nil
	case keyRefreshOrRestore:
		if m.notificationRecoveryAvailable() {
			// Publish current policy/telemetry before requesting recovery.
			m.syncNotifications()
			m.notificationClient.Retry()
		}
		return nil
	case keyUp, keyVimUp:
		d.index--
	case keyDown, keyVimDown:
		d.index++
	case keyPageUp:
		d.index -= page
	case keyPageDown:
		d.index += page
	case keyHome, keyFirst:
		d.index, d.follow = 0, true
	case keyEnd, keyLast:
		d.index, d.follow = len(d.entries)-1, false
	default:
		return nil
	}
	if key != keyHome && key != keyFirst {
		d.follow = false
	}
	if len(d.entries) > 0 {
		d.index = max(0, min(len(d.entries)-1, d.index))
		d.selected = d.entries[d.index].ID
	}
	return nil
}

func (m Model) activityOutcome(entry notifications.Activity) string {
	switch entry.Outcome {
	case notifications.OutcomeAccepted:
		return m.styles.success.Render(entry.Outcome)
	case notifications.OutcomeFailed, notifications.OutcomePaused:
		return m.styles.err.Render(entry.Outcome)
	case notifications.OutcomeUnconfirmed:
		return m.styles.warning.Render(entry.Outcome)
	default:
		return m.styles.secondary.Render(entry.Outcome)
	}
}

func (m Model) notificationActivityView() string {
	d := m.notificationActivity
	width := max(1, m.width-2)
	hints := "↑/↓ select  PgUp/PgDn page  N/Esc close"
	if m.notificationRecoveryAvailable() {
		hints = "↑/↓ select  PgUp/PgDn  r recheck  N/Esc close"
	}
	hint := m.headerHints(hints)
	status := notifications.Status{}
	if m.notificationClient != nil {
		status = m.notificationClient.Status()
	}
	summary := fmt.Sprintf("Accepted %d · Failed %d · Suppressed %d", status.Accepted, status.Failed, status.Suppressed)
	ownership := notifications.OwnershipText(status)
	ownerStyle := m.styles.secondary
	if status.Ownership == notifications.OwnershipOwner {
		ownerStyle = m.styles.info
	}
	if status.Ownership == notifications.OwnershipStandby {
		ownerStyle = m.styles.info
	}
	if status.Disabled {
		ownerStyle = m.styles.err
	}
	if m.notificationClient == nil {
		ownership = "Desktop delivery unavailable in this session"
	}
	lines := []string{m.styles.accent.Bold(true).Render("NOTIFICATION ACTIVITY"), hint, fmt.Sprintf("Last %d decisions · this session only · newest first", notifications.ActivityLimit), summary, ownerStyle.Render(ansi.Truncate(ownership, width, "…"))}
	visible := max(1, m.height-activityReservedRows)
	if len(d.entries) == 0 {
		lines = append(lines, "No live notification decisions yet.")
		explanation := "Waiting for evaluated incidents or print completions."
		if m.notificationSetupError != "" {
			explanation = m.notificationSetupError
		} else if m.notificationClient == nil {
			explanation = "No live desktop notification worker in this session."
		} else if status.Ownership == notifications.OwnershipStandby {
			explanation = "Monitoring continues. Takeover follows owner exit."
		} else if status.Disabled {
			explanation = "Delivery is stopping; recheck when idle."
			if m.notificationRecoveryAvailable() {
				explanation = "Fix the reported cause, then press r to check existing state."
			}
		} else if !m.alertPrefs.DesktopEnabled() {
			explanation = "Desktop delivery is off. Enable it in Config > Alerts."
		}
		lines = append(lines, strings.Split(ansi.Wrap(explanation, width, ""), "\n")...)
	} else {
		start := max(0, d.index-visible+1)
		for i := start; i < min(len(d.entries), start+visible); i++ {
			entry := d.entries[i]
			marker := "  "
			if i == d.index {
				marker = "> "
			}
			// Reserve the outcome column so long/untrusted printer names cannot hide it.
			subject := ansi.Truncate(safe(entry.Subject), max(12, width/3), "…")
			suffix := " · " + subject + " · " + m.activityOutcome(entry)
			prefix := marker + entry.At.Local().Format("15:04:05") + " " + safe(entry.Printer)
			row := ansi.Truncate(prefix, max(1, width-ansi.StringWidth(suffix)), "…") + suffix
			if i == d.index {
				row = m.styles.selection.Render(row)
			}
			lines = append(lines, row)
		}
	}
	for len(lines) < m.height-6 {
		lines = append(lines, "")
	}
	lines = append(lines, "")
	if len(d.entries) > 0 {
		entry := d.entries[d.index]
		detail := safe(entry.Channel) + " · " + entry.Outcome + " · " + safe(entry.Subject)
		lines = append(lines, ansi.Truncate(detail, width, "…"))
		reason := strings.Split(ansi.Wrap(notifications.ReasonText(entry.Reason), width, ""), "\n")
		for i := range 2 {
			if i < len(reason) {
				lines = append(lines, reason[i])
			} else {
				lines = append(lines, "")
			}
		}
		extra := fmt.Sprintf("Seen %d · first %s", entry.Repeats, entry.FirstAt.Local().Format("Jan 02 15:04:05"))
		if entry.GroupSize > 1 {
			extra = fmt.Sprintf("Request #%d · %d notices grouped · seen %d", entry.Request, entry.GroupSize, entry.Repeats)
		}
		lines = append(lines, ansi.Truncate(extra, width, "…"))
	} else {
		lines = append(lines, "History is held in memory and clears on restart.", "Config tests and doctor are separate from live activity.", "Accepted does not confirm a visible banner or sound.", "No automatic retries of failed or unconfirmed attempts.")
	}
	return m.fullView(lines, "Home newest  End oldest  N/Esc close")
}

// Recovery checks worker state; it never retries an accepted or consumed attempt.
func (m Model) notificationRecoveryAvailable() bool {
	if m.notificationClient == nil || m.ended {
		return false
	}
	status := m.notificationClient.Status()
	return !status.Pending && !status.Recovering && (status.Disabled || status.Ownership == notifications.OwnershipStandby)
}

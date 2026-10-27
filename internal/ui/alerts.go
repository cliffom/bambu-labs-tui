package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

type alertDialog struct {
	printerID string
	view      viewport.Model
}

// WithAlertPreferences installs validated configuration without changing notice reviews.
func (m Model) WithAlertPreferences(prefs config.AlertPreferences) Model {
	m.alertPrefs = prefs
	m.syncPanels()
	return m
}

// WithSnoozePersistence restores deadlines; failed loads preserve the original file.
func (m Model) WithSnoozePersistence(snoozes map[string]time.Time, save func(map[string]time.Time) error, loadErr error) Model {
	m.snoozes = make(map[string]time.Time, len(snoozes))
	for id, until := range snoozes {
		m.snoozes[id] = until
	}
	m.saveSnoozes = nil
	if loadErr != nil {
		m.alertStorageError = "Saved snoozes unavailable; changes apply only to this session"
	} else if !m.Demo {
		m.saveSnoozes = save
	}
	return m
}

// automaticAlertAllowed applies interruption preferences after verified classification.
func (m Model) automaticAlertAllowed(item farm.Attention) bool {
	return !m.snoozed(item.PrinterID) && m.alertPrefs.RulesFor(item.PrinterID).Allows(item.Categories)
}

// snoozed checks a deadline rather than relying on the next expiry cleanup tick.
func (m Model) snoozed(id string) bool {
	return m.now.Before(m.snoozes[id])
}

// rearmPrinterAlerts removes only a printer's session prompt suppression.
func (m *Model) rearmPrinterAlerts(id string) {
	for occurrence := range m.attentionSeen {
		if strings.HasPrefix(occurrence.key, id+"/") {
			delete(m.attentionSeen, occurrence)
		}
	}
}

// pruneSnoozes expires deadlines and drops removed printers after successful discovery.
func (m *Model) pruneSnoozes() {
	for id, until := range m.snoozes {
		_, present := m.Farm.Printers[id]
		if !m.now.Before(until) || (m.printersDiscovered && !present) {
			delete(m.snoozes, id)
			m.rearmPrinterAlerts(id)
			m.snoozesDirty = true
		}
	}
}

// setSnooze changes a printer deadline without acknowledging, dismissing, or hiding notices.
func (m *Model) setSnooze(id string, duration time.Duration) {
	if _, exists := m.Farm.Printers[id]; !exists {
		return
	}
	if m.snoozes == nil {
		m.snoozes = map[string]time.Time{}
	}
	if duration > 0 {
		m.snoozes[id] = m.now.Add(duration)
	} else {
		delete(m.snoozes, id)
	}
	m.snoozesDirty = true
	m.rearmPrinterAlerts(id)
	if duration > 0 && m.attention != nil {
		// Remove only the snoozed printer; keep other printers queued for review.
		groups := m.attention.groups[:0]
		for _, group := range m.attention.groups {
			if group.id != id {
				groups = append(groups, group)
			}
		}
		m.attention.groups = groups
		if len(groups) == 0 {
			m.dismissDialog(dialogAttention)
		} else {
			m.attention.index = min(m.attention.index, len(groups)-1)
			m.attention.view.GotoTop()
		}
	}
}

// openAlerts opens per-printer interruption status and snooze controls.
func (m *Model) openAlerts(id string) {
	m.openDialog(dialogAlerts)
	m.alertDialog = &alertDialog{printerID: id, view: viewport.New(1, 1)}
	m.syncAlertDialog()
}

// alertKey captures modal input and applies explicit snooze/resume actions.
func (m *Model) alertKey(key string, msg tea.Msg) tea.Cmd {
	switch key {
	case keyAlerts, keyEscape:
		m.closeDialog()
	case keyQuit, keyInterrupt:
		return nil
	case keySnoozeShort, keySnoozeHour, keySnoozeLong, keyUnreviewOrUnsnooze:
		durations := map[string]time.Duration{keySnoozeShort: snoozeShort, keySnoozeHour: snoozeHour, keySnoozeLong: snoozeLong}
		m.setSnooze(m.alertDialog.printerID, durations[key])
		m.closeDialog()
	default:
		var cmd tea.Cmd
		m.alertDialog.view, cmd = m.alertDialog.view.Update(msg)
		return cmd
	}
	return nil
}

// alertStatus summarizes automatic interruptions without changing the printer status.
func (m Model) alertStatus(id string) string {
	if m.snoozed(id) {
		return "Snoozed until " + m.snoozes[id].Local().Format("Mon 02 Jan 15:04")
	}
	rules := m.alertPrefs.RulesFor(id)
	if !rules.AutomaticModals {
		return "Automatic modals off"
	}
	if !rules.AnyCategoryEnabled() {
		return "All automatic categories off"
	}
	return "Automatic modals on · s preferences"
}

// syncAlertDialog shows effective settings and stable IDs for configuration overrides.
func (m *Model) syncAlertDialog() {
	d := m.alertDialog
	if d == nil {
		return
	}
	d.view.Width, d.view.Height = max(1, m.width-2), max(1, m.height-6)
	p := m.Farm.Printers[d.printerID]
	if p == nil {
		d.view.SetContent("Printer no longer in this account.\nPress Esc to close.")
		return
	}
	rules := m.alertPrefs.RulesFor(p.ID)
	label := func(enabled bool) string {
		if enabled {
			return "On"
		}
		return "Off"
	}
	field := func(name, value string) string { return m.detailField(name, safe(value), d.view.Width, m.styles.text) }
	lines := []string{
		field("Printer", p.Name), field("Printer ID", p.ID), field("Alerts", m.alertStatus(p.ID)), "",
		field("Desktop", label(rules.DesktopNotifications)), field("Sound", label(rules.NotificationSound)), field("Completion", label(rules.CompletedPrints)),
		field("Auto Modals", label(rules.AutomaticModals)), field("Filament", label(rules.Filament)), field("Extrusion", label(rules.Extrusion)),
		field("Printer Faults", label(rules.PrinterFaults)), field("Failed Prints", label(rules.FailedPrints)), field("Unknown Pauses", label(rules.UnknownPauses)), "",
		", opens Config from the dashboard: General, Alerts, and Printers. Changes apply immediately except region (restart/login).",
		"1: snooze 15 minutes · 2: 1 hour · 3: 4 hours", "u: end snooze and re-arm eligible alerts", "Snoozing keeps all printer state and notices visible.",
		"Expiry re-arms eligible unreviewed alerts, including new faults during the snooze.", "!: manually inspect alerts from the dashboard, even while muted or snoozed.", "Desktop delivery is macOS-only and requires this TUI session. Sound follows notification_sound.",
	}
	if m.notificationClient != nil {
		status := m.notificationClient.Status()
		lines = append(lines, "N from the dashboard opens recent notification activity.", "Delivery counts cover this entire TUI session.", field("Owner", notifications.OwnershipText(status)), field("Delivery", status.Last), field("Accepted", fmt.Sprint(status.Accepted)), field("Failed", fmt.Sprint(status.Failed)), field("Suppressed", fmt.Sprint(status.Suppressed)))
	}
	if m.Demo || m.saveSnoozes == nil {
		lines = append(lines, "Snooze changes apply only to this session.")
	}
	d.view.SetContent(wrapLines(lines, d.view.Width))
}

// alertView renders interruption controls with persistent storage warnings.
func (m Model) alertView() string {
	return m.fullView([]string{m.styles.accent.Bold(true).Render("PRINTER ALERTS / SNOOZE"), "1: 15m · 2: 1h · 3: 4h · u: resume · s / Esc close", "", m.alertDialog.view.View()}, "PgUp/PgDn scroll  s/Esc close")
}

// storageWarning keeps storage and desktop delivery failures visible in every view.
func (m Model) storageWarning() string {
	if m.noticeStorageError != "" && m.alertStorageError != "" && m.notificationError == "" && m.configStorageError == "" {
		return "Notice and snooze storage unavailable; changes may not persist"
	}
	var warnings []string
	if m.configStorageError != "" {
		warnings = append(warnings, m.configStorageError)
	}
	if m.noticeStorageError != "" {
		warnings = append(warnings, m.noticeStorageError)
	}
	if m.alertStorageError != "" {
		warnings = append(warnings, m.alertStorageError)
	}
	if m.notificationError != "" {
		warnings = append(warnings, m.notificationError)
	}
	return strings.Join(warnings, " · ")
}

// alertSummary identifies muted/snoozed printers in the dashboard overview.
func (m Model) alertSummary() string {
	snoozed, muted := 0, 0
	for id := range m.Farm.Printers {
		if m.snoozed(id) {
			snoozed++
		} else if !m.alertPrefs.RulesFor(id).AnyCategoryEnabled() {
			muted++
		}
	}
	if snoozed == 0 && muted == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d snoozed · %d muted", snoozed, muted)
}

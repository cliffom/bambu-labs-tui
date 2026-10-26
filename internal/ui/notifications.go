package ui

import (
	"github.com/cliffom/bambu-tui/internal/notifications"
)

type notificationClient interface {
	Submit(notifications.Input)
	Status() notifications.Status
	Stop()
	Retry() bool
}

func (m Model) WithNotifications(client notificationClient, setupErr error) Model {
	m.notificationClient = client
	if setupErr != nil {
		m.notificationSetupError = "Desktop notifications unavailable; requires local macOS and writable identity storage"
		if m.alertPrefs.DesktopEnabled() {
			m.notificationError = m.notificationSetupError
		}
	}
	return m
}

func (m *Model) syncNotifications() {
	enabled := m.alertPrefs.DesktopEnabled()
	if m.notificationClient == nil {
		m.notificationError = ""
		if enabled {
			m.notificationError = m.notificationSetupError
		}
		return
	}
	if !enabled && !m.notificationStarted {
		return
	}
	m.notificationStarted = true
	snapshot := m.Farm.NotificationSnapshot(m.now)
	policies := map[string]notifications.Policy{}
	allowed := map[string]bool{}
	for _, item := range m.Farm.Attention() {
		allowed[item.Key] = m.alertPrefs.RulesFor(item.PrinterID).AllowsNotification(item.Categories)
	}
	for id := range snapshot {
		rules := m.alertPrefs.RulesFor(id)
		policies[id] = notifications.Policy{PrinterName: m.Farm.Printers[id].Name, Desktop: rules.DesktopNotifications, Completion: rules.CompletedPrints, Sound: rules.NotificationSound, Snoozed: m.snoozed(id), Allowed: allowed}
	}
	m.notificationClient.Submit(notifications.Input{Snapshot: snapshot, Policies: policies, Active: !m.quitting && !m.quitProblem && !m.ended, Demo: m.Demo, ObserveOnly: !enabled})
	status := m.notificationClient.Status()
	m.notificationError = status.Error
	if status.Revision != m.notificationRevision {
		m.notificationRevision = status.Revision
		m.toast, m.toastAt = status.Last, m.now
	}
}

func (m Model) stopNotifications() {
	if m.notificationClient != nil {
		m.notificationClient.Stop()
	}
}

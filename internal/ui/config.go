package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
)

const (
	notificationSettingCount = 3
	notificationTestAction   = notificationSettingCount
	configActionCount        = notificationSettingCount + 1
)

type configResultMsg struct {
	revision uint64
	err      error
}

// WithConfigPersistence writes only global notification defaults to the selected
// configuration file. The command owns a value snapshot, never the live model.
func (m Model) WithConfigPersistence(path string, save func(config.AlertRules) error) Model {
	m.configPath, m.saveConfig = path, save
	return m
}

func (m *Model) configKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyEscape, keyConfig:
		m.closeDialog()
	case keyUp, keyVimUp:
		m.configSelection = (m.configSelection + configActionCount - 1) % configActionCount
	case keyDown, keyVimDown:
		m.configSelection = (m.configSelection + 1) % configActionCount
	case keyEnter, " ":
		if m.configSelection == notificationTestAction {
			return m.startNotificationTest()
		}
		settings := []*bool{&m.alertPrefs.DesktopNotifications, &m.alertPrefs.NotificationSound, &m.alertPrefs.CompletedPrints}
		value := settings[m.configSelection]
		*value = !*value
		m.configRevision++
	case keyRefreshOrRestore:
		if m.configWrites.failed != 0 && !m.configWrites.inFlight {
			m.configRevision++
		}
	}
	return nil
}

func (m *Model) persistConfig() tea.Cmd {
	if m.saveConfig == nil || m.configWrites.inFlight || m.configWrites.attempted == m.configRevision {
		return nil
	}
	revision, snapshot, save := m.configRevision, m.alertPrefs.AlertRules, m.saveConfig
	m.configWrites.inFlight, m.configWrites.attempted = true, revision
	return func() tea.Msg { return configResultMsg{revision: revision, err: save(snapshot)} }
}

func (m *Model) configResult(msg configResultMsg) {
	if !m.configWrites.inFlight || msg.revision != m.configWrites.attempted {
		return
	}
	m.configWrites.inFlight = false
	if msg.err != nil {
		m.configWrites.failed = msg.revision
		m.configStorageError = "Config save failed; settings apply this session. Open , and r to retry"
	} else {
		m.configWrites.saved, m.configWrites.failed = msg.revision, 0
		m.configStorageError = ""
	}
}

func (m Model) configView() string {
	bindings := []string{
		white.Bold(true).Render("↑/↓") + muted.Render(" select"),
		white.Bold(true).Render("Space/Enter") + muted.Render(" change/test"),
		white.Bold(true).Render("Esc / ,") + muted.Render(" close"),
	}
	lines := []string{accent.Bold(true).Render("CONFIG / NOTIFICATIONS"), strings.Join(bindings, muted.Render(" · ")), ""}
	settings := []struct {
		name    string
		enabled bool
	}{
		{"Desktop notifications", m.alertPrefs.DesktopNotifications},
		{"Notification sound", m.alertPrefs.NotificationSound},
		{"Completed prints", m.alertPrefs.CompletedPrints},
	}
	for i, setting := range settings {
		value := "Off"
		if setting.enabled {
			value = "On"
		}
		marker := "  "
		if i == m.configSelection {
			marker = "> "
		}
		line := fmt.Sprintf("%s%s: %s", marker, setting.name, value)
		if i == m.configSelection {
			line = selected.Render(line)
		}
		lines = append(lines, line)
	}
	action := "  Test Notification"
	if m.configSelection == notificationTestAction {
		action = selected.Render("> Test Notification")
	}
	lines = append(lines, action)
	if m.notificationTestStatus != "" {
		lines = append(lines, m.notificationTestStatus)
	}
	if m.configStorageError != "" {
		lines = append(lines, danger.Render("Save failed · r retry; settings apply this session."))
	} else if message := m.storageWarning(); message != "" {
		lines = append(lines, ansi.Truncate(warning.Render(message), max(1, m.width-2), "…"))
	}
	lines = append(lines, "")
	lines = append(lines, "Global defaults; printer overrides still apply.", "Changes take effect immediately. macOS only.", "Test may request macOS notification permission.", "Saving normalizes TOML and removes comments.")
	if m.configPath != "" {
		lines = append(lines, "File: "+safe(m.configPath))
	}
	if len(m.alertPrefs.Printers) > 0 {
		lines = append(lines, fmt.Sprintf("%d printer overrides configured; s shows effective values.", len(m.alertPrefs.Printers)))
	}
	hints := m.configHelp()
	rows := strings.Split(m.fullView(strings.Split(wrapLines(lines, max(1, m.width-2)), "\n"), hints), "\n")
	// Config reserves its footer for helper text; warnings remain in the body.
	rows[len(rows)-1] = m.footerRow(hints)
	return strings.Join(rows, "\n")
}

func (m Model) configHelp() string {
	help := [...]string{
		"Show macOS alerts while this TUI runs.",
		"Request sound on desktop notifications.",
		"Notify on confirmed print completion.",
		"Test delivery using the current sound.",
	}
	if m.configSelection == notificationTestAction && m.notificationTestPending {
		return "Sending test; closing cancels it."
	}
	return help[m.configSelection]
}

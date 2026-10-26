package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/config"
)

const notificationSettingCount = 3

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
		m.configSelection = (m.configSelection + notificationSettingCount - 1) % notificationSettingCount
	case keyDown, keyVimDown:
		m.configSelection = (m.configSelection + 1) % notificationSettingCount
	case keyEnter, " ":
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
	lines := []string{accent.Bold(true).Render("CONFIG / NOTIFICATIONS"), "↑/↓ select · Space/Enter toggle · , / Esc close", ""}
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
	status := "Saved"
	if m.saveConfig == nil {
		status = "Session only"
	} else if m.configWrites.inFlight || m.configWrites.attempted != m.configRevision {
		status = "Saving…"
	} else if m.configStorageError != "" {
		status = "Save failed · r retry"
	}
	lines = append(lines, "", status, "Global defaults; printer overrides still apply.", "Changes take effect immediately. macOS only.", "Permission: run bambu-tui doctor --notify.", "Saving normalizes TOML and removes comments.")
	if m.configPath != "" {
		lines = append(lines, "File: "+safe(m.configPath))
	}
	if len(m.alertPrefs.Printers) > 0 {
		lines = append(lines, fmt.Sprintf("%d printer overrides configured; s shows effective values.", len(m.alertPrefs.Printers)))
	}
	hints := "Esc / , close  ↑/↓ select  Space/Enter toggle"
	if message := m.storageWarning(); message != "" {
		hints = "Esc / , close  " + message
		if m.configStorageError != "" {
			hints = "Esc / , close  r retry  Config save failed"
		}
	}
	rows := strings.Split(m.fullView(strings.Split(wrapLines(lines, max(1, m.width-2)), "\n"), hints), "\n")
	// Keep close keys visible even when fullView prioritizes a storage warning.
	rows[len(rows)-1] = m.footerRow(footerHints(hints))
	return strings.Join(rows, "\n")
}

package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
)

type (
	configSection   uint8
	configResultMsg struct {
		revision uint64
		err      error
	}
	configOption struct {
		field      config.AlertField
		name, help string
	}
)

const (
	configGeneral configSection = iota
	configAlerts
	configPrinters
	configSectionCount
	notificationTestAction = int(config.AlertFieldCount)
	// Four header rows, three metadata rows, and the one-row footer.
	configReservedRows   = 8
	configTrailingRows   = 4
	configMaximumPercent = 100
	configInputMaxLength = 12
	configInherit        = 0
	configOverrideOn     = 1
	configOverrideOff    = 2
	configOverrideCount  = 3
)

var configOptions = [...]configOption{
	{config.DesktopNotifications, "Desktop notifications", "Show macOS alerts while this TUI runs."},
	{config.NotificationSound, "Notification sound", "Request sound on desktop notifications."},
	{config.CompletedPrints, "Completed prints", "Notify on confirmed print completion."},
	{config.AutomaticModals, "Automatic modals", "Open dialogs for actionable printer conditions."},
	{config.Filament, "Filament alerts", "Alert on verified filament intervention conditions."},
	{config.Extrusion, "Extrusion alerts", "Alert on verified extrusion intervention conditions."},
	{config.PrinterFaults, "Printer faults", "Alert on other verified printer faults."},
	{config.FailedPrints, "Failed prints", "Alert on reported print failures."},
	{config.UnknownPauses, "Unknown pauses", "Alert on pauses without a classified cause."},
}

// WithConfiguration installs startup settings. A region edit never changes the
// running cloud connection or the account/region owning persistence workers.
func (m Model) WithConfiguration(cfg config.Config) Model {
	cfg = cfg.Clone()
	m.configRegion, m.activeRegion = cfg.Region, cfg.Region
	m.restoreView(cfg.View)
	m.alertPrefs = cfg.Alerts
	m.Farm.LowFilament = cfg.LowFilamentPercent
	m.syncPanels()
	return m
}

func (m Model) WithConfigPersistence(path string, save func(config.Config) error) Model {
	m.configPath, m.saveConfig = path, save
	return m
}

func (m Model) configuration() config.Config {
	return config.Config{View: m.viewPreferences(), Region: m.configRegion, LowFilamentPercent: m.Farm.LowFilament, Alerts: m.alertPrefs}.Clone()
}

func (m Model) configPrinterIDs() []string {
	ids := append([]string(nil), m.Farm.Order...)
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	absent := []string{}
	for id := range m.alertPrefs.Printers {
		if !seen[id] {
			absent = append(absent, id)
			seen[id] = true
		}
	}
	if m.configPrinterID != "" && !seen[m.configPrinterID] {
		absent = append(absent, m.configPrinterID)
	}
	sort.Strings(absent)
	return append(ids, absent...)
}

func (m Model) configTarget() string {
	if m.configPrinterID != "" {
		return m.configPrinterID
	}
	if p := m.current(); p != nil {
		return p.ID
	}
	ids := m.configPrinterIDs()
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func (m Model) configRowCount() int {
	switch m.configSection {
	case configGeneral:
		return 2
	case configPrinters:
		if m.configTarget() == "" {
			return 1
		}
		return len(configOptions) + 2 // Printer selector, all overrides, reset.
	default:
		return len(configOptions) + 1 // Global rules plus explicit test.
	}
}

func (m *Model) configKey(key string, msg tea.Msg) tea.Cmd {
	if m.configEditing {
		m.configEditKey(key, msg)
		return nil
	}
	if m.configResetConfirm {
		switch key {
		case keyEscape:
			m.configResetConfirm = false
		case keyConfig:
			m.closeDialog()
		case keyEnter:
			m.alertPrefs = m.configuration().Alerts
			delete(m.alertPrefs.Printers, m.configTarget())
			m.configRevision++
			m.configResetConfirm = false
		}
		return nil
	}
	switch key {
	case keyEscape, keyConfig:
		m.closeDialog()
	case keyNextPanel, keyPreviousPanel:
		delta := 1
		if key == keyPreviousPanel {
			delta = int(configSectionCount) - 1
		}
		m.configSection = (m.configSection + configSection(delta)) % configSectionCount
		m.configSelection = 0
		m.syncConfig()
	case keyUp, keyVimUp:
		m.configSelection = (m.configSelection + m.configRowCount() - 1) % m.configRowCount()
	case keyDown, keyVimDown:
		m.configSelection = (m.configSelection + 1) % m.configRowCount()
	case keyHome:
		m.configSelection = 0
	case keyEnd:
		m.configSelection = m.configRowCount() - 1
	case keyPageUp:
		m.configSelection = max(0, m.configSelection-max(1, m.height-configReservedRows))
	case keyPageDown:
		m.configSelection = min(m.configRowCount()-1, m.configSelection+max(1, m.height-configReservedRows))
	case keyEnter, " ", keyLeft, keyRight, keyVimLeft, keyVimRight:
		if m.configSection == configAlerts && m.configSelection == notificationTestAction {
			if key == keyEnter || key == " " {
				return m.startNotificationTest()
			}
			return nil
		}
		direction := 1
		if key == keyLeft || key == keyVimLeft {
			direction = -1
		}
		m.changeConfig(direction, key == keyEnter || key == " ")
	case keyRefreshOrRestore:
		if m.configWrites.failed != 0 && !m.configWrites.inFlight {
			m.configRevision++
		}
	}
	return nil
}

func (m *Model) changeConfig(direction int, activate bool) {
	switch m.configSection {
	case configGeneral:
		if m.configSelection == 0 {
			if m.configRegion == config.RegionGlobal {
				m.configRegion = config.RegionChina
			} else {
				m.configRegion = config.RegionGlobal
			}
		} else if activate {
			m.configEditing, m.configInputError = true, ""
			m.configInput = strconv.Itoa(m.Farm.LowFilament)
			m.configReplaceInput = true
			return
		} else {
			threshold := max(0, min(configMaximumPercent, m.Farm.LowFilament+direction))
			if threshold == m.Farm.LowFilament {
				return
			}
			m.Farm.LowFilament = threshold
			m.Farm.Reconcile(m.now)
		}
	case configAlerts:
		field := configOptions[m.configSelection].field
		m.alertPrefs.Set(field, !m.alertPrefs.Value(field))
	case configPrinters:
		id := m.configTarget()
		if id == "" {
			return
		}
		if m.configSelection == 0 {
			ids := m.configPrinterIDs()
			index := 0
			for i, candidate := range ids {
				if candidate == id {
					index = i
					break
				}
			}
			m.configPrinterID = ids[(index+len(ids)+direction)%len(ids)]
			return
		}
		// Clone before changing the map: async snapshots must never share ownership.
		m.alertPrefs = m.configuration().Alerts
		if m.configSelection == len(configOptions)+1 {
			if activate {
				m.configResetConfirm = true
			}
			return
		} else {
			field := configOptions[m.configSelection-1].field
			override := m.alertPrefs.Printers[id]
			value := override.Value(field)
			state := configInherit // Inherit -> On -> Off -> Inherit.
			if value != nil {
				if *value {
					state = configOverrideOn
				} else {
					state = configOverrideOff
				}
			}
			state = (state + configOverrideCount + direction) % configOverrideCount
			var next *bool
			if state != configInherit {
				enabled := state == configOverrideOn
				next = &enabled
			}
			override.Set(field, next)
			if m.alertPrefs.Printers == nil {
				m.alertPrefs.Printers = map[string]config.AlertOverride{}
			}
			empty := true
			for field := range config.AlertFieldCount {
				empty = empty && override.Value(field) == nil
			}
			if empty {
				delete(m.alertPrefs.Printers, id)
			} else {
				m.alertPrefs.Printers[id] = override
			}
		}
	}
	m.configRevision++
}

func (m *Model) configEditKey(key string, msg tea.Msg) {
	switch key {
	case keyEscape:
		m.configEditing, m.configInputError = false, ""
	case keyConfig:
		m.closeDialog()
	case keyEnter:
		value, err := strconv.Atoi(m.configInput)
		if err != nil || value < 0 || value > configMaximumPercent {
			m.configInputError = "Enter a whole percentage from 0 to 100."
			return
		}
		if value != m.Farm.LowFilament {
			m.Farm.LowFilament = value
			m.Farm.Reconcile(m.now)
			m.configRevision++
		}
		m.configEditing, m.configInputError = false, ""
	case "backspace", "ctrl+h":
		m.configReplaceInput = false
		if len(m.configInput) > 0 {
			m.configInput = m.configInput[:len(m.configInput)-1]
		}
	case "ctrl+u":
		m.configReplaceInput = false
		m.configInput = ""
	default:
		input, ok := msg.(tea.KeyMsg)
		if !ok || len(input.Runes) == 0 {
			return
		}
		for _, char := range input.Runes {
			if char < ' ' || char > '~' {
				return
			}
		}
		prefix := m.configInput
		if m.configReplaceInput {
			prefix = ""
		}
		if len(prefix)+len(input.Runes) <= configInputMaxLength {
			m.configInput = prefix + string(input.Runes)
			m.configReplaceInput, m.configInputError = false, ""
		}
	}
}

func (m *Model) syncConfig() {
	if m.configSection == configPrinters && m.configPrinterID == "" {
		m.configPrinterID = m.configTarget()
	}
	m.configSelection = min(m.configSelection, m.configRowCount()-1)
}

func (m *Model) clearConfig() {
	m.cancelNotificationTest()
	m.configEditing, m.configInputError = false, ""
	m.configResetConfirm = false
}

func (m *Model) persistConfig() tea.Cmd {
	if m.saveConfig == nil || m.configWrites.inFlight || m.configWrites.attempted == m.configRevision {
		return nil
	}
	revision, snapshot, save := m.configRevision, m.configuration(), m.saveConfig
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
		m.configStorageError = "Config save failed; open , and r to retry"
	} else {
		m.configWrites.saved, m.configWrites.failed = msg.revision, 0
		m.configStorageError = ""
	}
}

func configOnOff(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}

func (m Model) configRows() []string {
	switch m.configSection {
	case configGeneral:
		region := m.configRegion
		if region != m.activeRegion {
			region += " (restart + login required)"
		}
		threshold := fmt.Sprintf("%d%%", m.Farm.LowFilament)
		if m.configEditing {
			threshold = "[" + m.configInput + "▏]  Enter apply / Esc cancel"
		}
		return []string{"Cloud region: " + region, "Low filament threshold: " + threshold}
	case configPrinters:
		id := m.configTarget()
		if id == "" {
			return []string{"No discovered or configured printers."}
		}
		label := safe(id) + " (absent)"
		if p := m.Farm.Printers[id]; p != nil {
			label = safe(p.Name) + " [" + safe(id) + "]"
		}
		rows := []string{"Printer: " + label}
		override, effective := m.alertPrefs.Printers[id], m.alertPrefs.RulesFor(id)
		for _, option := range configOptions {
			value := override.Value(option.field)
			label := "Inherit"
			if value != nil {
				label = configOnOff(*value)
			}
			rows = append(rows, fmt.Sprintf("%-24s %s (effective: %s)", option.name, label, configOnOff(effective.Value(option.field))))
		}
		if m.configResetConfirm {
			return append(rows, "Reset all overrides? Enter confirm / Esc cancel")
		}
		return append(rows, "Reset this printer to global defaults")
	default:
		rows := []string{}
		for _, option := range configOptions {
			rows = append(rows, fmt.Sprintf("%-24s %s", option.name, configOnOff(m.alertPrefs.Value(option.field))))
		}
		return append(rows, "Test Notification")
	}
}

func (m Model) configView() string {
	sectionNames := []string{"General", "Alerts", "Printers"}
	bindings := headerHints("Tab section  ↑/↓  Enter edit  Esc / , close  ^E status")
	if m.configEditing {
		bindings = headerHints("Enter apply  Esc cancel  , close  ^E status")
	} else if m.configResetConfirm {
		bindings = headerHints("Enter reset  Esc cancel  , close  ^E status")
	}
	tabs := append([]string(nil), sectionNames...)
	tabs[m.configSection] = accent.Bold(true).Render("[" + tabs[m.configSection] + "]")
	title := "CONFIG / " + strings.ToUpper(sectionNames[m.configSection])
	if m.configSection == configPrinters && m.configTarget() != "" {
		label := m.configTarget()
		if p := m.Farm.Printers[label]; p != nil {
			label = p.Name
		}
		title += " · " + safe(label)
	}
	lines := []string{accent.Bold(true).Render(title), bindings, strings.Join(tabs, " · "), ""}
	rows := m.configRows()
	visible := max(1, m.height-configReservedRows)
	start := max(0, m.configSelection-visible+1)
	for i := start; i < min(len(rows), start+visible); i++ {
		marker := "  "
		if i == m.configSelection {
			marker = "> "
		}
		row := ansi.Truncate(marker+rows[i], max(1, m.width-2), "…")
		if i == m.configSelection {
			row = selected.Render(row)
		}
		lines = append(lines, row)
	}
	for len(lines) < m.height-configTrailingRows {
		lines = append(lines, "")
	}
	status := ""
	if m.configInputError != "" {
		status = danger.Render(m.configInputError)
	} else if m.configStorageError != "" {
		message := "Save failed · r retry; changes may be lost on exit."
		if count := len(m.statusProblems()); count > 1 {
			message = fmt.Sprintf("Save failed · r retry · %d errors · ^E status", count)
		}
		if m.configEditing || m.configResetConfirm {
			message = "Save failed · ^E status; finish/cancel before retry."
		}
		status = danger.Render(message)
	} else if len(m.statusProblems()) > 0 {
		status = danger.Render(m.problemSummary(max(1, m.width-15)) + " · ^E status")
	} else if m.configRegion != m.activeRegion {
		status = warning.Render("Region pending: restart and log in; current session unchanged.")
	} else if m.configSection == configPrinters {
		status = muted.Render("←/→ change · Inherit uses global defaults · scroll ↑/↓")
	}
	lines = append(lines, ansi.Truncate(status, max(1, m.width-2), "…"), ansi.Truncate("Config file location: "+safe(m.configPath), max(1, m.width-2), "…"), muted.Render("Saving normalizes TOML and removes comments."))
	return m.frame(lines, m.configFooter())
}

func (m Model) configHelp() string {
	if m.configResetConfirm {
		return "Enter resets all this printer’s overrides; Esc cancels."
	}
	if m.configEditing {
		return "0–100 · Ctrl+U clear · Enter apply · Esc cancel"
	}
	switch m.configSection {
	case configGeneral:
		if m.configSelection == 0 {
			return "Cloud endpoint; restart and log in after changing."
		}
		return "Low filament warning: ←/→ adjust, Enter type 0–100."
	case configPrinters:
		if m.configSelection == 0 {
			return "←/→ choose printer; saved absent printers included."
		}
		if m.configSelection == len(configOptions)+1 {
			return "Remove this printer's overrides; keep reviews/snoozes."
		}
		return "←/→ cycle Inherit / On / Off. " + configOptions[m.configSelection-1].help
	default:
		if m.configSelection == notificationTestAction {
			if m.notificationTestPending {
				return "Sending test; closing cancels it."
			}
			return "Test delivery using the current sound."
		}
		return configOptions[m.configSelection].help
	}
}

func (m Model) configFooter() string {
	helper := m.configHelp()
	feedback := m.notificationTestStatus
	if feedback == "" || m.configEditing || m.configResetConfirm {
		return helper
	}
	elapsed := m.now.Sub(m.notificationTestStatusAt)
	if !m.notificationTestPending && !m.notificationTestFailed && elapsed >= notificationTestFeedbackLifetime {
		return helper
	}
	rendered := accent.Bold(true).Render(feedback)
	if m.notificationTestFailed {
		rendered = danger.Bold(true).Render(feedback)
	} else if !m.notificationTestPending && elapsed >= notificationTestDimAfter {
		rendered = muted.Render(feedback)
	}
	const separator = " · "
	helperWidth := max(1, m.width-2-ansi.StringWidth(rendered)-ansi.StringWidth(separator))
	return ansi.Truncate(helper, helperWidth, "…") + separator + rendered
}

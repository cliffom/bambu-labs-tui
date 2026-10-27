package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

func TestConfigGeneralValidationAndRegionIsolation(t *testing.T) {
	m := demoModel().WithConfiguration(config.Config{Region: config.RegionGlobal, LowFilamentPercent: 10, Alerts: config.DefaultAlerts()})
	m = key(m, ",")
	m = key(m, "enter")
	if m.configRegion != config.RegionChina || m.activeRegion != config.RegionGlobal || !strings.Contains(ansi.Strip(m.View()), "restart + login required") {
		t.Fatal("region switched live account or hid restart requirement")
	}
	m = key(m, "down")
	m = key(m, "enter")
	m = key(m, "101")
	revision := m.configRevision
	m = key(m, "enter")
	if !m.configEditing || m.configInputError == "" || m.Farm.LowFilament != 10 || m.configRevision != revision {
		t.Fatal("invalid percentage applied")
	}
	m = key(m, "esc")
	if m.configEditing || !m.hasOpenDialog() {
		t.Fatal("Esc should cancel input before closing Config")
	}
	m = key(m, "enter")
	m = key(m, "0")
	m = key(m, "enter")
	if m.Farm.LowFilament != 0 || m.configEditing {
		t.Fatal("zero threshold not applied")
	}
	revision = m.configRevision
	m = key(m, "left")
	if m.configRevision != revision {
		t.Fatal("clamped value wrote a new revision")
	}
	m = key(m, "enter")
	m = key(m, "100")
	m = key(m, "enter")
	m = key(m, "right")
	if m.Farm.LowFilament != 100 {
		t.Fatal("threshold exceeds maximum")
	}
	// No implicit restart, account rebinding, or review/snooze clearing.
	if m.activeRegion != config.RegionGlobal || m.configuration().Region != config.RegionChina {
		t.Fatal("region isolation changed")
	}
}

func TestConfigPercentageRejectsFractionalAndNonNumericInput(t *testing.T) {
	for _, input := range []string{"0.5", "-1", "abc", "", "1000"} {
		m := key(key(key(demoModel(), ","), "down"), "enter")
		m = key(m, "ctrl+u")
		if input != "" {
			m = key(m, input)
		}
		m = key(m, "enter")
		if !m.configEditing || m.configInputError == "" || m.Farm.LowFilament != 10 || m.configRevision != 0 {
			t.Fatalf("invalid percentage %q accepted", input)
		}
	}
}

func TestConfigThresholdReconcilesOnlyReportedFilament(t *testing.T) {
	m := demoModel()
	m = attentionReport(t, m, "H", `{"print":{"ams":{"ams":[{"id":0,"tray":[{"id":0,"remain":20}]}]}}}`)
	m = key(m, ",")
	m = key(m, "down")
	m = key(m, "enter")
	m = key(m, "20")
	m = key(m, "enter")
	if _, exists := m.Farm.Active["H/low/0/0"]; !exists {
		t.Fatal("threshold change waited for another report")
	}
	if _, exists := m.Farm.Active["X/low/0/0"]; exists {
		t.Fatal("unknown telemetry created a warning")
	}
}

func TestAllGlobalAndPrinterFieldsWithInheritanceAndConfirmedReset(t *testing.T) {
	m, client, _ := notificationModel(t)
	m = openAlertConfig(m)
	seen := map[config.AlertField]bool{}
	for _, option := range configOptions {
		if seen[option.field] {
			t.Fatal("duplicate setting")
		}
		seen[option.field] = true
		before := m.alertPrefs.Value(option.field)
		m = key(m, "enter")
		if m.alertPrefs.Value(option.field) == before {
			t.Fatal("global control failed", option.name)
		}
		m = key(m, "down")
	}
	if len(seen) != int(config.AlertFieldCount) {
		t.Fatal("a supported alert field is inaccessible")
	}
	m = key(m, "tab")
	target := m.configTarget()
	originalReviews := m.Farm.Reviews()
	m.snoozes = map[string]time.Time{target: m.now.Add(time.Hour)}
	m = key(m, "down")
	for _, option := range configOptions {
		if m.alertPrefs.Printers[target].Value(option.field) != nil {
			t.Fatal("initial override should inherit")
		}
		m = key(m, "enter")
		if value := m.alertPrefs.Printers[target].Value(option.field); value == nil || !*value {
			t.Fatal("On missing", option.name)
		}
		m = key(m, "enter")
		if value := m.alertPrefs.Printers[target].Value(option.field); value == nil || *value {
			t.Fatal("Off missing", option.name)
		}
		m = key(m, "enter")
		if m.alertPrefs.Printers[target].Value(option.field) != nil {
			t.Fatal("Inherit missing", option.name)
		}
		m = key(m, "left") // Reverse cycling gives explicit Off from Inherit.
		m = key(m, "down")
	}
	if len(client.inputs) == 0 {
		t.Fatal("policy changes did not reach delivery worker")
	}
	m = key(m, "enter")
	if !m.configResetConfirm || len(m.alertPrefs.Printers) == 0 {
		t.Fatal("reset must request confirmation")
	}
	m = key(m, "esc")
	if m.configResetConfirm || len(m.alertPrefs.Printers) == 0 {
		t.Fatal("cancel reset changed overrides")
	}
	m = key(m, "enter")
	m = key(m, "enter")
	if len(m.alertPrefs.Printers) != 0 || !reflect.DeepEqual(m.alertPrefs.RulesFor(target), m.alertPrefs.AlertRules) {
		t.Fatal("confirmed reset did not restore inheritance")
	}
	if !reflect.DeepEqual(originalReviews, m.Farm.Reviews()) || !m.snoozed(target) {
		t.Fatal("editing preferences changed reviews/snoozes")
	}
}

func TestPrinterDesktopOverrideChangesDeliveryPolicyImmediately(t *testing.T) {
	m, client, _ := notificationModel(t)
	m.alertPrefs.DesktopNotifications = false
	m = key(key(key(m, ","), "tab"), "tab")
	m = key(m, "down")
	for _, expected := range []bool{true, false, false} {
		m = key(m, "enter")
		if len(client.inputs) == 0 || client.inputs[len(client.inputs)-1].Policies["TEST"].Desktop != expected {
			t.Fatal("per-printer delivery policy missed override transition")
		}
	}
	if m.alertPrefs.DesktopNotifications || m.alertPrefs.Printers["TEST"].DesktopNotifications != nil {
		t.Fatal("inherit changed global preference")
	}
}

func TestConfigAbsentPrintersAndStableTargetThroughDiscovery(t *testing.T) {
	m := demoModel()
	enabled := true
	m.alertPrefs.Printers = map[string]config.AlertOverride{"ABSENT": {AutomaticModals: &enabled}}
	m = key(key(key(m, ","), "tab"), "tab")
	m = key(m, "left") // First discovered printer wraps to saved absent printer.
	if m.configTarget() != "ABSENT" || !strings.Contains(ansi.Strip(m.View()), "(absent)") {
		t.Fatal("saved absent printer is inaccessible")
	}
	m = key(m, "down")
	next, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "NEW", Name: "Renamed", Online: true}}, At: m.now})
	m = next.(Model)
	if m.configTarget() != "ABSENT" || m.configSelection != 1 {
		t.Fatal("discovery changed editing target")
	}
	m = key(m, "enter")
	if value := m.alertPrefs.Printers["ABSENT"].DesktopNotifications; value == nil || !*value {
		t.Fatal("absent override not editable")
	}
	m = key(m, "end")
	m = key(key(m, "enter"), "enter")
	if m.configTarget() != "ABSENT" || len(m.alertPrefs.Printers) != 0 {
		t.Fatal("reset changed target or left overrides")
	}
}

func TestConfigWholeSnapshotsRemainImmutableWhileSaving(t *testing.T) {
	var saved []config.Config
	m := demoModel().WithConfigPersistence("synthetic", func(cfg config.Config) error { saved = append(saved, cfg); return nil })
	m = key(key(key(m, ","), "tab"), "tab")
	m = key(m, "down")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m = key(m, "enter")
	for m.configSection != configGeneral {
		m = key(m, "tab")
	}
	m = key(m, "enter") // Saved region must join queued snapshot.
	m = completePersistence(m, cmd)
	if m.HasUnsavedPersistence() {
		t.Fatal("serialized snapshots left unsaved settings")
	}
	if len(saved) != 2 {
		t.Fatalf("expected two serialized writes: %d", len(saved))
	}
	first, last := saved[0], saved[1]
	if !*first.Alerts.Printers["H"].DesktopNotifications || *last.Alerts.Printers["H"].DesktopNotifications || first.Region != config.RegionGlobal || last.Region != config.RegionChina {
		t.Fatal("snapshot aliased live edits or coalescing lost fields")
	}
}

func TestConfigAllSectionsBoundsScrollingAndHiddenEditing(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m := resizeModel(demoModel(), size[0], size[1])
		m = key(m, ",")
		for section := range configSectionCount {
			if m.configSection != section {
				t.Fatal("Tab skipped section")
			}
			for row := range m.configRowCount() {
				view := ansi.Strip(m.View())
				lines := strings.Split(view, "\n")
				if len(lines) != size[1] {
					t.Fatal("Config height exceeds terminal")
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("Config width exceeds terminal")
					}
				}
				selected := ansi.Truncate("> "+m.configRows()[row], size[0]-2, "…")
				if !strings.Contains(view, selected) {
					t.Fatalf("selection scrolled out at %v section %d row %d", size, section, row)
				}
				m = key(m, "down")
			}
			m = key(m, "tab")
		}
		m = key(m, "down")
		m = key(m, "enter")
		before := m.configuration()
		m = resizeModel(m, 59, 17)
		for _, keyName := range []string{"1", "enter", "tab", "right", "R", "r"} {
			m = key(m, keyName)
		}
		if !reflect.DeepEqual(before, m.configuration()) || !m.configEditing {
			t.Fatal("hidden editing acted below minimum")
		}
		m = key(m, "esc")
		if m.configEditing || !m.hasOpenDialog() {
			t.Fatal("hidden Escape failed to cancel editing")
		}
		m = key(m, "esc")
		if m.hasOpenDialog() {
			t.Fatal("hidden Escape failed to close")
		}
	}
}

func TestConfigEmptyPrinterSectionAndUntrustedNames(t *testing.T) {
	m := New(farm.New(10), nil, nil, true)
	m = key(key(key(m, ","), "tab"), "tab")
	if !strings.Contains(ansi.Strip(m.View()), "No discovered or configured printers") {
		t.Fatal("empty printer guidance missing")
	}
	revision := m.configRevision
	for _, keyName := range []string{"enter", "left", "right", "up", "down"} {
		m = key(m, keyName)
	}
	if m.configRevision != revision {
		t.Fatal("empty printer controls changed config")
	}
	next, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "SAFE", Name: "name\x1b[2J\nunsafe", Online: true}}, At: m.now})
	m = next.(Model)
	if m.configTarget() != "SAFE" || strings.Contains(m.View(), "\x1b[2J") || len(strings.Split(ansi.Strip(m.View()), "\n")) != m.height {
		t.Fatal("discovery failed to initialize target or allowed terminal injection")
	}
	m = key(m, "end")
	if !strings.Contains(strings.Split(ansi.Strip(m.View()), "\n")[0], "name") {
		t.Fatal("scrolling hid the target context")
	}
}

package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/diagnostics"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

// TestDiagnosticsRequiresExplicitConfirmation snapshots only after D and Enter, never on telemetry.
func TestDiagnosticsRequiresExplicitConfirmation(t *testing.T) {
	m := demoModel()
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","mc_percent":11,"subtask_name":"PRIVATE_JOB","token":"PRIVATE_TOKEN"}}`)
	path := filepath.Join(t.TempDir(), "diagnostics.json")
	m = key(m, "D")
	if m.exportDialog == nil || m.exportDialog.writing || !strings.Contains(ansi.Strip(m.View()), "LOCAL DIAGNOSTIC EXPORT") {
		t.Fatal("export did not open an explicit preview")
	}
	m.exportDialog.destination = path
	m = key(m, "esc")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancel created an artifact")
	}
	m = key(m, "D")
	m.exportDialog.destination = path
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || !m.exportDialog.writing {
		t.Fatal("confirmation did not schedule a write")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("write happened before the command executed")
	}
	_, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("duplicate confirmation queued another write")
	}
	// The command owns an immutable, sanitized snapshot from confirmation time.
	m = attentionReport(t, m, "H", `{"print":{"mc_percent":99,"hms":[{"attr":50331904,"code":65543,"message":"PRIVATE_URL https://private.invalid"}]}}`)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.exportDialog.saved || m.exportDialog.writing || m.exportDialog.problem != "" {
		t.Fatal("successful local write not reported")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "PRIVATE_") || strings.Contains(string(body), "private.invalid") {
		t.Fatal("UI exported private values")
	}
	var doc diagnostics.Document
	if err = json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Normalized["progress"] != float64(11) {
		t.Fatal("write borrowed live telemetry rather than the confirmed snapshot")
	}
	_, duplicate = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("saved result could be silently exported twice")
	}
}

// TestDiagnosticFailureAndRemovedPrinter preserves existing files and omits raw errors from feedback.
func TestDiagnosticFailureAndRemovedPrinter(t *testing.T) {
	m := demoModel()
	m = key(m, "D")
	path := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(path, []byte("keep existing"), 0600); err != nil {
		t.Fatal(err)
	}
	m.exportDialog.destination = path
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.exportDialog.problem == "" || m.exportDialog.saved {
		t.Fatal("export failure was hidden")
	}
	body, _ := os.ReadFile(path)
	if string(body) != "keep existing" {
		t.Fatal("existing file was overwritten")
	}
	next, _ = m.Update(exportResultMsg{destination: path, err: errors.New("PRIVATE_ERROR token=PRIVATE_TOKEN")})
	m = next.(Model)
	if strings.Contains(ansi.Strip(m.View()), "PRIVATE_") {
		t.Fatal("raw error text was echoed")
	}
	next, _ = m.Update(monitor.Event{Kind: monitor.EventDevices, At: m.now})
	m = next.(Model)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("removed printer scheduled an export")
	}
	m = key(m, "esc")
	m = key(m, "D")
	if m.exportDialog != nil {
		t.Fatal("empty list exported a synthetic printer")
	}
}

// TestDiagnosticsRemainPinnedAcrossFilterChanges follows the requested printer, not a new selected row.
func TestDiagnosticsRemainPinnedAcrossFilterChanges(t *testing.T) {
	m := demoModel()
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","mc_percent":11}}`)
	m = attentionReport(t, m, "X", `{"print":{"gcode_state":"RUNNING","mc_percent":22}}`)
	m.printerFilter = filterPrinting
	m.syncPanels()
	m.selectPrinter("H")
	m = key(m, "D")
	m.exportDialog.destination = filepath.Join(t.TempDir(), "pinned.json")
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"FINISH"}}`)
	if m.current().ID != "X" || m.exportDialog.printerID != "H" {
		t.Fatal("export lost requested printer identity")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	body, err := os.ReadFile(m.exportDialog.destination)
	if err != nil {
		t.Fatal(err)
	}
	var doc diagnostics.Document
	if err = json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Normalized["state"] != "FINISH" || doc.Normalized["progress"] != float64(11) {
		t.Fatal("export used another visible printer")
	}
}

// TestDiagnosticsDefersAttentionAndHandlesLateResults keeps transport flowing while confirmation is open.
func TestDiagnosticsDefersAttentionAndHandlesLateResults(t *testing.T) {
	m := demoModel()
	m = key(m, "D")
	old := m.exportDialog.destination
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.attention != nil || len(m.Farm.Attention()) == 0 {
		t.Fatal("diagnostics interrupted telemetry or failed to defer prompts")
	}
	m = key(m, "esc")
	if m.attention == nil {
		t.Fatal("queued attention did not appear after closing")
	}
	m = key(m, "esc")
	m = key(m, "D")
	m.exportDialog.destination = "another-destination"
	next, _ := m.Update(exportResultMsg{destination: old})
	m = next.(Model)
	if m.exportDialog.saved || m.exportDialog.destination != "another-destination" {
		t.Fatal("late result updated a different confirmation")
	}
}

// TestDiagnosticPreviewBounds keeps destination and confirmation usable in supported terminal sizes.
func TestDiagnosticPreviewBounds(t *testing.T) {
	m := demoModel()
	m = key(m, "D")
	for _, size := range [][2]int{{120, 35}, {80, 24}, {60, 18}, {40, 10}, {0, 0}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		view := ansi.Strip(m.View())
		if view != "" && len(strings.Split(view, "\n")) > size[1] {
			t.Fatal("height overflow")
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow")
			}
		}
		if size[0] >= 60 && size[1] >= 18 {
			for _, text := range []string{"Enter export", "Esc close", "Destination:"} {
				if !strings.Contains(view, text) {
					t.Fatalf("preview hides %q at %v", text, size)
				}
			}
		}
	}
}

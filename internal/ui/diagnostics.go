package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/diagnostics"
)

type (
	exportDialog struct {
		printerID, destination, problem string
		writing, saved                  bool
		view                            viewport.Model
	}

	exportResultMsg struct {
		destination string
		err         error
	}
)

// openExport previews scope and a local destination without writing anything.
func (m *Model) openExport() {
	p := m.current()
	if p == nil {
		m.setToast(feedbackInfo, "Select a printer to export diagnostics")
		return
	}
	destination, err := diagnostics.Destination(time.Now())
	m.openDialog(dialogExport)
	m.exportDialog = &exportDialog{printerID: p.ID, destination: destination, view: viewport.New(1, 1)}
	if err != nil {
		m.exportDialog.problem = "Diagnostic destination unavailable; check your cache-directory settings."
	}
	m.syncExport()
}

// exportKey snapshots on explicit confirmation and sends only sanitized data to the writer.
func (m *Model) exportKey(key string, msg tea.Msg) tea.Cmd {
	d := m.exportDialog
	switch key {
	case keyEscape:
		m.closeDialog()
	case keyQuit, keyInterrupt:
		return nil
	case keyEnter:
		if d.writing || d.saved || d.destination == "" {
			return nil
		}
		doc, err := m.diagnostics.Build(m.Farm, d.printerID, m.now)
		if err != nil {
			d.problem = "Printer unavailable. Close this view and select another printer."
			m.exportError = d.problem
			return nil
		}
		d.writing = true
		d.problem = ""
		destination := d.destination
		return func() tea.Msg {
			return exportResultMsg{destination: destination, err: diagnostics.Write(destination, doc)}
		}
	default:
		var cmd tea.Cmd
		d.view, cmd = d.view.Update(msg)
		return cmd
	}
	return nil
}

// exportResult reports local success/failure without logging raw telemetry or operating-system errors.
func (m *Model) exportResult(result exportResultMsg) {
	m.exportError = ""
	if result.err != nil {
		m.exportError = "Diagnostic export failed; check destination permissions, then retry from the dashboard (D)."
	}
	if d := m.exportDialog; d != nil && d.destination == result.destination {
		d.writing = false
		if result.err == nil {
			d.saved = true
		} else {
			d.problem = "Export failed. Check the destination permissions; Esc then D chooses a new filename."
		}
	} else {
		m.setToast(feedbackSuccess, "Diagnostics saved locally: "+result.destination)
		if result.err != nil {
			m.setToast(feedbackError, "Diagnostic export failed; press D to try again")
		}
	}
}

// syncExport keeps destination, export scope, and write status readable during live updates.
func (m *Model) syncExport() {
	d := m.exportDialog
	if d == nil {
		return
	}
	d.view.Width, d.view.Height = max(1, m.width-2), max(1, m.height-6)
	name := "Printer no longer available"
	if p := m.Farm.Printers[d.printerID]; p != nil {
		name = p.Name
	}
	lines := []string{
		"Export diagnostics for one printer?", m.detailField("Printer", safe(name), d.view.Width, m.styles.text), "", m.detailField("Destination", safe(d.destination), d.view.Width, m.styles.text), "", diagnostics.Included(), "",
		"Unknown values are omitted, not generically redacted. Only allowlisted operational values and structural types are included.",
		"This is a troubleshooting snapshot, not a complete raw telemetry capture.",
	}
	status := "Enter creates this local JSON file. Esc cancels."
	if d.writing {
		status = "Writing local diagnostic file…"
	}
	if d.saved {
		status = "Saved locally. No upload occurred. Inspect the JSON before sharing."
	}
	if d.problem != "" {
		status = d.problem
	}
	lines = append([]string{status, ""}, lines...)
	d.view.SetContent(wrapLines(lines, d.view.Width))
}

// exportView captures quit keys until the export confirmation/result view is closed.
func (m Model) exportView() string {
	hints := "Enter export  Esc close"
	d := m.exportDialog
	if d.writing {
		hints = "Writing…  Esc close"
	} else if d.saved {
		hints = "Saved locally  Esc close"
	} else if d.destination == "" || m.Farm.Printers[d.printerID] == nil {
		hints = "Unavailable  Esc close"
	} else if d.problem != "" {
		hints = "Enter retry  Esc close"
	}
	return m.fullView([]string{m.styles.accent.Bold(true).Render("LOCAL DIAGNOSTIC EXPORT"), "↑/↓ or PgUp/PgDn scroll · local file only; no upload", "", d.view.View()}, hints)
}

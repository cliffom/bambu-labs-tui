package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type clearNoticesDialog struct {
	active map[string]farm.Condition
	events []uint64
}

func (m *Model) openClearNotices() {
	d := &clearNoticesDialog{active: map[string]farm.Condition{}}
	for key, c := range m.Farm.Active {
		if !c.Dismissed {
			d.active[key] = c
		}
	}
	for _, e := range m.Farm.Events {
		d.events = append(d.events, e.ID)
	}
	if len(d.active)+len(d.events) == 0 {
		m.toast, m.toastAt = "No notices or recent events to clear", m.now
		return
	}
	m.openDialog(dialogClearNotices)
	m.clearNotices = d
}

func (m *Model) clearNoticesKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case "y", "Y":
		for key, before := range m.clearNotices.active {
			current, ok := m.Farm.Active[key]
			// Match occurrence identity using the same fields as reconciliation.
			// A resolved/replaced occurrence must survive this older confirmation.
			sameText := key != "cloud" && !strings.HasSuffix(key, "/state") || current.Text == before.Text
			if ok && current.Since.Equal(before.Since) && current.Level == before.Level && current.Code == before.Code && sameText {
				m.Farm.Dismiss(key)
			}
		}
		for _, id := range m.clearNotices.events {
			m.Farm.DeleteEvent(id)
		}
		m.closeDialog()
		m.toast, m.toastAt = "Confirmed notices cleared locally; newer notices and events kept", m.now
	case "esc", "n", "N":
		m.closeDialog()
	}
	return nil
}

func (m Model) clearNoticesView() string {
	d := m.clearNotices
	persistence := "Dismissals are saved when notice storage is available."
	if m.Demo {
		persistence = "Demo dismissals apply only to this session."
	}
	return m.fullView([]string{
		warning.Bold(true).Render("CLEAR ALL NOTICES?"), "",
		fmt.Sprintf("Dismiss locally: %d visible active notices", len(d.active)),
		fmt.Sprintf("Remove: %d recent session events", len(d.events)),
		"Scope: entire account farm (all printers and filters).", "",
		"Clear only the occurrences and events captured here.",
		"New notices and events are kept. Telemetry continues.", "",
		persistence,
		"Resolved conditions return unreviewed if they recur.",
		"No printer commands are sent or faults resolved.", "",
		"y: clear all notices     n / Esc: cancel",
	}, "y clear all notices · n / Esc cancel")
}

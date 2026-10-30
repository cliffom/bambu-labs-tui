package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambustand/internal/farm"
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
		m.focus = panelPrinters
		m.setToast(feedbackInfo, "No notices or recent events to clear; focus returned to printers")
		return
	}
	m.openDialog(dialogClearNotices)
	m.clearNotices = d
}

func (m *Model) clearNoticesKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyYes, keyYesUpper:
		for key, before := range m.clearNotices.active {
			current, ok := m.Farm.Active[key]
			// Match occurrence identity using the same fields as reconciliation.
			// A resolved/replaced occurrence must survive this older confirmation.
			sameText := key != farm.ConditionCloud && !strings.HasSuffix(key, "/"+farm.ConditionState) || current.Text == before.Text
			if ok && current.Since.Equal(before.Since) && current.Level == before.Level && current.Code == before.Code && sameText {
				m.Dismiss(key)
			}
		}
		for _, id := range m.clearNotices.events {
			m.DeleteEvent(id)
		}
		m.closeDialog()
		m.setToast(feedbackSuccess, "Confirmed notices cleared locally; newer notices and events kept")
		if len(m.noticeItems()) == 0 {
			m.focus = panelPrinters
			m.setToast(feedbackSuccess, "Confirmed notices cleared locally; focus returned to printers")
		}
	case keyEscape, keyDecisionsOrNo, keyNoUpper:
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
		m.styles.warning.Bold(true).Render("CLEAR ALL NOTICES?"), "",
		fmt.Sprintf("Dismiss locally: %d visible active notices", len(d.active)),
		fmt.Sprintf("Remove: %d recent session events", len(d.events)),
		"Scope: entire account farm (all printers and filters).", "",
		"Clear only the occurrences and events captured here.",
		"New notices and events are kept. Telemetry continues.", "",
		persistence,
		"Resolved conditions return unreviewed if they recur.",
		"No printer commands are sent or faults resolved.", "",
		"y: clear all notices     n / Esc: cancel",
	}, "y clear notices  n/Esc cancel")
}

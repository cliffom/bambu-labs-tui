package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type (
	attentionOccurrence struct {
		key   string
		since time.Time
	}

	attentionGroup struct {
		id, name string
		items    []farm.Attention
	}

	attentionDialog struct {
		groups []attentionGroup
		index  int
		view   viewport.Model
		manual bool
	}
)

// syncAttention batches new reports without replacing a dialog being read.
func (m *Model) syncAttention(force bool) {
	if m.attentionSeen == nil {
		m.attentionSeen = map[attentionOccurrence]bool{}
	}
	for occurrence := range m.attentionSeen {
		if c, ok := m.Farm.Active[occurrence.key]; !ok || !c.Since.Equal(occurrence.since) {
			delete(m.attentionSeen, occurrence)
		}
	}
	manual := force || (m.attention != nil && m.attention.manual)
	var pending []farm.Attention
	for _, item := range m.Farm.Attention() {
		if !m.attentionSeen[attentionOccurrence{item.Key, item.Since}] && (manual || m.automaticAlertAllowed(item)) {
			pending = append(pending, item)
		}
	}
	if m.attention != nil {
		m.attention.add(pending)
		return
	}
	if len(pending) == 0 {
		m.attentionPending = time.Time{}
		return
	}
	if m.attentionPending.IsZero() {
		m.attentionPending = m.now
	}
	if m.hasOpenDialog() || !m.terminalFits() {
		return
	}
	if !force && m.now.Sub(m.attentionPending) < attentionDeferral {
		return
	}
	m.openDialog(dialogAttention)
	m.attention = &attentionDialog{view: viewport.New(1, 1), manual: force}
	m.attention.add(pending)
	m.attentionPending = time.Time{}
}

// add groups incoming occurrences by printer while preserving existing order.
func (d *attentionDialog) add(items []farm.Attention) {
	for _, item := range items {
		index := -1
		for i, group := range d.groups {
			if group.id == item.PrinterID {
				index = i
				break
			}
		}
		if index < 0 {
			d.groups = append(d.groups, attentionGroup{id: item.PrinterID, name: item.Printer})
			index = len(d.groups) - 1
		}
		group := &d.groups[index]
		group.name = item.Printer
		replaced := false
		for i, old := range group.items {
			if old.Key == item.Key && old.Since.Equal(item.Since) {
				group.items[i] = item
				replaced = true
				break
			}
		}
		if !replaced {
			group.items = append(group.items, item)
		}
	}
}

// attentionActive prevents actions from applying to a resolved or replaced occurrence.
func (m Model) attentionActive(item farm.Attention) (farm.Condition, bool) {
	c, ok := m.Farm.Active[item.Key]
	return c, ok && c.Since.Equal(item.Since) && !c.Dismissed
}

// closeAttention suppresses only this session's automatic prompt, without reviewing notices.
func (m *Model) closeAttention() {
	for _, group := range m.attention.groups {
		for _, item := range group.items {
			m.attentionSeen[attentionOccurrence{item.Key, item.Since}] = true
		}
	}
	m.dismissDialog(dialogAttention)
}

// attentionKey handles local actions while transport events continue normally.
func (m *Model) attentionKey(key string, msg tea.Msg) tea.Cmd {
	group := m.attention.groups[m.attention.index]
	switch key {
	case keyEscape:
		m.closeAttention()
	case keyAlerts:
		m.openAlerts(group.id)
	case keyNextPanel, keyPreviousPanel:
		delta := 1
		if key == keyPreviousPanel {
			delta = len(m.attention.groups) - 1
		}
		m.attention.index = (m.attention.index + delta) % len(m.attention.groups)
		m.attention.view.GotoTop()
	case keyAcknowledge, keyClear:
		for _, item := range group.items {
			if _, active := m.attentionActive(item); active {
				if key == keyAcknowledge {
					m.Farm.Acknowledge(item.Key, true)
				} else {
					m.Farm.Dismiss(item.Key)
				}
			}
			m.attentionSeen[attentionOccurrence{item.Key, item.Since}] = true
		}
		m.attention.groups = append(m.attention.groups[:m.attention.index], m.attention.groups[m.attention.index+1:]...)
		if len(m.attention.groups) == 0 {
			m.dismissDialog(dialogAttention)
		} else {
			m.attention.index %= len(m.attention.groups)
			m.attention.view.GotoTop()
		}
	case keyEnter, keyPrinter:
		m.closeAttention()
		if m.selectPrinter(group.id) {
			m.focus = panelDetails
		}
		if key == keyEnter {
			for _, item := range group.items {
				if _, active := m.attentionActive(item); active {
					m.focus, m.noticeFocused, m.noticeKey = panelNotices, true, activeNoticePrefix+item.Key
					m.openNotice()
					break
				}
			}
		}
	case keyOpenURL:
		for _, item := range group.items {
			if url := hmsURL(item.Code); url != "" {
				return openTroubleshooting(url)
			}
		}
	default:
		var cmd tea.Cmd
		m.attention.view, cmd = m.attention.view.Update(msg)
		return cmd
	}
	return nil
}

// observationTime describes the app's observation, never a printer-side start time.
func observationTime(since, now time.Time) string {
	if since.IsZero() {
		return "First observed: unavailable"
	}
	age := "just now"
	elapsed := now.Sub(since)
	switch {
	case elapsed < 0:
		age = "clock skew; observation is in the future"
	case elapsed >= 24*time.Hour:
		age = observationAge(int(elapsed/(24*time.Hour)), "day")
	case elapsed >= time.Hour:
		age = observationAge(int(elapsed/time.Hour), "hour")
	case elapsed >= time.Minute:
		age = observationAge(int(elapsed/time.Minute), "minute")
	}
	return "First observed: " + since.Local().Format("2006-01-02 15:04:05 MST (Z07:00)") + " (" + age + ")"
}

func observationAge(count int, unit string) string {
	if count != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s ago", count, unit)
}

// syncAttentionView refreshes reported state and marks resolved notices in place.
func (m *Model) syncAttentionView() {
	d := m.attention
	group := d.groups[d.index]
	var lines []string
	if p := m.Farm.Printers[group.id]; p != nil {
		lines = append(lines, white.Bold(true).Render("Reported state: "+safe(stateLabel(p.State))+" · "+safe(p.Connection(m.now, m.Farm.Connected))))
	}
	lines = append(lines, muted.Render("Review each notice below. Actions apply to this printer's displayed active notices."))
	relevant := map[attentionOccurrence]bool{}
	for _, item := range m.Farm.Attention() {
		relevant[attentionOccurrence{item.Key, item.Since}] = true
	}
	for i, item := range group.items {
		c, active := m.attentionActive(item)
		status, text := "Resolved / no longer active", item.Text
		if current, exists := m.Farm.Active[item.Key]; exists {
			if !current.Since.Equal(item.Since) {
				status = "Replaced by a new occurrence"
			} else if current.Dismissed {
				status = "Hidden locally"
			}
		}
		if active {
			status, text = "Unreviewed", c.Text
			if c.Acknowledged {
				status = "Acknowledged locally"
			} else if !relevant[attentionOccurrence{item.Key, item.Since}] {
				status = "No longer requires an alert"
			}
		}
		// Farm appends the canonical code to condition text; show it once as metadata.
		if item.Code != "" {
			text = strings.TrimSuffix(text, " ["+item.Code+"]")
		}
		lines = append(lines, "", warning.Bold(true).Render(fmt.Sprintf("%d. %s", i+1, safe(text))))
		if active && relevant[attentionOccurrence{item.Key, item.Since}] {
			lines = append(lines, white.Render("Next: "+safe(item.Guidance)))
		}
		lines = append(lines, muted.Render(status), muted.Render(observationTime(item.Since, m.now)))
		if item.Code != "" {
			lines = append(lines, muted.Render("Code: "+safe(item.Code)))
		}
		if url := hmsURL(item.Code); url != "" {
			lines = append(lines, muted.Render("HMS troubleshooting available (o opens first link; Enter for URL)"))
		}
	}
	if p := m.Farm.Printers[group.id]; p != nil {
		lines = append(lines, "", muted.Render("Alerts: "+m.alertStatus(p.ID)))
	}
	lines = append(lines, "", "Local actions: acknowledge marks reviewed; hide removes notices locally.", "Snooze delays this printer's alerts; Esc closes all queued prompts for this session.", "Local review does not resolve a fault. Use the printer or Bambu app for physical intervention and printer controls.", "First observed is app observation time, not printer fault start or desktop delivery time.")
	d.view.Width, d.view.Height = max(1, m.width), max(1, m.height-4)
	offset := d.view.YOffset
	d.view.SetContent(ansi.Wrap(strings.Join(lines, "\n"), d.view.Width, ""))
	d.view.SetYOffset(offset)
}

// attentionView uses the shared full-screen notice layout and single-row footer.
func (m Model) attentionView() string {
	d := m.attention
	hints := "a acknowledge  c hide  s snooze  Esc close"
	if m.width >= 100 {
		hints = "Enter notice  p printer  o link  " + hints
	}
	lines := []string{danger.Bold(true).Render("ACTION NEEDED"), accent.Bold(true).Render(ansi.Truncate(fmt.Sprintf("Printer %d / %d · %s", d.index+1, len(d.groups), safe(d.groups[d.index].name)), max(1, m.width), "…")), muted.Render("Tab printer · ↑/↓ scroll · Enter notice · p printer · o link"), d.view.View()}
	return m.fullView(lines, hints)
}

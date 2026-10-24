package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type attentionOccurrence struct {
	key   string
	since time.Time
}

type attentionGroup struct {
	id, name string
	items    []farm.Attention
}

type attentionDialog struct {
	groups []attentionGroup
	index  int
	view   viewport.Model
}

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
	var pending []farm.Attention
	for _, item := range m.Farm.Attention() {
		if !m.attentionSeen[attentionOccurrence{item.Key, item.Since}] {
			pending = append(pending, item)
		}
	}
	if m.attention != nil {
		m.attention.add(pending)
		m.syncAttentionView()
		return
	}
	if len(pending) == 0 {
		m.attentionPending = time.Time{}
		return
	}
	if m.attentionPending.IsZero() {
		m.attentionPending = m.now
	}
	if m.inspected != nil || m.filament != nil || m.decisions != nil || m.confirmClear || m.help || m.width < 60 || m.height < 18 {
		return
	}
	if !force && m.now.Sub(m.attentionPending) < 2*time.Second {
		return
	}
	m.attention = &attentionDialog{view: viewport.New(1, 1)}
	m.attention.add(pending)
	m.attentionPending = time.Time{}
	m.syncAttentionView()
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
			if old.Key == item.Key {
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
	m.attention = nil
}

// attentionKey handles local actions while transport events continue normally.
func (m *Model) attentionKey(key string, msg tea.Msg) tea.Cmd {
	group := m.attention.groups[m.attention.index]
	switch key {
	case "esc":
		m.closeAttention()
	case "tab", "shift+tab":
		delta := 1
		if key == "shift+tab" {
			delta = len(m.attention.groups) - 1
		}
		m.attention.index = (m.attention.index + delta) % len(m.attention.groups)
		m.attention.view.GotoTop()
	case "a", "c":
		for _, item := range group.items {
			if _, active := m.attentionActive(item); active {
				if key == "a" {
					m.Farm.Acknowledge(item.Key, true)
				} else {
					m.Farm.Dismiss(item.Key)
				}
			}
			m.attentionSeen[attentionOccurrence{item.Key, item.Since}] = true
		}
		m.attention.groups = append(m.attention.groups[:m.attention.index], m.attention.groups[m.attention.index+1:]...)
		if len(m.attention.groups) == 0 {
			m.attention = nil
		} else {
			m.attention.index %= len(m.attention.groups)
			m.attention.view.GotoTop()
		}
	case "enter", "p":
		m.closeAttention()
		for i, id := range m.Farm.Order {
			if id == group.id {
				m.selection, m.focus = i, 1
				m.details.GotoTop()
				break
			}
		}
		if key == "enter" {
			for _, item := range group.items {
				if _, active := m.attentionActive(item); active {
					m.focus, m.noticeFocused, m.noticeKey = 2, true, "active:"+item.Key
					m.openNotice()
					break
				}
			}
		}
	case "o":
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

// syncAttentionView refreshes reported state and marks resolved notices in place.
func (m *Model) syncAttentionView() {
	d := m.attention
	group := d.groups[d.index]
	lines := []string{accent.Bold(true).Render(safe(group.name))}
	if p := m.Farm.Printers[group.id]; p != nil {
		lines = append(lines, "Reported state: "+safe(stateLabel(p.State))+" · "+safe(p.Connection(m.now, m.Farm.Connected)))
	}
	relevant := map[attentionOccurrence]bool{}
	for _, item := range m.Farm.Attention() {
		relevant[attentionOccurrence{item.Key, item.Since}] = true
	}
	for _, item := range group.items {
		c, active := m.attentionActive(item)
		status, text := "Resolved / no longer active", item.Text
		if active {
			status, text = "Needs review", c.Text
			if c.Acknowledged {
				status = "Acknowledged locally"
			} else if !relevant[attentionOccurrence{item.Key, item.Since}] {
				status = "No longer requires an alert"
			}
		}
		lines = append(lines, "", warning.Render(status), safe(text))
		if item.Code != "" {
			lines = append(lines, "Code: "+safe(item.Code))
		}
		if active && relevant[attentionOccurrence{item.Key, item.Since}] {
			lines = append(lines, safe(item.Guidance))
		}
		if url := hmsURL(item.Code); url != "" {
			lines = append(lines, url)
		}
	}
	lines = append(lines, "", "a acknowledges only the notices shown here.", "c hides those notices locally; it does not resolve the printer fault.", "Esc closes all queued prompts for this session without acknowledging.", "Use the printer or Bambu app for physical intervention and printer controls.")
	d.view.Width, d.view.Height = max(1, m.width-8), max(1, m.height-8)
	d.view.SetContent(wrapLines(lines, d.view.Width))
}

// attentionView renders a centered keyboard-driven attention modal.
func (m Model) attentionView() string {
	d := m.attention
	lines := []string{danger.Bold(true).Render("ACTION NEEDED"), muted.Render(fmt.Sprintf("Printer %d / %d · Tab switches printer · ↑/↓ scroll", d.index+1, len(d.groups))), "", d.view.View(), "", "Enter notice · p printer · a ack · c hide · o help", "Esc close"}
	if m.noticeStorageError != "" {
		lines[len(lines)-1] = m.noticeStorageError
	}
	box := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#F4BA72")).Padding(0, 1).Width(max(1, m.width-4)).Render(strings.Join(lines, "\n"))
	return fit(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box), m.width, m.height)
}

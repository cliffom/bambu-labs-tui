package ui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// noticeItem identities stay stable as reports arrive and historical events are
// prepended or evicted. Active conditions and historical events are distinct.
type noticeItem struct {
	Key, ActiveKey             string
	EventID                    uint64
	Printer, Level, Text, Code string
	At                         time.Time
	Active, Acknowledged       bool
}

type browserResultMsg struct{ err error }

func openTroubleshooting(url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		program := "xdg-open"
		if runtime.GOOS == "darwin" {
			program = "/usr/bin/open"
		}
		return browserResultMsg{exec.CommandContext(ctx, program, url).Run()}
	}
}

func (m Model) noticeItems() []noticeItem {
	items := make([]noticeItem, 0, len(m.Farm.Active)+len(m.Farm.Events))
	keys := make([]string, 0, len(m.Farm.Active))
	for k := range m.Farm.Active {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := m.Farm.Active[k]
		if c.Dismissed {
			continue
		}
		items = append(items, noticeItem{Key: "active:" + k, ActiveKey: k, Printer: c.Printer, Level: c.Level, Text: c.Text, Code: c.Code, At: c.Since, Active: true, Acknowledged: c.Acknowledged})
	}
	for i := len(m.Farm.Events) - 1; i >= 0; i-- {
		e := m.Farm.Events[i]
		items = append(items, noticeItem{Key: fmt.Sprintf("event:%d", e.ID), EventID: e.ID, Printer: e.Printer, Level: e.Level, Text: e.Text, Code: e.Code, At: e.At})
	}
	return items
}
func (m *Model) syncNoticeSelection(items []noticeItem) {
	if len(items) == 0 {
		m.noticeKey = ""
		m.noticeIndex = 0
		return
	}
	for i, item := range items {
		if item.Key == m.noticeKey {
			m.noticeIndex = i
			return
		}
	}
	m.noticeIndex = min(m.noticeIndex, len(items)-1)
	m.noticeKey = items[m.noticeIndex].Key
}
func (m Model) selectedNotice() (noticeItem, bool) {
	for _, item := range m.noticeItems() {
		if item.Key == m.noticeKey {
			return item, true
		}
	}
	return noticeItem{}, false
}
func (m *Model) moveNotice(delta int, absolute bool) {
	items := m.noticeItems()
	if len(items) == 0 {
		return
	}
	index := m.noticeIndex + delta
	if absolute {
		index = delta
	}
	m.noticeIndex = max(0, min(len(items)-1, index))
	m.noticeKey = items[m.noticeIndex].Key
}
func (m *Model) openNotice() {
	if item, ok := m.selectedNotice(); ok {
		m.inspected = &item
		m.inspection.GotoTop()
	}
}
func (m *Model) focusFault() {
	p := m.current()
	if p == nil {
		return
	}
	for _, item := range m.noticeItems() {
		if strings.HasPrefix(item.ActiveKey, p.ID+"/fault/") {
			m.focus = 2
			m.noticeFocused = true
			m.noticeKey = item.Key
			m.openNotice()
			return
		}
	}
	m.toast = "No visible faults for " + p.Name
	m.toastAt = m.now
}
func (m *Model) acknowledge(item noticeItem, toggle bool) {
	if item.ActiveKey == "" || !item.Active {
		m.toast = "This is a historical event; press c to remove it"
	} else {
		value := true
		if toggle {
			value = !item.Acknowledged
		}
		if m.Farm.Acknowledge(item.ActiveKey, value) {
			m.toast = "Acknowledged locally; the printer condition remains active"
			if !value {
				m.toast = "Notice marked as unreviewed"
			}
		}
	}
	m.toastAt = m.now
}
func (m *Model) clearNotice(item noticeItem) {
	if item.Active {
		m.Farm.Dismiss(item.ActiveKey)
		m.toast = "Notice dismissed locally; printer state unchanged"
		m.toastAt = m.now
		m.inspected = nil
		return
	}
	if item.EventID != 0 {
		m.Farm.DeleteEvent(item.EventID)
		m.toast = "Event removed from this session's history"
	} else {
		m.toast = "Condition has resolved"
	}
	m.toastAt = m.now
	m.inspected = nil
}
func (m *Model) inspectorKey(key string, msg tea.Msg) tea.Cmd {
	switch key {
	case "esc", "enter":
		m.inspected = nil
	case "a":
		m.acknowledge(*m.inspected, true)
	case "c":
		m.clearNotice(*m.inspected)
	case "o":
		if url := hmsURL(m.inspected.Code); url != "" {
			return openTroubleshooting(url)
		}
	default:
		var cmd tea.Cmd
		m.inspection, cmd = m.inspection.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) noticeText(w int) string {
	items := m.noticeItems()
	m.noticeSpans = nil
	var lines []string
	if m.Farm.NoticeCount() == 0 {
		lines = append(lines, accent.Render("✓ No active notices"))
	} else {
		lines = append(lines, warning.Bold(true).Render(fmt.Sprintf("%d ACTIVE · %d unreviewed", m.Farm.NoticeCount(), m.Farm.Unacknowledged())))
	}
	historyHeader := false
	for _, item := range items {
		if !item.Active && !historyHeader {
			lines = append(lines, "", muted.Render("RECENT EVENTS · THIS SESSION"))
			historyHeader = true
		}
		style := muted
		switch item.Level {
		case "error":
			style = danger
		case "warning":
			style = warning
		}
		marker := "  "
		if item.Key == m.noticeKey {
			marker = "› "
		}
		title := item.Printer
		if item.Active {
			title = "! " + title
			if item.Acknowledged {
				title += " [ACK]"
				style = muted
			}
		} else {
			title = item.At.Local().Format("15:04:05") + "  " + title
		}
		start := len(lines)
		text := wrapLines([]string{style.Render(marker + safe(title)), "  " + safe(item.Text), ""}, w)
		wrapped := strings.Split(text, "\n")
		lines = append(lines, wrapped...)
		m.noticeSpans = append(m.noticeSpans, [2]int{start, len(lines) - 1})
	}
	if !historyHeader {
		lines = append(lines, "", muted.Render("RECENT EVENTS · THIS SESSION"), muted.Render("No recent events"))
	}
	return strings.Join(lines, "\n")
}
func (m *Model) keepNoticeVisible() {
	if m.focus != 2 || m.noticeIndex >= len(m.noticeSpans) {
		return
	}
	span := m.noticeSpans[m.noticeIndex]
	offset := m.notices.YOffset
	if span[1] >= offset+m.notices.Height {
		offset = span[1] - m.notices.Height + 1
	}
	if span[0] < offset {
		offset = span[0]
	}
	m.notices.SetYOffset(offset)
}

func hmsURL(code string) string {
	parts := strings.Split(code, "_")
	if len(parts) != 4 {
		return ""
	}
	for _, p := range parts {
		if len(p) != 4 {
			return ""
		}
		for _, r := range strings.ToUpper(p) {
			if !strings.ContainsRune("0123456789ABCDEF", r) {
				return ""
			}
		}
	}
	return "https://wiki.bambulab.com/en/x1/troubleshooting/hmscode/" + strings.ToLower(code)
}
func (m *Model) syncInspector() {
	if m.inspected == nil {
		return
	}
	item := *m.inspected
	if item.ActiveKey != "" {
		item.Active = false
		if c, ok := m.Farm.Active[item.ActiveKey]; ok {
			item.Active = true
			item.Acknowledged = c.Acknowledged
			item.Text = c.Text
			item.Printer = c.Printer
			item.At, item.Level, item.Code = c.Since, c.Level, c.Code
		}
	}
	m.inspected = &item
	status := "Historical event"
	if item.Active {
		status = "Active · unreviewed"
		if item.Acknowledged {
			status = "Active · acknowledged locally"
		}
	} else if item.ActiveKey != "" {
		status = "Resolved by printer / telemetry"
	}
	when := "Reported"
	if item.ActiveKey != "" {
		when = "First seen"
	}
	lines := []string{accent.Bold(true).Render(safe(item.Printer)), "", safe(item.Text), "", "Status    " + status, "Severity  " + safe(item.Level), when + "  " + item.At.Local().Format("2006-01-02 15:04:05 MST")}
	if item.Code != "" {
		lines = append(lines, "Code      "+safe(item.Code))
	}
	if url := hmsURL(item.Code); url != "" {
		lines = append(lines, "", accent.Render("BAMBU TROUBLESHOOTING · o opens in browser"), url)
	}
	if item.Active {
		persistence := "Acknowledgements and dismissals are saved across restarts until resolution."
		if m.Demo || m.saveReviews == nil || m.noticeStorageError != "" {
			persistence = "Acknowledgements and dismissals apply only to this session."
		}
		lines = append(lines, "", "a: toggle acknowledgement · c: dismiss from this TUI", persistence, "Resolved conditions return unreviewed if they recur.", "Printer-side dismissal is not sent by this monitoring application.")
	} else if item.EventID != 0 {
		lines = append(lines, "", "c: remove this event from the current session's history")
	}
	m.inspection.Width = max(1, m.width-4)
	m.inspection.Height = max(1, m.height-6)
	m.inspection.SetContent(wrapLines(lines, m.inspection.Width))
}
func (m Model) inspectorView() string {
	lines := []string{accent.Bold(true).Render("NOTICE / FAULT DETAILS"), muted.Render("↑/↓ or PgUp/PgDown scroll · Esc/Enter back"), "", m.inspection.View()}
	footer := "a acknowledge  c clear locally  Esc back  q quit"
	if m.toast != "" {
		footer = safe(m.toast)
	}
	return m.fullView(lines, footer)
}
func (m Model) fullView(lines []string, footer string) string {
	if m.noticeStorageError != "" {
		footer = m.noticeStorageError
	}
	rows := strings.Split(strings.Join(lines, "\n"), "\n")
	if len(rows) > max(0, m.height-1) {
		rows = rows[:max(0, m.height-1)]
	}
	for len(rows) < m.height-1 {
		rows = append(rows, "")
	}
	rows = append(rows, muted.Render(footer))
	return fit(strings.Join(rows, "\n"), m.width, m.height)
}
func (m Model) clearHistoryView() string {
	return m.fullView([]string{warning.Bold(true).Render("CLEAR RECENT EVENTS?"), "", fmt.Sprintf("Remove %d events from this session's history?", len(m.Farm.Events)), "", "Active notices and printer faults remain visible.", "", "y: clear history     n / Esc: keep history"}, "q quit")
}

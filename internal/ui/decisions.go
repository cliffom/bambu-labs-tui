package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type decisionItem struct {
	key, printer, text, code, level string
	review                          farm.Review
	active                          bool
}

type decisionsDialog struct {
	key          string
	index        int
	view         viewport.Model
	confirmReset bool
}

// decisionItems includes reviewed active conditions and decisions awaiting telemetry.
func (m Model) decisionItems() []decisionItem {
	items := make(map[string]decisionItem)
	for key, r := range m.Farm.Reviews() {
		id, _, _ := strings.Cut(key, "/")
		printer := "Printer " + id
		if key == "cloud" {
			printer = "Farm"
		} else if p := m.Farm.Printers[id]; p != nil {
			printer = p.Name
		}
		text := r.Text
		if text == "" {
			text = pendingDecisionDescription(key)
		}
		items[key] = decisionItem{key: key, printer: printer, text: text, code: r.Code, level: r.Level, review: r}
	}
	for key, c := range m.Farm.Active {
		if c.Acknowledged || c.Dismissed {
			items[key] = decisionItem{key: key, printer: c.Printer, text: c.Text, code: c.Code, level: c.Level, active: true,
				review: farm.Review{Since: c.Since, Acknowledged: c.Acknowledged, Dismissed: c.Dismissed}}
		}
	}
	result := make([]decisionItem, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].key < result[j].key })
	return result
}

// pendingDecisionDescription identifies saved decisions without claiming a live condition.
func pendingDecisionDescription(key string) string {
	_, family, found := strings.Cut(key, "/")
	if !found {
		family = key
	}
	parts := strings.Split(family, "/")
	if len(parts) == 3 && parts[0] == "low" {
		unit, unitErr := strconv.ParseUint(parts[1], 10, 8)
		slot, slotErr := strconv.ParseUint(parts[2], 10, 8)
		if unitErr == nil && slotErr == nil && slot < 4 {
			return fmt.Sprintf("%s slot %d: saved low-filament decision (awaiting telemetry)", farm.AMSLabel(int(unit)), slot+1)
		}
	}
	return fmt.Sprintf("Saved notice (%s); description awaits printer telemetry", family)
}

// selectItem preserves identity through incoming reports, falling back after removal.
func (d *decisionsDialog) selectItem(items []decisionItem) (decisionItem, bool) {
	if len(items) == 0 {
		d.key, d.index = "", 0
		return decisionItem{}, false
	}
	for i, item := range items {
		if item.key == d.key {
			d.index = i
			return item, true
		}
	}
	d.index = min(d.index, len(items)-1)
	d.key = items[d.index].key
	d.view.GotoTop()
	return items[d.index], true
}

// openDecisions opens account-scoped local reviews without changing dashboard focus.
func (m *Model) openDecisions() {
	m.decisions = &decisionsDialog{view: viewport.New(1, 1)}
	m.syncDecisions()
}

// decisionsKey edits local decisions and captures all keys until the dialog closes.
func (m *Model) decisionsKey(key string, msg tea.Msg) tea.Cmd {
	d := m.decisions
	if d.confirmReset {
		switch key {
		case "y", "Y":
			m.Farm.ResetReviews()
			m.attentionSeen = nil
			d.confirmReset = false
			m.toast, m.toastAt = "Local notice decisions reset; printer state unchanged", m.now
		case "n", "N", "esc":
			d.confirmReset = false
		}
		return nil
	}
	switch key {
	case "esc", "n":
		m.decisions = nil
		return nil
	case "q", "ctrl+c":
		return nil
	case "R":
		d.confirmReset = len(m.decisionItems()) > 0
		return nil
	}
	items := m.decisionItems()
	item, ok := d.selectItem(items)
	if !ok {
		return nil
	}
	switch key {
	case "up", "k", "down", "j", "home", "g", "end", "G":
		switch key {
		case "up", "k":
			d.index--
		case "down", "j":
			d.index++
		case "home", "g":
			d.index = 0
		case "end", "G":
			d.index = len(items) - 1
		}
		d.index = max(0, min(len(items)-1, d.index))
		d.key = items[d.index].key
		d.view.GotoTop()
	case "r", "u":
		changed := false
		if key == "r" {
			changed = m.Farm.RestoreDismissal(item.key)
		} else {
			changed = m.Farm.MarkUnreviewed(item.key)
		}
		if changed {
			// Explicit review reversal re-arms only this occurrence's session prompt.
			delete(m.attentionSeen, attentionOccurrence{item.key, item.review.Since})
			m.toast, m.toastAt = "Local decision updated; printer state unchanged", m.now
		}
	default:
		var cmd tea.Cmd
		d.view, cmd = d.view.Update(msg)
		return cmd
	}
	return nil
}

// syncDecisions keeps descriptions and status current without creating active faults.
func (m *Model) syncDecisions() {
	d := m.decisions
	if d == nil {
		return
	}
	d.view.Width, d.view.Height = max(1, m.width-2), max(1, m.height-6)
	items := m.decisionItems()
	item, ok := d.selectItem(items)
	var lines []string
	if !ok {
		lines = []string{"No local notice decisions.", "", "Acknowledged and dismissed notices appear here.", "Resolved decisions are removed when confirmed by telemetry."}
	} else {
		status := "Awaiting telemetry · active status not confirmed"
		if item.active {
			status = "Active · reported by printer / connection status"
		}
		dismissal, acknowledgement := "Visible", "Unreviewed"
		if item.review.Dismissed {
			dismissal = "Dismissed locally"
		}
		if item.review.Acknowledged {
			acknowledgement = "Acknowledged locally"
		}
		code := item.code
		if code == "" {
			code = "—"
		}
		field := func(label, value string) string { return detailField(label, safe(value), d.view.Width, white) }
		lines = []string{accent.Render(fmt.Sprintf("Notice %d / %d", d.index+1, len(items))), "",
			field("Printer", item.printer), field("Description", item.text), field("Code", code), field("Severity", item.level), field("Status", status),
			field("Visibility", dismissal), field("Review", acknowledgement), field("First Seen", item.review.Since.Local().Format("2006-01-02 15:04:05 MST")), "",
			"r restores visibility and keeps acknowledgement.", "u marks unreviewed and keeps visibility / dismissal.", "Actionable unreviewed notices can alert after this view closes.", "Restoring a notice does not recreate deleted history."}
	}
	d.view.SetContent(wrapLines(lines, d.view.Width))
}

// decisionsView presents saved decisions and an explicit account-scoped reset confirmation.
func (m Model) decisionsView() string {
	d := m.decisions
	if d.confirmReset {
		lines := []string{warning.Bold(true).Render("RESET NOTICE DECISIONS?"), "",
			"Reset all notice decisions for this account and region?", "",
			"Includes saved decisions awaiting telemetry.", "Active notices become visible and unreviewed.", "Session alert suppression resets; actionable notices can prompt again.", "Printer state and event history remain unchanged.", "",
			"y: reset decisions     n / Esc: cancel"}
		return m.fullView([]string{wrapLines(lines, m.width)}, "n / Esc cancel")
	}
	mode := "Account and region · local decisions"
	if m.Demo {
		mode = "Demo · session decisions only"
	}
	return m.fullView([]string{accent.Bold(true).Render("NOTICE DECISIONS"), muted.Render(mode), muted.Render("↑/↓ select · Home/End first/last · PgUp/PgDn scroll"), "", d.view.View()}, "r restore · u unreviewed · R reset all · n / Esc close")
}

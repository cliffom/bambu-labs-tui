package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
)

type (
	printerFilter int

	printerSort int

	printerOverview struct {
		attention, unavailable bool
		notices                int
		eta                    time.Time
		liveETA                bool
	}

	farmOverview struct {
		printing, stopped, attention, unavailable, idle int
	}
)

const (
	filterAll printerFilter = iota
	filterAttention
	filterPrinting
	filterUnavailable
	filterCount
)

const (
	sortName printerSort = iota
	sortAttention
	sortState
	sortFinish
	sortCount
)

// selectedPrinterID stays usable even when discovery removes the selected printer.
func (m Model) selectedPrinterID() string {
	if m.selection < 0 || m.selection >= len(m.printerIDs) {
		return ""
	}
	return m.printerIDs[m.selection]
}

// finishTimeCurrent excludes unavailable printers and old remaining-time estimates.
func (m Model) finishTimeCurrent(p *farm.Printer) bool {
	_, reported := p.ETA()
	return reported && m.printerTelemetryCurrent(p) && m.now.Sub(p.RemainingReported) <= 120*time.Second
}

// syncPrinterList builds a view without changing farm discovery order or alert eligibility.
func (m *Model) syncPrinterList() {
	selectedID := m.selectedPrinterID()
	attention := map[string]bool{}
	for _, item := range m.Farm.Attention() {
		attention[item.PrinterID] = true
	}
	notices := map[string]int{}
	for key, c := range m.Farm.Active {
		if !c.Dismissed {
			id, _, _ := strings.Cut(key, "/")
			notices[id]++
		}
	}
	m.overview = farmOverview{}
	m.printerMetrics = make(map[string]printerOverview, len(m.Farm.Printers))
	ids := make([]string, 0, len(m.Farm.Printers))
	for id, p := range m.Farm.Printers {
		eta, _ := p.ETA()
		metric := printerOverview{attention: attention[id], unavailable: !m.printerTelemetryCurrent(p), notices: notices[id], eta: eta, liveETA: m.finishTimeCurrent(p)}
		m.printerMetrics[id] = metric
		if metric.attention {
			m.overview.attention++
		}
		if metric.unavailable {
			m.overview.unavailable++
		}
		switch p.State {
		case "RUNNING":
			m.overview.printing++
		case "PAUSE", "FAILED":
			m.overview.stopped++
		case "IDLE", "FINISH":
			m.overview.idle++
		}
		if m.matchesFilter(p, metric) {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, m.comparePrinters)
	m.printerIDs = ids
	m.selection = max(0, min(m.selection, len(ids)-1))
	if index := slices.Index(ids, selectedID); index >= 0 {
		m.selection = index
	}
	if selectedID != m.selectedPrinterID() {
		m.details.GotoTop()
	}
}

// matchesFilter uses verified actionable notices, never raw severity or interruption policy.
func (m Model) matchesFilter(p *farm.Printer, metric printerOverview) bool {
	switch m.printerFilter {
	case filterAttention:
		return metric.attention
	case filterPrinting:
		return p.State == "RUNNING"
	case filterUnavailable:
		return metric.unavailable
	default:
		return true
	}
}

// attentionRank puts unreviewed actionable printers ahead of stopped or unavailable ones.
func (m Model) attentionRank(id string) int {
	p, metric := m.Farm.Printers[id], m.printerMetrics[id]
	switch {
	case metric.attention:
		return 0
	case p.State == "PAUSE" || p.State == "FAILED":
		return 1
	case metric.unavailable:
		return 2
	case p.State == "RUNNING":
		return 3
	default:
		return 4
	}
}

// comparePrinters provides deterministic name/ID ties for every live sort.
func (m Model) comparePrinters(aID, bID string) int {
	a, b := m.Farm.Printers[aID], m.Farm.Printers[bID]
	comparison := 0
	switch m.printerSort {
	case sortAttention:
		comparison = cmp.Compare(m.attentionRank(aID), m.attentionRank(bID))
	case sortState:
		comparison = strings.Compare(strings.ToLower(stateLabel(a.State)), strings.ToLower(stateLabel(b.State)))
	case sortFinish:
		x, y := m.printerMetrics[aID], m.printerMetrics[bID]
		if x.liveETA != y.liveETA {
			if x.liveETA {
				return -1
			}
			return 1
		}
		if x.liveETA {
			comparison = x.eta.Compare(y.eta)
		}
	}
	if comparison != 0 {
		return comparison
	}
	if comparison = strings.Compare(strings.ToLower(safe(a.Name)), strings.ToLower(safe(b.Name))); comparison != 0 {
		return comparison
	}
	return strings.Compare(aID, bID)
}

// selectPrinter reveals explicit attention-dialog navigation even outside the current filter.
func (m *Model) selectPrinter(id string) bool {
	if m.Farm.Printers[id] == nil {
		return false
	}
	if !slices.Contains(m.printerIDs, id) {
		m.printerFilter = filterAll
		m.syncPrinterList()
	}
	m.selection = slices.Index(m.printerIDs, id)
	m.details.GotoTop()
	return true
}

// printerListStatus keeps active settings, counts, and shortcuts visible at minimum width.
func (m Model) printerListStatus() string {
	filters := []string{"All", "Attention", "Printing", "Unavailable"}
	sorts := []string{"Name", "Attention", "State", "Finish"}
	return fmt.Sprintf("Filter: %s (%d/%d) F · Sort: %s S", filters[m.printerFilter], len(m.printerIDs), len(m.Farm.Printers), sorts[m.printerSort])
}

// overviewLines counts all printers independently of filtering; state counts are last reported.
func (m Model) overviewLines() []string {
	v := m.overview
	state := fmt.Sprintf("%d printers · %d printing · %d paused/failed · %d idle/done", len(m.Farm.Printers), v.printing, v.stopped, v.idle)
	attention := fmt.Sprintf("%d attention · %d unavailable · %d notices (%d unreviewed)", v.attention, v.unavailable, m.Farm.NoticeCount(), m.Farm.Unacknowledged())
	if ansi.StringWidth(state) > m.width {
		state = fmt.Sprintf("%d total · %d print · %d stopped · %d idle/done", len(m.Farm.Printers), v.printing, v.stopped, v.idle)
	}
	if ansi.StringWidth(attention) > m.width {
		attention = fmt.Sprintf("%d attn · %d unavail · %d notices (%d unrev)", v.attention, v.unavailable, m.Farm.NoticeCount(), m.Farm.Unacknowledged())
	}
	if policy := m.alertSummary(); ansi.StringWidth(attention+policy) <= m.width {
		attention += policy
	}
	return []string{muted.Render(state), muted.Render(attention)}
}

// emptyPrinterList distinguishes unmatched filters from discovery and empty accounts.
func (m Model) emptyPrinterList() string {
	if len(m.Farm.Printers) > 0 {
		return "No printers match this filter. Press F to change it."
	}
	if m.printersDiscovered {
		return "No printers found. Bind your printers to this Bambu account."
	}
	return "Discovering account printers…"
}

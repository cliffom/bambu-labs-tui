package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

// overviewModel includes every summary group and unavailable/missing ETA cases.
func overviewModel(t *testing.T) Model {
	t.Helper()
	m := demoModel()
	devices := []farm.Device{{ID: "H", Name: "Alpha", Online: true}, {ID: "X", Name: "Beta", Online: true}, {ID: "I", Name: "Idle", Online: true}, {ID: "D", Name: "Done", Online: true}, {ID: "O", Name: "Offline"}, {ID: "T", Name: "Stale", Online: true}, {ID: "W", Name: "Waiting", Online: true}}
	updated, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: devices, At: m.now})
	m = updated.(Model)
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","mc_remaining_time":20,"mc_percent":40}}`)
	m = attentionReport(t, m, "X", runoutReport)
	m = attentionReport(t, m, "I", `{"print":{"gcode_state":"IDLE"}}`)
	m = attentionReport(t, m, "D", `{"print":{"gcode_state":"FINISH"}}`)
	m = attentionReport(t, m, "T", `{"print":{"gcode_state":"RUNNING","mc_remaining_time":1}}`)
	m.Farm.Printers["T"].LastReport = m.now.Add(-121 * time.Second)
	m.syncPanels()
	return m
}

func TestIdleFreshnessMatchesUnavailableFilterAndNotices(t *testing.T) {
	for _, state := range []string{"IDLE", "FINISH"} {
		t.Run(state, func(t *testing.T) {
			m := demoModel()
			m.Farm = farm.New(10)
			m.Farm.Cloud(true, "", m.now)
			m.Farm.Discover([]farm.Device{{ID: "I", Name: "Idle", Online: true}}, m.now)
			if err := m.Farm.Report("I", []byte(fmt.Sprintf(`{"print":{"gcode_state":%q}}`, state)), m.now); err != nil {
				t.Fatal(err)
			}
			at := m.now
			m.printerFilter = filterUnavailable
			for _, age := range []time.Duration{3 * time.Minute, 10 * time.Minute, 10*time.Minute + time.Second} {
				m = alertTickAt(m, at.Add(age))
				_, notice := m.Farm.Active["I/stale"]
				unavailable := age > 10*time.Minute
				wantCount := 0
				if unavailable {
					wantCount = 1
				}
				if notice != unavailable || m.overview.unavailable != wantCount || len(m.printerIDs) != wantCount || m.printerTelemetryCurrent(m.Farm.Printers["I"]) == unavailable {
					t.Fatalf("age %v: notice=%v overview=%+v ids=%v", age, notice, m.overview, m.printerIDs)
				}
			}
		})
	}
}

// TestFarmSummaryAndFilters counts printers independently from visible rows and interruption policy.
func TestFarmSummaryAndFilters(t *testing.T) {
	m := overviewModel(t)
	if m.overview != (farmOverview{printing: 2, stopped: 1, attention: 1, unavailable: 3, idle: 2}) {
		t.Fatalf("wrong whole-farm counts: %+v", m.overview)
	}
	original := slices.Clone(m.Farm.Order)
	prefs := config.DefaultAlerts()
	prefs.AutomaticModals = false
	m = m.WithAlertPreferences(prefs)
	m.setSnooze("X", time.Hour)
	m = key(m, "F")
	if !slices.Equal(m.printerIDs, []string{"X"}) || m.overview.attention != 1 || m.current().ID != "X" || !strings.Contains(m.printerListStatus(), "Attention (1/7)") {
		t.Fatal("attention filter/count depended on interruption policy")
	}
	m = key(m, "F")
	if !slices.Equal(m.printerIDs, []string{"H", "T"}) {
		t.Fatalf("wrong printing filter: %v", m.printerIDs)
	}
	m = key(m, "F")
	if !slices.Equal(m.printerIDs, []string{"T", "W", "O"}) {
		t.Fatalf("wrong unavailable filter: %v", m.printerIDs)
	}
	m = key(m, "F")
	if len(m.printerIDs) != 7 || !slices.Equal(m.Farm.Order, original) || len(m.Farm.Reviews()) != 0 {
		t.Fatal("filtering changed farm state or reviews")
	}
}

// TestAttentionHonorsReviewsAndNeverUsesSeverity keeps low/maintenance/offline notices out of urgency.
func TestAttentionHonorsReviewsAndNeverUsesSeverity(t *testing.T) {
	m := overviewModel(t)
	m.printerFilter = filterAttention
	m.syncPanels()
	for _, item := range m.Farm.Attention() {
		m.Farm.Acknowledge(item.Key, true)
	}
	m.syncPanels()
	if len(m.printerIDs) != 0 || m.overview.attention != 0 || m.overview.stopped != 1 {
		t.Fatal("reviewed conditions still count as needing attention")
	}
	if !strings.Contains(ansi.Strip(m.View()), "No printers match") || m.current() != nil {
		t.Fatal("empty attention filter retained a selected printer")
	}
	for keyName := range m.Farm.Reviews() {
		m.Farm.MarkUnreviewed(keyName)
	}
	m.syncPanels()
	if !slices.Equal(m.printerIDs, []string{"X"}) {
		t.Fatal("reversed reviews did not restore attention membership")
	}
	for _, item := range m.Farm.Attention() {
		m.Farm.Dismiss(item.Key)
	}
	m.syncPanels()
	if len(m.printerIDs) != 0 {
		t.Fatal("dismissed conditions still counted")
	}
}

// TestLiveSortingPreservesIdentityAndScroll follows the printer rather than its row number.
func TestLiveSortingPreservesIdentityAndScroll(t *testing.T) {
	m := overviewModel(t)
	m.selectPrinter("H")
	m.focus = panelDetails
	m.details.SetYOffset(4)
	m = key(m, "S") // Name -> Attention; Alpha moves below Beta.
	if m.current().ID != "H" || m.selection == 0 || m.focus != panelDetails || m.details.YOffset != 4 {
		t.Fatal("sort lost selected identity or details scroll")
	}
	m = attentionReport(t, m, "I", runoutReport)
	if m.current().ID != "H" || m.details.YOffset != 4 {
		t.Fatal("incoming alert changed selected printer")
	}
	m.printerSort = sortName
	m.syncPanels()
	devices := []farm.Device{{ID: "H", Name: "Zulu", Online: true}, {ID: "X", Name: "Beta", Online: true}, {ID: "I", Name: "Idle", Online: true}}
	updated, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: devices, At: m.now})
	m = updated.(Model)
	if m.current().ID != "H" || m.selection != 2 || m.details.YOffset != 4 {
		t.Fatal("renaming/discovery moved selection to a different printer")
	}
	devices = devices[1:]
	updated, _ = m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: devices, At: m.now})
	m = updated.(Model)
	if m.current() == nil || m.current().ID == "H" || m.details.YOffset != 0 || m.focus != panelDetails {
		t.Fatal("removed selection did not fall back safely")
	}
}

// TestFinishSortRejectsOldMissingAndPausedEstimates puts only live comparable ETAs first.
func TestFinishSortRejectsOldMissingAndPausedEstimates(t *testing.T) {
	m := overviewModel(t)
	m.Farm.Printers["I"].State = "RUNNING"
	ten := 10
	m.Farm.Printers["I"].Remaining = &ten
	m.Farm.Printers["I"].RemainingReported = m.now
	m.printerSort = sortFinish
	m.syncPanels()
	if !slices.Equal(m.printerIDs[:2], []string{"I", "H"}) {
		t.Fatalf("bad ETA order: %v", m.printerIDs)
	}
	for _, id := range []string{"X", "D", "O", "T", "W"} {
		if m.printerMetrics[id].liveETA {
			t.Fatalf("invalid ETA treated as comparable: %s", id)
		}
	}
	m.selectPrinter("H")
	m.focus = panelDetails
	m.details.SetYOffset(3)
	m = attentionReport(t, m, "H", `{"print":{"mc_remaining_time":1}}`)
	if m.printerIDs[0] != "H" || m.current().ID != "H" || m.details.YOffset != 3 {
		t.Fatal("live ETA reorder lost selection/scroll")
	}
	// Temperature-only updates keep the link fresh without refreshing the time estimate.
	m.Farm.Printers["H"].RemainingReported = m.now.Add(-121 * time.Second)
	m.Farm.Printers["H"].LastReport = m.now
	m.syncPanels()
	if m.printerMetrics["H"].liveETA || m.printerIDs[0] != "I" || !strings.Contains(ansi.Strip(m.printerRow(m.selection)), "~") {
		t.Fatal("old ETA sorted as live or was not marked last-known")
	}
}

// TestFilteringOutSelectionFallsBackAndResetsDetails handles live state changes and no matches.
func TestFilteringOutSelectionFallsBackAndResetsDetails(t *testing.T) {
	m := overviewModel(t)
	m.printerFilter = filterPrinting
	m.syncPanels()
	m.selectPrinter("H")
	m.focus = panelDetails
	m.details.SetYOffset(4)
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"FINISH"}}`)
	if m.current() == nil || m.current().ID != "T" || m.details.YOffset != 0 || m.focus != panelDetails {
		t.Fatal("filter exit did not choose a visible fallback")
	}
	m = attentionReport(t, m, "T", `{"print":{"gcode_state":"IDLE"}}`)
	if m.current() != nil || len(m.printerIDs) != 0 || !strings.Contains(ansi.Strip(m.details.View()), "No printers match") {
		t.Fatal("empty filter not explicit")
	}
	m = key(m, "F") // Unavailable, then All.
	m = key(m, "F")
	if m.current() == nil || len(m.printerIDs) != 7 {
		t.Fatal("filter navigation could not leave empty results")
	}
}

// TestAttentionNavigationRevealsHiddenPrinter leaves an excluding filter for an explicit jump.
func TestAttentionNavigationRevealsHiddenPrinter(t *testing.T) {
	m := demoModel()
	m.printerFilter = filterPrinting
	m.syncPanels()
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.attention == nil {
		t.Fatal("filter suppressed a printer's automatic alert")
	}
	m = key(m, "p")
	if m.printerFilter != filterAll || m.current() == nil || m.current().ID != "X" || m.focus != panelDetails {
		t.Fatal("explicit printer jump remained hidden by the filter")
	}
}

// TestOverviewBoundsAndSettingsVisibility covers a larger farm at every supported layout.
func TestOverviewBoundsAndSettingsVisibility(t *testing.T) {
	m := demoModel()
	devices := make([]farm.Device, 30)
	for i := range devices {
		devices[i] = farm.Device{ID: fmt.Sprintf("P%02d", i), Name: fmt.Sprintf("Printer %02d", i), Online: true}
	}
	updated, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, Devices: devices, At: m.now})
	m = updated.(Model)
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
			for _, text := range []string{"Filter: All (30/30) F", "Sort: Name S", "PRINTER DETAILS", "NOTICES"} {
				if !strings.Contains(view, text) {
					t.Fatalf("size %v hides %q", size, text)
				}
			}
		}
	}
}

// TestStateNameAndTieSortingUsesIdentity keeps duplicate names deterministic and counts independent.
func TestStateNameAndTieSortingUsesIdentity(t *testing.T) {
	m := demoModel()
	m.Farm.Printers["H"].Name = "same"
	m.Farm.Printers["X"].Name = "Same"
	m = attentionReport(t, m, "X", runoutReport)
	m.syncPanels()
	if !slices.Equal(m.printerIDs, []string{"H", "X"}) || m.printerMetrics["H"].notices != 0 || m.printerMetrics["X"].notices != 3 {
		t.Fatal("name ties mixed printer identities/notice counts")
	}
	m.printerSort = sortState
	m.syncPanels()
	if m.printerIDs[0] != "X" {
		t.Fatal("state sort is not alphabetical by displayed state")
	}
}

// TestUnavailableFilterTracksCloudConnection preserves identity as all links disconnect/reconnect.
func TestUnavailableFilterTracksCloudConnection(t *testing.T) {
	m := overviewModel(t)
	m.printerFilter = filterUnavailable
	m.syncPanels()
	m.selectPrinter("T")
	for _, connected := range []bool{false, true} {
		updated, _ := m.Update(monitor.Event{Kind: monitor.EventConnection, Connected: connected, At: m.now})
		m = updated.(Model)
		want := 7
		if connected {
			want = 3
		}
		if len(m.printerIDs) != want || m.overview.unavailable != want || m.current().ID != "T" {
			t.Fatal("cloud availability lost counts or selected identity")
		}
	}
}

// TestStateSortingUsesDisplayedLabels sorts displayed labels within each activity group.
func TestStateSortingUsesDisplayedLabels(t *testing.T) {
	m := overviewModel(t)
	m.printerSort = sortState
	m.syncPanels()
	if !slices.Equal(m.printerIDs, []string{"X", "H", "T", "W", "D", "I", "O"}) {
		t.Fatalf("wrong displayed-state order: %v", m.printerIDs)
	}
}

func TestOfflinePrintersFollowOnlineForEverySort(t *testing.T) {
	for _, tc := range []struct {
		name string
		sort printerSort
		want []string
	}{
		{"Name", sortName, []string{"H", "X", "T", "W", "D", "I", "O", "Z"}},
		{"Attention", sortAttention, []string{"X", "T", "W", "H", "D", "I", "Z", "O"}},
		{"State", sortState, []string{"X", "H", "T", "W", "D", "I", "Z", "O"}},
		{"Finish", sortFinish, []string{"H", "X", "T", "W", "D", "I", "O", "Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := overviewModel(t)
			m.Farm.Printers["O"].Name = "Aardvark"
			m.Farm.Printers["Z"] = &farm.Printer{Device: farm.Device{ID: "Z", Name: "Zulu"}, State: farm.StateFailed}
			m.printerSort = tc.sort
			for _, connected := range []bool{true, false} {
				m.Farm.Connected = connected
				m.syncPanels()
				want := tc.want
				if !connected && tc.sort == sortAttention {
					want = []string{"X", "H", "T", "W", "D", "I", "Z", "O"}
				}
				if !slices.Equal(m.printerIDs, want) {
					t.Fatalf("connected=%v: got %v, want %v", connected, m.printerIDs, want)
				}
			}
		})
	}
}

func TestOnlineChangesReorderWithoutMovingSelection(t *testing.T) {
	for order := sortName; order < sortCount; order++ {
		t.Run(string(sortValues[order]), func(t *testing.T) {
			for _, state := range []string{farm.StateRunning, farm.StatePaused, farm.StateFailed, farm.StateIdle, farm.StateFinished, farm.StateUnknown} {
				m := demoModel()
				m.printerSort = order
				m.Farm.Printers["H"].State = state
				m.Farm.Printers["X"].State = state
				m.selectPrinter("H")
				m.focus = panelDetails
				m.details.SetYOffset(3)
				for _, online := range []bool{false, true} {
					next, _ := m.Update(monitor.Event{Kind: monitor.EventDevices, At: m.now, Devices: []farm.Device{
						{ID: "H", Name: "H2D", Online: online},
						{ID: "X", Name: "X2D", Online: true},
					}})
					m = next.(Model)
					want := []string{"X", "H"}
					if online {
						want = []string{"H", "X"}
					}
					if !slices.Equal(m.printerIDs, want) || m.current().ID != "H" || m.focus != panelDetails || m.details.YOffset != 3 {
						t.Fatalf("online=%v: order=%v selection=%s scroll=%d", online, m.printerIDs, m.current().ID, m.details.YOffset)
					}
				}
			}
		})
	}
}

// Unknown/missing states stay outside confirmed idle/done, including during a
// cloud outage. Staleness changes availability, never reported state or online.
func TestActivityGroupsPreserveTelemetryDistinctions(t *testing.T) {
	for _, state := range []string{farm.StateRunning, farm.StatePreparing, farm.StateSlicing, farm.StatePaused, farm.StateFailed, farm.StateUnknown, "", "FUTURE_STATE"} {
		for order := sortName; order < sortCount; order++ {
			t.Run(fmt.Sprintf("%s/%s", state, sortValues[order]), func(t *testing.T) {
				m := demoModel()
				m.Farm.Printers = map[string]*farm.Printer{
					"active":  {Device: farm.Device{ID: "active", Name: "Zulu", Online: true}, State: state},
					"idle":    {Device: farm.Device{ID: "idle", Name: "Alpha", Online: true}, State: farm.StateIdle},
					"done":    {Device: farm.Device{ID: "done", Name: "Beta", Online: true}, State: farm.StateFinished},
					"offline": {Device: farm.Device{ID: "offline", Name: "Aardvark"}, State: state},
				}
				m.printerSort = order
				for _, connected := range []bool{true, false} {
					m.Farm.Connected = connected
					for _, age := range []time.Duration{0, 121 * time.Second, 11 * time.Minute} {
						for _, p := range m.Farm.Printers {
							p.LastReport = m.now.Add(-age)
						}
						// The unknown state also exercises a genuinely missing report.
						if state == "" {
							m.Farm.Printers["active"].LastReport = time.Time{}
						}
						m.syncPanels()
						if m.printerIDs[0] != "active" || m.printerIDs[3] != "offline" || m.overview.idle != 2 {
							t.Fatalf("connected=%v age=%v: order=%v summary=%+v", connected, age, m.printerIDs, m.overview)
						}
						if p := m.Farm.Printers["active"]; p.State != state || !p.Online {
							t.Fatal("grouping changed telemetry")
						}
					}
				}
			})
		}
	}
}

func TestActivityGroupTiesAndSingleGroupLists(t *testing.T) {
	for order := sortName; order < sortCount; order++ {
		for _, state := range []string{farm.StatePaused, farm.StateIdle, farm.StateFinished, farm.StateUnknown} {
			for _, online := range []bool{true, false} {
				m := demoModel()
				m.Farm.Printers = map[string]*farm.Printer{
					"B": {Device: farm.Device{ID: "B", Name: "same", Online: online}, State: state},
					"A": {Device: farm.Device{ID: "A", Name: "Same", Online: online}, State: state},
					"C": {Device: farm.Device{ID: "C", Name: "Zulu", Online: online}, State: state},
				}
				m.printerSort = order
				for range 5 {
					m.syncPanels()
					if !slices.Equal(m.printerIDs, []string{"A", "B", "C"}) {
						t.Fatalf("sort=%s state=%s online=%v: %v", sortValues[order], state, online, m.printerIDs)
					}
				}
				m.printerFilter = filterPrinting
				m.syncPanels()
				if m.current() != nil || !strings.Contains(ansi.Strip(m.View()), "No printers match") {
					t.Fatal("empty single-group filter retained selection")
				}
				m = key(m, "0")
				if m.current() == nil || len(m.printerIDs) != 3 || m.printerSort != sortName {
					t.Fatal("could not reset empty filter")
				}
			}
		}
	}
}

func TestActivityTransitionsPreserveSelectionFocusAndScroll(t *testing.T) {
	for order := sortName; order < sortCount; order++ {
		t.Run(string(sortValues[order]), func(t *testing.T) {
			m := overviewModel(t)
			m.printerSort = order
			m.syncPanels()
			m.selectPrinter("H")
			m.focus = panelDetails
			m.details.SetYOffset(3)
			for _, state := range []string{farm.StateIdle, farm.StateFinished, farm.StateRunning, farm.StatePaused, farm.StateFailed, farm.StateUnknown} {
				m = attentionReport(t, m, "H", fmt.Sprintf(`{"print":{"gcode_state":%q}}`, state))
				if m.current().ID != "H" || m.focus != panelDetails || m.details.YOffset != 3 {
					t.Fatalf("state=%s lost identity/focus/scroll", state)
				}
				other := slices.Index(m.printerIDs, "X")
				idle := state == farm.StateIdle || state == farm.StateFinished
				if idle && m.selection < other || !idle && m.selection > slices.Index(m.printerIDs, "I") {
					t.Fatalf("state=%s wrong group: %v", state, m.printerIDs)
				}
			}
		})
	}
}

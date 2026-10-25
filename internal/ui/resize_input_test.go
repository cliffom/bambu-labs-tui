package ui

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

func resizeModel(m Model, width, height int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(Model)
}

type hiddenInputState struct {
	owner                         dialogState
	focus, selection, noticeIndex int
	noticeKey                     string
	filter                        printerFilter
	sort                          printerSort
	reviews                       map[string]farm.Review
	events                        []farm.Event
	snoozes                       map[string]time.Time
	reset, writing                bool
}

func hiddenState(m Model) hiddenInputState {
	state := hiddenInputState{owner: m.dialog, focus: m.focus, selection: m.selection, noticeIndex: m.noticeIndex, noticeKey: m.noticeKey, filter: m.printerFilter, sort: m.printerSort, reviews: m.Farm.Reviews(), events: slices.Clone(m.Farm.Events), snoozes: maps.Clone(m.snoozes)}
	if m.decisions != nil {
		state.reset = m.decisions.confirmReset
	}
	if m.exportDialog != nil {
		state.writing = m.exportDialog.writing
	}
	return state
}

func TestUndersizedInterfaceBlocksHiddenActionsAndNavigation(t *testing.T) {
	for _, dialog := range []string{"dashboard", "help", "inspector", "history", "decisions", "reset", "filament", "alerts", "nested alerts", "export", "attention"} {
		t.Run(dialog, func(t *testing.T) {
			m := noticeModel(t)
			switch dialog {
			case "help":
				m = key(m, "?")
			case "inspector":
				m = key(m, "enter")
			case "history":
				m = key(m, "C")
			case "decisions", "reset":
				m = key(m, "a")
				m = key(m, "n")
				if dialog == "reset" {
					m = key(m, "R")
				}
			case "filament":
				m = key(m, "m")
			case "alerts":
				m = key(m, "s")
			case "export":
				m = key(m, "D")
			case "attention", "nested alerts":
				m = attentionTick(attentionReport(t, m, "X", runoutReport))
				if dialog == "nested alerts" {
					m = key(m, "s")
				}
			}
			original := hiddenState(m)
			for _, size := range [][2]int{{59, 18}, {60, 17}, {40, 10}, {0, 0}} {
				m = resizeModel(m, size[0], size[1])
				for _, input := range []string{"j", "k", "tab", "?", "a", "c", "C", "n", "R", "y", "Y", "m", "s", "1", "2", "3", "u", "D", "enter", "r", "F", "S", "!", "o", "p"} {
					next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(input)})
					m = next.(Model)
					if cmd != nil || !reflect.DeepEqual(hiddenState(m), original) {
						t.Fatalf("hidden key %q changed state or scheduled work at %v", input, size)
					}
				}
				if dialog != "dashboard" {
					for _, input := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, {Type: tea.KeyCtrlC}} {
						next, cmd := m.Update(input)
						m = next.(Model)
						if cmd != nil || !reflect.DeepEqual(hiddenState(m), original) {
							t.Fatal("quit escaped a hidden modal")
						}
					}
				}
			}
			m = resizeModel(m, 100, 32)
			if !reflect.DeepEqual(hiddenState(m), original) {
				t.Fatal("resizing back lost owner, selection, or confirmation")
			}
		})
	}
}

func TestUndersizedCloseReturnsToSuspendedAttention(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(m, "s")
	m = resizeModel(m, 40, 10)
	m = key(m, "esc")
	if m.dialog.active != dialogAttention || m.dialog.returnTo != dialogNone || m.alertDialog != nil {
		t.Fatal("hidden Esc did not return to attention")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() {
		t.Fatal("hidden attention did not close")
	}
	for _, input := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, {Type: tea.KeyCtrlC}} {
		_, cmd := m.Update(input)
		if cmd == nil {
			t.Fatal("undersized dashboard cannot quit")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("dashboard did not quit")
		}
	}
}

func TestHiddenResetPreservesScrollAndResumesAfterResize(t *testing.T) {
	m := noticeModel(t)
	item, _ := m.selectedNotice()
	condition := m.Farm.Active[item.ActiveKey]
	condition.Text = strings.Repeat("Detailed fault description. ", 100)
	m.Farm.Active[item.ActiveKey] = condition
	m = key(m, "a")
	m = key(m, "n")
	m.decisions.view.SetYOffset(1)
	offset, keyName := m.decisions.view.YOffset, m.decisions.key
	m = key(m, "R")
	if !m.decisions.confirmReset || offset == 0 {
		t.Fatal("fixture lacks scroll or reset confirmation")
	}
	m = resizeModel(m, 40, 10)
	m = key(m, "y")
	m = resizeModel(m, 100, 32)
	if !m.decisions.confirmReset || m.decisions.key != keyName || m.decisions.view.YOffset != offset {
		t.Fatal("hidden input/resize lost pending confirmation or scroll")
	}
	m = key(m, "y")
	if len(m.Farm.Reviews()) != 0 || m.decisions.confirmReset {
		t.Fatal("visible confirmation did not work after resize")
	}
}

func TestUndersizedScreenContinuesTelemetryAsyncResultsAndAttentionDeferral(t *testing.T) {
	m := key(demoModel(), "D")
	destination := m.exportDialog.destination
	m = resizeModel(m, 40, 10)
	m = attentionTick(attentionReport(t, m, "X", runoutReport))
	if m.attention != nil || len(m.Farm.Attention()) == 0 || m.dialog.active != dialogExport {
		t.Fatal("hidden owner blocked telemetry or lost focus")
	}
	next, _ := m.Update(exportResultMsg{destination: destination})
	m = next.(Model)
	if !m.exportDialog.saved {
		t.Fatal("hidden export completion was ignored")
	}
	m = key(m, "esc")
	if m.attention != nil {
		t.Fatal("attention opened while terminal could not fit")
	}
	m = resizeModel(m, 100, 32)
	if m.dialog.active != dialogAttention {
		t.Fatal("deferred attention did not resume on resize")
	}
}

func TestCompactShutdownAndPersistenceContinueBelowMinimumSize(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	m = m.WithNoticePersistence(func(map[string]farm.Review) error { return errors.New("disk full") }, nil)
	item, _ := m.selectedNotice()
	m.Farm.Acknowledge(item.ActiveKey, true)
	next, save := m.Update(nil)
	m = next.(Model)
	m = resizeModel(m, 40, 10)
	m = key(m, "q")
	if !m.quitting || m.dialog.active != dialogShutdown {
		t.Fatal("undersized dashboard did not start persistence shutdown")
	}
	next, _ = m.Update(save())
	m = next.(Model)
	if !m.quitProblem || m.reviewWrites.failed == 0 || !strings.Contains(m.shutdownView(), "Storage failed") {
		t.Fatal("hidden persistence failure was lost")
	}
	_, quit := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if quit == nil {
		t.Fatal("compact shutdown cannot explicitly exit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("compact shutdown did not quit")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() || m.quitting || m.quitProblem {
		t.Fatal("compact shutdown cannot cancel")
	}
	// Monitor completion remains handled independently of the input gate.
	next, _ = m.Update(monitor.Event{Kind: monitor.EventProblem, Problem: "still handled"})
	if next.(Model).toast != "still handled" {
		t.Fatal("undersized input policy blocked monitor events")
	}
}

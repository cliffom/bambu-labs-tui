package ui

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/noticestore"
)

func TestClearAllNoticesConfirmationAndPersistence(t *testing.T) {
	m := noticeModel(t)
	m.Farm.Cloud(false, "Reconnecting", m.now)
	m.syncPanels()
	m.printerFilter = filterPrinting
	active, events := m.Farm.NoticeCount(), len(m.Farm.Events)
	before := make(map[string]farm.Condition)
	for k, c := range m.Farm.Active {
		before[k] = c
	}
	history := append([]farm.Event(nil), m.Farm.Events...)
	for _, cancel := range []string{"n", "esc"} {
		m = key(m, "X")
		view := ansi.Strip(m.View())
		if m.dialog.active != dialogClearNotices || !strings.Contains(view, fmt.Sprintf("Dismiss locally: %d visible active notices", active)) || !strings.Contains(view, fmt.Sprintf("Remove: %d recent session events", events)) {
			t.Fatal("missing snapshot counts")
		}
		m = key(m, cancel)
		if m.clearNotices != nil || !reflect.DeepEqual(before, m.Farm.Active) || !reflect.DeepEqual(history, m.Farm.Events) {
			t.Fatal("cancel changed notices")
		}
	}
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "notices.json")}
	m.Demo = false
	m = m.WithNoticePersistence(store.Save, nil)
	m = persistenceKey(m, "X")
	m = persistenceKey(m, "y")
	saved, err := store.Load()
	if err != nil || len(saved) != active {
		t.Fatalf("bulk dismissals not saved: %v, %v", saved, err)
	}
	if m.Farm.NoticeCount() != 0 || len(m.Farm.Events) != 0 || len(m.Farm.Active) != len(before) {
		t.Fatal("bulk clear did not preserve underlying conditions")
	}
	for k, c := range before {
		if !m.Farm.Active[k].Dismissed || !saved[k].Dismissed || saved[k].Acknowledged != c.Acknowledged {
			t.Fatal("dismissal/review semantics changed")
		}
	}
	m.Farm.Reconcile(m.now.Add(time.Second))
	if m.Farm.NoticeCount() != 0 || len(m.Farm.Events) != 0 {
		t.Fatal("routine reconciliation restored cleared notices")
	}
	m.Farm.Cloud(true, "", m.now.Add(2*time.Second))
	m.Farm.Cloud(false, "Reconnecting", m.now.Add(3*time.Second))
	if c := m.Farm.Active["cloud"]; c.Dismissed || c.Acknowledged {
		t.Fatal("recurrence inherited review")
	}
}

func TestClearAllNoticesPreservesArrivalsAndReplacedOccurrences(t *testing.T) {
	for _, change := range []string{"new", "recurrence", "replacement"} {
		t.Run(change, func(t *testing.T) {
			m := noticeModel(t)
			if change != "new" {
				m.Farm.Cloud(false, "Reconnecting", m.now)
			}
			m = key(m, "X")
			captured := len(m.clearNotices.events)
			if change == "recurrence" {
				m.Farm.Cloud(true, "", m.now.Add(time.Second))
			}
			problem := "Reconnecting"
			if change == "replacement" {
				problem = "Session expired; sign in again"
			}
			next, _ := m.Update(monitor.Event{Kind: monitor.EventConnection, Problem: problem, At: m.now.Add(2 * time.Second)})
			m = next.(Model)
			if m.dialog.active != dialogClearNotices || len(m.clearNotices.events) != captured {
				t.Fatal("telemetry changed confirmation snapshot")
			}
			var arrivals []farm.Event
			for _, e := range m.Farm.Events {
				found := false
				for _, id := range m.clearNotices.events {
					found = found || id == e.ID
				}
				if !found {
					arrivals = append(arrivals, e)
				}
			}
			m = key(m, "y")
			if c := m.Farm.Active["cloud"]; c.Dismissed || c.Acknowledged {
				t.Fatal("new occurrence cleared")
			}
			if len(arrivals) == 0 || !reflect.DeepEqual(arrivals, m.Farm.Events) {
				t.Fatalf("new events lost: want %v, got %v", arrivals, m.Farm.Events)
			}
		})
	}
}

func TestClearAllNoticesEmptyAndResize(t *testing.T) {
	m := New(farm.New(10), nil, nil, true)
	m.focus = 2
	m = key(m, "X")
	if m.hasOpenDialog() || !strings.Contains(m.toast, "No notices") {
		t.Fatal("empty panel handling")
	}
	m = noticeModel(t)
	m = key(m, "X")
	active, events := m.Farm.NoticeCount(), len(m.Farm.Events)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = next.(Model)
	view := ansi.Strip(m.View())
	for _, text := range []string{"Dismiss locally:", "Remove:", "all printers and filters", "No printer commands", "y: clear all notices"} {
		if !strings.Contains(view, text) {
			t.Fatalf("minimum-size confirmation hid %q", text)
		}
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	m = key(next.(Model), "y")
	if m.Farm.NoticeCount() != active || len(m.Farm.Events) != events {
		t.Fatal("hidden confirmation accepted")
	}
	m = key(m, "esc")
	if m.clearNotices != nil || m.dialog.active == dialogClearNotices {
		t.Fatal("resize cancel retained dialog")
	}
}

func TestClearAllNoticesHistoryOnly(t *testing.T) {
	m := New(farm.New(10), nil, nil, true)
	m.focus = 2
	m.Farm.Cloud(false, "Reconnecting", m.now)
	m.Farm.Cloud(true, "", m.now.Add(time.Second))
	m = key(m, "X")
	if m.clearNotices == nil || len(m.clearNotices.active) != 0 || len(m.clearNotices.events) == 0 {
		t.Fatal("history-only confirmation missing")
	}
	m = key(m, "y")
	if len(m.Farm.Events) != 0 || m.Farm.NoticeCount() != 0 {
		t.Fatal("history-only clear failed")
	}
}

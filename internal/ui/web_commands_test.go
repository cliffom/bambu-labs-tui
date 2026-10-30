package ui

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/noticestore"
	"github.com/cliffom/bambustand/internal/webserver"
)

func browserDecision(t *testing.T, m Model, target string, retry bool) (Model, string) {
	t.Helper()
	c := webserver.Command{Target: target, Retry: retry, Reply: make(chan string, 1)}
	next, cmd := m.Update(c)
	return completeWeb(next.(Model), cmd), <-c.Reply
}

func TestBrowserDismissalSharesPersistenceAndTUIRestoration(t *testing.T) {
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "notices.json")}
	m := noticeModel(t)
	m.Demo = false
	server := &fakeWebServer{}
	m = m.WithNoticePersistence(store.Save, nil).WithWebServer(server)
	m.webEnabled = true
	m = completeWeb(m, m.syncWebServer())
	item, _ := m.selectedNotice()
	target := m.webDecisions().Targets[item.ActiveKey]
	selection, focus := m.selection, m.focus
	m = key(m, "enter")
	m.openStatus() // Inspector suspended under status must keep receiving updates.
	var result string
	m, result = browserDecision(t, m, target, false)
	if result != "applied" || !m.Farm.Active[item.ActiveKey].Dismissed || m.selection != selection || m.focus != focus {
		t.Fatal("dismissal or focus", result)
	}
	if !strings.Contains(ansi.Strip(m.inspection.View()), "hidden locally") {
		t.Fatal("suspended inspector did not reflect browser hide")
	}
	saved, err := store.Load()
	if err != nil || !saved[item.ActiveKey].Dismissed || m.webDecisions().State != "saved" {
		t.Fatal("not saved", err)
	}
	revision := m.Farm.ReviewRevision()
	m, result = browserDecision(t, m, target, false)
	if result != "applied" || m.Farm.ReviewRevision() != revision {
		t.Fatal("duplicate changed state")
	}
	m.Farm.RestoreDismissal(item.ActiveKey)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = next.(Model)
	var snapshot struct {
		Printers []struct {
			Conditions []struct {
				ID     string
				Hidden bool
			}
		}
	}
	if err := json.Unmarshal(server.snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range snapshot.Printers {
		for _, c := range p.Conditions {
			if c.ID == target {
				found = true
				if c.Hidden {
					t.Fatal("TUI restore missing from browser")
				}
			}
		}
	}
	if !found {
		t.Fatal("identity lost on restoration")
	}
}

func TestBrowserStaleTargetsRecurrenceAndShutdown(t *testing.T) {
	m := noticeModel(t).WithWebServer(&fakeWebServer{})
	m.webEnabled = true
	m.webDecisions()
	item, _ := m.selectedNotice()
	original := m.Farm.Active[item.ActiveKey]
	target := m.webDecisions().Targets[item.ActiveKey]
	replacement := original
	replacement.Since = original.Since.Add(time.Second)
	m.Farm.Active[item.ActiveKey] = replacement
	var result string
	m, result = browserDecision(t, m, target, false)
	if result == "applied" || m.Farm.Active[item.ActiveKey].Dismissed {
		t.Fatal("dismissed replacement")
	}
	fresh := m.webDecisions().Targets[item.ActiveKey]
	if fresh == target {
		t.Fatal("reused occurrence ID")
	}
	before := m.Farm.NoticeCount()
	m, result = browserDecision(t, m, fresh, false)
	if result != "applied" || m.Farm.NoticeCount() != before-1 {
		t.Fatal("independent notices changed")
	}
	delete(m.Farm.Active, item.ActiveKey)
	m.webDecisions()
	m.Farm.Active[item.ActiveKey] = original
	if m.webDecisions().Targets[item.ActiveKey] == target {
		t.Fatal("ID reused after clear with same timestamp")
	}
	m.quitting = true
	_, result = browserDecision(t, m, m.webDecisions().Targets[item.ActiveKey], false)
	if result == "applied" {
		t.Fatal("accepted during shutdown")
	}
}

func TestBrowserFilamentEstimatePreservesOccurrenceTarget(t *testing.T) {
	for _, pollFirst := range []bool{false, true} {
		name := "command before poll"
		if pollFirst {
			name = "poll before command"
		}
		t.Run(name, func(t *testing.T) {
			m := demoModel().WithWebServer(&fakeWebServer{})
			m.webEnabled = true
			remaining := 9
			m.Farm.Printers["H"].AMS = []farm.AMS{{ID: 0, Slots: []farm.Slot{{ID: 0, Remaining: &remaining}}}}
			m.Farm.Reconcile(m.now)
			const key = "H/low/0/0"
			original := m.Farm.Active[key]
			target := m.webDecisions().Targets[key]
			if target == "" {
				t.Fatal("missing low-filament target")
			}
			m.Farm.Acknowledge(key, true)
			remaining = 8
			m.Farm.Reconcile(m.now.Add(time.Second))
			updated := m.Farm.Active[key]
			if updated.Text == original.Text || !updated.Since.Equal(original.Since) || !updated.Acknowledged {
				t.Fatal("fixture did not update an ongoing reviewed occurrence")
			}
			if pollFirst && m.webDecisions().Targets[key] != target {
				t.Fatal("estimate update rotated target during polling")
			}
			m, result := browserDecision(t, m, target, false)
			if result != "applied" || !m.Farm.Active[key].Dismissed {
				t.Fatal("rejected ongoing occurrence", result)
			}
			remaining = 7
			m.Farm.Reconcile(m.now.Add(2 * time.Second))
			if m.webDecisions().Targets[key] != target || !m.Farm.Active[key].Dismissed {
				t.Fatal("later estimate lost target or local dismissal")
			}
		})
	}
}

func TestBrowserTextReplacementsRejectOldTargets(t *testing.T) {
	for _, key := range []string{farm.ConditionCloud, "H/" + farm.ConditionState} {
		t.Run(key, func(t *testing.T) {
			m := demoModel().WithWebServer(&fakeWebServer{})
			m.webEnabled = true
			original := farm.Condition{Since: m.now, Level: farm.LevelWarning, Text: "Original condition"}
			m.Farm.Active[key] = original
			target := m.webDecisions().Targets[key]
			replacement := original
			replacement.Text = "Replacement at the same timestamp"
			m.Farm.Active[key] = replacement
			command := webserver.Command{Target: target, Reply: make(chan string, 1)}
			m.applyWebCommand(command)
			if <-command.Reply == "applied" || m.Farm.Active[key].Dismissed {
				t.Fatal("old target dismissed replacement")
			}
			if m.webDecisions().Targets[key] == target {
				t.Fatal("replacement retained old target")
			}
		})
	}
}

func TestBrowserSaveFailureRetryAndFailedLoadProtection(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	fail, writes := true, 0
	m = m.WithWebServer(&fakeWebServer{}).WithNoticePersistence(func(map[string]farm.Review) error {
		writes++
		if fail {
			return errors.New("disk full")
		}
		return nil
	}, nil)
	m.webEnabled = true
	ids := m.webDecisions().Targets
	item, _ := m.selectedNotice()
	target := ids[item.ActiveKey]
	m, _ = browserDecision(t, m, target, false)
	if writes != 1 || m.webDecisions().State != "failed" || !m.Farm.Active[item.ActiveKey].Dismissed {
		t.Fatal("failed save reported success")
	}
	m, _ = browserDecision(t, m, target, false)
	if writes != 1 {
		t.Fatal("duplicate silently retried")
	}
	fail = false
	m, _ = browserDecision(t, m, target, true)
	if writes != 2 || m.webDecisions().State != "saved" {
		t.Fatal("explicit retry failed")
	}
	m = m.WithNoticePersistence(func(map[string]farm.Review) error { t.Fatal("overwrote unreadable store"); return nil }, errors.New("corrupt"))
	_, result := browserDecision(t, m, target, true)
	if result == "applied" || m.webDecisions().State == "saved" {
		t.Fatal("failed load allowed write")
	}
}

func TestBrowserCommandsCoalesceWhileSaveIsPending(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	var saved []map[string]farm.Review
	m = m.WithWebServer(&fakeWebServer{}).WithNoticePersistence(func(r map[string]farm.Review) error { saved = append(saved, r); return nil }, nil)
	m.webEnabled = true
	targets := m.webDecisions().Targets
	var keys []string
	for k := range targets {
		keys = append(keys, k)
	}
	if len(keys) < 2 {
		t.Fatal("fixture requires independent conditions")
	}
	first := webserver.Command{Target: targets[keys[0]], Reply: make(chan string, 1)}
	next, firstSave := m.Update(first)
	m = next.(Model)
	if <-first.Reply != "applied" || m.webDecisions().State != "saving" {
		t.Fatal("missing pending save state")
	}
	second := webserver.Command{Target: targets[keys[1]], Reply: make(chan string, 1)}
	next, _ = m.Update(second)
	m = next.(Model)
	if <-second.Reply != "applied" || len(saved) != 0 {
		t.Fatal("reducer blocked or overlapping writes")
	}
	// Telemetry/ticks remain serviceable while the first immutable snapshot waits.
	next, _ = m.Update(tickMsg(m.now))
	m = next.(Model)
	m = completeWeb(m, firstSave)
	if len(saved) != 2 || !saved[1][keys[0]].Dismissed || !saved[1][keys[1]].Dismissed || m.webDecisions().State != "saved" {
		t.Fatal("latest complete snapshot was not saved")
	}
}

func TestBrowserDismissalUpdatesSuspendedAttention(t *testing.T) {
	m := noticeModel(t).WithWebServer(&fakeWebServer{})
	m.webEnabled = true
	m.syncAttention(true)
	if m.attention == nil {
		t.Fatal("fixture has no actionable condition")
	}
	item := m.attention.groups[0].items[0]
	target := m.webDecisions().Targets[item.Key]
	m.openStatus()
	m, result := browserDecision(t, m, target, false)
	if result != "applied" || !strings.Contains(ansi.Strip(m.attention.view.View()), "Hidden locally") {
		t.Fatal("suspended attention not synchronized", result)
	}
	for _, eligible := range m.Farm.Attention() {
		if eligible.Key == item.Key {
			t.Fatal("hidden condition still eligible")
		}
	}
	m.closeDialog()
	if m.dialog.active != dialogAttention {
		t.Fatal("status lost suspended owner")
	}
}

func TestSessionTransitionsRejectBrowserTargetAcrossUnpublishedRecurrence(t *testing.T) {
	m := attentionReport(t, demoModel(), "H", runoutReport).WithWebServer(&fakeWebServer{})
	m.webEnabled = true
	key := "H/" + farm.ConditionState
	target := m.webDecisions().Targets[key]
	m.openStatus()
	owner, selection := m.dialog, m.selection
	// Exercise the extracted owner without a browser publication between transitions.
	// A future host must not need to render to invalidate cleared occurrence IDs.
	for _, payload := range []string{`{"print":{"gcode_state":"RUNNING","print_error":0,"hms":[]}}`, runoutReport} {
		if err := m.ApplyEvent(monitor.Event{Kind: monitor.EventReport, ID: "H", Payload: []byte(payload), At: m.now}); err != nil {
			t.Fatal(err)
		}
	}
	m, result := browserDecision(t, m, target, false)
	if result == "applied" || m.Farm.Active[key].Dismissed || m.dialog != owner || m.selection != selection {
		t.Fatal("stale target or suspended owner changed", result)
	}
	fresh := m.webDecisions().Targets[key]
	if fresh == "" || fresh == target {
		t.Fatal("recurrence did not get a new target")
	}
	m, result = browserDecision(t, m, fresh, false)
	if result != "applied" || !m.Farm.Active[key].Dismissed || m.dialog != owner {
		t.Fatal("fresh session decision failed", result)
	}
}

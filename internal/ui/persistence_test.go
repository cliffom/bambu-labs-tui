package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestNoticeDecisionsSavedBeforeQuitAndRestored(t *testing.T) {
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "notices.json")}
	m := noticeModel(t)
	m.Demo = false
	m = m.WithNoticePersistence(store.Save, nil)
	item, _ := m.selectedNotice()
	m = persistenceKey(m, "enter")
	m = persistenceKey(m, "a") // Inspector and list actions use the same persistence path.
	saved, err := store.Load()
	if err != nil || !saved[item.ActiveKey].Acknowledged {
		t.Fatalf("acknowledgement was not saved after command completion: %v, %v", saved, err)
	}
	m = persistenceKey(m, "a")
	saved, err = store.Load()
	if err != nil || len(saved) != 0 {
		t.Fatal("undo acknowledgement was not saved")
	}
	m = persistenceKey(m, "c")
	saved, err = store.Load()
	if err != nil || !saved[item.ActiveKey].Dismissed {
		t.Fatal("dismissal was not saved after command completion")
	}
	state := farm.New(10)
	state.RestoreReviews(saved)
	next := New(state, make(chan monitor.Event), make(chan struct{}, 1), false).WithNoticePersistence(store.Save, nil)
	for _, event := range []monitor.Event{
		{Kind: monitor.EventConnection, Connected: true},
		{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "H", Name: "H2D", Online: true}}},
		{Kind: monitor.EventReport, ID: "H", Payload: []byte(`{"print":{"mc_percent":75}}`)},
	} {
		event.At = time.Now()
		updated, _ := next.Update(event)
		next = updated.(Model)
	}
	// Restore the original fixture without touching the saved review snapshot.
	payload, err := json.Marshal(map[string]any{"print": m.Farm.Printers["H"].Raw})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := next.Update(monitor.Event{Kind: monitor.EventReport, ID: "H", Payload: payload, At: time.Now()})
	next = updated.(Model)
	if !next.Farm.Active[item.ActiveKey].Dismissed {
		t.Fatal("fresh model did not restore dismissal")
	}
}

func TestNoticePersistenceErrorsRemainVisible(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	writes := 0
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		writes++
		return errors.New("disk full")
	}, nil)
	m = persistenceKey(m, "enter")
	m = persistenceKey(m, "a")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("write failure not visible in inspector")
	}
	updated, _ := m.Update(tickMsg(time.Now().Add(time.Minute)))
	m = updated.(Model)
	m = persistenceKey(m, "esc")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("failure vanished or triggered continuous writes")
	}
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		writes++
		return nil
	}, errors.New("corrupt saved data"))
	m = persistenceKey(m, "a")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Saved notices unavailable") {
		t.Fatal("failed load overwrote saved data or warning was hidden")
	}
}

// persistenceKey explicitly runs the asynchronous command and delivers its result.
func persistenceKey(m Model, key string) Model {
	return persistenceUpdate(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func persistenceUpdate(m Model, msg tea.Msg) Model {
	next, cmd := m.Update(msg)
	result := next.(Model)
	switch msg.(type) {
	case tickMsg, monitor.Event:
		// Telemetry and ticks also schedule a blocking event read or a real timer.
		// The outer batch puts those first and persistence commands second.
		started := (!m.reviewWrites.inFlight && result.reviewWrites.inFlight) || (!m.snoozeWrites.inFlight && result.snoozeWrites.inFlight) || (!m.configWrites.inFlight && result.configWrites.inFlight)
		if !started {
			return result
		}
		batch := cmd().(tea.BatchMsg)
		cmd = batch[1]
	}
	return completePersistence(result, cmd)
}

func completePersistence(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, child := range msg {
			m = completePersistence(m, child)
		}
	case persistenceResultMsg, configResultMsg:
		next, follow := m.Update(msg)
		m = completePersistence(next.(Model), follow)
	}
	return m
}

func TestBlockedPersistenceCoalescesBothStoresWithoutBlockingReducer(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	item, _ := m.selectedNotice()
	reviews := make(chan map[string]farm.Review, 4)
	snoozes := make(chan map[string]time.Time, 4)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	m = m.WithNoticePersistence(func(snapshot map[string]farm.Review) error {
		reviews <- snapshot
		<-release
		return nil
	}, nil).WithSnoozePersistence(nil, func(snapshot map[string]time.Time) error {
		snoozes <- snapshot
		<-release
		return nil
	}, nil)
	m.Farm.Acknowledge(item.ActiveKey, true)
	m.setSnooze("H", time.Hour)
	initialDeadline := m.snoozes["H"]
	next, cmd := m.Update(nil)
	m = next.(Model)
	commands := cmd().(tea.BatchMsg)
	results := make(chan tea.Msg, 2)
	for _, save := range commands {
		go func() { results <- save() }()
	}
	// Both callbacks have entered their deliberately blocked disk operation.
	var firstReviews map[string]farm.Review
	var firstSnoozes map[string]time.Time
	select {
	case firstReviews = <-reviews:
	case <-time.After(time.Second):
		t.Fatal("notice save did not start")
	}
	select {
	case firstSnoozes = <-snoozes:
	case <-time.After(time.Second):
		t.Fatal("snooze save did not start")
	}
	if m.reviewWrites.saved == m.reviewWrites.attempted || !m.persistencePending() {
		t.Fatal("pending write reported saved")
	}

	// Input and telemetry must return while both save callbacks remain blocked.
	responsive := make(chan Model, 1)
	go func() {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
		model := updated.(Model)
		updated, _ = model.Update(monitor.Event{Kind: monitor.EventProblem, Problem: "telemetry handled", At: model.now})
		responsive <- updated.(Model)
	}()
	select {
	case m = <-responsive:
	case <-time.After(time.Second):
		t.Fatal("save blocked reducer")
	}
	if m.focus != panelPrinters || m.toast != "telemetry handled" {
		t.Fatal("input or telemetry was not handled")
	}
	for i := 0; i < 8; i++ {
		m.Farm.Acknowledge(item.ActiveKey, i%2 == 1)
		m.setSnooze("H", time.Duration(i+2)*time.Hour)
		updated, pending := m.Update(nil)
		m = updated.(Model)
		if pending != nil {
			t.Fatal("overlapping save scheduled")
		}
	}
	m.Farm.Dismiss(item.ActiveKey)
	m.setSnooze("X", 4*time.Hour)
	next, _ = m.Update(nil)
	m = next.(Model)
	if !firstReviews[item.ActiveKey].Acknowledged || firstReviews[item.ActiveKey].Dismissed || len(firstSnoozes) != 1 || !firstSnoozes["H"].Equal(initialDeadline) {
		t.Fatal("in-flight snapshots were mutated")
	}
	close(release)
	// Delayed completion of each older write starts exactly one latest snapshot.
	for range 2 {
		select {
		case msg := <-results:
			updated, latest := m.Update(msg)
			m = completePersistence(updated.(Model), latest)
		case <-time.After(time.Second):
			t.Fatal("blocked command did not finish")
		}
	}
	finalReviews, finalSnoozes := <-reviews, <-snoozes
	if !reflect.DeepEqual(finalReviews, m.Farm.Reviews()) || !reflect.DeepEqual(finalSnoozes, m.snoozes) {
		t.Fatal("newest complete snapshots were not persisted")
	}
	if m.HasUnsavedPersistence() || m.persistencePending() || m.reviewWrites.saved != m.Farm.ReviewRevision() || m.snoozeWrites.saved != m.snoozeRevision {
		t.Fatal("successful revisions not recorded")
	}
	if len(reviews) != 0 || len(snoozes) != 0 {
		t.Fatal("pending snapshots were not coalesced")
	}
}

func TestPersistenceQuitFlushAndCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint("cancel=", cancel), func(t *testing.T) {
			m := noticeModel(t)
			m.Demo = false
			m = m.WithNoticePersistence(func(map[string]farm.Review) error { return nil }, nil)
			item, _ := m.selectedNotice()
			m.Farm.Acknowledge(item.ActiveKey, true)
			next, save := m.Update(nil)
			m = next.(Model)
			m.closeDialog()
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			m = next.(Model)
			if !m.quitting || !strings.Contains(m.View(), "Saving") {
				t.Fatal("quit did not wait for pending save")
			}
			generation := m.quitGeneration
			// A newer change must also flush before exiting.
			m.Farm.Dismiss(item.ActiveKey)
			if cancel {
				m = key(m, "esc")
				if m.quitting {
					t.Fatal("Esc did not cancel shutdown")
				}
			}
			next, latest := m.Update(save())
			m = next.(Model)
			if !m.persistencePending() {
				t.Fatal("quit lost newer snapshot")
			}
			next, quit := m.Update(latest())
			m = next.(Model)
			if cancel {
				if quit != nil {
					t.Fatal("canceled shutdown still quit")
				}
				next, _ = m.Update(shutdownTimeoutMsg(generation))
				if next.(Model).quitProblem {
					t.Fatal("canceled timer changed model")
				}
			} else if quit == nil {
				t.Fatal("flush did not quit")
			} else if _, ok := quit().(tea.QuitMsg); !ok {
				t.Fatal("flush did not issue quit")
			}
		})
	}
}

func TestPersistenceShutdownFailureAndTimeout(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint("timeout=", timeout), func(t *testing.T) {
			m := demoModel()
			m.Demo = false
			m = m.WithSnoozePersistence(nil, func(map[string]time.Time) error { return errors.New("disk full") }, nil)
			m.setSnooze("H", time.Hour)
			next, save := m.Update(nil)
			m = next.(Model)
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			m = next.(Model)
			var msg tea.Msg = shutdownTimeoutMsg(m.quitGeneration)
			if !timeout {
				msg = save()
			}
			next, cmd := m.Update(msg)
			m = next.(Model)
			if cmd != nil || m.quitting || !m.quitProblem || !strings.Contains(m.View(), "changes may be lost") {
				t.Fatal("shutdown failure did not remain visible")
			}
			_, quit := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			if quit == nil {
				t.Fatal("explicit exit was ignored")
			}
			if _, ok := quit().(tea.QuitMsg); !ok {
				t.Fatal("explicit exit did not quit")
			}
			// Esc instead returns to the app without retrying the failed write.
			m = key(m, "esc")
			if m.quitProblem || m.quitting {
				t.Fatal("could not cancel failed shutdown")
			}
			if !timeout {
				_, retry := m.Update(tickMsg(m.now))
				if retry == nil || m.snoozeWrites.failed == 0 || m.snoozeWrites.saved != 0 {
					t.Fatal("failed revision was not retained")
				}
			}
		})
	}
}

func TestNoticeDemoPersistenceIsolation(t *testing.T) {
	m := noticeModel(t)
	writes := 0
	m = m.WithNoticePersistence(func(map[string]farm.Review) error { writes++; return nil }, nil)
	m = persistenceKey(m, "a")
	if writes != 0 || len(m.Farm.Reviews()) == 0 {
		t.Fatal("demo wrote reviews or lost session decisions")
	}
}

func TestCanceledProgramRetainsUnconfirmedPersistence(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		close(entered)
		<-release
		return nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Close the event source as well so the test's outstanding wait command exits.
	events := make(chan monitor.Event)
	m.Events = events
	defer close(events)
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	type outcome struct {
		model tea.Model
		err   error
	}
	done := make(chan outcome, 1)
	go func() { final, err := program.Run(); done <- outcome{final, err} }()
	program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not start persistence command")
	}
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, tea.ErrProgramKilled) || !errors.Is(result.err, context.Canceled) {
			t.Fatalf("unexpected cancellation: %v", result.err)
		}
		if !result.model.(Model).HasUnsavedPersistence() {
			t.Fatal("interrupted save was reported confirmed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation waited for uncancelable filesystem operation")
	}
}

func TestFailedCompletionKeepsNewestRevisionPendingUntilSuccess(t *testing.T) {
	m := noticeModel(t)
	m.Demo = false
	attempts := 0
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		attempts++
		if attempts == 1 {
			return errors.New("disk full")
		}
		return nil
	}, nil)
	item, _ := m.selectedNotice()
	m.Farm.Acknowledge(item.ActiveKey, true)
	next, first := m.Update(nil)
	m = next.(Model)
	m.Farm.Dismiss(item.ActiveKey)
	next, _ = m.Update(nil)
	m = next.(Model)
	failed := first()
	next, latest := m.Update(failed)
	m = next.(Model)
	if m.reviewWrites.failed == 0 || !m.reviewWrites.inFlight || !m.HasUnsavedPersistence() || m.noticeStorageError == "" {
		t.Fatal("failed revision or newest pending state lost")
	}
	// A duplicate/delayed old completion cannot finish the newer write.
	next, duplicate := m.Update(failed)
	m = next.(Model)
	if duplicate != nil || !m.reviewWrites.inFlight {
		t.Fatal("stale completion changed newest write")
	}
	m = completePersistence(m, latest)
	if attempts != 2 || m.HasUnsavedPersistence() || m.reviewWrites.failed != 0 || m.noticeStorageError != "" {
		t.Fatal("newest successful revision did not clear failure")
	}
}

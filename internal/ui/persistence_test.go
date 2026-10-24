package ui

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-labs-tui/internal/farm"
	"github.com/cliffom/bambu-labs-tui/internal/monitor"
	"github.com/cliffom/bambu-labs-tui/internal/noticestore"
)

func TestNoticeDecisionsSavedBeforeQuitAndRestored(t *testing.T) {
	store := noticestore.Store{Path: filepath.Join(t.TempDir(), "notices.json")}
	m := noticeModel(t)
	m.Demo = false
	m = m.WithNoticePersistence(store.Save, nil)
	item, _ := m.selectedNotice()
	m = key(m, "enter")
	m = key(m, "a") // Inspector and list actions use the same persistence path.
	saved, err := store.Load()
	if err != nil || !saved[item.ActiveKey].Acknowledged {
		t.Fatalf("acknowledgement was not saved immediately: %v, %v", saved, err)
	}
	m = key(m, "a")
	saved, err = store.Load()
	if err != nil || len(saved) != 0 {
		t.Fatal("undo acknowledgement was not saved")
	}
	m = key(m, "c")
	saved, err = store.Load()
	if err != nil || !saved[item.ActiveKey].Dismissed {
		t.Fatal("dismissal was not saved immediately")
	}
	state := farm.New(10)
	state.RestoreReviews(saved)
	next := New(state, make(chan monitor.Event), make(chan struct{}, 1), false).WithNoticePersistence(store.Save, nil)
	for _, event := range []monitor.Event{
		{Kind: "connection", Connected: true},
		{Kind: "devices", Devices: []farm.Device{{ID: "H", Name: "H2D", Online: true}}},
		{Kind: "report", ID: "H", Payload: []byte(`{"print":{"mc_percent":75}}`)},
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
	updated, _ := next.Update(monitor.Event{Kind: "report", ID: "H", Payload: payload, At: time.Now()})
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
	m = key(m, "enter")
	m = key(m, "a")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("write failure not visible in inspector")
	}
	updated, _ := m.Update(tickMsg(time.Now().Add(time.Minute)))
	m = updated.(Model)
	m = key(m, "esc")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Could not save notices") {
		t.Fatal("failure vanished or triggered continuous writes")
	}
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		writes++
		return nil
	}, errors.New("corrupt saved data"))
	m = key(m, "a")
	if writes != 1 || !strings.Contains(ansi.Strip(m.View()), "Saved notices unavailable") {
		t.Fatal("failed load overwrote saved data or warning was hidden")
	}
}

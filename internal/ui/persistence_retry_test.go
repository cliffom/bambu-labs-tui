package ui

import (
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/muesli/termenv"
)

func failedSaveModel(t *testing.T) (Model, *bool) {
	t.Helper()
	m := noticeModel(t)
	m.Demo = false
	fail := true
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil).
		WithSnoozePersistence(nil, func(map[string]time.Time) error {
			if fail {
				return errors.New("full")
			}
			return nil
		}, nil)
	item, _ := m.selectedNotice()
	m.Acknowledge(item.ActiveKey, true)
	m.SetSnooze("H", time.Hour, m.now)
	r, z := m.Persistence()
	m.Complete(r())
	m.Complete(z())
	return m, &fail
}

func TestStatusRetryLayoutsAndRecovery(t *testing.T) {
	original := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(original) })
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		for _, size := range [][2]int{{60, 18}, {120, 32}} {
			m, fail := failedSaveModel(t)
			m = resizeModel(m, size[0], size[1])
			m = key(m, "X") // Suspend a real destructive confirmation.
			parent := hiddenState(m)
			m = key(m, "ctrl+e")
			inspect := func(state string) {
				t.Helper()
				view := m.View()
				lines := strings.Split(view, "\n")
				if len(lines) != size[1] {
					t.Fatal("height overflow", state)
				}
				for _, line := range lines {
					if ansi.StringWidth(line) != size[0] {
						t.Fatal("row width", state)
					}
				}
				if !strings.Contains(lastRow(m), "Esc back") {
					t.Fatal("missing return control")
				}
				if profile == termenv.Ascii && strings.Contains(view, "\x1b") {
					t.Fatal("ANSI in no-color view")
				}
				t.Logf("profile=%v size=%dx%d %s:\n%s", profile, size[0], size[1], state, ansi.Strip(view))
			}
			if !strings.Contains(lastRow(m), "r retry saves") || !strings.Contains(m.View(), "Notice saves: failed") || !strings.Contains(m.View(), "Snooze saves: failed") {
				t.Fatal("failed saves lack retry/status")
			}
			inspect("failed")
			m = key(m, "end")
			offset := m.status.YOffset
			m = resizeModel(m, 59, 17)
			t.Logf("below minimum:\n%s", ansi.Strip(m.View()))
			m = key(m, "r")
			if m.PersistencePending() || m.status.YOffset != offset || m.dialog.active != dialogStatus {
				t.Fatal("hidden retry allowed")
			}
			m = resizeModel(m, size[0], size[1])
			if m.status.YOffset != offset {
				t.Fatal("resize lost status scroll")
			}
			// Retry while scrolled: owning confirmation and scroll must survive.
			next, retry := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
			m = next.(Model)
			if !m.ReviewWrites().InFlight || !m.SnoozeWrites().InFlight || strings.Contains(lastRow(m), "r retry saves") || m.status.YOffset != offset {
				t.Fatal("retry did not start exclusively or lost scroll")
			}
			m = key(m, "r")
			m = key(m, "home")
			inspect("saving")
			if !strings.Contains(m.View(), "Notice saves: saving") || !strings.Contains(m.View(), "Snooze saves: saving") {
				t.Fatal("missing pending feedback")
			}
			// Telemetry still flows while retry commands have not completed.
			next, _ = m.Update(monitor.Event{Kind: monitor.EventProblem, Problem: "Synthetic monitoring update", At: m.now})
			m = next.(Model)
			if m.toast != "Synthetic monitoring update" || !m.persistencePending() {
				t.Fatal("retry blocked telemetry")
			}
			// First explicit retry fails; it must offer another action, never loop.
			m = completePersistence(m, retry)
			if !strings.Contains(lastRow(m), "r retry saves") {
				t.Fatal("failed retry unavailable")
			}
			inspect("retry failed")
			*fail = false
			next, retry = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
			m = completePersistence(next.(Model), retry)
			if m.HasUnsavedPersistence() || m.NoticeStorageError != "" || m.AlertStorageError != "" || strings.Contains(lastRow(m), "r retry saves") {
				t.Fatal("retry not saved")
			}
			if !strings.Contains(m.View(), "Notice saves: saved") || !strings.Contains(m.View(), "Snooze saves: saved") {
				t.Fatal("missing saved feedback")
			}
			inspect("saved")
			m = key(m, "esc")
			if !reflect.DeepEqual(hiddenState(m), parent) {
				t.Fatal("retry changed decisions or suspended confirmation")
			}
		}
	}
}

func TestStatusRetrySessionOnlyAndProtectedLayouts(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		for _, protected := range []bool{false, true} {
			m := resizeModel(noticeModel(t), size[0], size[1])
			var loadErr error
			if protected {
				m.Demo = false
				loadErr = errors.New("unsupported")
			}
			m = m.WithNoticePersistence(func(map[string]farm.Review) error { t.Fatal("protected notices written"); return nil }, loadErr).
				WithSnoozePersistence(nil, func(map[string]time.Time) error { t.Fatal("protected snoozes written"); return nil }, loadErr)
			m = key(key(m, "ctrl+e"), "r")
			if strings.Contains(lastRow(m), "r retry saves") || m.PersistencePending() {
				t.Fatal("session-only retry offered")
			}
			if !strings.Contains(m.View(), "Notice saves: session-only") || !strings.Contains(m.View(), "Snooze saves: session-only") {
				t.Fatal("session-only status hidden")
			}
			t.Logf("size=%dx%d protected=%v:\n%s", size[0], size[1], protected, ansi.Strip(m.View()))
		}
	}
}

func TestRetryParticipatesInShutdownAndCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		m, fail := failedSaveModel(t)
		m = key(m, "ctrl+e")
		*fail = false
		next, retry := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		m = next.(Model)
		m = key(m, "esc")
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		m = next.(Model)
		if !m.quitting || !m.persistencePending() {
			t.Fatal("quit did not wait for retry")
		}
		if cancel {
			m = key(m, "esc")
		}
		m = completePersistence(m, retry)
		if m.HasUnsavedPersistence() || m.quitProblem || (cancel && m.quitting) {
			t.Fatal("retry shutdown/cancel lost save result")
		}
	}
}

func TestStatusRetryDoesNotRearmAttentionOrDesktopAttempts(t *testing.T) {
	m, client, condition := notificationModel(t)
	m = resizeModel(m, 60, 18)
	fail := true
	m = m.WithNoticePersistence(func(map[string]farm.Review) error {
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	m.Acknowledge(condition, true)
	m = persistenceUpdate(m, nil)
	m = key(m, "ctrl+e")
	before := m.Farm.NotificationSnapshot(m.now)
	m.attentionSeen = map[attentionOccurrence]bool{{condition, m.Farm.Active[condition].Since}: true}
	seen := maps.Clone(m.attentionSeen)
	fail = false
	m = persistenceKey(m, "r")
	if client.retries != 0 || !reflect.DeepEqual(m.Farm.NotificationSnapshot(m.now), before) || !reflect.DeepEqual(m.attentionSeen, seen) {
		t.Fatal("save retry changed attention/delivery identity")
	}
}

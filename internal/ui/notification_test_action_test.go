package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/monitor"
)

func TestConfigNotificationProbeUsesSoundSnapshotWithoutSaving(t *testing.T) {
	for _, sound := range []bool{false, true} {
		calls := 0
		m := demoModel().WithNotificationTest(context.Background(), func(_ context.Context, requested bool) error {
			calls++
			if requested != sound {
				t.Fatal("probe missed captured sound setting")
			}
			return nil
		})
		m.alertPrefs.NotificationSound = sound
		m = key(m, ",")
		m = key(m, "up") // Wrap from the first setting to Test Notification.
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
		if calls != 0 || !m.notificationTestPending || m.configRevision != 0 {
			t.Fatal("probe ran in reducer or changed config")
		}
		if !strings.Contains(ansi.Strip(m.View()), "Sending") {
			t.Fatal("pending status missing")
		}
		next, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
		if duplicate != nil {
			t.Fatal("duplicate probe scheduled while pending")
		}
		m.alertPrefs.NotificationSound = !sound
		msg := cmd()
		next, _ = m.Update(msg)
		m = next.(Model)
		if calls != 1 || m.notificationTestPending || !strings.Contains(ansi.Strip(m.View()), "accepted by macOS") || m.alertPrefs.DesktopNotifications {
			t.Fatal("probe outcome or opt-out independence incorrect")
		}
	}
}

func TestConfigNotificationProbeCancelsOnCloseAndIgnoresStaleResult(t *testing.T) {
	started := make(chan struct{})
	m := demoModel().WithNotificationTest(context.Background(), func(ctx context.Context, _ bool) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	m = key(key(m, ","), "up")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	// Live telemetry still reaches the reducer while the helper waits.
	next, _ = m.Update(monitor.Event{Kind: monitor.EventConnection, Connected: true, At: time.Now()})
	m = next.(Model)
	if !m.notificationTestPending || m.dialog.active != dialogConfig {
		t.Fatal("probe lost focus/state during telemetry")
	}
	m = key(m, "esc")
	if m.notificationTestPending || m.notificationTestCancel != nil || m.hasOpenDialog() {
		t.Fatal("closing Config did not cancel probe")
	}
	var stale tea.Msg
	select {
	case stale = <-result:
	case <-time.After(time.Second):
		t.Fatal("close did not release helper")
	}
	m = key(m, ",")
	m.notificationTestPending = true
	m.notificationTestStatus = "New test pending"
	next, _ = m.Update(stale)
	m = next.(Model)
	if !m.notificationTestPending || m.notificationTestStatus != "New test pending" {
		t.Fatal("stale result replaced newer test state")
	}
}

func TestConfigNotificationProbeFailureAndHiddenAction(t *testing.T) {
	m := demoModel().WithNotificationTest(context.Background(), func(context.Context, bool) error { return errors.New("permission denied") })
	m = key(key(m, ","), "up")
	m = resizeModel(m, 59, 17)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil || m.notificationTestPending {
		t.Fatal("hidden probe started below minimum")
	}
	m = resizeModel(m, 60, 18)
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.notificationTestPending || !strings.Contains(ansi.Strip(m.View()), "Test failed") {
		t.Fatal("failure not visible")
	}
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m = resizeModel(m, size[0], size[1])
		rows := strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.Contains(rows[len(rows)-1], "Esc / , close") {
			t.Fatal("probe hid close keys")
		}
		for _, line := range rows {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("probe status exceeded layout")
			}
		}
	}
}

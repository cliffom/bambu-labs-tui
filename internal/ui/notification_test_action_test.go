package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/muesli/termenv"
)

func TestConfigNotificationProbeUsesSoundSnapshotWithoutSaving(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
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
		m = openAlertConfig(m)
		m = key(m, "up") // Wrap from the first setting to Test Notification.
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
		if calls != 0 || !m.notificationTestPending || m.ConfigRevision() != 0 {
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
		for _, width := range []int{60, 120} {
			m = resizeModel(m, width, 18)
			rows := strings.Split(ansi.Strip(m.View()), "\n")
			if strings.Contains(strings.Join(rows[:len(rows)-1], "\n"), "Test accepted") || !strings.Contains(rows[len(rows)-1], " · "+m.notificationTestStatus) {
				t.Fatal("test result belongs after helper text in the status bar")
			}
			if !strings.Contains(m.configFooter(), m.styles.footerSuccess.Render(m.notificationTestStatus)) {
				t.Fatal("fresh test result should be prominent green")
			}
		}
		m.now = m.notificationTestStatusAt.Add(notificationTestDimAfter)
		if !strings.Contains(m.configFooter(), m.styles.footerMuted.Render(m.notificationTestStatus)) {
			t.Fatal("accepted test result should dim before expiry")
		}
		m.now = m.notificationTestStatusAt.Add(notificationTestFeedbackLifetime)
		if m.configFooter() != m.configHelp() {
			t.Fatal("expired test result should leave only helper text")
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
	m = key(openAlertConfig(m), "up")
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
	m = openAlertConfig(m)
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
	m = key(openAlertConfig(m), "up")
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
	m.now = m.now.Add(notificationTestFeedbackLifetime)
	if !strings.Contains(ansi.Strip(m.configFooter()), "Test failed") {
		t.Fatal("actionable test failures should remain visible")
	}
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m = resizeModel(m, size[0], size[1])
		rows := strings.Split(ansi.Strip(m.View()), "\n")
		if !strings.Contains(rows[1], "Esc / ,") || strings.Contains(rows[len(rows)-1], "Esc / ,") {
			t.Fatal("close keys belong in the header only")
		}
		for _, line := range rows {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("probe status exceeded layout")
			}
		}
	}
}

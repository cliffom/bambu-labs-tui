package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/notification_state"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

type notificationStub struct {
	inputs     []notifications.Input
	status     notifications.Status
	stopped    bool
	retries    int
	retryInput notifications.Input
}

func (n *notificationStub) Submit(input notifications.Input) { n.inputs = append(n.inputs, input) }
func (n *notificationStub) Status() notifications.Status     { return n.status }
func (n *notificationStub) Stop()                            { n.stopped = true }

func (n *notificationStub) Retry() bool {
	n.retries++
	if len(n.inputs) > 0 {
		n.retryInput = n.inputs[len(n.inputs)-1]
	}
	return true
}

func notificationModel(t *testing.T) (Model, *notificationStub, string) {
	t.Helper()
	state := farm.New(10)
	now := time.Now()
	state.Cloud(true, "", now)
	state.Discover([]farm.Device{{ID: "TEST", Name: "Synthetic", Online: true}}, now)
	if err := state.Report("TEST", []byte(`{"print":{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}}`), now); err != nil {
		t.Fatal(err)
	}
	prefs := config.DefaultAlerts()
	prefs.DesktopNotifications = true
	prefs.NotificationSound = true
	stub := &notificationStub{}
	m := New(state, make(chan monitor.Event), nil, false).WithAlertPreferences(prefs).WithNotifications(stub, nil)
	m.now = now
	key := state.Attention()[0].Key
	return m, stub, key
}

func TestNotificationSubmissionHonorsReviewsCategoriesAndSnooze(t *testing.T) {
	m, client, key := notificationModel(t)
	update := func() { next, _ := m.Update(tickMsg(m.now)); m = next.(Model) }
	update()
	input := client.inputs[len(client.inputs)-1]
	if !input.Active || !input.Policies["TEST"].Desktop || !input.Policies["TEST"].Sound || !input.Policies["TEST"].Allowed[key] {
		t.Fatal("notification preference snapshot missing")
	}
	m.Farm.Acknowledge(key, true)
	update()
	if client.inputs[len(client.inputs)-1].Snapshot["TEST"].Conditions[key].Eligible {
		t.Fatal("reviewed occurrence submitted as eligible")
	}
	m.Farm.Acknowledge(key, false)
	m.alertPrefs.Filament = false
	update()
	if client.inputs[len(client.inputs)-1].Policies["TEST"].Allowed[key] {
		t.Fatal("disabled category allowed")
	}
	m.alertPrefs.Filament = true
	m.snoozes = map[string]time.Time{"TEST": m.now.Add(time.Hour)}
	update()
	if !client.inputs[len(client.inputs)-1].Policies["TEST"].Snoozed {
		t.Fatal("snooze missing")
	}
	m.now = m.now.Add(time.Hour + time.Second)
	// A new report is required; elapsed time alone cannot make stale data current.
	if err := m.Farm.Report("TEST", []byte(`{"print":{"print_error":117473297}}`), m.now); err != nil {
		t.Fatal(err)
	}
	update()
	if client.inputs[len(client.inputs)-1].Policies["TEST"].Snoozed {
		t.Fatal("expired snooze still suppresses")
	}
}

func TestNotificationShutdownPausesAndCanBeCanceled(t *testing.T) {
	m, client, _ := notificationModel(t)
	client.status.Pending = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = next.(Model)
	if !m.quitting || client.inputs[len(client.inputs)-1].Active || client.stopped {
		t.Fatal("shutdown did not pause and wait")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.quitting || !client.inputs[len(client.inputs)-1].Active {
		t.Fatal("canceling shutdown did not resume delivery")
	}
	client.status.Pending = false
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !client.stopped {
		t.Fatal("completed shutdown did not stop notifications")
	}
	_ = next
}

func TestNotificationStatusFitsSupportedLayoutsAndDialogs(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}, {59, 17}} {
		m, client, _ := notificationModel(t)
		m.width, m.height = size[0], size[1]
		client.status = notifications.Status{Revision: 1, Accepted: 2, Suppressed: 1, Last: "Desktop notification suppressed: snoozed", Error: "Desktop notification failed; run doctor --notify"}
		next, _ := m.Update(tickMsg(m.now))
		m = next.(Model)
		for _, dialog := range []bool{false, true} {
			if dialog && m.terminalFits() {
				m.openAlerts("TEST")
				m.syncDialogs()
			}
			view := ansi.Strip(m.View())
			lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
			if len(lines) > size[1] {
				t.Fatalf("height overflow %v: %d", size, len(lines))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatalf("width overflow %v: %q", size, line)
				}
			}
			if m.terminalFits() && !strings.Contains(view, "Desktop notification failed") {
				t.Fatalf("notification error hidden at %v", size)
			}
			if dialog && m.terminalFits() && !strings.Contains(view, "Desktop") {
				t.Fatal("desktop preference not visible")
			}
		}
	}
}

func TestTelemetryAndSnoozeRemainResponsiveDuringNativeDelivery(t *testing.T) {
	m, _, _ := notificationModel(t)
	started, canceled := make(chan struct{}), make(chan struct{})
	worker := notifications.NewWorker(context.Background(), notification_state.Store{Path: filepath.Join(t.TempDir(), "ledger.json")}, func(ctx context.Context, _ notifications.Message) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	})
	defer func() { worker.Stop(); <-worker.Done() }()
	m = m.WithNotifications(worker, nil)
	next, _ := m.Update(tickMsg(m.now))
	m = next.(Model)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("delivery did not start")
	}
	updated := make(chan Model, 1)
	go func() {
		next, _ := m.Update(monitor.Event{Kind: monitor.EventReport, ID: "TEST", At: m.now, Payload: []byte(`{"print":{"mc_percent":73}}`)})
		updated <- next.(Model)
	}()
	select {
	case m = <-updated:
	case <-time.After(3 * time.Second):
		t.Fatal("native delivery blocked telemetry reducer")
	}
	if m.Farm.Printers["TEST"].Progress == nil || *m.Farm.Printers["TEST"].Progress != 73 {
		t.Fatal("telemetry was not applied")
	}
	m.openAlerts("TEST")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = next.(Model)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("snooze did not cancel in-flight delivery")
	}
}

func TestNotificationScheduleIndependentOfModalsAndNotices(t *testing.T) {
	m, client, key := notificationModel(t)
	m.alertPrefs.QuietHours = config.QuietHours{Enabled: true, Timezone: "UTC", Start: "00:00", End: "23:59", Days: "sun,mon,tue,wed,thu,fri,sat"}
	m.alertPrefs.BurstWindowSeconds = 3
	m.now = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	m.syncNotifications()
	input := client.inputs[len(client.inputs)-1]
	if !input.Quiet || input.BurstWindow != 3*time.Second {
		t.Fatal(input)
	}
	if !m.alertPrefs.RulesFor("TEST").AutomaticModals || m.Farm.Printers["TEST"] == nil {
		t.Fatal("quiet hours changed monitoring/modal policy")
	}
	for _, item := range m.Farm.Attention() {
		if item.Key == key {
			return
		}
	}
	t.Fatal("quiet hours removed visible notice")
}

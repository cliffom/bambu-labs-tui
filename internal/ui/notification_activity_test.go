package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

func activityModel(count int) (Model, *notificationStub) {
	m := demoModel()
	client := &notificationStub{}
	for i := range count {
		at := time.Date(2026, 10, 26, 12, 0, i, 0, time.UTC)
		client.status.History = append(client.status.History, notifications.Activity{ID: uint64(i + 1), At: at, FirstAt: at, Printer: fmt.Sprintf("Synthetic printer %d", i+1), Subject: "Fault 0700_8011", Channel: "macOS desktop", Outcome: notifications.OutcomeSuppressed, Reason: notifications.ReasonSnoozed, Repeats: 3})
	}
	return m.WithNotifications(client, nil), client
}

func TestActivityExplainsSelectedDecisionAndKeepsSelectionThroughUpdates(t *testing.T) {
	m, client := activityModel(5)
	m = key(m, "N")
	if m.dialog.active != dialogNotificationActivity || m.notificationActivity.selected != 5 {
		t.Fatal("activity did not open at newest")
	}
	view := ansi.Strip(m.View())
	for _, text := range []string{"NOTIFICATION ACTIVITY", "macOS desktop", "Fault 0700_8011", "Suppressed", "This printer is snoozed.", "Seen 3"} {
		if !strings.Contains(view, text) {
			t.Fatal("missing activity detail", text)
		}
	}
	if client.status.History[0].ID != 1 {
		t.Fatal("rendering mutated provider history")
	}
	m = key(m, "down")
	selected := m.notificationActivity.selected
	entry := client.status.History[4]
	entry.ID = 6
	entry.Printer = "Newer printer"
	client.status.History = append(client.status.History, entry)
	next, _ := m.Update(tickMsg(m.now))
	m = next.(Model)
	if m.notificationActivity.selected != selected {
		t.Fatal("new entry moved the operator's selection")
	}
	m = key(m, "home")
	if m.notificationActivity.selected != 6 || !m.notificationActivity.follow {
		t.Fatal("Home failed to resume following newest")
	}
	m = key(m, "end")
	if m.notificationActivity.selected != 1 {
		t.Fatal("End failed to select oldest")
	}
	client.status.History = client.status.History[1:]
	next, _ = m.Update(tickMsg(m.now))
	m = next.(Model)
	if m.notificationActivity.selected != 2 {
		t.Fatal("eviction should fall back to oldest retained")
	}
	m = key(m, "N")
	if m.hasOpenDialog() || m.notificationActivity != nil {
		t.Fatal("activity close did not release payload")
	}
}

func TestActivityOutcomesGroupingBoundsAndSanitization(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m, client := activityModel(25)
		m = resizeModel(m, size[0], size[1])
		entry := &client.status.History[24]
		entry.Printer = strings.Repeat("Very long synthetic name ", 12) + "\x1b[2J\nunsafe"
		entry.Outcome, entry.Reason = notifications.OutcomeAccepted, notifications.ReasonAccepted
		entry.GroupSize, entry.Request = 3, 9
		m = key(m, "N")
		view := ansi.Strip(m.View())
		for _, text := range []string{"Accepted", "banner and sound are unconfirmed", "Request #9", "3 notices grouped"} {
			if !strings.Contains(strings.Join(strings.Fields(view), " "), text) {
				t.Fatal(size, "missing", text, "\n"+view)
			}
		}
		for range 25 {
			view = m.View()
			if strings.Contains(view, "\x1b[2J") {
				t.Fatal("terminal control escaped sanitization")
			}
			lines := strings.Split(ansi.Strip(view), "\n")
			if len(lines) != size[1] {
				t.Fatal("activity exceeds height", size, len(lines))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("activity exceeds width", size)
				}
			}
			if !strings.Contains(ansi.Strip(view), "> "+m.notificationActivity.entries[m.notificationActivity.index].At.Local().Format("15:04:05")) {
				t.Fatal("selection hidden outside viewport")
			}
			m = key(m, "down")
		}
		m = key(m, "pgup")
		if m.notificationActivity.index >= 24 {
			t.Fatal("paging did not move selection")
		}
		for _, outcome := range []string{notifications.OutcomeFailed, notifications.OutcomeUnconfirmed, notifications.OutcomePaused} {
			client.status.History[24].Outcome = outcome
			m = key(m, "home")
			if !strings.Contains(ansi.Strip(m.View()), outcome) {
				t.Fatal("outcome hidden", outcome)
			}
		}
	}
}

func TestEmptyActivityAvailabilityAndHiddenInput(t *testing.T) {
	m := key(resizeModel(demoModel(), 60, 18), "N")
	if !strings.Contains(ansi.Strip(m.View()), "No live notification decisions yet.") || !strings.Contains(ansi.Strip(m.View()), "clears on restart") {
		t.Fatal("empty activity guidance missing")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() {
		t.Fatal("empty activity did not close")
	}
	m, client := activityModel(3)
	m = key(m, "N")
	m = key(m, "down")
	selected := m.notificationActivity.selected
	m = resizeModel(m, 59, 17)
	for _, input := range []string{"home", "end", "down", "pgdown", "N", "1", ",", "enter", "q"} {
		m = key(m, input)
	}
	if m.notificationActivity.selected != selected || m.dialog.active != dialogNotificationActivity {
		t.Fatal("hidden activity navigation escaped guard")
	}
	next, _ := m.Update(monitor.Event{Kind: monitor.EventConnection, Connected: true, At: m.now})
	m = next.(Model)
	if m.Farm.Connected != true || m.notificationActivity.selected != selected {
		t.Fatal("live update blocked or moved selection")
	}
	m = resizeModel(m, 60, 18)
	if m.notificationActivity.selected != selected {
		t.Fatal("restore changed selection")
	}
	m = resizeModel(m, 59, 17)
	m = key(m, "esc")
	if m.hasOpenDialog() {
		t.Fatal("undersized close failed")
	}
	m = resizeModel(m, 60, 18)
	m = key(m, "N")
	client.status.History = nil
	m.syncNotificationActivity()
	if !strings.Contains(ansi.Strip(m.View()), "Desktop delivery is off") {
		t.Fatal("opt-out empty state missing")
	}
}

func TestActivityOwnershipAndRecoveryAreVisibleAndUseFreshPolicy(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m, client, keyID := notificationModel(t)
		m = resizeModel(m, size[0], size[1])
		m = key(m, "N")
		cases := []struct {
			status notifications.Status
			label  string
		}{
			{notifications.Status{Ownership: notifications.OwnershipOwner}, "This TUI owns desktop delivery"},
			{notifications.Status{Ownership: notifications.OwnershipStandby}, "Another TUI owns desktop delivery"},
			{notifications.Status{Ownership: notifications.OwnershipObserver}, "Desktop off; no delivery ownership"},
			{notifications.Status{Disabled: true}, "Paused; r retries after fixing the cause"},
			{notifications.Status{Recovering: true}, "Checking existing identity state"},
		}
		for _, tc := range cases {
			client.status = tc.status
			m.syncNotificationActivity()
			view := ansi.Strip(m.View())
			if !strings.Contains(view, tc.label) || !strings.Contains(view, "r retry") {
				t.Fatal("ownership/recovery hidden", size, view)
			}
			lines := strings.Split(view, "\n")
			if len(lines) != size[1] {
				t.Fatal("ownership state height overflow")
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("ownership state width overflow")
				}
			}
		}
		m.snoozes = map[string]time.Time{"TEST": m.now.Add(time.Hour)}
		m.Farm.Acknowledge(keyID, true)
		before := m.configRevision
		m = key(m, "r")
		input := client.retryInput
		if client.retries != 1 || !input.Policies["TEST"].Snoozed || !input.Snapshot["TEST"].Conditions[keyID].Reviewed {
			t.Fatal("recovery used an old preference/review snapshot")
		}
		if m.configRevision != before || !m.Farm.Active[keyID].Acknowledged {
			t.Fatal("recovery edited config or reviews")
		}
		m = resizeModel(m, 59, 17)
		m = key(m, "r")
		if client.retries != 1 {
			t.Fatal("hidden recovery ran below minimum size")
		}
		m = key(m, "esc")
		if m.hasOpenDialog() {
			t.Fatal("undersized recovery screen did not close")
		}
	}
}

package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/notifications"
	"github.com/muesli/termenv"
)

func lastRow(m Model) string {
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	return rows[len(rows)-1]
}

func TestHelpScrollsAndAdvertisesOnlyItsControls(t *testing.T) {
	m := resizeModel(key(demoModel(), "?"), 60, 18)
	if row := lastRow(m); !strings.Contains(row, "?/Esc close") || strings.Contains(row, "q quit") || strings.Contains(row, "Tab panel") {
		t.Fatal("help advertises dashboard controls", row)
	}
	for _, input := range []string{"q", "ctrl+c", "tab", "m"} {
		m = key(m, input)
		if m.dialog.active != dialogHelp || m.help.YOffset != 0 {
			t.Fatal("dashboard input escaped help")
		}
	}
	m = key(m, "pgdown")
	if m.help.YOffset == 0 {
		t.Fatal("help cannot scroll at minimum size")
	}
	offset := m.help.YOffset
	m = resizeModel(resizeModel(m, 59, 17), 60, 18)
	if m.help.YOffset != offset {
		t.Fatal("resize lost help position")
	}
	m = key(m, "end")
	if !strings.Contains(ansi.Strip(m.View()), "Quit from dashboard") {
		t.Fatal("lower help content is inaccessible")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() || m.focus != panelPrinters {
		t.Fatal("help did not restore dashboard")
	}
}

func TestCompactFooterTracksFocusAndNoticeKind(t *testing.T) {
	m := resizeModel(noticeModel(t), 60, 18)
	for _, tc := range []struct {
		focus dashboardPanel
		want  string
	}{
		{panelPrinters, "↑/↓ select"}, {panelDetails, "↑/↓ scroll"}, {panelNotices, "a ack"},
	} {
		m.focus = tc.focus
		m.syncPanels()
		row := lastRow(m)
		if !strings.Contains(row, tc.want) || !strings.Contains(row, "q quit") || !strings.Contains(row, "^E status") {
			t.Fatal("compact controls hidden", tc.focus, row)
		}
	}
	m = key(m, "G")
	if row := lastRow(m); strings.Contains(row, "a ack") || !strings.Contains(row, "c delete event") {
		t.Fatal("history has active-notice hints", row)
	}
	m = key(m, "enter")
	if row := lastRow(m); strings.Contains(row, "a ack") || !strings.Contains(row, "c delete event") {
		t.Fatal("historical inspector has active-notice hints", row)
	}
}

func TestOverlappingErrorsKeepControlsAndCompleteDetails(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m := resizeModel(noticeModel(t), size[0], size[1])
		m = key(m, "X")
		m.configStorageError = "Config save failed; open , and r to retry"
		m.NoticeStorageError = "Could not save notices; decisions may be lost on restart"
		m.AlertStorageError = "Could not save snoozes; changes may be lost on restart"
		m.exportError = "Diagnostic export failed; check destination permissions"
		m.browserError = "Could not open browser; use the troubleshooting URL shown"
		if row := lastRow(m); !strings.Contains(row, "5 errors") || !strings.Contains(row, "y clear") || !strings.Contains(row, "n/Esc cancel") || !strings.Contains(row, "^E status") {
			t.Fatal("overlapping errors hid confirmation", row)
		}
		before := hiddenState(m)
		m = key(m, "ctrl+e")
		if m.dialog.active != dialogStatus || m.dialog.returnTo != dialogClearNotices {
			t.Fatal("status did not suspend confirmation")
		}
		view := m.status
		view.Height = 100
		text := ansi.Strip(view.View())
		for _, want := range []string{m.configStorageError, m.NoticeStorageError, m.AlertStorageError, m.exportError, m.browserError, "retry", "preserved"} {
			if !strings.Contains(text, want) {
				t.Fatal("full error/recovery unavailable", want)
			}
		}
		m = key(m, "pgdown")
		if size[0] == 60 && m.status.YOffset == 0 {
			t.Fatal("status details do not scroll")
		}
		m = resizeModel(m, 59, 17)
		for _, input := range []string{"y", "ctrl+e", "q", "ctrl+c"} {
			m = key(m, input)
			if m.dialog.active != dialogStatus || !reflect.DeepEqual(m.Farm.Reviews(), before.reviews) {
				t.Fatal("hidden status allowed an action")
			}
		}
		m = resizeModel(m, size[0], size[1])
		m = key(m, "esc")
		if !reflect.DeepEqual(hiddenState(m), before) {
			t.Fatal("status lost parent confirmation/focus")
		}
	}
}

func TestStatusSuspendsNestedAlertsAndContinuesTelemetry(t *testing.T) {
	m := attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
	m = key(key(m, "s"), "ctrl+e")
	if m.dialog != (dialogState{active: dialogStatus, returnTo: dialogAlerts, resume: dialogAttention}) {
		t.Fatal("status lost nested owners")
	}
	m = attentionReport(t, m, "H", `{"print":{"gcode_state":"RUNNING","hms":[],"print_error":0}}`)
	m = resizeModel(m, 60, 18)
	if m.attention.view.Width != 60 || m.alertDialog.view.Width != 58 {
		t.Fatal("suspended owners were not resized")
	}
	for _, input := range []string{"q", "ctrl+c", "1", "y"} {
		m = key(m, input)
		if m.dialog.active != dialogStatus || len(m.Snoozes) != 0 {
			t.Fatal("status input leaked to parent")
		}
	}
	m = key(m, "ctrl+e")
	if m.dialog != (dialogState{active: dialogAlerts, returnTo: dialogAttention}) {
		t.Fatal("status did not restore nested alerts")
	}
	m = key(m, "esc")
	if m.dialog.active != dialogAttention || !strings.Contains(attentionText(m), "Resolved / no longer active") {
		t.Fatal("suspended telemetry or return path lost")
	}
}

func TestConfigControlsSurviveRetainedTestFailureAndStatusInspection(t *testing.T) {
	for _, reset := range []bool{false, true} {
		m := resizeModel(key(demoModel(), ","), 60, 18)
		m.notificationTestFailed = true
		m.notificationTestStatus = "Test failed; run bambustand doctor --notify."
		m.configStorageError = "Config save failed"
		if reset {
			m.configSection = configPrinters
			m.configSelection = len(configOptions) + 1
			m = key(m, "enter")
		} else {
			m.configSelection = 1
			m = key(m, "enter")
		}
		row := lastRow(m)
		if !strings.Contains(row, "Enter") || !strings.Contains(row, "Esc cancel") && !strings.Contains(row, "Esc cancels") || strings.Contains(row, "Test failed") {
			t.Fatal("test result hid active controls", row)
		}
		if strings.Contains(ansi.Strip(m.View()), "r retry") {
			t.Fatal("Config advertises retry while input/confirmation owns r")
		}
		m = key(m, "ctrl+e")
		view := m.status
		view.Height = 100
		if !strings.Contains(ansi.Strip(view.View()), "Test failed") {
			t.Fatal("retained failure unavailable during edit/confirmation")
		}
		m = key(m, "esc")
		if m.dialog.active != dialogConfig || m.configResetConfirm != reset || m.configEditing == reset {
			t.Fatal("status changed edit/confirmation state")
		}
		m = key(m, "esc")
		if !strings.Contains(lastRow(m), "Test failed") {
			t.Fatal("canceling edit discarded failure")
		}
	}
}

func TestLocalFeedbackVisibleAndLateFailuresPersistAcrossActivity(t *testing.T) {
	m := key(key(noticeModel(t), "a"), "n")
	m.toast = ""
	m = key(m, "u")
	if !strings.Contains(lastRow(m), "Local decision updated") {
		t.Fatal("decision success invisible in its owner")
	}
	m = key(m, "esc")
	m = key(m, "D")
	destination := m.exportDialog.destination
	m = key(key(m, "esc"), "N")
	next, _ := m.Update(exportResultMsg{destination: destination, err: errors.New("disk full")})
	m = next.(Model)
	m = persistenceUpdate(m, tickMsg(m.now.Add(toastLifetime+time.Second)))
	if m.dialog.active != dialogNotificationActivity || !strings.Contains(lastRow(m), "Diagnostic export failed") {
		t.Fatal("late failure expired or changed owner", lastRow(m))
	}
	m = key(m, "ctrl+e")
	if !strings.Contains(ansi.Strip(m.View()), "destination permissions") {
		t.Fatal("late failure details unavailable")
	}
	next, _ = m.Update(exportResultMsg{destination: destination})
	m = next.(Model)
	if m.exportError != "" {
		t.Fatal("successful retry did not clear export failure")
	}
}

func TestClearHistoryPreservesEventsArrivingDuringConfirmation(t *testing.T) {
	m := key(noticeModel(t), "C")
	original := append([]uint64(nil), m.clearHistoryEvents...)
	if len(original) == 0 {
		t.Fatal("fixture has no captured events")
	}
	m = persistenceUpdate(m, monitor.Event{Kind: monitor.EventConnection, Connected: false, Problem: "Synthetic disconnect", At: m.now.Add(time.Second)})
	latest := m.Farm.Events[len(m.Farm.Events)-1].ID
	if !reflect.DeepEqual(m.clearHistoryEvents, original) {
		t.Fatal("telemetry changed confirmation scope")
	}
	m = key(m, "ctrl+e")
	m = resizeModel(resizeModel(m, 59, 17), 60, 18)
	m = key(key(m, "esc"), "y")
	if len(m.Farm.Events) != 1 || m.Farm.Events[0].ID != latest || len(m.Farm.Active) == 0 || m.clearHistoryEvents != nil {
		t.Fatal("confirmation deleted a later event or active condition")
	}
}

func TestRecoveryHintsAndRoutingRequireIdlePausedOrStandbyWorker(t *testing.T) {
	for _, state := range []notifications.Status{
		{},
		{Ownership: notifications.OwnershipOwner},
		{Ownership: notifications.OwnershipObserver},
		{Disabled: true, Pending: true},
		{Disabled: true, Recovering: true},
		{Disabled: true},
		{Ownership: notifications.OwnershipStandby},
	} {
		m, client, _ := notificationModel(t)
		client.status = state
		m = key(m, "N")
		eligible := !state.Pending && !state.Recovering && (state.Disabled || state.Ownership == notifications.OwnershipStandby)
		if strings.Contains(ansi.Strip(m.View()), "r recheck") != eligible {
			t.Fatal("incorrect recovery hint", state)
		}
		_ = key(m, "r")
		if (client.retries != 0) != eligible {
			t.Fatal("recovery eligibility differs from hint", state)
		}
	}
}

func TestStatusKeepsConfigTestRunningAndShowsItsAsyncResult(t *testing.T) {
	m := openAlertConfig(demoModel())
	canceled := false
	m.notificationTestPending = true
	m.notificationTestGeneration = 2
	m.notificationTestCancel = func() { canceled = true }
	m.notificationTestStatus = "Test: Sending…"
	m = key(m, "ctrl+e")
	if canceled || !m.notificationTestPending {
		t.Fatal("status canceled the suspended Config test")
	}
	next, _ := m.Update(notificationTestResultMsg{generation: 2})
	m = next.(Model)
	if m.dialog.active != dialogStatus || !strings.Contains(ansi.Strip(m.View()), "accepted by macOS") {
		t.Fatal("async result did not reach status")
	}
	m = key(m, "esc")
	if m.dialog.active != dialogConfig || !strings.Contains(lastRow(m), "accepted by macOS") {
		t.Fatal("Config result lost on return")
	}
}

func TestBackgroundNotificationsPreserveLocalFeedbackAndBrowserFailure(t *testing.T) {
	m, client, _ := notificationModel(t)
	m.toast, m.toastAt = "Local decision updated; printer state unchanged", m.now
	client.status = notifications.Status{Revision: 1, Last: "Background notification accepted"}
	m = persistenceUpdate(m, tickMsg(m.now.Add(time.Second)))
	if !strings.HasPrefix(m.toast, "Local decision updated") {
		t.Fatal("background notification replaced local feedback")
	}
	next, _ := m.Update(browserResultMsg{err: errors.New("unavailable")})
	m = next.(Model)
	m = persistenceUpdate(m, tickMsg(m.now.Add(toastLifetime+time.Second)))
	if m.browserError == "" || !strings.Contains(lastRow(m), "Could not open browser") {
		t.Fatal("browser failure expired")
	}
	next, _ = m.Update(browserResultMsg{})
	if next.(Model).browserError != "" {
		t.Fatal("browser success did not clear failure")
	}
}

func TestExportHintsFollowAvailabilityAndWriteState(t *testing.T) {
	for _, state := range []string{"preview", "writing", "saved", "unavailable", "failure"} {
		m := resizeModel(key(demoModel(), "D"), 60, 18)
		switch state {
		case "writing":
			m.exportDialog.writing = true
		case "saved":
			m.exportDialog.saved = true
		case "unavailable":
			m.exportDialog.destination = ""
		case "failure":
			m.exportDialog.problem = "Export failed. Check permissions."
		}
		m.syncDialogs()
		view := ansi.Strip(m.View())
		wantEnter := state == "preview" || state == "failure"
		if strings.Contains(view, "Enter export") || strings.Contains(view, "Enter retry") {
			if !wantEnter {
				t.Fatal(state, "advertises disabled export")
			}
		} else if wantEnter {
			t.Fatal(state, "export action hidden")
		}
		if !strings.Contains(lastRow(m), "Esc close") {
			t.Fatal(state, "close action hidden")
		}
	}
}

func TestFooterLayoutsAndExceptionsWithColorAndOverlappingFeedback(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, color := range []termenv.Profile{termenv.TrueColor, termenv.Ascii} {
		lipgloss.SetColorProfile(color)
		for _, size := range [][2]int{{60, 18}, {80, 24}, {120, 32}} {
			for _, screen := range []string{"dashboard", "details", "notices", "empty", "help", "inspector", "history", "clear notices", "decisions", "reset", "attention", "alerts", "filament", "activity", "Config General", "Config Alerts", "Config Printers", "export", "status"} {
				for _, feedback := range []string{"none", "success", "failure"} {
					m := noticeModel(t)
					switch screen {
					case "dashboard":
						m.focus = panelPrinters
					case "details":
						m.focus = panelDetails
					case "empty":
						m = New(farm.New(10), nil, nil, true)
					case "help":
						m = key(m, "?")
					case "inspector":
						m = key(m, "enter")
					case "history":
						m = key(m, "C")
					case "clear notices":
						m = key(m, "X")
					case "decisions", "reset":
						m = key(key(m, "a"), "n")
						if screen == "reset" {
							m = key(m, "R")
						}
					case "attention":
						m = attentionTick(attentionReport(t, demoModel(), "H", runoutReport))
					case "alerts":
						m = key(m, "s")
					case "filament":
						m = key(filamentModel(), "m")
					case "activity":
						m, _ = activityModel(20)
						m = key(m, "N")
					case "Config General", "Config Alerts", "Config Printers":
						m = key(m, ",")
						if screen != "Config General" {
							m = key(m, "tab")
						}
						if screen == "Config Printers" {
							m = key(m, "tab")
						}
					case "export":
						m = key(m, "D")
					case "status":
						m = key(m, "ctrl+e")
					}
					if feedback == "success" {
						m.toast = strings.Repeat("Long local feedback ", 20)
						m.toastAt = m.now
					}
					if feedback == "failure" {
						m.configStorageError = "Config save failed"
						m.NoticeStorageError = "Could not save notices"
						m.exportError = "Diagnostic export failed"
					}
					m = resizeModel(m, size[0], size[1])
					view := m.View()
					rows := strings.Split(view, "\n")
					if len(rows) != size[1] {
						t.Fatal(screen, "height overflow", len(rows))
					}
					for _, row := range rows {
						if ansi.StringWidth(row) > size[0] {
							t.Fatal(screen, "width overflow")
						}
					}
					if color == termenv.Ascii && strings.Contains(view, "\x1b") {
						t.Fatal(screen, "ANSI in no-color view")
					}
					if screen != "filament" && ansi.StringWidth(rows[len(rows)-1]) != size[0] {
						t.Fatal(screen, "footer lost full width")
					}
					if screen != "status" && !strings.Contains(ansi.Strip(view), "^E status") {
						t.Fatal(screen, "full feedback inaccessible")
					}
				}
			}
		}
	}
}

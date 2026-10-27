package ui

import tea "github.com/charmbracelet/bubbletea"

type (
	dialogKind uint8

	// dialogState owns focus. Status can suspend any owner, including alerts
	// over attention; reset confirmations remain substates of their owner.
	dialogState struct {
		active   dialogKind
		returnTo dialogKind
		resume   dialogKind
	}

	dialogRoute struct {
		key     func(*Model, string, tea.Msg) tea.Cmd
		view    func(Model) string
		sync    func(*Model)
		clear   func(*Model)
		compact bool
	}
)

const (
	dialogNone dialogKind = iota
	dialogHelp
	dialogAbout
	dialogInspector
	dialogClearHistory
	dialogClearNotices
	dialogAttention
	dialogFilament
	dialogDecisions
	dialogAlerts
	dialogConfig
	dialogNotificationActivity
	dialogExport
	dialogShutdown
	dialogStatus
	dialogServerStatus
)

// routeFor is the single catalog for dialog input, rendering, and lifecycle.
// Order carries no priority: the explicit active owner selects exactly one route.
func routeFor(kind dialogKind) dialogRoute {
	switch kind {
	case dialogAbout:
		return dialogRoute{key: (*Model).aboutKey, view: Model.aboutView, sync: (*Model).syncAbout}
	case dialogHelp:
		return dialogRoute{key: (*Model).helpKey, view: Model.helpView, sync: (*Model).syncHelp, clear: func(m *Model) { m.help.GotoTop() }}
	case dialogInspector:
		return dialogRoute{key: (*Model).inspectorKey, view: Model.inspectorView, sync: (*Model).syncInspector, clear: func(m *Model) { m.inspected = nil }}
	case dialogClearHistory:
		return dialogRoute{key: (*Model).clearHistoryKey, view: Model.clearHistoryView, clear: func(m *Model) { m.clearHistoryEvents = nil }}
	case dialogClearNotices:
		return dialogRoute{key: (*Model).clearNoticesKey, view: Model.clearNoticesView, clear: func(m *Model) { m.clearNotices = nil }}
	case dialogAttention:
		return dialogRoute{key: (*Model).attentionKey, view: Model.attentionView, sync: (*Model).syncAttentionView, clear: func(m *Model) { m.attention = nil }}
	case dialogFilament:
		return dialogRoute{key: (*Model).filamentKey, view: Model.filamentView, sync: (*Model).syncFilament, clear: func(m *Model) { m.filament = nil }}
	case dialogDecisions:
		return dialogRoute{key: (*Model).decisionsKey, view: Model.decisionsView, sync: (*Model).syncDecisions, clear: func(m *Model) { m.decisions = nil }}
	case dialogAlerts:
		return dialogRoute{key: (*Model).alertKey, view: Model.alertView, sync: (*Model).syncAlertDialog, clear: func(m *Model) { m.alertDialog = nil }}
	case dialogNotificationActivity:
		return dialogRoute{key: (*Model).notificationActivityKey, view: Model.notificationActivityView, sync: (*Model).syncNotificationActivity, clear: func(m *Model) { m.notificationActivity = nil }}
	case dialogConfig:
		return dialogRoute{key: (*Model).configKey, view: Model.configView, sync: (*Model).syncConfig, clear: (*Model).clearConfig}
	case dialogExport:
		return dialogRoute{key: (*Model).exportKey, view: Model.exportView, sync: (*Model).syncExport, clear: func(m *Model) { m.exportDialog = nil }}
	case dialogServerStatus:
		return dialogRoute{key: (*Model).serverStatusKey, view: Model.serverStatusView, sync: (*Model).syncServerStatus}
	case dialogStatus:
		return dialogRoute{key: (*Model).statusKey, view: Model.statusView, sync: (*Model).syncStatus}
	case dialogShutdown:
		return dialogRoute{
			key: (*Model).shutdownKey, view: Model.shutdownView, compact: true,
			clear: func(m *Model) { m.quitting, m.quitProblem = false, false },
		}
	default:
		return dialogRoute{}
	}
}

// openDialog replaces the owner, except for status inspection and the attention
// -> alerts return path. Concrete payloads are installed by the caller after this transition.
func (m *Model) openDialog(kind dialogKind) {
	if kind == dialogStatus || (kind == dialogServerStatus && m.dialog.active == dialogConfig) {
		m.dialog = dialogState{active: kind, returnTo: m.dialog.active, resume: m.dialog.returnTo}
		return
	}
	if kind == dialogAlerts && m.dialog.active == dialogAttention {
		m.dialog = dialogState{active: kind, returnTo: dialogAttention}
		return
	}
	for m.hasOpenDialog() {
		m.closeDialog()
	}
	m.dialog = dialogState{active: kind}
}

func (m *Model) closeDialog() { m.dismissDialog(m.dialog.active) }

// dismissDialog also supports removing suspended attention when snoozing drains
// its last printer. Closing alerts then returns directly to the dashboard.
func (m *Model) dismissDialog(kind dialogKind) {
	if kind == dialogNone {
		return
	}
	if clear := routeFor(kind).clear; clear != nil {
		clear(m)
	}
	if m.dialog.active == kind {
		m.dialog.active, m.dialog.returnTo, m.dialog.resume = m.dialog.returnTo, m.dialog.resume, dialogNone
	} else if m.dialog.returnTo == kind {
		m.dialog.returnTo, m.dialog.resume = m.dialog.resume, dialogNone
	} else if m.dialog.resume == kind {
		m.dialog.resume = dialogNone
	}
}

func (m Model) hasOpenDialog() bool { return m.dialog.active != dialogNone }

// acceptsKey keeps actions behind an undersized resize screen inactive. Esc
// still follows the owner's normal close/return path. Dashboard quit and compact
// shutdown controls stay available without making hidden confirmations actionable.
func (m Model) acceptsKey(key string) bool {
	if m.terminalFits() || routeFor(m.dialog.active).compact {
		return true
	}
	if key == keyEscape {
		return true
	}
	return !m.hasOpenDialog() && (key == keyQuit || key == keyInterrupt)
}

// syncDialogs collects pending attention, then refreshes the owner and its
// explicitly suspended parent during telemetry, resizing, and async completion.
func (m *Model) syncDialogs() {
	m.syncAttention(false)
	for _, kind := range []dialogKind{m.dialog.resume, m.dialog.returnTo, m.dialog.active} {
		if sync := routeFor(kind).sync; sync != nil {
			sync(m)
		}
	}
}

func (m *Model) openClearHistory() {
	m.openDialog(dialogClearHistory)
	for _, event := range m.Farm.Events {
		m.clearHistoryEvents = append(m.clearHistoryEvents, event.ID)
	}
}

func (m *Model) clearHistoryKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyYes, keyYesUpper:
		for _, id := range m.clearHistoryEvents {
			m.DeleteEvent(id)
		}
		m.closeDialog()
		m.setToast(feedbackSuccess, "Confirmed history cleared; newer events and active conditions kept")
	case keyEscape, keyDecisionsOrNo, keyNoUpper:
		m.closeDialog()
	}
	return nil
}

// Shutdown owns input through both the bounded wait and the explicit exit choice.
func (m *Model) shutdownKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyEscape:
		// Cancel shutdown, keeping in-flight saves serialized and tracked.
		m.closeDialog()
	case keyQuit, keyInterrupt:
		if m.quitProblem {
			m.stopNotifications()
			return tea.Quit
		}
	}
	return nil
}

func (m Model) shutdownView() string {
	if m.quitting {
		return fit(wrapLines([]string{"Esc cancels quitting; saves continue.", "Saving settings/notices/snoozes and stopping notifications (up to 5 seconds)."}, m.width), m.width, m.height)
	}
	return fit(wrapLines([]string{"q / Ctrl+C exits anyway; Esc cancels.", "Storage failed or did not finish within 5 seconds; changes may be lost.", m.storageWarning()}, m.width), m.width, m.height)
}

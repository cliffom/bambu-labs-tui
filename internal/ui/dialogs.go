package ui

import tea "github.com/charmbracelet/bubbletea"

type (
	dialogKind uint8

	// dialogState owns focus. Only alerts opened from attention suspend a parent;
	// reset confirmation remains an explicit substate of the decisions dialog.
	dialogState struct {
		active   dialogKind
		returnTo dialogKind
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
	dialogInspector
	dialogClearHistory
	dialogClearNotices
	dialogAttention
	dialogFilament
	dialogDecisions
	dialogAlerts
	dialogConfig
	dialogExport
	dialogShutdown
)

// routeFor is the single catalog for dialog input, rendering, and lifecycle.
// Order carries no priority: the explicit active owner selects exactly one route.
func routeFor(kind dialogKind) dialogRoute {
	switch kind {
	case dialogHelp:
		return dialogRoute{key: (*Model).helpKey, view: func(m Model) string { return m.dashboardView(true) }}
	case dialogInspector:
		return dialogRoute{key: (*Model).inspectorKey, view: Model.inspectorView, sync: (*Model).syncInspector, clear: func(m *Model) { m.inspected = nil }}
	case dialogClearHistory:
		return dialogRoute{key: (*Model).clearHistoryKey, view: Model.clearHistoryView}
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
	case dialogConfig:
		return dialogRoute{key: (*Model).configKey, view: Model.configView, sync: (*Model).syncConfig, clear: (*Model).clearConfig}
	case dialogExport:
		return dialogRoute{key: (*Model).exportKey, view: Model.exportView, sync: (*Model).syncExport, clear: func(m *Model) { m.exportDialog = nil }}
	case dialogShutdown:
		return dialogRoute{
			key: (*Model).shutdownKey, view: Model.shutdownView, compact: true,
			clear: func(m *Model) { m.quitting, m.quitProblem = false, false },
		}
	default:
		return dialogRoute{}
	}
}

// openDialog replaces the owner, except for the intentional attention -> alerts
// return path. Concrete payloads are installed by the caller after this transition.
func (m *Model) openDialog(kind dialogKind) {
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
		m.dialog.active, m.dialog.returnTo = m.dialog.returnTo, dialogNone
	} else if m.dialog.returnTo == kind {
		m.dialog.returnTo = dialogNone
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
	for _, kind := range []dialogKind{m.dialog.returnTo, m.dialog.active} {
		if sync := routeFor(kind).sync; sync != nil {
			sync(m)
		}
	}
}

func (m *Model) helpKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyHelp:
		m.closeDialog()
	case keyEscape:
		m.closeDialog()
		m.focus = panelPrinters
	}
	return nil
}

func (m *Model) clearHistoryKey(key string, _ tea.Msg) tea.Cmd {
	switch key {
	case keyYes, keyYesUpper:
		m.Farm.ClearEvents()
		m.closeDialog()
		m.toast, m.toastAt = "Session event history cleared; active conditions remain", m.now
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
		return fit("Saving settings/notices/snoozes and stopping notifications (up to 5 seconds).\nEsc cancels quitting; saves continue in the background.", m.width, m.height)
	}
	return fit("Storage failed or did not finish within 5 seconds; changes may be lost.\nPress q / Ctrl+C to exit anyway, or Esc to keep using the app.\n"+m.storageWarning(), m.width, m.height)
}

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/webserver"
	"github.com/muesli/termenv"
)

func serverStatusText(m Model) string {
	view := m.serverStatus
	view.Height = view.TotalLineCount()
	view.GotoTop()
	return ansi.Strip(view.View())
}

func TestServerStatusRoutesRestoreConfigAndDashboard(t *testing.T) {
	m := resizeModel(demoModel(), 60, 18)
	m = key(key(m, "j"), "tab")
	selection, focus := m.selection, m.focus
	m = key(m, "W")
	for _, blocked := range []string{"q", "ctrl+c", "F", ",", "r", "a"} {
		m = key(m, blocked)
	}
	if m.dialog.active != dialogServerStatus || m.selection != selection || m.focus != focus {
		t.Fatal("inspection leaked input")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() || m.selection != selection || m.focus != focus {
		t.Fatal("dashboard position lost")
	}
	m = webSettings(m)
	m = key(m, "end")
	if m.configSelection != 2 || !strings.Contains(m.View(), "Server Status") {
		t.Fatal("missing Config entry")
	}
	m = key(m, "enter")
	m = key(m, "end")
	offset := m.serverStatus.YOffset
	if offset == 0 {
		t.Fatal("inspection did not scroll")
	}
	m = key(m, "ctrl+e")
	if m.dialog != (dialogState{active: dialogStatus, returnTo: dialogServerStatus, resume: dialogConfig}) {
		t.Fatal("wrong nested owners", m.dialog)
	}
	m = key(m, "esc")
	if m.dialog.active != dialogServerStatus || m.serverStatus.YOffset != offset {
		t.Fatal("feedback lost inspection position")
	}
	m = key(m, "esc")
	if m.dialog.active != dialogConfig || m.configSection != configWebServer || m.configSelection != 2 {
		t.Fatal("Config position lost")
	}
	// Editing/confirmation keys retain Config ownership; inspection cannot consume
	// a typed W or replace a pending confirmation.
	m = key(key(m, "up"), "enter")
	m = key(m, "W")
	if !m.configEditing || m.dialog.active != dialogConfig {
		t.Fatal("inspection stole port input")
	}
	m = key(m, "esc")
	m = key(m, "W")
	if m.dialog.returnTo != dialogConfig {
		t.Fatal("Config shortcut missing")
	}
}

func TestServerStatusLiveLifecycleClientsAndPersistence(t *testing.T) {
	server := &fakeWebServer{}
	saves := 0
	m := webSettings(resizeModel(demoModel(), 60, 18)).WithWebServer(server).WithConfigPersistence("synthetic", func(config.Config) error { saves++; return nil })
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(m, "W")
	if text := serverStatusText(m); !strings.Contains(text, "Server: Starting") || strings.Contains(text, "http://") {
		t.Fatal("unbound URL or missing starting state", text)
	}
	m = completeWeb(m, cmd)
	if saves != 1 {
		t.Fatal("inspection blocked asynchronous configuration save")
	}
	if !strings.Contains(serverStatusText(m), "Server: Running") || !strings.Contains(m.View(), server.status.URL) {
		t.Fatal("lifecycle result not live")
	}
	m.NoticeStorageError = "Synthetic notice write failure"
	server.clients = webserver.ClientStatus{Active: 1, Omitted: 7, Events: []webserver.ClientEvent{{Sequence: 9, Client: 4, At: m.now, Connected: true, IP: "127.0.0.1", UserAgent: strings.Repeat("Synthetic client ", 10)}}}
	next, _ := m.Update(monitor.Event{Kind: monitor.EventReport, ID: "H", Payload: []byte(`{"print":{"mc_percent":77}}`)})
	m = next.(Model)
	text := serverStatusText(m)
	for _, want := range []string{"HTTP clients: 1 active", "7 client details omitted", "Connected #4", "Earlier client events", "Synthetic notice write failure"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if *m.Farm.Printers["H"].Progress != 77 {
		t.Fatal("modal blocked telemetry")
	}
	m = key(m, "esc")
	m = key(m, "home")
	m, cmd = webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(m, "W")
	if !strings.Contains(serverStatusText(m), "Server: Stopping") {
		t.Fatal("missing stopping state")
	}
	m = completeWeb(m, cmd)
	if !strings.Contains(serverStatusText(m), "Stopped (disabled)") || strings.Contains(serverStatusText(m), "http://") {
		t.Fatal("disabled state retained URL")
	}
	calls := len(server.calls)
	m = key(m, "esc")
	if len(server.calls) != calls {
		t.Fatal("close changed listener")
	}
}

func TestServerStatusDiagnosticsPrivacyBoundsAndMonitorStates(t *testing.T) {
	m := key(demoModel(), "W")
	m.Demo = false
	for _, event := range []monitor.Event{
		{Kind: monitor.EventConnection, Connected: true},
		{Kind: monitor.EventConnection, Connected: false},
		{Kind: monitor.EventProblem, Problem: "private-token-serial"},
		{Kind: monitor.EventConnection, StopReason: monitor.StopAuthenticationRejected, Problem: "private-token-serial"},
	} {
		next, _ := m.Update(event)
		m = next.(Model)
	}
	text := serverStatusText(m)
	for _, want := range []string{"Mode: cloud", "Authentication rejected", "Monitoring: Reconnecting", "Monitor warning; monitoring continues", "does not establish fresh"} {
		// Wrapping may split fixed prose across physical lines.
		if !strings.Contains(strings.Join(strings.Fields(text), " "), want) {
			t.Fatal("monitor status missing", want, text)
		}
	}
	if strings.Contains(text, "private-") || strings.Contains(m.View(), "private-") {
		t.Fatal("raw backend diagnostic retained")
	}
	for i := range serverDiagnosticLimit * 2 {
		m.noteServer(fmt.Sprintf("Synthetic diagnostic %d", i), feedbackWarning)
	}
	if len(m.serverDiagnostics) != serverDiagnosticLimit {
		t.Fatal("unbounded diagnostics")
	}
	m.noteServer(m.serverDiagnostics[len(m.serverDiagnostics)-1].text, feedbackWarning)
	if len(m.serverDiagnostics) != serverDiagnosticLimit {
		t.Fatal("duplicate diagnostic expanded history")
	}
}

func TestServerStatusLayoutsScrollThemesAndResize(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		for _, size := range [][2]int{{60, 18}, {120, 32}} {
			m := resizeModel(demoModel(), size[0], size[1])
			m.webStatus = webserver.Status{URL: "http://127.0.0.1:65535", Started: m.now.Add(-time.Minute)}
			m.sessionStarted = m.now.Add(-time.Hour)
			m.webClients.Omitted = 3
			for i := range 32 {
				m.webClients.Events = append(m.webClients.Events, webserver.ClientEvent{Sequence: uint64(i + 1), Client: uint64(i + 1), IP: "127.0.0.1", UserAgent: strings.Repeat("LongSanitizedUserAgent", 7), At: m.now, Connected: i%2 == 0})
			}
			m = key(m, "W")
			text := serverStatusText(m)
			for _, want := range []string{"http://127.0.0.1:65535", "Session uptime: 1h0m0s", "Listener uptime: 1m0s", "LongSanitizedUserAgent"} {
				if !strings.Contains(text, want) {
					t.Fatal("status clipped", want)
				}
			}
			assertServerStatusLayout(t, m, size[0], size[1])
			m = key(m, "pgdown")
			offset := m.serverStatus.YOffset
			if offset == 0 {
				t.Fatal("page navigation failed")
			}
			m = resizeModel(m, 59, 17)
			for _, blocked := range []string{"home", "end", "j", "W", "ctrl+e", "q", "ctrl+c"} {
				m = key(m, blocked)
			}
			if m.serverStatus.YOffset != offset || m.dialog.active != dialogServerStatus {
				t.Fatal("hidden controls changed state")
			}
			m = resizeModel(m, size[0], size[1])
			if m.serverStatus.YOffset != offset {
				t.Fatal("minimum-size restore lost scroll")
			}
			next, _ := m.Update(tickMsg(m.now.Add(time.Second)))
			m = next.(Model)
			if m.serverStatus.YOffset != offset {
				t.Fatal("live tick lost scroll")
			}
			m = key(m, "ctrl+e")
			palette := DefaultPalette()
			palette.Background = "#102030"
			palette.Success = "#ABCDEF"
			m = m.WithPalette(palette)
			m = key(m, "esc")
			if m.serverStatus.YOffset != offset || m.dialog.active != dialogServerStatus {
				t.Fatal("recolor lost suspended state")
			}
			m = key(m, "home")
			assertPaint(t, m.View(), "Server: Running", palette.Success, palette.Background, true)
			if profile == termenv.Ascii && strings.Contains(m.View(), "\x1b") {
				t.Fatal("ANSI in no-color view")
			}
			m = key(m, "end")
			if !strings.Contains(m.View(), "Recent diagnostics") {
				t.Fatal("cannot reach bottom")
			}
			assertServerStatusLayout(t, m, size[0], size[1])
			t.Logf("profile=%v size=%dx%d history bottom:\n%s", profile, size[0], size[1], ansi.Strip(m.View()))
		}
	}
}

func assertServerStatusLayout(t *testing.T, m Model, width, height int) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != height {
		t.Fatal("incorrect height", len(lines))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) != width {
			t.Fatal("incorrect row width", ansi.StringWidth(line))
		}
	}
	if strings.Contains(m.View(), "Ctrl+C to stop") || !strings.Contains(ansi.Strip(m.View()), "Esc back") {
		t.Fatal("wrong navigation hint")
	}
}

func TestServerStatusFailedEmptyAndStoppedStream(t *testing.T) {
	server := &fakeWebServer{fail: true}
	m := webSettings(demoModel()).WithWebServer(server)
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = completeWeb(m, cmd)
	m = key(m, "W")
	for _, want := range []string{"Server: Failed", "Could not start localhost", "Recent client activity: none", "Bound URL: unavailable"} {
		if !strings.Contains(serverStatusText(m), want) {
			t.Fatal("missing failure state", want)
		}
	}
	next, _ := m.Update(endMsg{})
	m = next.(Model)
	t.Logf("failed/empty frame:\n%s", ansi.Strip(m.View()))
	if !strings.Contains(serverStatusText(m), "Stopped (event stream ended)") {
		t.Fatal("missing ended monitor")
	}
}

func TestServerStatusFailureAndStorageWarningLayouts(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		m := resizeModel(demoModel(), size[0], size[1])
		m.webEnabled = true
		m.webStatus.Error = "Port 65535 is already in use; web server not started"
		m.configStorageError = "Synthetic configuration write failure"
		m.NoticeStorageError = "Synthetic notice write failure"
		m.AlertStorageError = "Synthetic snooze write failure"
		m = key(m, "W")
		text := serverStatusText(m)
		for _, want := range []string{"Server: Failed", "Bound URL: unavailable", "Config storage:", "Notices storage:", "snoozes storage:", "Recent client activity: none"} {
			if !strings.Contains(text, want) {
				t.Fatal("missing persistent failure", want)
			}
		}
		assertServerStatusLayout(t, m, size[0], size[1])
		t.Logf("%dx%d failed top:\n%s", size[0], size[1], ansi.Strip(m.View()))
		m = key(m, "end")
		assertServerStatusLayout(t, m, size[0], size[1])
		if !strings.Contains(m.View(), "Recent diagnostics") {
			t.Fatal("failures prevented bottom navigation")
		}
		t.Logf("%dx%d failed bottom:\n%s", size[0], size[1], ansi.Strip(m.View()))
	}
}

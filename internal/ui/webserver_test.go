package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/notifications"
	"github.com/cliffom/bambustand/internal/session"
	"github.com/cliffom/bambustand/internal/webserver"
)

type fakeWebServer struct {
	themes        webserver.ThemeCatalog
	status        webserver.Status
	clients       webserver.ClientStatus
	calls         []bool
	ports         []int
	snapshot      []byte
	notifications []notifications.Input
	fail          bool
}

func (s *fakeWebServer) SubmitNotifications(input notifications.Input) {
	s.notifications = append(s.notifications, input)
}

func (s *fakeWebServer) Clients() webserver.ClientStatus {
	result := s.clients
	result.Events = append([]webserver.ClientEvent(nil), result.Events...)
	return result
}

func (s *fakeWebServer) PublishThemes(c webserver.ThemeCatalog) { s.themes = c }

func (s *fakeWebServer) Publish(data []byte)      { s.snapshot = data }
func (s *fakeWebServer) Status() webserver.Status { return s.status }
func (s *fakeWebServer) SetEnabled(enabled bool, port int) webserver.Status {
	s.calls = append(s.calls, enabled)
	s.ports = append(s.ports, port)
	s.status = webserver.Status{}
	if enabled {
		if s.fail {
			s.status.Error = "Could not start localhost web server"
		} else {
			s.status.URL = fmt.Sprintf("http://127.0.0.1:%d", port)
		}
	}
	return s.status
}

func webSettings(m Model) Model {
	m = key(m, ",")
	for m.configSection != configWebServer {
		m = key(m, "tab")
	}
	return m
}

func webCommand(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// Run only lifecycle messages in a Batch, leaving ticker/monitor waits untouched.
func completeWeb(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = completeWeb(m, c)
		}
		return m
	}
	switch msg.(type) {
	case webServerResultMsg, configResultMsg, persistenceResultMsg:
		next, follow := m.Update(msg)
		return completeWeb(next.(Model), follow)
	}
	return m
}

func TestWebSettingsEnableURLDisableAndPersistence(t *testing.T) {
	server := &fakeWebServer{}
	m := demoModel().WithWebServer(server)
	startup := m.syncWebServer()
	m = completeWeb(m, startup)
	m = webSettings(m)
	var saved []config.Config
	m = m.WithConfigPersistence("synthetic", func(c config.Config) error { saved = append(saved, c); return nil })
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if len(server.calls) != 1 || !m.webBusy || m.webStatus.URL != "" || !m.webEnabled {
		t.Fatal("lifecycle ran in reducer or URL appeared before bind")
	}
	m = completeWeb(m, cmd) // Handles both save and lifecycle result messages.
	if m.webStatus.URL != "http://127.0.0.1:8080" || m.webBusy || len(saved) != 1 || !saved[0].WebServer.Enabled {
		t.Fatal("enable/save incomplete", m.webStatus, saved)
	}
	if !strings.Contains(ansi.Strip(m.View()), server.status.URL) {
		t.Fatal("running port hidden")
	}
	m, cmd = webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = completeWeb(m, cmd)
	if m.webEnabled || m.webStatus.URL != "" || saved[len(saved)-1].WebServer.Enabled {
		t.Fatal("disable did not stop or save")
	}
}

func TestWebRapidTogglesCoalesceAndTelemetryContinues(t *testing.T) {
	server := &fakeWebServer{}
	m := webSettings(demoModel())
	m = m.WithWebServer(server)
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(m, "enter") // Disable while start is pending.
	if len(server.calls) != 0 {
		t.Fatal("overlapping lifecycle work")
	}
	next, _ := m.Update(monitor.Event{Kind: monitor.EventReport, ID: "H", At: m.now, Payload: []byte(`{"print":{"mc_percent":77}}`)})
	m = next.(Model)
	if *m.Farm.Printers["H"].Progress != 77 {
		t.Fatal("pending web work blocked telemetry")
	}
	m = completeWeb(m, cmd)
	if m.webStatus.URL != "" || len(server.calls) != 2 || !server.calls[0] || server.calls[1] {
		t.Fatal("rapid toggles lost final disable", server.calls)
	}
}

func TestWebFailureExplicitRetryAndHiddenControls(t *testing.T) {
	server := &fakeWebServer{fail: true}
	m := webSettings(demoModel()).WithWebServer(server)
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = completeWeb(m, cmd)
	if m.webStatus.Error == "" || !strings.Contains(ansi.Strip(m.View()), "Failed") {
		t.Fatal("failed start hidden")
	}
	count := len(server.calls)
	next, _ := m.Update(tickMsg(m.now.Add(time.Second)))
	m = next.(Model)
	if len(server.calls) != count {
		t.Fatal("tick retried failure")
	}
	m = resizeModel(m, 59, 17)
	for _, keyName := range []string{"enter", "r", "right", "tab"} {
		m = key(m, keyName)
	}
	if !m.webEnabled || !m.webInitialized || len(server.calls) != count {
		t.Fatal("hidden settings acted below minimum")
	}
	m = resizeModel(m, 60, 18)
	server.fail = false
	m, cmd = webCommand(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = completeWeb(m, cmd)
	if m.webStatus.Error != "" || m.webStatus.URL == "" {
		t.Fatal("explicit retry failed")
	}
	m = key(m, "ctrl+e")
	m = key(m, "esc")
	if m.configSection != configWebServer || m.dialog.active != dialogConfig {
		t.Fatal("status lost settings owner")
	}
	m = key(m, "esc")
	if m.hasOpenDialog() {
		t.Fatal("Escape did not close settings")
	}
}

func TestWebEnabledStartupAndSettingsBounds(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {120, 32}} {
		server := &fakeWebServer{}
		m := demoModel().WithConfiguration(config.Config{Region: config.RegionGlobal, LowFilamentPercent: 10, Alerts: config.DefaultAlerts(), WebServer: config.WebServer{Enabled: true}}).WithWebServer(server)
		startup := m.syncWebServer()
		m = completeWeb(m, startup)
		if m.webStatus.URL == "" || !strings.Contains(string(server.snapshot), `"demo":true`) {
			t.Fatal("startup omitted server or snapshot")
		}
		m = webSettings(resizeModel(m, size[0], size[1]))
		view := ansi.Strip(m.View())
		lines := strings.Split(view, "\n")
		if len(lines) != size[1] || !strings.Contains(view, server.status.URL) || !strings.Contains(view, "> Enable web server: On") {
			t.Fatal("URL or toggle hidden", view)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("settings overflow")
			}
		}
	}
}

func TestWebPortEditingValidationPersistenceAndRestart(t *testing.T) {
	server := &fakeWebServer{}
	m := webSettings(demoModel()).WithWebServer(server)
	var saved []config.Config
	m = m.WithConfigPersistence("synthetic", func(c config.Config) error { saved = append(saved, c); return nil })
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = completeWeb(m, cmd)
	m = key(m, "down")
	for _, input := range []string{"0", "-1", "65536", "1.5", "abc", ""} {
		m = key(m, "enter")
		m = key(m, "ctrl+u")
		if input != "" {
			m = key(m, input)
		}
		m = key(m, "enter")
		if !m.configEditing || m.configInputError == "" || m.webPort != config.DefaultWebPort {
			t.Fatal("invalid port applied", input)
		}
		m = key(m, "esc")
	}
	m = key(m, "enter")
	m = key(m, "9090")
	m = key(m, "ctrl+e")
	m = key(m, "esc")
	if !m.configEditing || m.configInput != "9090" {
		t.Fatal("status lost port edit")
	}
	m = resizeModel(m, 59, 17)
	m = key(m, "enter")
	if m.webPort != config.DefaultWebPort || !m.configEditing {
		t.Fatal("hidden port apply acted")
	}
	m = resizeModel(m, 60, 18)
	m, cmd = webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = completeWeb(m, cmd)
	if m.configEditing || m.webPort != 9090 || m.webStatus.URL != "http://127.0.0.1:9090" || len(saved) != 2 || saved[1].WebServer.Port != 9090 {
		t.Fatal("port restart/save failed", m.webStatus, saved)
	}
	m = key(m, "enter")
	m = key(m, "9999")
	m = key(m, "esc")
	if m.webPort != 9090 {
		t.Fatal("Escape applied canceled port")
	}
}

func TestPendingWebPortChangesCoalesce(t *testing.T) {
	server := &fakeWebServer{}
	m := webSettings(demoModel()).WithWebServer(server)
	m, cmd := webCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = key(m, "down")
	m = key(m, "enter")
	m = key(m, "9090")
	m = key(m, "enter")
	m = key(m, "enter")
	m = key(m, "10000")
	m = key(m, "enter")
	if len(server.calls) != 0 {
		t.Fatal("port changes started overlapping lifecycle commands")
	}
	m = completeWeb(m, cmd)
	if len(server.calls) != 2 || server.ports[0] != config.DefaultWebPort || server.ports[1] != 10000 || m.webStatus.URL != "http://127.0.0.1:10000" {
		t.Fatal("port changes did not coalesce", server.ports, m.webStatus)
	}
}

func TestWebNotificationsObserveWhileDisabledAndStopDuringShutdown(t *testing.T) {
	server := &fakeWebServer{}
	m := demoModel().WithWebServer(server)
	m.syncWebServer()
	if len(server.notifications) == 0 || !server.notifications[len(server.notifications)-1].Active {
		t.Fatal("disabled listener lost browser lifecycle observations")
	}
	m.quitting = true
	m.syncWebServer()
	if server.notifications[len(server.notifications)-1].Active {
		t.Fatal("shutdown allowed browser delivery")
	}
	m.quitting = false
	m.syncWebServer()
	if !server.notifications[len(server.notifications)-1].Active {
		t.Fatal("cancelled shutdown did not resume browser observation")
	}
}

func (s *fakeWebServer) PublishDisplay(state session.DisplayState) {
	s.Publish(webserver.EncodeDisplay(state))
}

package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cliffom/bambustand/internal/webserver"
	"github.com/muesli/termenv"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/session"
)

func TestServeStatusPlainTransitionsAndPrivacy(t *testing.T) {
	var output bytes.Buffer
	display := newServeDisplay(&output, "http://127.0.0.1:8080", false)
	if err := display.begin(); err != nil {
		t.Fatal(err)
	}
	now := display.rendered.Add(time.Second)
	for range 1000 {
		display.observe(monitor.Event{Kind: monitor.EventConnection, Problem: "private-token\x1b[2Jserial"})
		display.observe(monitor.Event{Kind: monitor.EventProblem, Problem: "private-account"})
	}
	if err := display.render(now, false); err != nil {
		t.Fatal(err)
	}
	size := output.Len()
	if err := display.render(now.Add(time.Second), false); err != nil {
		t.Fatal(err)
	}
	if output.Len() != size {
		t.Fatal("unchanged status flooded redirected output")
	}
	display.observe(monitor.Event{Kind: monitor.EventConnection, Connected: true})
	if err := display.render(now.Add(2*time.Second), false); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Listening on http://127.0.0.1:8080", "Mode: cloud", "Connecting", "Reconnecting", "Connected", "Ctrl+C to stop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "private") || strings.Contains(text, "\x1b") || len(display.recent) > serveRecentLimit || strings.Count(text, "Monitor warning") != 1 {
		t.Fatal("unsafe or unbounded diagnostics", text)
	}
}

func TestServeStatusLayoutsAndPersistentWarnings(t *testing.T) {
	display := newServeDisplay(&bytes.Buffer{}, "http://127.0.0.1:65535", true)
	state := session.New(farm.New(10), true)
	state.NoticeStorageError, state.AlertStorageError = "private-path", "private-account"
	display.persistence(&state)
	display.observe(monitor.Event{Kind: monitor.EventConnection, Connected: true})
	for range 10 {
		display.persistence(&state)
	}
	if display.recent[0].text != "Monitoring: Connected" {
		t.Fatal("persistent warnings displaced recent events")
	}
	for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 18}, {40, 12}, {20, 10}, {10, 6}, {1, 1}} {
		view := display.view(size[0], size[1], display.started.Add(65*time.Second))
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Fatal("height overflow", size, view)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow", size, view)
			}
		}
		if size[0] >= 20 && (!strings.Contains(strings.ReplaceAll(view, "\n", ""), display.url) || !strings.Contains(view, "Ctrl+C to stop")) {
			t.Fatal("lost essential guidance", size, view)
		}
		if size[0] >= 60 && (!strings.Contains(view, "Notice storage:") || !strings.Contains(view, "Snooze storage:")) {
			t.Fatal("lost persistent warning", size, view)
		}
		if strings.Contains(view, "private") {
			t.Fatal("private storage error rendered")
		}
		t.Logf("%dx%d\n%s", size[0], size[1], view)
	}
}

func TestServeStatusRestoresScreenAfterOutputFailure(t *testing.T) {
	var output bytes.Buffer
	display := newServeDisplay(&output, "http://127.0.0.1:8080", false)
	display.size = func() (int, int, error) { return 0, 0, errors.New("size failed") }
	if display.begin() == nil {
		t.Fatal("size error lost")
	}
	if err := display.close(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(output.String(), "\x1b[?25h\x1b[?1049l") {
		t.Fatal("terminal cleanup missing")
	}
}

func TestServeStatusColorProfilesAndClientLedger(t *testing.T) {
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		t.Run(fmt.Sprint(profile), func(t *testing.T) {
			var output bytes.Buffer
			display := newServeDisplay(&output, "http://127.0.0.1:8080", false)
			renderer := lipgloss.NewRenderer(&output)
			renderer.SetColorProfile(profile)
			display.styles = newServeStyles(renderer)
			if profile != termenv.Ascii {
				green := display.styled("state", serveSuccess)
				amber := display.styled("state", serveWarning)
				red := display.styled("state", serveError)
				if green == amber || green == red || amber == red {
					t.Fatal("attention colors do not differ")
				}
			}
			display.observe(monitor.Event{Kind: monitor.EventConnection})
			display.clients = webserver.ClientStatus{Active: 1, Events: []webserver.ClientEvent{{Sequence: 1, Client: 1, Connected: true, At: display.started, IP: "127.0.0.1", UserAgent: "Synthetic client"}}}
			for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 18}, {40, 12}, {20, 10}} {
				view := display.view(size[0], size[1], display.started)
				if profile != termenv.Ascii && !strings.Contains(view, "\x1b[") {
					t.Fatal("color missing")
				}
				if profile == termenv.Ascii && strings.Contains(view, "\x1b[") {
					t.Fatal("no-color emitted styling")
				}
				for _, line := range strings.Split(view, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("colored width overflow", size)
					}
				}
				if len(strings.Split(view, "\n")) > size[1] {
					t.Fatal("colored height overflow")
				}
				if size[0] >= 60 && !strings.Contains(ansi.Strip(view), "Synthetic client") {
					t.Fatal("client ledger missing", view)
				}
			}
			if err := display.render(display.started, true); err != nil {
				t.Fatal(err)
			}
			initial := output.String()
			if !strings.Contains(initial, "HTTP ") || !strings.Contains(initial, "127.0.0.1 | Synthetic client") || strings.Contains(initial, "\x1b") {
				t.Fatal("plain ledger missing/colored", initial)
			}
			if err := display.render(display.rendered.Add(time.Second), false); err != nil {
				t.Fatal(err)
			}
			if output.String() != initial {
				t.Fatal("unchanged ledger reprinted")
			}
			display.clients.Active = 0
			display.clients.Events = append(display.clients.Events, webserver.ClientEvent{Sequence: 2, Client: 1, At: display.started.Add(time.Second), IP: "127.0.0.1", UserAgent: "Synthetic client"})
			if err := display.render(display.rendered.Add(time.Second), false); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "Disconnected #1") {
				t.Fatal("disconnect missing")
			}
		})
	}
}

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/session"
)

const (
	serveRecentLimit    = 3
	serveStatusInterval = time.Second
)

// The serve loop owns this presenter. It never reads terminal input or changes
// terminal modes; canonical Ctrl+C remains owned by the CLI signal context.
type serveDisplay struct {
	output                        io.Writer
	size                          func() (int, int, error)
	started, rendered             time.Time
	url, mode, connection, server string
	recent                        []string
	lastPlain                     string
	noticeWarning, snoozeWarning  bool
	active                        bool
}

func newServeDisplay(output io.Writer, url string, demo bool) *serveDisplay {
	d := &serveDisplay{output: output, url: url, mode: "cloud", connection: "Connecting", server: "Running", started: time.Now()}
	if demo {
		d.mode = "demo (synthetic)"
	}
	if file, ok := output.(*os.File); ok && term.IsTerminal(file.Fd()) && term.IsTerminal(os.Stdin.Fd()) && os.Getenv("TERM") != "dumb" {
		d.size = func() (int, int, error) { return term.GetSize(file.Fd()) }
	}
	return d
}

func (d *serveDisplay) begin() error {
	if d.size != nil {
		// Mark active before writing so a partial write still gets cleanup.
		d.active = true
		if _, err := io.WriteString(d.output, "\x1b[?1049h\x1b[?25l"); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(d.output, "Listening on %s (%s; native desktop notifications disabled)\n", d.url, d.mode); err != nil {
		return err
	}
	return d.render(time.Now(), true)
}

func (d *serveDisplay) note(message string) {
	if len(d.recent) > 0 && d.recent[0] == message {
		return
	}
	d.recent = append([]string{message}, d.recent...)
	if len(d.recent) > serveRecentLimit {
		d.recent = d.recent[:serveRecentLimit]
	}
}

// Only fixed, application-owned text crosses this boundary. Backend problems,
// payloads, names, serials, account IDs and paths must never reach the display.
func (d *serveDisplay) observe(event monitor.Event) {
	switch {
	case event.StopReason == monitor.StopAuthenticationRejected:
		d.connection = "Authentication rejected"
		d.note("Session rejected; run bambu-tui login and restart serve")
	case event.Kind == monitor.EventConnection:
		connection := "Reconnecting"
		if event.Connected {
			connection = "Connected"
		}
		if d.connection != connection {
			d.connection = connection
			d.note("Monitoring: " + connection)
		}
	case event.Kind == monitor.EventProblem:
		d.note("Monitor warning; monitoring continues")
	}
}

func (d *serveDisplay) persistence(state *session.Session) {
	notice, snooze := state.NoticeStorageError != "", state.AlertStorageError != ""
	if notice && !d.noticeWarning {
		d.note("Notice storage unavailable; decisions may be session-only; inspect browser status")
	}
	if snooze && !d.snoozeWarning {
		d.note("Snooze storage unavailable; changes may be session-only; repair storage and restart")
	}
	d.noticeWarning, d.snoozeWarning = notice, snooze
}

func (d *serveDisplay) plain() string {
	lines := []string{"Server: " + d.server + " | Mode: " + d.mode + " | Monitoring: " + d.connection}
	lines = append(lines, d.warnings()...)
	if len(d.recent) > 0 {
		lines = append(lines, "Recent: "+d.recent[0])
	}
	return strings.Join(lines, "\n")
}

func (d *serveDisplay) warnings() []string {
	var lines []string
	if d.noticeWarning {
		lines = append(lines, "Notice storage: unavailable / changes may be session-only")
	}
	if d.snoozeWarning {
		lines = append(lines, "Snooze storage: unavailable / changes may be session-only")
	}
	return lines
}

func (d *serveDisplay) view(width, height int, now time.Time) string {
	// Prioritize the complete URL and exit guidance when space is scarce. Wrap
	// rather than truncate the URL, including on terminals narrower than it.
	wrap := func(s string) []string { return strings.Split(ansi.Wrap(s, max(1, width), ""), "\n") }
	lines := wrap(d.url)
	lines = append(lines, wrap("Bambu TUI / serve")...)
	lines = append(lines, wrap("Mode: "+d.mode)...)
	lines = append(lines, wrap("Server: "+d.server)...)
	lines = append(lines, wrap("Monitoring: "+d.connection)...)
	lines = append(lines, wrap("Uptime: "+now.Sub(d.started).Truncate(time.Second).String())...)
	lines = append(lines, wrap("Native desktop notifications: disabled")...)
	for _, warning := range d.warnings() {
		lines = append(lines, wrap(warning)...)
	}
	if len(d.recent) == 0 {
		lines = append(lines, "Recent: none")
	}
	for _, message := range d.recent {
		lines = append(lines, wrap("Recent: "+message)...)
	}
	hint := wrap("Ctrl+C to stop")
	// Reserve shutdown guidance before optional status rows. On very small
	// screens the URL has priority and continues across physical rows.
	available := max(1, height-len(hint))
	if len(lines) > available {
		lines = lines[:available]
	}
	lines = append(lines, hint...)
	if len(lines) > max(1, height) {
		lines = lines[:max(1, height)]
	}
	return strings.Join(lines, "\n")
}

func (d *serveDisplay) render(now time.Time, force bool) error {
	if !force && now.Sub(d.rendered) < serveStatusInterval {
		return nil
	}
	d.rendered = now
	if d.size != nil {
		width, height, err := d.size()
		if err != nil {
			return fmt.Errorf("read serve terminal size: %w", err)
		}
		_, err = io.WriteString(d.output, "\x1b[H\x1b[2J"+strings.ReplaceAll(d.view(width, height, now), "\n", "\r\n"))
		return err
	}
	plain := d.plain()
	if !force && plain == d.lastPlain {
		return nil
	}
	d.lastPlain = plain
	_, err := fmt.Fprintln(d.output, plain)
	if force && d.server == "Running" {
		_, hintErr := fmt.Fprintln(d.output, "Ctrl+C to stop")
		if err == nil {
			err = hintErr
		}
	}
	return err
}

func (d *serveDisplay) close() error {
	if !d.active {
		return nil
	}
	d.active = false
	_, err := io.WriteString(d.output, "\x1b[?25h\x1b[?1049l")
	return err
}

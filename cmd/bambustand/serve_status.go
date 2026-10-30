package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/session"
	"github.com/cliffom/bambustand/internal/ui"
	"github.com/cliffom/bambustand/internal/webserver"
	"github.com/muesli/termenv"
)

const (
	serveClientRows     = 6
	serveRecentLimit    = 3
	serveStatusInterval = time.Second
)

// The serve loop owns this presenter. It never reads terminal input or changes
// terminal modes; canonical Ctrl+C remains owned by the CLI signal context.
type (
	serveTone   uint8
	serveNotice struct {
		text string
		tone serveTone
	}
	serveStyles  struct{ accent, text, secondary, success, warning, failure lipgloss.Style }
	serveDisplay struct {
		output                        io.Writer
		size                          func() (int, int, error)
		started, rendered             time.Time
		url, mode, connection, server string
		connectionTone                serveTone
		recent                        []serveNotice
		lastPlain                     string
		noticeWarning, snoozeWarning  bool
		active                        bool
		styles                        *serveStyles
		clients                       webserver.ClientStatus
		clientSequence                uint64
	}
)

const (
	serveInfo serveTone = iota
	serveSuccess
	serveWarning
	serveError
)

func newServeStyles(renderer *lipgloss.Renderer) *serveStyles {
	p := ui.DefaultPalette()
	style := func(color lipgloss.Color) lipgloss.Style { return renderer.NewStyle().Foreground(color) }
	return &serveStyles{accent: style(p.Accent).Bold(true), text: style(p.Text), secondary: style(p.SecondaryText), success: style(p.Success).Bold(true), warning: style(p.Warning).Bold(true), failure: style(p.Error).Bold(true)}
}

func (d *serveDisplay) styled(text string, tone serveTone) string {
	if d.styles == nil {
		return text
	}
	style := d.styles.text
	switch tone {
	case serveSuccess:
		style = d.styles.success
	case serveWarning:
		style = d.styles.warning
	case serveError:
		style = d.styles.failure
	}
	return style.Render(text)
}

func newServeDisplay(output io.Writer, url string, demo bool) *serveDisplay {
	d := &serveDisplay{output: output, url: url, mode: "cloud", connection: "Connecting", server: "Running", started: time.Now(), connectionTone: serveWarning}
	if demo {
		d.mode = "demo (synthetic)"
	}
	if file, ok := output.(*os.File); ok && term.IsTerminal(file.Fd()) && term.IsTerminal(os.Stdin.Fd()) && os.Getenv("TERM") != "dumb" {
		renderer := lipgloss.NewRenderer(output)
		if os.Getenv("NO_COLOR") != "" {
			renderer.SetColorProfile(termenv.Ascii)
		}
		d.styles = newServeStyles(renderer)
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

func (d *serveDisplay) note(message string, tone serveTone) {
	if len(d.recent) > 0 && d.recent[0].text == message {
		return
	}
	d.recent = append([]serveNotice{{text: message, tone: tone}}, d.recent...)
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
		d.connectionTone = serveError
		d.note("Session rejected; run bambustand login and restart serve", serveError)
	case event.Kind == monitor.EventConnection:
		connection := "Reconnecting"
		d.connectionTone = serveWarning
		if event.Connected {
			d.connectionTone = serveSuccess
			connection = "Connected"
		}
		if d.connection != connection {
			d.connection = connection
			d.note("Monitoring: "+connection, d.connectionTone)
		}
	case event.Kind == monitor.EventProblem:
		d.note("Monitor warning; monitoring continues", serveWarning)
	}
}

func (d *serveDisplay) persistence(state *session.Session) {
	notice, snooze := state.NoticeStorageError != "", state.AlertStorageError != ""
	if notice && !d.noticeWarning {
		d.note("Notice storage unavailable; decisions may be session-only; inspect browser status", serveError)
	}
	if snooze && !d.snoozeWarning {
		d.note("Snooze storage unavailable; changes may be session-only; repair storage and restart", serveError)
	}
	d.noticeWarning, d.snoozeWarning = notice, snooze
}

func (d *serveDisplay) plain() string {
	lines := []string{"Server: " + d.server + " | Mode: " + d.mode + " | Monitoring: " + d.connection}
	lines = append(lines, d.warnings()...)
	lines = append(lines, fmt.Sprintf("HTTP clients: %d active", d.clients.Active))
	if d.clients.Omitted > 0 {
		lines = append(lines, fmt.Sprintf("%d client details omitted (tracking limit)", d.clients.Omitted))
	}
	if len(d.recent) > 0 {
		lines = append(lines, "Recent: "+d.recent[0].text)
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
	url, title := d.url, "BambuStand / serve"
	if d.styles != nil {
		url = d.styles.accent.Underline(true).Render(url)
		title = d.styles.accent.Render(title)
	}
	lines := wrap(url)
	lines = append(lines, wrap(title)...)
	lines = append(lines, wrap(d.secondary("Mode: "+d.mode))...)
	lines = append(lines, wrap(d.styled("Server: "+d.server, d.serverTone()))...)
	lines = append(lines, wrap(d.styled("Monitoring: "+d.connection, d.connectionTone))...)
	lines = append(lines, wrap(d.secondary("Uptime: "+now.Sub(d.started).Truncate(time.Second).String()))...)
	lines = append(lines, wrap(d.secondary("Native desktop notifications: disabled"))...)
	for _, warning := range d.warnings() {
		lines = append(lines, wrap(d.styled(warning, serveError))...)
	}
	if len(d.recent) == 0 {
		lines = append(lines, "Recent: none")
	}
	for _, message := range d.recent {
		lines = append(lines, wrap(d.styled("Recent: "+message.text, message.tone))...)
	}
	clientTitle := fmt.Sprintf("HTTP clients: %d active", d.clients.Active)
	if d.styles != nil {
		clientTitle = d.styles.accent.Render(clientTitle)
	}
	lines = append(lines, wrap(clientTitle)...)
	if d.clients.Omitted > 0 {
		lines = append(lines, wrap(d.styled(fmt.Sprintf("%d client details omitted (tracking limit)", d.clients.Omitted), serveWarning))...)
	}
	for i := len(d.clients.Events) - 1; i >= max(0, len(d.clients.Events)-serveClientRows); i-- {
		event := d.clients.Events[i]
		tone := serveInfo
		if event.Connected {
			tone = serveSuccess
		}
		lines = append(lines, d.styled(ansi.Truncate(serveClientLine(event), max(1, width), "…"), tone))
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
	if err := d.logClients(); err != nil {
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

func (d *serveDisplay) serverTone() serveTone {
	switch d.server {
	case "Running":
		return serveSuccess
	case "Failed":
		return serveError
	case "Stopping":
		return serveWarning
	default:
		return serveInfo
	}
}

func serveClientLine(event webserver.ClientEvent) string {
	return event.DisplayLine()
}

// Redirected output uses sequence cursors so polling never reprints the ledger.
func (d *serveDisplay) logClients() error {
	for i, event := range d.clients.Events {
		if event.Sequence <= d.clientSequence {
			continue
		}
		if i == 0 && event.Sequence > d.clientSequence+1 {
			if _, err := fmt.Fprintf(d.output, "HTTP ledger: %d earlier events no longer retained\n", event.Sequence-d.clientSequence-1); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(d.output, "HTTP "+serveClientLine(event)); err != nil {
			return err
		}
		d.clientSequence = event.Sequence
	}
	return nil
}

func (d *serveDisplay) secondary(text string) string {
	if d.styles == nil {
		return text
	}
	return d.styles.secondary.Render(text)
}

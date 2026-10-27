//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/ui"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

const ptyStepLimit = 5 * time.Second

// TestPTYHelperProcess is invoked only in a dedicated child test process. The
// normal path calls the real CLI in demo mode. The failure path wraps the real
// UI with a test-only panic trigger; production has no fault-injection switch.
func TestPTYHelperProcess(t *testing.T) {
	mode := os.Getenv("BAMBU_TUI_PTY_CHILD")
	if mode == "" {
		return
	}
	var err error
	switch mode {
	case "demo":
		args := []string{"--demo", "--config", os.Getenv("BAMBU_TUI_PTY_CONFIG")}
		if theme := os.Getenv("BAMBU_TUI_PTY_THEME"); theme != "" {
			args = append(args, "--theme", theme)
		}
		err = run(args)
	case "panic":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		events := make(chan monitor.Event, 3)
		now := time.Now()
		events <- monitor.Event{Kind: monitor.EventDevices, At: now, Devices: []farm.Device{{ID: "PTY", Name: "PTY fixture", Model: "H2D", Online: true}}}
		events <- monitor.Event{Kind: monitor.EventConnection, At: now, Connected: true}
		events <- monitor.Event{Kind: monitor.EventReport, At: now, ID: "PTY", Payload: []byte(`{"print":{"gcode_state":"RUNNING","mc_percent":42}}`)}
		done := make(chan struct{})
		go func() { <-ctx.Done(); close(events); close(done) }()
		model := ptyPanicModel{Model: ui.New(farm.New(10), events, make(chan struct{}, 1), true)}
		_, err = runTerminalProgram(ctx, model, tea.WithAltScreen())
		err = finishDashboard(ctx, cancel, done, err)
	default:
		t.Fatalf("unknown PTY helper mode %q", mode)
	}
	code := 0
	if err != nil {
		fmt.Fprintln(os.Stderr, "bambu-tui:", err)
		code = 1
	}
	// Darwin revokes the controlling slave when its session leader exits.
	// Separate control pipes let the parent inspect restored settings after
	// Program.Run returns, without competing with the terminal input reader.
	ready, release := os.NewFile(3, "pty-ready"), os.NewFile(4, "pty-release")
	if _, err := ready.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	var ack [1]byte
	if _, err := io.ReadFull(release, ack[:]); err != nil {
		t.Fatal(err)
	}
	os.Exit(code)
}

type (
	ptyPanicModel struct{ ui.Model }

	ptyChild struct {
		configPath                  string
		release                     *os.File
		returned                    chan struct{}
		returnErr                   error
		writeMu                     sync.Mutex
		cmd                         *exec.Cmd
		master, slave               *os.File
		before, after               *term.State
		stateErr, waitErr           error
		mu                          sync.Mutex
		output                      []byte
		readErr                     error
		changed, readDone, waitDone chan struct{}
	}
)

func (m ptyPanicModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+g" {
		panic("PTY smoke injected failure")
	}
	next, cmd := m.Model.Update(msg)
	m.Model = next.(ui.Model)
	return m, cmd
}

func startPTYChild(t *testing.T, mode string) *ptyChild {
	t.Helper()
	return startPTYChildWithConfig(t, mode, "")
}

func startPTYChildWithConfig(t *testing.T, mode, configPath string) *ptyChild {
	t.Helper()
	return startPTYChildWithTheme(t, mode, configPath, "")
}

func startPTYChildWithTheme(t *testing.T, mode, configPath, theme string) *ptyChild {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	// A nonblocking descriptor registered with Go's poller lets Close and write
	// deadlines interrupt I/O during failure cleanup on both Darwin and Linux.
	fd, err := unix.Dup(int(master.Fd()))
	if err != nil {
		_ = master.Close()
		_ = slave.Close()
		t.Fatal(err)
	}
	unix.CloseOnExec(fd)
	if err = unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		_ = master.Close()
		_ = slave.Close()
		t.Fatal(err)
	}
	_ = master.Close()
	master = os.NewFile(uintptr(fd), "pty-master")
	child := &ptyChild{master: master, slave: slave, changed: make(chan struct{}, 1), readDone: make(chan struct{}), waitDone: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	started := false
	var control []*os.File
	t.Cleanup(func() {
		cancel()
		if started {
			select {
			case <-child.waitDone:
			default:
				_ = unix.Kill(-child.cmd.Process.Pid, unix.SIGKILL)
			}
			select {
			case <-child.waitDone:
			case <-time.After(ptyStepLimit):
				t.Error("PTY child was not reaped")
			}
		}
		_ = slave.Close()
		_ = master.Close()
		for _, file := range control {
			_ = file.Close()
		}
		if started {
			select {
			case <-child.returned:
			case <-time.After(ptyStepLimit):
				t.Error("PTY control reader did not exit")
			}
			select {
			case <-child.readDone:
			case <-time.After(ptyStepLimit):
				t.Error("PTY reader did not exit")
			}
		}
	})
	child.before, err = term.GetState(slave.Fd())
	if err != nil {
		t.Fatal(err)
	}
	if err = pty.Setsize(master, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatal(err)
	}
	newConfig := configPath == ""
	if newConfig {
		configPath = filepath.Join(t.TempDir(), "config.toml")
	}
	child.configPath = configPath
	// The real demo reports on a timer. Disable automatic interruptions so test
	// semantics do not depend on whether a slow CI host reaches the runout step.
	if newConfig {
		if err = os.WriteFile(configPath, []byte("[alerts]\nautomatic_modals = false\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child.cmd = exec.CommandContext(ctx, executable, "-test.run=^TestPTYHelperProcess$", "-test.count=1")
	child.cmd.Env = append(os.Environ(), "TERM=xterm-256color", "NO_COLOR=1", "CLICOLOR_FORCE=0", "BAMBU_TUI_PTY_CHILD="+mode, "BAMBU_TUI_PTY_CONFIG="+configPath, "BAMBU_TUI_PTY_THEME="+theme)
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	control = append(control, readyRead, readyWrite)
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	control = append(control, releaseRead, releaseWrite)
	child.release, child.returned = releaseWrite, make(chan struct{})
	child.cmd.ExtraFiles = []*os.File{readyWrite, releaseRead}
	child.cmd.Stdin, child.cmd.Stdout, child.cmd.Stderr = slave, slave, slave
	child.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err = child.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	started = true
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	go func() {
		var ready [1]byte
		_, child.returnErr = io.ReadFull(readyRead, ready[:])
		close(child.returned)
	}()
	go func() {
		child.waitErr = child.cmd.Wait()
		close(child.waitDone)
	}()
	go func() {
		defer close(child.readDone)
		buffer := make([]byte, 4096)
		cursorReplies := 0
		for {
			n, err := master.Read(buffer)
			child.mu.Lock()
			if len(child.output)+n > 1024*1024 {
				err = errors.New("PTY transcript exceeded 1 MiB")
			} else {
				child.output = append(child.output, buffer[:n]...)
			}
			// PTYs transport bytes but do not answer terminal queries. Reply
			// to cursor-position probes so color detection can fall back from
			// unsupported OSC queries without waiting for a wall-clock timeout.
			queries := bytes.Count(child.output, []byte("\x1b[6n"))
			reply := strings.Repeat("\x1b[1;1R", queries-cursorReplies)
			cursorReplies = queries
			child.mu.Unlock()
			if reply != "" && err == nil {
				err = child.write([]byte(reply))
			}
			if err != nil {
				child.mu.Lock()
				child.readErr = err
				child.mu.Unlock()
			}
			select {
			case child.changed <- struct{}{}:
			default:
			}
			if err != nil {
				return
			}
		}
	}()
	child.expect(t, 0, "/ DEMO CONNECTED / FARM")
	state, err := term.GetState(slave.Fd())
	if err != nil || reflect.DeepEqual(child.before, state) {
		t.Fatalf("child did not enter raw terminal mode: %v", err)
	}
	return child
}

func (p *ptyChild) transcript() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return bytes.Clone(p.output), p.readErr
}

func (p *ptyChild) mark() int { output, _ := p.transcript(); return len(output) }

func (p *ptyChild) expect(t *testing.T, after int, text string) {
	t.Helper()
	deadline := time.NewTimer(ptyStepLimit)
	defer deadline.Stop()
	for {
		output, err := p.transcript()
		if strings.Contains(ansi.Strip(string(output[after:])), text) {
			return
		}
		if err != nil {
			t.Fatalf("PTY stream ended before %q: %v\n%s", text, err, ansi.Strip(string(output)))
		}
		select {
		case <-p.changed:
		case <-deadline.C:
			t.Fatalf("timed out waiting for %q\nraw=%q", text, output)
		}
	}
}

func (p *ptyChild) write(data []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := p.master.SetWriteDeadline(time.Now().Add(ptyStepLimit)); err != nil {
		return err
	}
	_, err := p.master.Write(data)
	return err
}

func (p *ptyChild) send(t *testing.T, keys string) int {
	t.Helper()
	after := p.mark()
	if err := p.write([]byte(keys)); err != nil {
		t.Fatal(err)
	}
	return after
}

func (p *ptyChild) resize(t *testing.T, width, height uint16) int {
	t.Helper()
	after := p.mark()
	if err := pty.Setsize(p.master, &pty.Winsize{Rows: height, Cols: width}); err != nil {
		t.Fatal(err)
	}
	if err := p.cmd.Process.Signal(syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	return after
}

func (p *ptyChild) exited(t *testing.T, code int) {
	t.Helper()
	select {
	case <-p.returned:
	case <-time.After(ptyStepLimit):
		t.Fatal("Bubble Tea did not return within five seconds")
	}
	if p.returnErr != nil {
		output, _ := p.transcript()
		t.Fatalf("child exited before restoration handshake: %v\n%s", p.returnErr, ansi.Strip(string(output)))
	}
	p.after, p.stateErr = term.GetState(p.slave.Fd())
	if p.stateErr != nil || !reflect.DeepEqual(p.before, p.after) {
		t.Fatalf("program did not restore terminal settings: %v", p.stateErr)
	}
	if _, err := p.release.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.waitDone:
	case <-time.After(ptyStepLimit):
		t.Fatal("PTY child did not exit within five seconds")
	}
	got := 0
	if p.waitErr != nil {
		var exit *exec.ExitError
		if !errors.As(p.waitErr, &exit) {
			t.Fatal(p.waitErr)
		}
		got = exit.ExitCode()
	}
	if got != code {
		output, _ := p.transcript()
		t.Fatalf("child exit %d, want %d\n%s", got, code, ansi.Strip(string(output)))
	}
	_ = p.slave.Close()
	select {
	case <-p.readDone:
	case <-time.After(ptyStepLimit):
		t.Fatal("PTY output did not drain after exit")
	}
	output, _ := p.transcript()
	for _, pair := range [][2]string{
		{"\x1b[?1049h", "\x1b[?1049l"}, // alternate screen
		{"\x1b[?25l", "\x1b[?25h"},     // cursor visibility
		{"\x1b[?2004h", "\x1b[?2004l"}, // bracketed paste
	} {
		start, end := []byte(pair[0]), []byte(pair[1])
		if !bytes.Contains(output, start) || !bytes.Contains(output, end) {
			t.Errorf("terminal lifecycle missing pair %q", pair)
		}
		if bytes.LastIndex(output, end) < bytes.LastIndex(output, start) {
			t.Errorf("terminal mode was not restored: %q", pair)
		}
	}
}

func TestPTYNavigationResizeAndNormalQuit(t *testing.T) {
	child := startPTYChild(t, "demo")
	child.expect(t, 0, "Gridfinity baseplate")
	mark := child.send(t, "j")
	child.expect(t, mark, "Printer Name:    X2D")
	mark = child.send(t, "m")
	child.expect(t, mark, "AMS / FILAMENT")
	// A subsequent visible unit change proves each quit key stayed inside the
	// dialog, including a raw Ctrl+C byte that must not become an OS signal.
	mark = child.send(t, "q\x1b[C")
	child.expect(t, mark, "Unit 2 / 4")
	mark = child.send(t, "\x03\x1b[C")
	child.expect(t, mark, "Unit 3 / 4")
	mark = child.send(t, "\x05") // Ctrl+E suspends the browser without closing it.
	child.expect(t, mark, "STATUS / FEEDBACK")
	mark = child.send(t, "q\x03\x05")
	child.expect(t, mark, "Unit 3 / 4")
	mark = child.resize(t, 40, 10)
	child.expect(t, mark, "Resize to at least 60 columns")
	mark = child.resize(t, 120, 40)
	child.expect(t, mark, "Unit 3 / 4")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
}

func TestPTYHelpScrollsAtMinimumSizeAndReturnsFromStatus(t *testing.T) {
	child := startPTYChild(t, "demo")
	child.expect(t, 0, "Gridfinity baseplate")
	mark := child.resize(t, 60, 18)
	child.expect(t, mark, "q quit")
	mark = child.send(t, "?")
	child.expect(t, mark, "KEYBOARD")
	child.expect(t, mark, "?/Esc close")
	mark = child.send(t, "q\x03\x1b[F") // Quit stays captured; End scrolls help.
	child.expect(t, mark, "Quit from dashboard")
	mark = child.send(t, "\x05")
	child.expect(t, mark, "STATUS / FEEDBACK")
	mark = child.resize(t, 59, 17)
	child.expect(t, mark, "Resize to at least 60 columns")
	child.send(t, "\x05y") // Neither status toggle nor confirmation runs hidden.
	mark = child.resize(t, 60, 18)
	child.expect(t, mark, "STATUS / FEEDBACK")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "Quit from dashboard")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
}

func TestPTYConfigEditsPersistToLaunchFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(filepath.Join(dir, "custom.json"), []byte(`{"version":1,"colors":{"background":"#282828","text":"#ebdbb2"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("theme_file='custom.json'\n[alerts]\nautomatic_modals=false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := startPTYChildWithConfig(t, "demo", configPath)
	child.expect(t, 0, "Gridfinity baseplate")
	mark := child.send(t, ",")
	child.expect(t, mark, "CONFIG / GENERAL")
	mark = child.send(t, "\r")
	child.expect(t, mark, "restart + login required")
	child.send(t, "\x1b[B\r")
	mark = child.send(t, "0.5\r")
	child.expect(t, mark, "Enter a whole percentage from 0 to 100.")
	mark = child.send(t, "\x1525\r") // Ctrl+U clears before replacement.
	child.expect(t, mark, "Low filament threshold: 25%")
	mark = child.send(t, "\t")
	child.expect(t, mark, "CONFIG / ALERTS")
	child.send(t, "\x1b[B\x1b[B\x1b[B\x1b[B\r") // Global filament Off.
	mark = child.send(t, "\t")
	child.expect(t, mark, "CONFIG / PRINTERS")
	mark = child.send(t, "\x1b[B\x1b[B\x1b[B\x1b[B\x1b[B\r") // Printer filament On.
	child.expect(t, mark, "On (effective: On)")
	mark = child.resize(t, 59, 17)
	child.expect(t, mark, "Resize to at least 60 columns")
	mark = child.resize(t, 60, 18)
	child.expect(t, mark, "On (effective: On)")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
	cfg, err := config.Load(child.configPath)
	if err != nil {
		t.Fatal(err)
	}
	override := cfg.Alerts.Printers["DEMOH2D"].Filament
	if cfg.ThemeFile != "custom.json" || cfg.Region != config.RegionChina || cfg.LowFilamentPercent != 25 || cfg.Alerts.Filament || override == nil || !*override || cfg.Alerts.DesktopNotifications {
		t.Fatalf("CLI did not persist all edits to selected file: %+v", cfg)
	}
}

func TestPTYThemeOverridePreservesConfiguredThemeThroughSave(t *testing.T) {
	for _, theme := range []string{"gruvbox-dark", "default", "hotdog", "c64", "ibm3270"} {
		t.Run(theme, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("theme_file='missing.json'\n[alerts]\nautomatic_modals=false\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			child := startPTYChildWithTheme(t, "demo", path, theme)
			child.expect(t, 0, "Gridfinity baseplate")
			mark := child.send(t, "F")
			child.expect(t, mark, "No printers match.")
			child.send(t, "q")
			child.exited(t, 0)
			cfg, err := config.Load(path)
			if err != nil || cfg.ThemeFile != "missing.json" || cfg.View.Filter != config.FilterAttention {
				t.Fatal("launch override changed saved theme", cfg, err)
			}
		})
	}
}

func TestPTYNotificationActivityKeepsFocusThroughResize(t *testing.T) {
	child := startPTYChild(t, "demo")
	child.expect(t, 0, "Gridfinity baseplate")
	mark := child.send(t, "N")
	child.expect(t, mark, "NOTIFICATION ACTIVITY")
	child.expect(t, mark, "No live notification decisions yet.")
	child.send(t, "q") // Quit remains dashboard-only.
	mark = child.resize(t, 59, 17)
	child.expect(t, mark, "Resize to at least 60 columns")
	mark = child.resize(t, 60, 18)
	child.expect(t, mark, "NOTIFICATION ACTIVITY")
	mark = child.send(t, "\x03N") // Raw Ctrl+C stays inside; N closes.
	child.expect(t, mark, "PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
}

func TestPTYClearAllNoticesRemainsResponsive(t *testing.T) {
	child := startPTYChild(t, "demo")
	child.expect(t, 0, "Gridfinity baseplate")
	child.send(t, "\t")
	mark := child.send(t, "\t")
	child.expect(t, mark, "X clear all")
	mark = child.send(t, "X")
	child.expect(t, mark, "CLEAR ALL NOTICES?")
	mark = child.send(t, "y")
	child.expect(t, mark, "Confirmed notices cleared locally")
	// Input and rendering must continue after confirmation empties the panel.
	mark = child.send(t, "j")
	child.expect(t, mark, "Printer Name:    X2D")
	// Re-entering the empty panel and clearing again must also restore navigation.
	child.send(t, "\t")
	child.send(t, "\t")
	mark = child.send(t, "X")
	child.expect(t, mark, "No notices or recent events to clear")
	mark = child.send(t, "k")
	child.expect(t, mark, "Printer Name:    H2D")
	mark = child.send(t, "?")
	child.expect(t, mark, "KEYBOARD")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	mark = child.send(t, "m")
	child.expect(t, mark, "AMS / FILAMENT")
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
}

func TestPTYInterruptedExitRestoresTerminal(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			child := startPTYChild(t, "demo")
			if err := child.cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			child.exited(t, 0)
		})
	}
}

func TestPTYRecoveredPanicRestoresTerminalAndFails(t *testing.T) {
	child := startPTYChild(t, "panic")
	child.send(t, "\x07")
	child.exited(t, 1)
	child.expect(t, 0, "program experienced a panic")
}

func TestPTYFarmViewRestartAndReset(t *testing.T) {
	child := startPTYChild(t, "demo")
	child.expect(t, 0, "Gridfinity baseplate")
	mark := child.send(t, "F")
	child.expect(t, mark, "Filter: Attention")
	child.expect(t, mark, "No printers match")
	mark = child.send(t, "S")
	child.expect(t, mark, "Sort: Attention")
	child.send(t, "q")
	child.exited(t, 0)
	cfg, err := config.Load(child.configPath)
	if err != nil || cfg.View != (config.ViewPreferences{Filter: config.FilterAttention, Sort: config.SortAttention}) {
		t.Fatal(cfg, err)
	}
	restarted := startPTYChildWithConfig(t, "demo", child.configPath)
	restarted.expect(t, 0, "No printers match")
	restarted.expect(t, 0, "Sort: Attention")
	mark = restarted.resize(t, 60, 18)
	restarted.expect(t, mark, "0 resets All / Name")
	mark = restarted.send(t, "0")
	restarted.expect(t, mark, "Sort: Name")
	restarted.expect(t, mark, "Printer Name:    H2D")
	restarted.send(t, "q")
	restarted.exited(t, 0)
	cfg, err = config.Load(child.configPath)
	if err != nil || cfg.View != (config.ViewPreferences{Filter: config.FilterAll, Sort: config.SortName}) {
		t.Fatal(cfg, err)
	}
}

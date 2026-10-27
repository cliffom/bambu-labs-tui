//go:build darwin || linux

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPTYServeDemoResizesAndCtrlC(t *testing.T) {
	child := startPTYChildWithConfig(t, "serve-demo", ptyWebConfig(t, false))
	address := ptyWebURL(t, child, 0)
	child.expect(t, 0, "demo (synthetic)")
	child.expect(t, 0, "Monitoring: Connected")
	for _, size := range [][2]uint16{{80, 24}, {60, 18}, {40, 12}, {20, 10}, {120, 40}} {
		mark := child.resize(t, size[0], size[1])
		if size[0] < uint16(len(address)) {
			child.expect(t, mark, "http://127.0.0.1:")
		} else {
			child.expect(t, mark, address)
		}
		child.expect(t, mark, "Ctrl+C to stop")
	}
	mark := child.send(t, "\x03")
	child.expect(t, mark, "Server: Stopping")
	child.expect(t, mark, "Serve stopped; dashboard closed.")
	child.exited(t, 0)
	assertWebClosed(t, address)
}

func TestPTYServeCloudReconnectAndAuthenticationFailure(t *testing.T) {
	child := startPTYChildWithConfig(t, "serve-fixture", ptyWebConfig(t, false))
	address := ptyWebURL(t, child, 0)
	child.expect(t, 0, "Mode: cloud")
	child.expect(t, 0, "Monitoring: Connected")
	for _, step := range []struct {
		signal syscall.Signal
		text   string
	}{{syscall.SIGUSR1, "Monitoring: Reconnecting"}, {syscall.SIGUSR2, "Monitoring: Connected"}, {syscall.SIGHUP, "Authentication rejected"}} {
		output, _ := child.transcript()
		mark := len(output)
		if err := child.cmd.Process.Signal(step.signal); err != nil {
			t.Fatal(err)
		}
		child.expect(t, mark, step.text)
		frame, _ := child.transcript()
		if start := strings.LastIndex(string(frame), "\x1b[H\x1b[2J"); start >= 0 {
			t.Logf("%s frame:\n%s", step.text, ansi.Strip(string(frame[start:])))
		}
	}
	child.expect(t, 0, "run bambu-tui login")
	child.exited(t, 1)
	assertWebClosed(t, address)
	output, _ := child.transcript()
	if strings.Contains(string(output), "private-") {
		t.Fatal("private monitor diagnostic leaked")
	}
}

func TestPTYServeOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, fmt.Appendf(nil, "[web_server]\nport=%d\n", listener.Addr().(*net.TCPAddr).Port), 0o600); err != nil {
		t.Fatal(err)
	}
	child := startPTYChildWithConfig(t, "serve-occupied", path)
	child.expect(t, 0, "already in use")
	child.exited(t, 1)
	output, _ := child.transcript()
	if strings.Contains(string(output), "\x1b[?1049h") {
		t.Fatal("entered screen for failed listener")
	}
}

func TestPTYServeDemoStopsOnSIGTERM(t *testing.T) {
	child := startPTYChildWithConfig(t, "serve-demo", ptyWebConfig(t, false))
	address := ptyWebURL(t, child, 0)
	if err := child.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	child.expect(t, 0, "Serve stopped; dashboard closed.")
	child.exited(t, 0)
	assertWebClosed(t, address)
}

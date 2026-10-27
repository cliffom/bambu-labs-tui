//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/config"
)

func ptyWebURL(t *testing.T, child *ptyChild, after int) string {
	t.Helper()
	pattern := regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+[ \r\n]`)
	deadline := time.NewTimer(ptyStepLimit)
	defer deadline.Stop()
	for {
		output, _ := child.transcript()
		if match := pattern.FindString(ansi.Strip(string(output[after:]))); match != "" {
			return strings.TrimSpace(match)
		}
		select {
		case <-child.changed:
		case <-deadline.C:
			t.Fatal("running localhost URL not rendered")
		}
	}
}

func assertWebClosed(t *testing.T, address string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(address, "http://"), time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("web listener survived disable/exit")
	}
}

func TestPTYWebSettingsAndShutdown(t *testing.T) {
	path := ptyWebConfig(t, false)
	child := startPTYChildWithConfig(t, "demo", path)
	child.expect(t, 0, "Gridfinity baseplate")
	child.send(t, ",")
	mark := child.send(t, "\t\t\t")
	child.expect(t, mark, "CONFIG / WEB SERVER")
	mark = child.send(t, "\r")
	address := ptyWebURL(t, child, mark)
	client := &http.Client{Timeout: ptyStepLimit}
	response, err := client.Get(address + "/api/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || !json.Valid(body) || !strings.Contains(string(body), `"demo":true`) || !strings.Contains(string(body), "Gridfinity baseplate") {
		t.Fatal("real CLI did not publish demo telemetry", string(body), err)
	}
	mark = child.resize(t, 59, 17)
	child.expect(t, mark, "Resize to at least 60 columns")
	mark = child.resize(t, 60, 18)
	child.expect(t, mark, address)
	mark = child.send(t, "\r")
	child.expect(t, mark, "Status: Stopped")
	assertWebClosed(t, address)
	mark = child.send(t, "\r")
	address = ptyWebURL(t, child, mark)
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
	assertWebClosed(t, address)
	cfg, err := config.Load(child.configPath)
	if err != nil || !cfg.WebServer.Enabled {
		t.Fatal("enabled preference not saved", err)
	}
}

func TestPTYWebSavedStartupStopsOnSignal(t *testing.T) {
	path := ptyWebConfig(t, true)
	child := startPTYChildWithConfig(t, "demo", path)
	child.expect(t, 0, "Gridfinity baseplate")
	child.send(t, ",")
	mark := child.send(t, "\t\t\t")
	address := ptyWebURL(t, child, mark)
	if err := child.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	child.exited(t, 0)
	assertWebClosed(t, address)
}

func ptyWebConfig(t *testing.T, enabled bool) string {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	body := fmt.Sprintf("[web_server]\nenabled=%t\nport=%d\n[alerts]\nautomatic_modals=false\n", enabled, port)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPTYWebOccupiedPortShowsErrorAndRetriesExplicitly(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port
	path := filepath.Join(t.TempDir(), "config.toml")
	body := fmt.Sprintf("[web_server]\nenabled=true\nport=%d\n[alerts]\nautomatic_modals=false\n", port)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	child := startPTYChildWithConfig(t, "demo", path)
	child.expect(t, 0, "Gridfinity baseplate")
	child.send(t, ",")
	mark := child.send(t, "\t\t\t")
	child.expect(t, mark, "Status: Failed")
	child.expect(t, mark, fmt.Sprintf("Port %d is already in use", port))
	if err := occupied.Close(); err != nil {
		t.Fatal(err)
	}
	mark = child.send(t, "r")
	address := ptyWebURL(t, child, mark)
	if address != fmt.Sprintf("http://127.0.0.1:%d", port) {
		t.Fatal("retry changed configured port", address)
	}
	mark = child.send(t, "\x1b")
	child.expect(t, mark, "› PRINTERS")
	child.send(t, "q")
	child.exited(t, 0)
	assertWebClosed(t, address)
}

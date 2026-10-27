//go:build darwin || linux

package main

import (
	"encoding/json"
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
	child := startPTYChild(t, "demo")
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
	child.send(t, "\x1b")
	child.send(t, "q")
	child.exited(t, 0)
	assertWebClosed(t, address)
	cfg, err := config.Load(child.configPath)
	if err != nil || !cfg.WebServer.Enabled {
		t.Fatal("enabled preference not saved", err)
	}
}

func TestPTYWebSavedStartupStopsOnSignal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[web_server]\nenabled=true\n[alerts]\nautomatic_modals=false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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

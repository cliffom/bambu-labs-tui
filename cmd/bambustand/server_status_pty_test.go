//go:build darwin || linux

package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestPTYServerStatusModalProfilesClientsAndReturn(t *testing.T) {
	for _, mode := range []string{"demo", "demo-color", "demo-ansi256"} {
		t.Run(mode, func(t *testing.T) {
			child := startPTYChildWithConfig(t, mode, ptyWebConfig(t, true))
			child.expect(t, 0, "Gridfinity baseplate")
			mark := child.send(t, "W")
			child.expect(t, mark, "SERVER STATUS")
			address := ptyWebURL(t, child, mark)
			mark = child.resize(t, 120, 32)
			child.expect(t, mark, "Session uptime:")
			child.expect(t, mark, "Listener uptime:")
			transport := &http.Transport{}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: time.Second}
			request, err := http.NewRequest(http.MethodGet, address+"/api/snapshot", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("User-Agent", "Synthetic modal browser")
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			child.expect(t, mark, "HTTP clients: 1 active")
			child.expect(t, mark, "127.0.0.1 | Synthetic modal browser")
			output, _ := child.transcript()
			colored := string(output[mark:])
			switch mode {
			case "demo-color":
				if !strings.Contains(colored, ";38;2;") {
					t.Fatal("missing TrueColor")
				}
			case "demo-ansi256":
				if !strings.Contains(colored, ";38;5;") || strings.Contains(colored, ";38;2;") {
					t.Fatal("wrong ANSI256 profile")
				}
			case "demo":
				if strings.Contains(colored, "38;2;") || strings.Contains(colored, "38;5;") {
					t.Fatal("color in no-color mode")
				}
			}
			t.Logf("%s 120x32 client frame:\n%s", mode, ansi.Strip(colored))
			mark = child.resize(t, 60, 18)
			child.expect(t, mark, address)
			mark = child.send(t, "\x1b[F") // End reaches wrapped client metadata and diagnostics.
			child.expect(t, mark, "Recent diagnostics")
			output, _ = child.transcript()
			t.Logf("%s 60x18 bottom frame:\n%s", mode, ansi.Strip(string(output[mark:])))
			mark = child.resize(t, 59, 17)
			child.expect(t, mark, "Resize to at least 60 columns")
			child.send(t, "Wq\x03") // Hidden close/quit actions remain blocked.
			mark = child.resize(t, 60, 18)
			child.expect(t, mark, "Recent diagnostics")
			mark = child.send(t, "\x05")
			child.expect(t, mark, "STATUS / FEEDBACK")
			mark = child.send(t, "\x1b")
			child.expect(t, mark, "SERVER STATUS")
			mark = child.resize(t, 120, 32)
			child.expect(t, mark, "SERVER STATUS")
			child.send(t, "\x1b[H")
			child.expect(t, mark, "Server: Running")
			output, _ = child.transcript()
			mark = len(output)
			transport.CloseIdleConnections()
			child.expectWithin(t, mark, "Disconnected #1", 2*ptyStepLimit)
			child.expect(t, mark, "HTTP clients: 0 active")
			mark = child.send(t, "\x1b")
			child.expect(t, mark, "PRINTERS")
			child.send(t, ",\t\t\t")
			child.expect(t, mark, "CONFIG / WEB SERVER")
			mark = child.send(t, "\x1b[F\r")
			child.expect(t, mark, "SERVER STATUS")
			mark = child.send(t, "\x1b")
			child.expect(t, mark, "CONFIG / WEB SERVER")
			mark = child.send(t, "\x1b")
			child.expect(t, mark, "PRINTERS")
			child.send(t, "q")
			child.exited(t, 0)
			assertWebClosed(t, address)
		})
	}
}

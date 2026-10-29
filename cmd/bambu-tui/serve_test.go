//go:build darwin || linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/accountstore"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/ui"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

func TestServeHelperProcess(t *testing.T) {
	mode := os.Getenv("BAMBU_TUI_SERVE_CHILD")
	if mode == "" {
		return
	}
	var err error
	switch mode {
	case "cloud":
		cfg, loadErr := config.Load(os.Getenv("BAMBU_TUI_SERVE_CONFIG"))
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		stores, storeErr := accountstore.Open(cfg.Region, "synthetic-account")
		if storeErr != nil {
			fmt.Fprintln(os.Stderr, storeErr)
			os.Exit(1)
		}
		defer stores.Close()
		state := cloudServeState(cfg, stores, os.Stderr)
		runner := &monitor.Runner{API: serveAPI{}, Token: "synthetic-token", Dial: func(context.Context) (monitor.Transport, error) {
			return &serveTransport{closed: make(chan struct{})}, nil
		}}
		theme, themeErr := ui.LoadTheme(os.Getenv("BAMBU_TUI_SERVE_CONFIG"), cfg.ThemeFile)
		if themeErr != nil {
			t.Fatal(themeErr)
		}
		err = serveMonitor(ctx, cfg, &state, os.Stdout, os.Stderr, func(ctx context.Context, events chan<- monitor.Event) { runner.Run(ctx, events, nil) }, serveOptions{theme: theme})
	case "close":
		cfg, loadErr := config.Load(os.Getenv("BAMBU_TUI_SERVE_CONFIG"))
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		web := webserver.New(context.Background())
		status := web.SetEnabled(true, cfg.WebServer.Port)
		if status.Error != "" {
			t.Fatal(status.Error)
		}
		state := session.New(farm.New(10), true)
		go func() { var b [1]byte; _, _ = os.Stdin.Read(b[:]); web.Close() }()
		fmt.Println("Listening on", status.URL)
		err = serveSession(context.Background(), &state, web, nil, io.Discard, time.Second)
	default:
		args := []string{"serve", "--demo", "--config", os.Getenv("BAMBU_TUI_SERVE_CONFIG")}
		if override := os.Getenv("BAMBU_TUI_SERVE_THEME"); override != "" {
			args = append(args, "--theme", override)
		}
		err = run(args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

type serveChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr bytes.Buffer
	url    string
}

func startServeChild(t *testing.T, path, mode string) *serveChild {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	c := &serveChild{}
	c.cmd = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestServeHelperProcess$")
	c.cmd.Env = append(os.Environ(), "BAMBU_TUI_SERVE_CHILD="+mode, "BAMBU_TUI_SERVE_CONFIG="+path, "HOME="+filepath.Dir(path), "XDG_CONFIG_HOME="+filepath.Dir(path))
	c.cmd.Stderr = &c.stderr
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.cmd.Process.Kill(); _ = c.cmd.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = c.cmd.Wait()
		t.Fatalf("startup: %v: %s", err, c.stderr.String())
	}
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "Listening" || strings.Contains(line, "\x1b") {
		t.Fatal(line)
	}
	c.url = fields[2]
	return c
}

func serveGet(t *testing.T, address string, value any) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func TestServeWithoutTerminal(t *testing.T) {
	for _, mode := range []string{"demo", "cloud"} {
		t.Run(mode, func(t *testing.T) {
			for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
				t.Run(signal.String(), func(t *testing.T) {
					path := ptyWebConfig(t, false)
					// Saved desktop opt-in must not enable delivery or change on exit.
					file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := file.WriteString("desktop_notifications=true\n"); err != nil {
						t.Fatal(err)
					}
					_ = file.Close()
					original, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					original = append([]byte("theme_file='custom-theme.json'\n"), original...)
					if err := os.WriteFile(filepath.Join(filepath.Dir(path), "custom-theme.json"), []byte(`{"version":1,"colors":{"background":"#123456","accent":"#FEDCBA"}}`), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, original, 0o600); err != nil {
						t.Fatal(err)
					}
					child := startServeChild(t, path, mode)
					var themes webserver.ThemeCatalog
					serveGet(t, child.url+"/api/themes", &themes)
					if len(themes.Themes) < 2 || themes.Themes[0].Colors["background"] != "#123456" || themes.Themes[0].Colors["accent"] != "#FEDCBA" {
						t.Fatal(themes)
					}
					var snapshot struct {
						Demo      bool   `json:"demo"`
						Decisions string `json:"decisions"`
						Printers  []struct {
							Conditions []struct {
								ID     string `json:"id"`
								Hidden bool   `json:"hidden"`
							} `json:"conditions"`
						} `json:"printers"`
					}
					deadline := time.Now().Add(3 * time.Second)
					target := ""
					for target == "" {
						serveGet(t, child.url+"/api/snapshot", &snapshot)
						for _, p := range snapshot.Printers {
							for _, c := range p.Conditions {
								target = c.ID
							}
						}
						if time.Now().After(deadline) {
							t.Fatal("no synthetic condition")
						}
						if target == "" {
							time.Sleep(10 * time.Millisecond)
						}
					}
					wantDecisions, wantPrinters := "session-only", 2
					if mode == "cloud" {
						wantDecisions, wantPrinters = "saved", 1
					}
					if snapshot.Demo != (mode == "demo") || snapshot.Decisions != wantDecisions || len(snapshot.Printers) != wantPrinters {
						t.Fatalf("unexpected snapshot: %+v", snapshot)
					}
					var token struct {
						Token string `json:"token"`
					}
					serveGet(t, child.url+"/api/session", &token)
					request, err := http.NewRequest(http.MethodPost, child.url+"/api/dismiss", strings.NewReader(fmt.Sprintf(`{"target":%q}`, target)))
					if err != nil {
						t.Fatal(err)
					}
					request.Header.Set("Origin", child.url)
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("X-CSRF-Token", token.Token)
					response, err := (&http.Client{Timeout: 4 * time.Second}).Do(request)
					if err != nil {
						t.Fatal(err)
					}
					_ = response.Body.Close()
					if response.StatusCode != http.StatusOK {
						t.Fatal(response.Status)
					}
					found := false
					for !found {
						serveGet(t, child.url+"/api/snapshot", &snapshot)
						for _, p := range snapshot.Printers {
							for _, c := range p.Conditions {
								if c.ID == target && c.Hidden {
									found = true
								}
							}
						}
						if time.Now().After(deadline) {
							t.Fatal("decision not published")
						}
					}
					if err := child.cmd.Process.Signal(signal); err != nil {
						t.Fatal(err)
					}
					if err := child.cmd.Wait(); err != nil {
						t.Fatalf("signal exit: %v: %s", err, child.stderr.String())
					}
					assertWebClosed(t, child.url)
					if mode == "cloud" {
						restarted := startServeChild(t, path, mode)
						deadline = time.Now().Add(3 * time.Second)
						restored := false
						for !restored {
							serveGet(t, restarted.url+"/api/snapshot", &snapshot)
							for _, p := range snapshot.Printers {
								for _, c := range p.Conditions {
									if c.Hidden {
										restored = true
									}
								}
							}
							if time.Now().After(deadline) {
								t.Fatal("saved cloud decision not restored after restart")
							}
							if !restored {
								time.Sleep(time.Millisecond)
							}
						}
						if err := restarted.cmd.Process.Signal(signal); err != nil {
							t.Fatal(err)
						}
						if err := restarted.cmd.Wait(); err != nil {
							t.Fatal(err, restarted.stderr.String())
						}
						assertWebClosed(t, restarted.url)
					}
					after, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(original, after) {
						t.Fatal("serve changed saved configuration", err)
					}
				})
			}
		})
	}
}

func TestServeBindFailureAndArgumentErrors(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("[web_server]\nport=%d\n", occupied.Addr().(*net.TCPAddr).Port)), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestServeHelperProcess$")
	cmd.Env = append(os.Environ(), "BAMBU_TUI_SERVE_CHILD=demo", "BAMBU_TUI_SERVE_CONFIG="+path)
	output, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("already in use")) || bytes.Contains(output, []byte("Listening")) {
		t.Fatalf("%v: %s", err, output)
	}
	for _, args := range [][]string{{"serve", "--demo", "--session"}, {"serve", "--demo", "--notify"}} {
		if err := run(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
}

func TestServeUnexpectedListenerClosureFailsProcess(t *testing.T) {
	child := startServeChild(t, ptyWebConfig(t, false), "close")
	if _, err := child.stdin.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := child.cmd.Wait(); err == nil || !strings.Contains(child.stderr.String(), "stopped unexpectedly") {
		t.Fatalf("%v: %s", err, child.stderr.String())
	}
	assertWebClosed(t, child.url)
}

func TestServeDrainsPendingPersistence(t *testing.T) {
	for _, outcome := range []string{"saved", "blocked", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			web := webserver.New(ctx)
			defer web.Close()
			path := ptyWebConfig(t, false)
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if status := web.SetEnabled(true, cfg.WebServer.Port); status.Error != "" {
				t.Fatal(status.Error)
			}
			state := session.New(farm.New(10), false)
			at := time.Now()
			if err := state.ApplyEvent(monitor.Event{Kind: monitor.EventDevices, At: at, Devices: []farm.Device{{ID: "TEST", Online: true}}}); err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			state.WithSnoozePersistence(nil, func(map[string]time.Time) error {
				close(started)
				if outcome == "failed" {
					return errors.New("disk full")
				}
				<-release
				return nil
			}, nil)
			state.SetSnooze("TEST", time.Hour, at)
			result := make(chan error, 1)
			go func() { result <- serveSession(ctx, &state, web, nil, io.Discard, 100*time.Millisecond) }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("write not started")
			}
			cancel()
			if outcome == "saved" {
				release <- struct{}{}
			}
			select {
			case err := <-result:
				if outcome != "saved" {
					if err == nil || !strings.Contains(err.Error(), "not confirmed saved") {
						t.Fatal(err)
					}
				} else if err != nil || state.HasUnsavedPersistence() {
					t.Fatal("write not drained", err)
				}
			case <-time.After(time.Second):
				t.Fatal("shutdown did not finish")
			}
		})
	}
}

func TestServeShutdownDrainsLatestCoalescedNoticeSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	web := webserver.New(ctx)
	defer web.Close()
	cfg, err := config.Load(ptyWebConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	status := web.SetEnabled(true, cfg.WebServer.Port)
	if status.Error != "" {
		t.Fatal(status.Error)
	}
	state := session.New(farm.New(10), false)
	at := time.Now()
	for _, event := range []monitor.Event{
		{Kind: monitor.EventDevices, At: at, Devices: []farm.Device{{ID: "SYNTHETIC", Online: true}}},
		{Kind: monitor.EventConnection, At: at, Connected: true},
		{Kind: monitor.EventReport, At: at, ID: "SYNTHETIC", Payload: []byte(`{"print":{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}}`)},
	} {
		if err := state.ApplyEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	var keys, targets []string
	for key, target := range state.WebDecisions().Targets {
		keys = append(keys, key)
		targets = append(targets, target)
	}
	if len(keys) < 2 {
		t.Fatal("need independent notices")
	}
	started, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	writes := make(chan map[string]farm.Review, 2)
	state.WithNoticePersistence(func(reviews map[string]farm.Review) error {
		writes <- reviews
		if len(writes) == 1 {
			close(started)
			<-release
		}
		return nil
	}, nil)
	state.Dismiss(keys[0])
	result := make(chan error, 1)
	go func() { result <- serveSession(ctx, &state, web, nil, io.Discard, time.Second) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first write not started")
	}
	var token struct {
		Token string `json:"token"`
	}
	serveGet(t, status.URL+"/api/session", &token)
	request, err := http.NewRequest(http.MethodPost, status.URL+"/api/dismiss", strings.NewReader(fmt.Sprintf(`{"target":%q}`, targets[1])))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", status.URL)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", token.Token)
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.Status)
	}
	cancel()
	unblock()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain timed out")
	}
	if len(writes) != 2 || state.HasUnsavedPersistence() {
		t.Fatal("coalesced write not drained")
	}
	first, latest := <-writes, <-writes
	if first[keys[1]].Dismissed || !latest[keys[0]].Dismissed || !latest[keys[1]].Dismissed {
		t.Fatal("wrong immutable/coalesced snapshots")
	}
}

func TestServeThemeOverrideDoesNotLoadOrRewriteConfiguredFile(t *testing.T) {
	path := ptyWebConfig(t, false)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original = append([]byte("theme_file='missing.json'\n"), original...)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BAMBU_TUI_SERVE_THEME", "hotdog")
	child := startServeChild(t, path, "demo")
	var themes webserver.ThemeCatalog
	serveGet(t, child.url+"/api/themes", &themes)
	theme, err := ui.LoadThemeOverride("hotdog")
	if err != nil {
		t.Fatal(err)
	}
	if themes.Themes[0].Colors["background"] != string(theme.Palette.Background) {
		t.Fatal(themes)
	}
	if err := child.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := child.cmd.Wait(); err != nil {
		t.Fatal(err, child.stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("override changed configuration", err)
	}
}

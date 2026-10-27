//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/alertstore"
	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/credentials"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/mqtt"
	"github.com/cliffom/bambu-tui/internal/noticestore"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

func TestServeSavedAccountFailures(t *testing.T) {
	for _, failure := range []error{credentials.ErrNotFound, credentials.ErrInvalid, credentials.ErrUnavailable} {
		t.Run(failure.Error(), func(t *testing.T) {
			account, err := loadServeAccount(func() (cloud.Session, error) { return cloud.Session{}, failure })
			if !errors.Is(err, failure) || account != (cloud.Session{}) || !strings.Contains(err.Error(), "bambu-tui login") || !strings.Contains(err.Error(), "credential store") {
				t.Fatal(account, err)
			}
		})
	}
	for _, account := range []cloud.Session{{}, {Token: "synthetic"}, {Username: "synthetic"}} {
		if _, err := loadServeAccount(func() (cloud.Session, error) { return account, nil }); !errors.Is(err, credentials.ErrInvalid) {
			t.Fatal(err)
		}
	}
	want := cloud.Session{Token: "synthetic-token", Username: "synthetic-account"}
	got, err := loadServeAccount(func() (cloud.Session, error) { return want, nil })
	if err != nil || got != want {
		t.Fatal("valid saved session rejected", err)
	}
}

func applyServeFixture(t *testing.T, state *session.Session, at time.Time) string {
	t.Helper()
	for _, event := range []monitor.Event{
		{Kind: monitor.EventDevices, At: at, Devices: []farm.Device{{ID: "SYNTHETIC", Name: "Synthetic", Online: true}}},
		{Kind: monitor.EventConnection, At: at, Connected: true},
		{Kind: monitor.EventReport, At: at, ID: "SYNTHETIC", Payload: []byte(`{"print":{"gcode_state":"PAUSE","print_error":117473297}}`)},
	} {
		if err := state.ApplyEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	for key := range state.WebDecisions().Targets {
		if strings.HasPrefix(key, "SYNTHETIC/") {
			return key
		}
	}
	t.Fatal("no synthetic notice")
	return ""
}

func TestCloudServeStoresRestartScopeAndFailedLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	state := cloudServeState(cfg, "synthetic-account", &diagnostics)
	at := time.Now()
	key := applyServeFixture(t, &state, at)
	state.Dismiss(key)
	state.SetSnooze("SYNTHETIC", time.Hour, at)
	reviews, snoozes := state.Persistence()
	if reviews == nil || snoozes == nil {
		t.Fatal("missing persistence")
	}
	state.Complete(reviews())
	state.Complete(snoozes())
	restored := cloudServeState(cfg, "synthetic-account", &diagnostics)
	applyServeFixture(t, &restored, at)
	if !restored.Farm.Active[key].Dismissed || !restored.Snoozes["SYNTHETIC"].Equal(at.Add(time.Hour)) || diagnostics.Len() != 0 {
		t.Fatal("restart lost decisions", diagnostics.String())
	}
	for _, other := range []struct{ region, account string }{{cfg.Region, "other-synthetic-account"}, {"china", "synthetic-account"}} {
		otherCfg := cfg
		otherCfg.Region = other.region
		isolated := cloudServeState(otherCfg, other.account, &diagnostics)
		applyServeFixture(t, &isolated, at)
		if isolated.Farm.Active[key].Dismissed || len(isolated.Snoozes) != 0 {
			t.Fatal("cross-account/region decisions")
		}
	}
	notices, err := noticestore.New(cfg.Region, "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	alerts, err := alertstore.New(cfg.Region, "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{notices.Path, alerts.Path} {
		if err := os.WriteFile(path, []byte("invalid saved data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	protected := cloudServeState(cfg, "synthetic-account", &diagnostics)
	key = applyServeFixture(t, &protected, at)
	protected.Dismiss(key)
	protected.SetSnooze("SYNTHETIC", time.Hour, at)
	reviews, snoozes = protected.Persistence()
	if reviews != nil || snoozes != nil || protected.NoticesWritable() || protected.SnoozesWritable() || !strings.Contains(protected.WebDecisions().State, "saved data unavailable") {
		t.Fatal("failed loads enabled writes")
	}
	for _, path := range []string{notices.Path, alerts.Path} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "invalid saved data" {
			t.Fatal("corrupt store overwritten", err)
		}
	}
	if !strings.Contains(diagnostics.String(), "restart") {
		t.Fatal("missing recovery guidance")
	}
}

type (
	serveAPI       struct{}
	serveTransport struct {
		closed   chan struct{}
		once     sync.Once
		reported bool
	}
	failingServeWriter struct{}
)

func (serveAPI) Devices(context.Context, string) ([]farm.Device, error) {
	return []farm.Device{{ID: "SYNTHETIC", Name: "Synthetic", Online: true}}, nil
}

func (f *serveTransport) Subscribe([]string) error { return nil }
func (f *serveTransport) Refresh([]string) error   { return nil }
func (f *serveTransport) Ping() error              { return nil }
func (f *serveTransport) Close() error             { f.once.Do(func() { close(f.closed) }); return nil }

func (f *serveTransport) Read() (string, []byte, error) {
	if !f.reported {
		f.reported = true
		return "SYNTHETIC", []byte(`{"print":{"gcode_state":"PAUSE","print_error":117473297}}`), nil
	}
	<-f.closed
	return "", nil, io.EOF
}

func TestCloudServeMonitorOutcomes(t *testing.T) {
	for _, outcome := range []string{"http-auth", "mqtt-auth", "reconnect", "cancel", "unexpected"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			var calls atomic.Int32
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Error("missing synthetic credential")
				}
				if outcome == "http-auth" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if outcome == "reconnect" && n == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = io.WriteString(w, `{"devices":[{"dev_id":"SYNTHETIC","name":"Synthetic","online":true}]}`)
			}))
			defer apiServer.Close()
			api := cloud.New("global")
			api.BaseURL = apiServer.URL
			runner := monitor.New(api, cloud.Session{Token: "synthetic-token", Username: "synthetic-account"})
			runner.RetryMin = time.Millisecond
			runner.RetryMax = time.Millisecond
			runner.Dial = func(context.Context) (monitor.Transport, error) {
				if outcome == "mqtt-auth" {
					return nil, mqtt.ErrAuth
				}
				return &serveTransport{closed: make(chan struct{})}, nil
			}
			cfg, err := config.Load(ptyWebConfig(t, false))
			if err != nil {
				t.Fatal(err)
			}
			state := session.New(farm.New(10), false)
			web := webserver.New(ctx)
			defer web.Close()
			status := web.SetEnabled(true, cfg.WebServer.Port)
			if status.Error != "" {
				t.Fatal(status.Error)
			}
			events := make(chan monitor.Event, 32)
			monitorDone := make(chan struct{})
			if outcome == "unexpected" {
				close(events)
				close(monitorDone)
			} else {
				go func() { defer close(monitorDone); runner.Run(ctx, events, nil) }()
			}
			result := make(chan error, 1)
			go func() { result <- serveSession(ctx, &state, web, events, time.Second) }()
			if outcome == "cancel" || outcome == "reconnect" {
				var snapshot struct {
					Printers []struct {
						State string `json:"state"`
					} `json:"printers"`
				}
				for {
					serveGet(t, status.URL+"/api/snapshot", &snapshot)
					if len(snapshot.Printers) > 0 {
						break
					}
					if ctx.Err() != nil {
						t.Fatal("no cloud discovery")
					}
					time.Sleep(time.Millisecond)
				}
				if outcome == "reconnect" && calls.Load() < 2 {
					t.Fatal("did not reconnect")
				}
				cancel()
			}
			select {
			case err := <-result:
				if outcome == "cancel" || outcome == "reconnect" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("unexpected successful exit")
				} else if outcome != "unexpected" && !strings.Contains(err.Error(), "session expired or rejected") {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("server did not terminate")
			}
			cancel()
			select {
			case <-monitorDone:
			case <-time.After(time.Second):
				t.Fatal("monitor did not stop")
			}
			assertWebClosed(t, status.URL)
		})
	}
}

// A closed browser does not own monitoring; a bind failure cannot start it.
func TestCloudServeHostBindAndOutputFailures(t *testing.T) {
	cfg, err := config.Load(ptyWebConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	web := webserver.New(ctx)
	defer web.Close()
	if status := web.SetEnabled(true, cfg.WebServer.Port); status.Error != "" {
		t.Fatal(status.Error)
	}
	state := session.New(farm.New(10), false)
	var output bytes.Buffer
	run := func(context.Context, chan<- monitor.Event) { t.Error("monitor started before successful bind/output") }
	if err := serveMonitor(ctx, cfg, &state, &output, run); err == nil || output.Len() != 0 {
		t.Fatal("bind failure", err, output.String())
	}
	web.Close()
	if err := serveMonitor(ctx, cfg, &state, failingServeWriter{}, run); err == nil {
		t.Fatal("output failure ignored")
	}
}

func (failingServeWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic output failure")
}

func TestCloudServeExpiredSnoozesSaved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := alertstore.New(cfg.Region, "synthetic-account")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(store.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(map[string]time.Time{"SYNTHETIC": time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	state := cloudServeState(cfg, "synthetic-account", io.Discard)
	state.PruneSnoozes(time.Now())
	_, job := state.Persistence()
	if job == nil {
		t.Fatal("no expiry cleanup")
	}
	state.Complete(job())
	saved, err := store.Load()
	if err != nil || len(saved) != 0 {
		t.Fatal(fmt.Sprint(saved), err)
	}
}

func TestCloudServeBrowserExplicitSaveRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg, err := config.Load(ptyWebConfig(t, false))
	if err != nil {
		t.Fatal(err)
	}
	web := webserver.New(ctx)
	defer web.Close()
	status := web.SetEnabled(true, cfg.WebServer.Port)
	if status.Error != "" {
		t.Fatal(status.Error)
	}
	state := session.New(farm.New(10), false)
	key := applyServeFixture(t, &state, time.Now())
	var attempts atomic.Int32
	saved := make(chan map[string]farm.Review, 1)
	state.WithNoticePersistence(func(reviews map[string]farm.Review) error {
		if attempts.Add(1) == 1 {
			return errors.New("synthetic disk failure")
		}
		saved <- reviews
		return nil
	}, nil)
	target := state.WebDecisions().Targets[key]
	result := make(chan error, 1)
	go func() { result <- serveSession(ctx, &state, web, nil, time.Second) }()
	var token struct {
		Token string `json:"token"`
	}
	serveGet(t, status.URL+"/api/session", &token)
	post := func(retry bool) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, status.URL+"/api/dismiss", strings.NewReader(fmt.Sprintf(`{"target":%q,"retry":%t}`, target, retry)))
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
	}
	waitState := func(want string) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			var snapshot struct {
				Decisions string `json:"decisions"`
			}
			serveGet(t, status.URL+"/api/snapshot", &snapshot)
			if snapshot.Decisions == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("storage state %q, want %q", snapshot.Decisions, want)
			}
			time.Sleep(time.Millisecond)
		}
	}
	post(false)
	waitState("failed")
	if attempts.Load() != 1 {
		t.Fatal("automatic save retry")
	}
	post(true)
	waitState("saved")
	select {
	case reviews := <-saved:
		if !reviews[key].Dismissed {
			t.Fatal("wrong retry snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not save")
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}

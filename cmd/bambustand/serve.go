package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cliffom/bambustand/internal/accountstore"
	"github.com/cliffom/bambustand/internal/cloud"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/credentials"
	"github.com/cliffom/bambustand/internal/farm"
	"github.com/cliffom/bambustand/internal/monitor"
	"github.com/cliffom/bambustand/internal/session"
	"github.com/cliffom/bambustand/internal/ui"
	"github.com/cliffom/bambustand/internal/webserver"
)

type serveOptions struct {
	username string
	theme    ui.Theme
}

const (
	serveShutdownTimeout = 5 * time.Second
	serveTickInterval    = time.Second
	serveStoreCount      = 2
)

// loadServeAccount never starts an interactive login or a session-only fallback.
func loadServeAccount(load func() (cloud.Session, error)) (cloud.Session, error) {
	account, err := load()
	if err == nil && (account.Token == "" || account.Username == "") {
		err = credentials.ErrInvalid
	}
	if err != nil {
		return cloud.Session{}, fmt.Errorf("cannot start cloud server: %w; run bambustand login to save a session for this config's region; unlock the OS credential store (Linux: install secret-tool and start Secret Service), then retry; bambustand doctor checks storage", err)
	}
	return account, nil
}

func serveCloud(ctx context.Context, cfg config.Config, output, diagnostics io.Writer, theme ui.Theme) error {
	account, err := loadServeAccount(func() (cloud.Session, error) { return (credentials.Store{Region: cfg.Region}).LoadContext(ctx) })
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	stores, err := accountstore.Open(cfg.Region, account.Username)
	if err != nil {
		return err
	}
	defer stores.Close()
	state := cloudServeState(cfg, stores, diagnostics)
	runner := monitor.New(cloud.New(cfg.Region), account)
	return serveMonitor(ctx, cfg, &state, output, diagnostics, func(ctx context.Context, events chan<- monitor.Event) {
		runner.Run(ctx, events, nil)
	}, serveOptions{username: account.Username, theme: theme})
}

// Stores use the same account/region scope and failed-load protection as the TUI.
func cloudServeState(cfg config.Config, stores *accountstore.Store, diagnostics io.Writer) session.Session {
	state := session.New(farm.New(cfg.LowFilamentPercent), false)
	for _, warning := range state.WithAccountPersistence(stores) {
		_, _ = fmt.Fprintln(diagnostics, warning)
	}
	return state
}

func serveDemo(ctx context.Context, cfg config.Config, output io.Writer, theme ui.Theme) error {
	state := session.New(farm.New(cfg.LowFilamentPercent), true)
	return serveMonitor(ctx, cfg, &state, output, io.Discard, func(ctx context.Context, events chan<- monitor.Event) {
		monitor.Demo(ctx, events, nil)
	}, serveOptions{theme: theme})
}

// Each foreground host has one monitor regardless of browser-client count.
// Neither host initializes native notification delivery or writes configuration.
func serveMonitor(ctx context.Context, cfg config.Config, state *session.Session, output, diagnostics io.Writer, run func(context.Context, chan<- monitor.Event), options ...serveOptions) (result error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if ctx.Err() != nil {
		return nil
	}
	option := serveOptions{theme: ui.Theme{Palette: ui.DefaultPalette()}}
	if len(options) > 0 {
		option = options[0]
	}
	web, closeBrowser := browserServer(ctx, cfg.Region, option.username, state.Demo)
	defer closeBrowser()
	web.PublishThemes(ui.BrowserThemes(option.theme.Palette))
	state.Publish(web, time.Now(), session.BrowserPolicy{Alerts: cfg.Alerts, Active: true})
	status := web.SetEnabled(true, cfg.WebServer.Port)
	if status.Error != "" {
		return errors.New(status.Error)
	}
	display := newServeDisplay(output, status.URL, state.Demo)
	defer func() {
		wasActive := display.active
		result = errors.Join(result, display.close())
		if wasActive {
			outcome := "stopped"
			if result != nil {
				outcome = "failed"
			}
			_, err := fmt.Fprintf(output, "Serve %s; dashboard closed.\n", outcome)
			result = errors.Join(result, err)
		}
	}()
	display.persistence(state)
	if err := display.begin(); err != nil {
		return err
	}

	events := make(chan monitor.Event, eventBufferSize)
	done := make(chan struct{})
	go func() { defer close(done); run(ctx, events) }()
	err := serveSessionStatus(ctx, state, web, events, diagnostics, serveShutdownTimeout, display, session.BrowserPolicy{Alerts: cfg.Alerts, Active: true})
	cancel()
	select {
	case <-done:
	case <-time.After(serveShutdownTimeout):
		err = errors.Join(err, errors.New("monitor shutdown timed out"))
	}
	display.clients = web.Clients()
	display.server = "Stopped"
	if err != nil {
		display.server = "Failed"
	}
	return errors.Join(err, display.render(time.Now(), true))
}

// serveSession is the sole mutable-state owner. Jobs receive snapshots and
// return results here; neither HTTP handlers nor workers touch live farm state.
func serveSession(ctx context.Context, state *session.Session, web *webserver.Server, events <-chan monitor.Event, diagnostics io.Writer, shutdownTimeout time.Duration, preferences ...session.BrowserPolicy) error {
	return serveSessionStatus(ctx, state, web, events, diagnostics, shutdownTimeout, nil, preferences...)
}

func serveSessionStatus(ctx context.Context, state *session.Session, web *webserver.Server, events <-chan monitor.Event, diagnostics io.Writer, shutdownTimeout time.Duration, display *serveDisplay, preferences ...session.BrowserPolicy) error {
	ticker := time.NewTicker(serveTickInterval)
	defer ticker.Stop()
	results := make(chan session.Result, serveStoreCount)
	startJobs := func() {
		reviews, snoozes := state.Persistence()
		for _, job := range []session.Job{reviews, snoozes} {
			if job != nil {
				go func() { results <- job() }()
			}
		}
	}
	var displayErr error
	publish := func() {
		startJobs()
		state.Publish(web, time.Now(), preferences...)
		if display != nil {
			display.persistence(state)
			display.clients = web.Clients()
			displayErr = display.render(time.Now(), false)
		}
	}
	publish()
	listening := web.ListeningDone()
	var runErr error
	warnedRejection := false
running:
	for {
		if ctx.Err() != nil || displayErr != nil {
			break running
		}
		select {
		case <-ctx.Done():
			break running
		case <-listening:
			if ctx.Err() == nil {
				runErr = errors.New("localhost web server stopped unexpectedly")
			}
			break running
		case event, ok := <-events:
			if !ok {
				if ctx.Err() == nil {
					runErr = errors.New("monitor stopped unexpectedly")
				}
				break running
			}
			if display != nil {
				display.observe(event)
			}
			if event.StopReason == monitor.StopAuthenticationRejected {
				runErr = errors.New("cloud session expired or rejected; run bambustand login with this config to save a new session, then restart serve")
				break running
			}
			if err := state.ApplyEvent(event); err != nil {
				// Decoder rejection leaves accepted telemetry and decisions intact.
				// Emit fixed text once: neither payloads nor error strings are safe
				// diagnostics, and a bad-report flood must not flood stderr.
				if !warnedRejection {
					if display != nil {
						display.note("Telemetry rejected; keeping last accepted state (further warnings suppressed)", serveWarning)
					}
					_, _ = fmt.Fprintln(diagnostics, "Telemetry report rejected; keeping last accepted state and continuing monitoring (further rejection warnings suppressed for this session)")
					warnedRejection = true
				}
				continue
			}
		case at := <-ticker.C:
			state.Reconcile(at)
			state.PruneSnoozes(at)
		case command := <-web.Commands():
			command.Reply <- string(state.ApplyDecision(session.DecisionCommand{Target: command.Target, Retry: command.Retry}, ctx.Err() != nil, true))
		case result := <-results:
			state.Complete(result)
		}
		publish()
	}
	runErr = errors.Join(runErr, displayErr)
	if display != nil {
		display.server = "Stopping"
		runErr = errors.Join(runErr, display.render(time.Now(), true))
	}
	// Closing the listener prevents further requests while accepted writes drain.
	web.Close()
	startJobs()
	deadline := time.NewTimer(shutdownTimeout)
	defer deadline.Stop()
	for state.PersistencePending() {
		select {
		case result := <-results:
			state.Complete(result)
			startJobs()
		case <-deadline.C:
			return errors.Join(runErr, errors.New("persistence shutdown timed out; changes were not confirmed saved"))
		}
	}
	if state.HasUnsavedPersistence() {
		return errors.Join(runErr, errors.New("changes were not confirmed saved before exit"))
	}
	return runErr
}

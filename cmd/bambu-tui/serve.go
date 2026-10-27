package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cliffom/bambu-tui/internal/alertstore"
	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/credentials"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/noticestore"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

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
		return cloud.Session{}, fmt.Errorf("cannot start cloud server: %w; run bambu-tui login to save a session for this config's region; unlock the OS credential store (Linux: install secret-tool and start Secret Service), then retry; bambu-tui doctor checks storage", err)
	}
	return account, nil
}

func serveCloud(ctx context.Context, cfg config.Config, output, diagnostics io.Writer) error {
	account, err := loadServeAccount(func() (cloud.Session, error) { return (credentials.Store{Region: cfg.Region}).LoadContext(ctx) })
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	state := cloudServeState(cfg, account.Username, diagnostics)
	runner := monitor.New(cloud.New(cfg.Region), account)
	return serveMonitor(ctx, cfg, &state, output, diagnostics, func(ctx context.Context, events chan<- monitor.Event) {
		runner.Run(ctx, events, nil)
	})
}

// Stores use the same account/region scope and failed-load protection as the TUI.
func cloudServeState(cfg config.Config, username string, diagnostics io.Writer) session.Session {
	state := session.New(farm.New(cfg.LowFilamentPercent), false)
	notices, loadErr := noticestore.New(cfg.Region, username)
	var reviews map[string]farm.Review
	if loadErr == nil {
		reviews, loadErr = notices.Load()
	}
	state.Farm.RestoreReviews(reviews)
	state.WithNoticePersistence(notices.Save, loadErr)
	if loadErr != nil {
		_, _ = fmt.Fprintln(diagnostics, "Notice persistence: saved data unavailable; decisions apply only to this session; repair the notice store and restart")
	}
	snoozes, snoozeErr := alertstore.New(cfg.Region, username)
	var deadlines map[string]time.Time
	if snoozeErr == nil {
		deadlines, snoozeErr = snoozes.Load()
	}
	state.WithSnoozePersistence(deadlines, snoozes.Save, snoozeErr)
	if snoozeErr != nil {
		_, _ = fmt.Fprintln(diagnostics, "Alert persistence: saved data unavailable; changes apply only to this session; repair the snooze store and restart")
	}
	return state
}

func serveDemo(ctx context.Context, cfg config.Config, output io.Writer) error {
	state := session.New(farm.New(cfg.LowFilamentPercent), true)
	return serveMonitor(ctx, cfg, &state, output, io.Discard, func(ctx context.Context, events chan<- monitor.Event) {
		monitor.Demo(ctx, events, nil)
	})
}

// Each foreground host has one monitor regardless of browser-client count.
// Neither host initializes native notification delivery or writes configuration.
func serveMonitor(ctx context.Context, cfg config.Config, state *session.Session, output, diagnostics io.Writer, run func(context.Context, chan<- monitor.Event)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if ctx.Err() != nil {
		return nil
	}
	web := webserver.New(ctx)
	defer web.Close()
	state.Publish(web, time.Now())
	status := web.SetEnabled(true, cfg.WebServer.Port)
	if status.Error != "" {
		return errors.New(status.Error)
	}
	mode := "cloud"
	if state.Demo {
		mode = "demo"
	}
	if _, err := fmt.Fprintf(output, "Listening on %s (%s; native desktop notifications disabled)\n", status.URL, mode); err != nil {
		return err
	}

	events := make(chan monitor.Event, eventBufferSize)
	done := make(chan struct{})
	go func() { defer close(done); run(ctx, events) }()
	err := serveSession(ctx, state, web, events, diagnostics, serveShutdownTimeout)
	cancel()
	select {
	case <-done:
	case <-time.After(serveShutdownTimeout):
		err = errors.Join(err, errors.New("monitor shutdown timed out"))
	}
	return err
}

// serveSession is the sole mutable-state owner. Jobs receive snapshots and
// return results here; neither HTTP handlers nor workers touch live farm state.
func serveSession(ctx context.Context, state *session.Session, web *webserver.Server, events <-chan monitor.Event, diagnostics io.Writer, shutdownTimeout time.Duration) error {
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
	publish := func() { startJobs(); state.Publish(web, time.Now()) }
	publish()
	listening := web.ListeningDone()
	var runErr error
	warnedRejection := false
running:
	for {
		if ctx.Err() != nil {
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
			if event.StopReason == monitor.StopAuthenticationRejected {
				runErr = errors.New("cloud session expired or rejected; run bambu-tui login with this config to save a new session, then restart serve")
				break running
			}
			if err := state.ApplyEvent(event); err != nil {
				// Decoder rejection leaves accepted telemetry and decisions intact.
				// Emit fixed text once: neither payloads nor error strings are safe
				// diagnostics, and a bad-report flood must not flood stderr.
				if !warnedRejection {
					_, _ = fmt.Fprintln(diagnostics, "Telemetry report rejected; keeping last accepted state and continuing monitoring (further rejection warnings suppressed for this session)")
					warnedRejection = true
				}
				continue
			}
		case at := <-ticker.C:
			state.Reconcile(at)
			state.PruneSnoozes(at)
		case command := <-web.Commands():
			state.ApplyWebCommand(command, ctx.Err() != nil, true)
		case result := <-results:
			state.Complete(result)
		}
		publish()
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

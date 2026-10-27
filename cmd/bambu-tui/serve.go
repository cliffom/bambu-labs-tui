package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

const (
	serveShutdownTimeout = 5 * time.Second
	serveTickInterval    = time.Second
	serveStoreCount      = 2
)

func serveDemo(ctx context.Context, cfg config.Config, output io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	web := webserver.New(ctx)
	defer web.Close()
	state := session.New(farm.New(cfg.LowFilamentPercent), true)
	state.Publish(web, time.Now())
	status := web.SetEnabled(true, cfg.WebServer.Port)
	if status.Error != "" {
		return errors.New(status.Error)
	}
	if _, err := fmt.Fprintln(output, "Listening on", status.URL, "(demo; native desktop notifications disabled)"); err != nil {
		return err
	}
	events := make(chan monitor.Event, eventBufferSize)
	done := make(chan struct{})
	go func() { defer close(done); monitor.Demo(ctx, events, nil) }()
	err := serveSession(ctx, &state, web, events, serveShutdownTimeout)
	cancel()
	select {
	case <-done:
	case <-time.After(serveShutdownTimeout):
		err = errors.Join(err, errors.New("demo monitor shutdown timed out"))
	}
	return err
}

// serveSession is the sole mutable-state owner. Jobs receive snapshots and
// return results here; neither HTTP handlers nor workers touch live farm state.
func serveSession(ctx context.Context, state *session.Session, web *webserver.Server, events <-chan monitor.Event, shutdownTimeout time.Duration) error {
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
					runErr = errors.New("demo monitor stopped unexpectedly")
				}
				break running
			}
			if err := state.ApplyEvent(event); err != nil {
				runErr = fmt.Errorf("demo telemetry: %w", err)
				break running
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

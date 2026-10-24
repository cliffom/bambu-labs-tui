// Package monitor bridges cloud transports to immutable UI messages.
package monitor

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/mqtt"
)

type Event struct {
	Kind      string
	At        time.Time
	Devices   []farm.Device
	ID        string
	Payload   []byte
	Connected bool
	Problem   string
}
type API interface {
	Devices(context.Context, string) ([]farm.Device, error)
}
type Transport interface {
	Subscribe([]string) error
	Refresh([]string) error
	Ping() error
	Read() (string, []byte, error)
	Close() error
}
type Dial func(context.Context) (Transport, error)
type Runner struct {
	API   API
	Token string
	Dial  Dial
	// Shorter intervals are injected in tests; production uses protocol-safe defaults.
	PollInterval, PingInterval, RetryMin, RetryMax time.Duration
}

func New(api *cloud.Client, session cloud.Session) *Runner {
	return &Runner{API: api, Token: session.Token, Dial: func(ctx context.Context) (Transport, error) {
		return mqtt.Dial(ctx, api.MQTTAddress, session.Username, session.Token)
	}}
}
func emit(ctx context.Context, out chan<- Event, e Event) bool {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	select {
	case out <- e:
		return true
	case <-ctx.Done():
		return false
	}
}
func (r *Runner) Run(ctx context.Context, out chan<- Event, refresh <-chan struct{}) {
	defer close(out)
	minimum, maximum := r.RetryMin, r.RetryMax
	if minimum == 0 {
		minimum = time.Second
	}
	if maximum == 0 {
		maximum = 60 * time.Second
	}
	delay := minimum
	for ctx.Err() == nil {
		devices, err := r.API.Devices(ctx, r.Token)
		if err == nil {
			if !emit(ctx, out, Event{Kind: "devices", Devices: devices}) {
				return
			}
			var conn Transport
			conn, err = r.Dial(ctx)
			if err == nil {
				err = r.watch(ctx, conn, devices, out, refresh)
			}
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, cloud.ErrAuth) || errors.Is(err, mqtt.ErrAuth) {
			emit(ctx, out, Event{Kind: "connection", Problem: "Session expired or rejected — quit and run bambu-tui login"})
			return
		}
		problem := "Cloud disconnected; reconnecting"
		if errors.Is(err, cloud.ErrBlocked) || errors.Is(err, cloud.ErrRateLimited) {
			problem = err.Error()
		}
		if !emit(ctx, out, Event{Kind: "connection", Problem: problem}) {
			return
		}
		wait := delay + time.Duration(rand.Float64()*float64(delay)/4)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		delay = min(delay*2, maximum)
	}
}
func (r *Runner) watch(ctx context.Context, conn Transport, devices []farm.Device, out chan<- Event, refresh <-chan struct{}) error {
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	known := map[string]bool{}
	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, d.ID)
		known[d.ID] = true
	}
	if err := conn.Subscribe(ids); err != nil {
		return err
	}
	if err := conn.Refresh(ids); err != nil {
		return err
	}
	if !emit(ctx, out, Event{Kind: "connection", Connected: true}) {
		return ctx.Err()
	}
	type report struct {
		id   string
		body []byte
		err  error
	}
	reports := make(chan report, 1)
	readerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			id, b, err := conn.Read()
			select {
			case reports <- report{id, b, err}:
			case <-readerCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	pollEvery, pingEvery := r.PollInterval, r.PingInterval
	if pollEvery == 0 {
		pollEvery = 60 * time.Second
	}
	if pingEvery == 0 {
		pingEvery = 25 * time.Second
	}
	poll, ping := time.NewTicker(pollEvery), time.NewTicker(pingEvery)
	defer poll.Stop()
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case report := <-reports:
			if report.err != nil {
				return report.err
			}
			if !emit(ctx, out, Event{Kind: "report", ID: report.id, Payload: report.body}) {
				return ctx.Err()
			}
		case <-refresh:
			if err := conn.Refresh(ids); err != nil {
				return err
			}
		case <-ping.C:
			if err := conn.Ping(); err != nil {
				return err
			}
		case <-poll.C:
			latest, err := r.API.Devices(ctx, r.Token)
			if errors.Is(err, cloud.ErrAuth) || errors.Is(err, cloud.ErrBlocked) || errors.Is(err, cloud.ErrRateLimited) {
				return err
			}
			if err != nil {
				emit(ctx, out, Event{Kind: "problem", Problem: "Device availability refresh failed; telemetry connection is still active"})
				continue
			}
			if !emit(ctx, out, Event{Kind: "devices", Devices: latest}) {
				return ctx.Err()
			}
			var added []string
			ids = ids[:0]
			for _, d := range latest {
				ids = append(ids, d.ID)
				if !known[d.ID] {
					added = append(added, d.ID)
					known[d.ID] = true
				}
			}
			if err := conn.Subscribe(added); err != nil {
				return err
			}
			if err := conn.Refresh(added); err != nil {
				return err
			}
		}
	}
}

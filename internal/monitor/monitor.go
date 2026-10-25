// Package monitor bridges cloud transports to immutable UI messages.
package monitor

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/mqtt"
)

// EventKind identifies the monitor messages consumed by the UI reducer.
type EventKind string

const (
	EventDevices    EventKind = "devices"
	EventConnection EventKind = "connection"
	EventReport     EventKind = "report"
	EventProblem    EventKind = "problem"
)

type Event struct {
	Kind      EventKind
	At        time.Time
	Devices   []farm.Device
	ID        string
	Payload   []byte
	Connected bool
	Problem   string
}

// API implementations must return when the supplied context is canceled.
type API interface {
	Devices(context.Context, string) ([]farm.Device, error)
}

// Transport serializes packet writes (including Read acknowledgements). Close
// must unblock every pending Read, Subscribe, Refresh, and Ping operation.
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
	HealthyAfter, WriteTimeout                     time.Duration
	// Tests can advance session time and observe backoff without wall-clock sleeps.
	now                  func() time.Time
	wait                 func(context.Context, time.Duration) bool
	pollTicks, pingTicks <-chan time.Time
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
	healthyAfter := r.HealthyAfter
	if healthyAfter == 0 {
		healthyAfter = 2 * time.Minute
	}
	delay := minimum
	for ctx.Err() == nil {
		devices, err := r.API.Devices(ctx, r.Token)
		if err == nil {
			if !emit(ctx, out, Event{Kind: EventDevices, Devices: devices}) {
				return
			}
			var conn Transport
			conn, err = r.Dial(ctx)
			if err == nil {
				var monitored time.Duration
				monitored, err = r.watch(ctx, conn, devices, out, refresh)
				if monitored >= healthyAfter {
					delay = minimum
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, cloud.ErrAuth) || errors.Is(err, mqtt.ErrAuth) {
			emit(ctx, out, Event{Kind: EventConnection, Problem: "Session expired or rejected — quit and run bambu-tui login"})
			return
		}
		problem := "Cloud disconnected; reconnecting"
		if errors.Is(err, cloud.ErrBlocked) || errors.Is(err, cloud.ErrRateLimited) {
			problem = err.Error()
		}
		if !emit(ctx, out, Event{Kind: EventConnection, Problem: problem}) {
			return
		}
		wait := delay + time.Duration(rand.Float64()*float64(delay)/4)
		pause := r.wait
		if pause == nil {
			pause = waitRetry
		}
		if !pause(ctx, wait) {
			return
		}
		delay = min(delay*2, maximum)
	}
}
func waitRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

type report struct {
	id   string
	body []byte
	err  error
}
type discovery struct {
	devices []farm.Device
	err     error
}
type writeRequest struct {
	subscribe, refresh []string
	ping               bool
}

// write bounds the whole batch, including time waiting for MQTT's packet mutex.
// Report forwarding stays independent, and a stalled socket cannot delay
// reconnect indefinitely. Close releases the operation before this worker exits.
func (r *Runner) write(ctx context.Context, conn Transport, request writeRequest) error {
	timeout := r.WriteTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	operation, cancel := context.WithTimeout(ctx, timeout)
	closed := make(chan struct{})
	stop := context.AfterFunc(operation, func() { _ = conn.Close(); close(closed) })
	var err error
	if request.ping {
		err = conn.Ping()
	} else {
		if len(request.subscribe) > 0 {
			err = conn.Subscribe(request.subscribe)
		}
		if err == nil && len(request.refresh) > 0 {
			err = conn.Refresh(request.refresh)
		}
	}
	if !stop() {
		<-closed
	}
	if operation.Err() != nil {
		err = operation.Err()
	}
	cancel()
	return err
}

func (r *Runner) watch(ctx context.Context, conn Transport, devices []farm.Device, out chan<- Event, refresh <-chan struct{}) (monitored time.Duration, err error) {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	var connectedAt time.Time
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = conn.Close(); close(closed) })
	defer func() {
		// Closing on cancellation can also surface as a read/write error.
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if !connectedAt.IsZero() {
			monitored = r.clock().Sub(connectedAt)
		}
		cancel()
		_ = conn.Close()
		workers.Wait()
		if !stop() {
			<-closed
		}
	}()
	known := map[string]bool{}
	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, d.ID)
		known[d.ID] = true
	}
	if err := r.write(ctx, conn, writeRequest{subscribe: ids, refresh: ids}); err != nil {
		return 0, err
	}
	if !emit(ctx, out, Event{Kind: EventConnection, Connected: true}) {
		return 0, ctx.Err()
	}
	connectedAt = r.clock()

	pollEvery, pingEvery := r.PollInterval, r.PingInterval
	if pollEvery == 0 {
		pollEvery = 60 * time.Second
	}
	if pingEvery == 0 {
		pingEvery = 25 * time.Second
	}
	pollTicks, pingTicks := r.pollTicks, r.pingTicks
	if pollTicks == nil {
		ticker := time.NewTicker(pollEvery)
		defer ticker.Stop()
		pollTicks = ticker.C
	}
	if pingTicks == nil {
		ticker := time.NewTicker(pingEvery)
		defer ticker.Stop()
		pingTicks = ticker.C
	}
	reports := make(chan report, 1)
	discoveries := make(chan discovery)
	writes := make(chan writeRequest)
	written := make(chan error, 1)
	workers.Add(3)
	go func() {
		defer workers.Done()
		for {
			id, body, err := conn.Read()
			select {
			case reports <- report{id, body, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pollTicks:
				latest, err := r.API.Devices(ctx, r.Token)
				select {
				case discoveries <- discovery{latest, err}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-writes:
				err := r.write(ctx, conn, request)
				select {
				case written <- err:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}
	}()
	var added []string
	manual, pingDue, busy := false, false, false
	for {
		// Requests coalesce while a write is pending. Pings take the next available
		// slot; newly discovered subscriptions are retained until actually submitted.
		var submit chan<- writeRequest
		var request writeRequest
		if !busy {
			if pingDue {
				request.ping = true
				submit = writes
			} else if manual || len(added) > 0 {
				request.subscribe = append([]string(nil), added...)
				request.refresh = request.subscribe
				if manual {
					request.refresh = append([]string(nil), ids...)
					for _, id := range added {
						if !slices.Contains(request.refresh, id) {
							request.refresh = append(request.refresh, id)
						}
					}
				}
				submit = writes
			}
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case report := <-reports:
			if report.err != nil {
				return 0, report.err
			}
			if !emit(ctx, out, Event{Kind: EventReport, ID: report.id, Payload: report.body}) {
				return 0, ctx.Err()
			}
		case _, open := <-refresh:
			if !open {
				refresh = nil
			} else {
				manual = true
			}
		case <-pingTicks:
			pingDue = true
		case submit <- request:
			busy = true
			if request.ping {
				pingDue = false
			} else {
				manual = false
				added = nil
			}
		case err := <-written:
			busy = false
			if err != nil {
				return 0, err
			}
		case result := <-discoveries:
			if errors.Is(result.err, cloud.ErrAuth) || errors.Is(result.err, cloud.ErrBlocked) || errors.Is(result.err, cloud.ErrRateLimited) {
				return 0, result.err
			}
			if result.err != nil {
				if !emit(ctx, out, Event{Kind: EventProblem, Problem: "Device availability refresh failed; telemetry connection is still active"}) {
					return 0, ctx.Err()
				}
				continue
			}
			// Publish discovery before subscribing so new reports follow their device event.
			if !emit(ctx, out, Event{Kind: EventDevices, Devices: result.devices}) {
				return 0, ctx.Err()
			}
			ids = ids[:0]
			for _, d := range result.devices {
				ids = append(ids, d.ID)
				if !known[d.ID] {
					added = append(added, d.ID)
					known[d.ID] = true
				}
			}
		}
	}
}

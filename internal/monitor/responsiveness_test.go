package monitor

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/mqtt"
)

type apiFunc func(context.Context, string) ([]farm.Device, error)

func (f apiFunc) Devices(ctx context.Context, token string) ([]farm.Device, error) {
	return f(ctx, token)
}

type operation struct {
	name string
	ids  []string
}
type controlledTransport struct {
	closed             chan struct{}
	once               sync.Once
	reports            chan report
	operations         chan operation
	onRefresh          func([]string) error
	active, concurrent atomic.Int32
}

func newControlledTransport() *controlledTransport {
	return &controlledTransport{closed: make(chan struct{}), reports: make(chan report, 8), operations: make(chan operation, 32)}
}
func (f *controlledTransport) record(name string, ids []string, run func() error) error {
	if f.active.Add(1) != 1 {
		f.concurrent.Store(1)
	}
	defer f.active.Add(-1)
	f.operations <- operation{name, append([]string(nil), ids...)}
	if run != nil {
		return run()
	}
	return nil
}
func (f *controlledTransport) Subscribe(ids []string) error { return f.record("subscribe", ids, nil) }
func (f *controlledTransport) Refresh(ids []string) error {
	return f.record("refresh", ids, func() error {
		if f.onRefresh != nil {
			return f.onRefresh(ids)
		}
		return nil
	})
}
func (f *controlledTransport) Ping() error { return f.record("ping", nil, nil) }
func (f *controlledTransport) Read() (string, []byte, error) {
	select {
	case r := <-f.reports:
		return r.id, r.body, r.err
	case <-f.closed:
		return "", nil, io.EOF
	}
}
func (f *controlledTransport) Close() error { f.once.Do(func() { close(f.closed) }); return nil }

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("operation did not respond within one second")
	}
}
func nextOperation(t *testing.T, f *controlledTransport, name string) operation {
	t.Helper()
	select {
	case op := <-f.operations:
		if op.name != name {
			t.Fatalf("operation %s, want %s", op.name, name)
		}
		return op
	case <-time.After(time.Second):
		t.Fatalf("missing %s within one second", name)
	}
	return operation{}
}

type sessionResult struct {
	monitored time.Duration
	err       error
}

func startWatch(t *testing.T, r *Runner, f *controlledTransport) (context.CancelFunc, chan Event, chan struct{}, <-chan sessionResult) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 32)
	refresh := make(chan struct{})
	done := make(chan sessionResult, 1)
	go func() {
		duration, err := r.watch(ctx, f, []farm.Device{{ID: "TEST", Online: true}}, events, refresh)
		done <- sessionResult{duration, err}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-f.closed:
		case <-time.After(time.Second):
			t.Error("transport was not closed")
		}
	})
	connection := receive(t, events, "connection")
	if !connection.Connected {
		t.Fatal("session did not connect")
	}
	nextOperation(t, f, "subscribe")
	nextOperation(t, f, "refresh")
	return cancel, events, refresh, done
}
func finishWatch(t *testing.T, done <-chan sessionResult) sessionResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not join pending workers")
	}
	return sessionResult{}
}

func TestHealthySessionsResetBackoffButShortFailuresDoNot(t *testing.T) {
	var attempts, clockCalls int
	var delays []time.Duration
	durations := []time.Duration{time.Second, 2 * time.Minute, time.Second}
	base := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	r := Runner{API: fakeAPI{}, RetryMin: time.Second, RetryMax: 8 * time.Second,
		now: func() time.Time {
			call := clockCalls
			clockCalls++
			start := base.Add(time.Duration(call/2) * time.Hour)
			if call%2 == 1 {
				return start.Add(durations[call/2])
			}
			return start
		},
		wait: func(_ context.Context, delay time.Duration) bool { delays = append(delays, delay); return true },
		Dial: func(context.Context) (Transport, error) {
			attempts++
			if attempts <= 2 {
				return nil, io.EOF
			}
			if attempts == 6 {
				return nil, mqtt.ErrAuth
			}
			return &fakeTransport{done: make(chan struct{}), refresh: make(chan []string, 1), fail: true}, nil
		},
	}
	r.Run(context.Background(), make(chan Event, 32), nil)
	expected := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, time.Second, 2 * time.Second}
	if len(delays) != len(expected) {
		t.Fatalf("retry delays: %v", delays)
	}
	for i, minimum := range expected {
		if delays[i] < minimum || delays[i] >= minimum+minimum/4 {
			t.Fatalf("delay %d = %s, want %s plus up to 25%% jitter", i, delays[i], minimum)
		}
	}
	if clockCalls != 6 {
		t.Fatalf("session clock calls: %d", clockCalls)
	}
}

func TestBlockedDiscoveryDoesNotStarveReportsOrKeepalives(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	poll, ping := make(chan time.Time), make(chan time.Time)
	f := newControlledTransport()
	r := Runner{pollTicks: poll, pingTicks: ping, API: apiFunc(func(ctx context.Context, _ string) ([]farm.Device, error) {
		close(entered)
		defer close(exited)
		<-ctx.Done()
		return nil, ctx.Err()
	})}
	cancel, events, _, done := startWatch(t, &r, f)
	sendTick(t, poll)
	awaitSignal(t, entered)
	for _, body := range []string{"first", "second"} {
		f.reports <- report{id: "TEST", body: []byte(body)}
		if got := receive(t, events, "report"); string(got.Payload) != body {
			t.Fatal("report order changed during discovery")
		}
	}
	sendTick(t, ping)
	nextOperation(t, f, "ping")
	cancel()
	if result := finishWatch(t, done); !errors.Is(result.err, context.Canceled) {
		t.Fatal(result.err)
	}
	awaitSignal(t, exited)
	if f.active.Load() != 0 || f.concurrent.Load() != 0 {
		t.Fatal("pending or concurrent writes after cancellation")
	}
}

func TestBlockedRefreshKeepsReportsLiveAndPrioritizesPing(t *testing.T) {
	entered, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{})
	f := newControlledTransport()
	calls := 0
	f.onRefresh = func([]string) error {
		calls++
		if calls == 2 {
			close(entered)
			defer close(exited)
			select {
			case <-release:
			case <-f.closed:
				return io.EOF
			}
		}
		return nil
	}
	poll, ping := make(chan time.Time), make(chan time.Time)
	latest := []farm.Device{{ID: "TEST", Online: true}, {ID: "NEW", Online: true}}
	r := Runner{pollTicks: poll, pingTicks: ping, API: apiFunc(func(context.Context, string) ([]farm.Device, error) { return latest, nil })}
	cancel, events, refresh, done := startWatch(t, &r, f)
	requestRefresh(t, refresh)
	nextOperation(t, f, "refresh")
	awaitSignal(t, entered)
	sendTick(t, poll)
	if got := receive(t, events, "devices"); !reflect.DeepEqual(got.Devices, latest) {
		t.Fatal("discovery blocked behind refresh")
	}
	sendTick(t, ping)
	for range 3 {
		requestRefresh(t, refresh)
	}
	for _, body := range []string{"one", "two"} {
		f.reports <- report{id: "TEST", body: []byte(body)}
		if got := receive(t, events, "report"); string(got.Payload) != body {
			t.Fatal("report lost or reordered during refresh")
		}
	}
	close(release)
	nextOperation(t, f, "ping")
	if got := nextOperation(t, f, "subscribe"); !reflect.DeepEqual(got.ids, []string{"NEW"}) {
		t.Fatal(got)
	}
	if got := nextOperation(t, f, "refresh"); !reflect.DeepEqual(got.ids, []string{"TEST", "NEW"}) {
		t.Fatal(got)
	}
	cancel()
	finishWatch(t, done)
	awaitSignal(t, exited)
	if f.concurrent.Load() != 0 || f.active.Load() != 0 || calls != 3 {
		t.Fatal("writes overlapped, leaked, or failed to coalesce")
	}
}

func TestStalledRefreshIsBoundedAndCancellationJoinsWorkers(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "timeout"}[timeout], func(t *testing.T) {
			entered, exited := make(chan struct{}), make(chan struct{})
			f := newControlledTransport()
			calls := 0
			f.onRefresh = func([]string) error {
				calls++
				if calls == 2 {
					close(entered)
					defer close(exited)
					<-f.closed
					return io.EOF
				}
				return nil
			}
			ping := make(chan time.Time)
			r := Runner{API: fakeAPI{}, pingTicks: ping, WriteTimeout: 500 * time.Millisecond}
			cancel, events, refresh, done := startWatch(t, &r, f)
			requestRefresh(t, refresh)
			awaitSignal(t, entered)
			sendTick(t, ping)
			f.reports <- report{id: "TEST", body: []byte("live")}
			if got := receive(t, events, "report"); string(got.Payload) != "live" {
				t.Fatal("blocked write starved report delivery")
			}
			if !timeout {
				cancel()
			}
			result := finishWatch(t, done)
			if timeout && result.err == nil {
				t.Fatalf("write was not bounded: %v", result.err)
			}
			if !timeout && !errors.Is(result.err, context.Canceled) {
				t.Fatal(result.err)
			}
			awaitSignal(t, exited)
			if f.concurrent.Load() != 0 || f.active.Load() != 0 {
				t.Fatal("write worker did not exit cleanly")
			}
		})
	}
}

func TestDiscoveryErrorPoliciesAndRecovery(t *testing.T) {
	for _, failure := range []error{cloud.ErrAuth, cloud.ErrBlocked, cloud.ErrRateLimited, io.ErrUnexpectedEOF} {
		t.Run(failure.Error(), func(t *testing.T) {
			poll, ping := make(chan time.Time), make(chan time.Time)
			calls := 0
			r := Runner{pollTicks: poll, pingTicks: ping, API: apiFunc(func(context.Context, string) ([]farm.Device, error) {
				calls++
				if calls == 1 {
					return nil, failure
				}
				return []farm.Device{{ID: "TEST"}}, nil
			})}
			f := newControlledTransport()
			cancel, events, _, done := startWatch(t, &r, f)
			sendTick(t, poll)
			if failure != io.ErrUnexpectedEOF {
				if result := finishWatch(t, done); !errors.Is(result.err, failure) {
					t.Fatal(result.err)
				}
				return
			}
			receive(t, events, "problem")
			f.reports <- report{id: "TEST", body: []byte("still live")}
			receive(t, events, "report")
			sendTick(t, ping)
			nextOperation(t, f, "ping")
			sendTick(t, poll)
			receive(t, events, "devices")
			cancel()
			finishWatch(t, done)
		})
	}
}

func sendTick(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("monitor did not accept tick")
	}
}
func requestRefresh(t *testing.T, refresh chan<- struct{}) {
	t.Helper()
	select {
	case refresh <- struct{}{}:
	case <-time.After(time.Second):
		t.Fatal("monitor did not accept refresh")
	}
}

func TestWriteDeadlineClosesAndReleasesTransport(t *testing.T) {
	f := newControlledTransport()
	f.onRefresh = func([]string) error { <-f.closed; return io.EOF }
	r := Runner{WriteTimeout: 20 * time.Millisecond}
	if err := r.write(context.Background(), f, writeRequest{refresh: []string{"TEST"}}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write timeout: %v", err)
	}
	if f.active.Load() != 0 {
		t.Fatal("timed out write did not exit")
	}
}

func TestFailedBootstrapRetainsCappedBackoff(t *testing.T) {
	clock := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	var delays []time.Duration
	attempts := 0
	r := Runner{API: fakeAPI{}, RetryMin: time.Second, RetryMax: 4 * time.Second,
		now:  func() time.Time { return clock },
		wait: func(_ context.Context, delay time.Duration) bool { delays = append(delays, delay); return true },
		Dial: func(context.Context) (Transport, error) {
			attempts++
			if attempts <= 2 {
				return nil, io.EOF
			}
			if attempts == 5 {
				return nil, mqtt.ErrAuth
			}
			f := newControlledTransport()
			f.onRefresh = func([]string) error { clock = clock.Add(3 * time.Minute); return io.EOF }
			return f, nil
		},
	}
	r.Run(context.Background(), make(chan Event, 32), nil)
	expected := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	if len(delays) != len(expected) {
		t.Fatalf("retry delays: %v", delays)
	}
	for i, minimum := range expected {
		if delays[i] < minimum || delays[i] >= minimum+minimum/4 {
			t.Fatalf("failed bootstrap reset or exceeded capped backoff: %v", delays)
		}
	}
}

func TestCancellationDuringInitialRefreshDoesNotEstablishHealthySession(t *testing.T) {
	entered := make(chan struct{})
	f := newControlledTransport()
	f.onRefresh = func([]string) error { close(entered); <-f.closed; return io.EOF }
	r := Runner{API: fakeAPI{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 1)
	done := make(chan sessionResult, 1)
	go func() {
		duration, err := r.watch(ctx, f, []farm.Device{{ID: "TEST"}}, events, nil)
		done <- sessionResult{duration, err}
	}()
	awaitSignal(t, entered)
	cancel()
	result := finishWatch(t, done)
	if !errors.Is(result.err, context.Canceled) || result.monitored != 0 || len(events) != 0 || f.active.Load() != 0 {
		t.Fatalf("canceled bootstrap became a healthy session or left a pending write: %+v", result)
	}
}

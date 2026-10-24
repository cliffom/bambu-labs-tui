package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/farm"
)

// TestDemoRunoutScenario verifies the demo's modal incident without a timed wait.
func TestDemoRunoutScenario(t *testing.T) {
	print := map[string]any{"gcode_state": "PAUSE"}
	addDemoRunout(print)
	payload, err := json.Marshal(map[string]any{"print": print})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s := farm.New(10)
	s.Cloud(true, "", now)
	s.Discover([]farm.Device{{ID: "demo", Online: true}}, now)
	if err := s.Report("demo", payload, now); err != nil {
		t.Fatal(err)
	}
	if len(s.Attention()) != 3 || s.Printers["demo"].Stage != "Filament runout" {
		t.Fatal("demo did not produce related runout/state notices")
	}
}

type fakeAPI struct{ err error }

func (f fakeAPI) Devices(context.Context, string) ([]farm.Device, error) {
	return []farm.Device{{ID: "TEST", Name: "Test", Online: true}}, f.err
}

type fakeTransport struct {
	done    chan struct{}
	once    sync.Once
	refresh chan []string
	fail    bool
}

func (f *fakeTransport) Subscribe([]string) error { return nil }
func (f *fakeTransport) Refresh(ids []string) error {
	f.refresh <- append([]string(nil), ids...)
	return nil
}
func (f *fakeTransport) Ping() error { return nil }
func (f *fakeTransport) Read() (string, []byte, error) {
	if f.fail {
		return "", nil, io.EOF
	}
	<-f.done
	return "", nil, io.EOF
}
func (f *fakeTransport) Close() error { f.once.Do(func() { close(f.done) }); return nil }
func receive(t *testing.T, events <-chan Event, kind string) Event {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatal("event stream closed early")
			}
			if e.Kind == kind {
				return e
			}
		case <-timeout:
			t.Fatal("timed out waiting for " + kind)
		}
	}
}
func TestReconnectResubscribesAndRefreshes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 32)
	refresh := make(chan struct{}, 1)
	refreshed := make(chan []string, 8)
	var calls atomic.Int32
	r := Runner{API: fakeAPI{}, RetryMin: time.Millisecond, RetryMax: 2 * time.Millisecond, Dial: func(context.Context) (Transport, error) {
		n := calls.Add(1)
		return &fakeTransport{done: make(chan struct{}), refresh: refreshed, fail: n == 1}, nil
	}}
	done := make(chan struct{})
	go func() { r.Run(ctx, events, refresh); close(done) }()
	for range 2 {
		select {
		case ids := <-refreshed:
			if len(ids) != 1 || ids[0] != "TEST" {
				t.Fatal(ids)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("missing full refresh")
		}
	}
	if calls.Load() < 2 {
		t.Fatal("no reconnect")
	}
	refresh <- struct{}{}
	select {
	case <-refreshed:
	case <-time.After(2 * time.Second):
		t.Fatal("manual refresh ignored")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}
func TestAuthFailureStopsRetries(t *testing.T) {
	for _, source := range []string{"http", "mqtt"} {
		t.Run(source, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			events := make(chan Event, 8)
			var calls atomic.Int32
			r := Runner{API: fakeAPI{}, RetryMin: time.Millisecond, Dial: func(context.Context) (Transport, error) { calls.Add(1); return nil, cloud.ErrAuth }}
			if source == "http" {
				r.API = fakeAPI{cloud.ErrAuth}
			}
			done := make(chan struct{})
			go func() { r.Run(ctx, events, make(chan struct{})); close(done) }()
			e := receive(t, events, "connection")
			if e.Connected || e.Problem == "" {
				t.Fatal(e)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("auth retry loop")
			}
			if calls.Load() > 1 {
				t.Fatal("repeated invalid login")
			}
		})
	}
}
func TestCancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan Event, 8)
	r := Runner{API: fakeAPI{errors.New("unreachable")}, RetryMin: time.Hour}
	done := make(chan struct{})
	go func() { r.Run(ctx, events, make(chan struct{})); close(done) }()
	receive(t, events, "connection")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("backoff ignored cancellation")
	}
}
func TestDemoProducesBothModels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 8)
	go Demo(ctx, events, make(chan struct{}))
	e := receive(t, events, "devices")
	if len(e.Devices) != 2 || e.Devices[0].Model != "H2D" || e.Devices[1].Model != "X2D" {
		t.Fatal(e)
	}
	s := farm.New(10)
	s.Discover(e.Devices, time.Now())
	for range 2 {
		e = receive(t, events, "report")
		if err := s.Report(e.ID, e.Payload, e.At); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range s.Printers {
		if len(p.Nozzles) != 2 || len(p.AMS) == 0 {
			t.Fatal("incomplete demo")
		}
	}
}

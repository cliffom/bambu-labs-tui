package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notification_state"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

const syntheticPrinter = "SYNTHETIC_PRIVATE_SERIAL"

func browserInput(state string, reviewed bool) notifications.Input {
	key := syntheticPrinter + "/state"
	conditions := map[string]farm.NotificationCondition{}
	if state == farm.StatePaused {
		conditions[key] = farm.NotificationCondition{Family: farm.NotificationStateFamily, State: state, Level: farm.LevelWarning, Eligible: !reviewed, Reviewed: reviewed}
	}
	return notifications.Input{Active: true, Snapshot: farm.NotificationSnapshot{syntheticPrinter: {StateKnown: true, PrintKnown: true, HMSKnown: true, Job: farm.NotificationJob{Task: "SYNTHETIC_PRIVATE_TASK", State: state}, Conditions: conditions}}, Policies: map[string]notifications.Policy{syntheticPrinter: {PrinterName: "Synthetic printer", Completion: true, Allowed: map[string]bool{key: true}}}}
}

func browserFixture(t *testing.T) (*browserWorker, notification_state.Store, *time.Time) {
	t.Helper()
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	ledger := notification_state.Store{Path: filepath.Join(t.TempDir(), "browser.json")}
	// Direct serial calls keep policy/clock tests deterministic, without a goroutine.
	b := &browserWorker{ctx: context.Background(), ledger: ledger, now: func() time.Time { return at }, seen: map[string]browserOccurrence{}}
	return b, ledger, &at
}

func observeBrowser(b *browserWorker, input notifications.Input) { b.latest = input; b.observe(input) }

func TestBrowserClaimsLifetimesAndRestart(t *testing.T) {
	b, ledger, _ := browserFixture(t)
	observeBrowser(b, browserInput(farm.StateRunning, false))
	baseline := b.claim(browserRequest{Baseline: true}).Cursor
	observeBrowser(b, browserInput(farm.StatePaused, false))
	first := b.claim(browserRequest{After: baseline})
	if len(first.Events) != 1 {
		t.Fatalf("first claim: %+v", first)
	}
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("duplicate tab replayed")
	}
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("poll replayed")
	}
	// Confirmed clear and recurrence between browser polls, at identical times.
	observeBrowser(b, browserInput(farm.StateRunning, false))
	observeBrowser(b, browserInput(farm.StatePaused, false))
	recurrence := b.claim(browserRequest{After: first.Cursor})
	if len(recurrence.Events) != 1 || recurrence.Events[0].ID == first.Events[0].ID {
		t.Fatalf("recurrence: %+v", recurrence)
	}
	b2, _, _ := browserFixture(t)
	b2.ledger = ledger
	observeBrowser(b2, browserInput(farm.StatePaused, false))
	if len(b2.claim(browserRequest{}).Events) != 0 {
		t.Fatal("restart replayed reserved outcome")
	}
	// Payload contains only display data and opaque identities, not protocol IDs.
	body, _ := json.Marshal(first)
	if strings.Contains(string(body), "PRIVATE") {
		t.Fatalf("private identity leaked: %s", body)
	}
}

func TestBrowserCompletionRequiresObservedJob(t *testing.T) {
	for _, test := range []struct {
		name   string
		states []string
		want   int
	}{
		{"startup finish", []string{farm.StateFinished}, 0},
		{"progress is not completion", []string{farm.StateRunning, farm.StateRunning}, 0},
		{"failed without actionable condition", []string{farm.StateRunning, farm.StateFailed}, 0},
		{"confirmed finish", []string{farm.StateRunning, farm.StateFinished}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, _, _ := browserFixture(t)
			for _, state := range test.states {
				observeBrowser(b, browserInput(state, false))
			}
			result := b.claim(browserRequest{})
			want := test.want
			if len(result.Events) != want {
				t.Fatalf("events: %+v", result)
			}
			if want == 1 && !strings.Contains(result.Events[0].Body, "finished") {
				t.Fatal("completion body")
			}
		})
	}
	b, _, _ := browserFixture(t)
	input := browserInput(farm.StateRunning, false)
	p := input.Snapshot[syntheticPrinter]
	p.Job.Task = ""
	input.Snapshot[syntheticPrinter] = p
	observeBrowser(b, input)
	p.Job.State = farm.StateFinished
	input.Snapshot[syntheticPrinter] = p
	observeBrowser(b, input)
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("missing identity guessed completion")
	}
}

func TestBrowserPolicyConsumesWithoutBacklog(t *testing.T) {
	for _, mute := range []string{"quiet", "snoozed", "category", "reviewed", "completion"} {
		t.Run(mute, func(t *testing.T) {
			b, _, _ := browserFixture(t)
			observeBrowser(b, browserInput(farm.StateRunning, false))
			input := browserInput(farm.StatePaused, mute == "reviewed")
			policy := input.Policies[syntheticPrinter]
			switch mute {
			case "quiet":
				input.Quiet = true
			case "snoozed":
				policy.Snoozed = true
			case "category":
				policy.Allowed = map[string]bool{}
			case "completion":
				input = browserInput(farm.StateFinished, false)
				policy.Completion = false
			}
			input.Policies[syntheticPrinter] = policy
			observeBrowser(b, input)
			if len(b.claim(browserRequest{}).Events) != 0 {
				t.Fatal("muted event sent")
			}
			state := farm.StatePaused
			if mute == "completion" {
				state = farm.StateFinished
			}
			observeBrowser(b, browserInput(state, false))
			if len(b.claim(browserRequest{}).Events) != 0 {
				t.Fatal("muted event replayed")
			}
		})
	}
}

func TestBrowserBaselinesUnknownAndExpiry(t *testing.T) {
	b, _, at := browserFixture(t)
	observeBrowser(b, browserInput(farm.StatePaused, false))
	baseline := b.claim(browserRequest{Baseline: true})
	if len(baseline.Events) != 0 || len(b.claim(browserRequest{After: baseline.Cursor}).Events) != 0 {
		t.Fatal("baseline replayed existing incident")
	}
	observeBrowser(b, notifications.Input{Active: true, Snapshot: farm.NotificationSnapshot{}})
	*at = at.Add(browserEventLifetime + time.Second)
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("expired unknown incident replayed")
	}
	observeBrowser(b, browserInput(farm.StateRunning, false))
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{After: baseline.Cursor}).Events) != 1 {
		t.Fatal("fresh recurrence lost")
	}
}

type (
	failingBrowserLedger struct{}
	blockedBrowserLedger struct {
		entered, release chan struct{}
		once             sync.Once
	}
)

func (failingBrowserLedger) ObserveActivity(farm.NotificationSnapshot) (notification_state.Observation, error) {
	return notification_state.Observation{}, errors.New("PRIVATE ERROR")
}

func (failingBrowserLedger) ReserveBatch([]string) ([]string, error) {
	return nil, errors.New("PRIVATE ERROR")
}

func TestBrowserWorkerConcurrentClaimsAndProtections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ledger := notification_state.Store{Path: filepath.Join(t.TempDir(), "browser.json")}
	s := New(ctx, ledger)
	defer s.Close()
	s.SubmitNotifications(browserInput(farm.StatePaused, false))
	handler := s.handler("127.0.0.1:8080", http.NotFoundHandler())
	request := func(body, origin, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/notifications", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		body, origin, token string
		status              int
	}{
		{`{}`, "http://evil.example", s.token, 403},
		{`{}`, "http://127.0.0.1:8080", "bad", 403},
		{`{"unknown":true}`, "http://127.0.0.1:8080", s.token, 400},
		{`{} {}`, "http://127.0.0.1:8080", s.token, 400},
		{`null`, "http://127.0.0.1:8080", s.token, 400},
		{strings.Repeat("x", maxCommandBytes+1), "http://127.0.0.1:8080", s.token, 400},
	} {
		if w := request(tc.body, tc.origin, tc.token); w.Code != tc.status {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	var wg sync.WaitGroup
	results := make(chan browserResult, 2)
	for range 2 {
		wg.Go(func() {
			w := request(`{}`, "http://127.0.0.1:8080", s.token)
			var result browserResult
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Error(err)
				return
			}
			results <- result
		})
	}
	wg.Wait()
	close(results)
	total := 0
	for result := range results {
		total += len(result.Events)
	}
	if total != 1 {
		t.Fatalf("concurrent tabs claimed %d", total)
	}
}

func TestBrowserStorageFailureIsAllowlisted(t *testing.T) {
	b, _, _ := browserFixture(t)
	b.ledger = failingBrowserLedger{}
	observeBrowser(b, browserInput(farm.StatePaused, false))
	result := b.claim(browserRequest{})
	if result.Error != browserLedgerError || len(result.Events) != 0 {
		t.Fatalf("storage failure: %+v", result)
	}
}

func TestBrowserRateLimitAndExpiry(t *testing.T) {
	b, _, at := browserFixture(t)
	for range browserBatchLimit {
		observeBrowser(b, browserInput(farm.StateRunning, false))
		observeBrowser(b, browserInput(farm.StatePaused, false))
		if len(b.claim(browserRequest{}).Events) != 1 {
			t.Fatal("rate slot missing")
		}
	}
	observeBrowser(b, browserInput(farm.StateRunning, false))
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("rate limit exceeded")
	}
	*at = at.Add(browserRateWindow)
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{}).Events) != 0 {
		t.Fatal("rate limit backlog replayed")
	}
	observeBrowser(b, browserInput(farm.StateRunning, false))
	observeBrowser(b, browserInput(farm.StatePaused, false))
	if len(b.claim(browserRequest{}).Events) != 1 {
		t.Fatal("rate limit did not release")
	}
}

func TestBrowserRateLimitConsumesExcessAcrossTabsAndRestart(t *testing.T) {
	for _, tc := range []struct {
		name string
		used int
	}{
		{"full window", browserBatchLimit},
		{"one remaining slot", browserBatchLimit - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			used := tc.used
			b, ledger, at := browserFixture(t)
			for range used {
				observeBrowser(b, browserInput(farm.StateRunning, false))
				observeBrowser(b, browserInput(farm.StatePaused, false))
				if result := b.claim(browserRequest{}); result.Error != "" || len(result.Events) != 1 {
					t.Fatalf("initial attempt: %+v", result)
				}
			}
			olderTab := browserRequest{After: b.sequence}
			*at = at.Add(59 * time.Second)
			observeBrowser(b, browserInput(farm.StateRunning, false))
			input := browserInput(farm.StatePaused, false)
			input.Snapshot["SYNTHETIC_SECOND_PRINTER"] = input.Snapshot[syntheticPrinter]
			input.Policies["SYNTHETIC_SECOND_PRINTER"] = input.Policies[syntheticPrinter]
			observeBrowser(b, input)
			first := b.claim(olderTab)
			if first.Error != "" || len(first.Events) != browserBatchLimit-used {
				t.Fatalf("rate-limited claim: %+v", first)
			}
			if len(b.attempts) != browserBatchLimit {
				t.Fatalf("suppressed events consumed rate slots: %d attempts", len(b.attempts))
			}
			// The first tab advances its cursor, but another tab still has the
			// old cursor when slots reopen before these events expire.
			*at = at.Add(2 * time.Second)
			if result := b.claim(olderTab); result.Error != "" || len(result.Events) != 0 {
				t.Fatalf("older tab replayed rate-limited events: %+v", result)
			}
			observeBrowser(b, input)
			if result := b.claim(olderTab); result.Error != "" || len(result.Events) != 0 {
				t.Fatalf("observation restored suppressed events: %+v", result)
			}
			restarted, _, _ := browserFixture(t)
			restarted.ledger = ledger
			observeBrowser(restarted, input)
			if result := restarted.claim(browserRequest{}); result.Error != "" || len(result.Events) != 0 {
				t.Fatalf("restart replayed rate-limited events: %+v", result)
			}
			observeBrowser(b, browserInput(farm.StateRunning, false))
			observeBrowser(b, browserInput(farm.StatePaused, false))
			if result := b.claim(browserRequest{After: first.Cursor}); result.Error != "" || len(result.Events) != 1 {
				t.Fatalf("fresh recurrence lost after rate window reopened: %+v", result)
			}
		})
	}
}

func TestBrowserRateLimitStorageFailurePausesDelivery(t *testing.T) {
	b, _, at := browserFixture(t)
	observeBrowser(b, browserInput(farm.StatePaused, false))
	for range browserBatchLimit {
		b.attempts = append(b.attempts, *at)
	}
	b.ledger = failingBrowserLedger{}
	if result := b.claim(browserRequest{}); result.Error != browserLedgerError || len(result.Events) != 0 || !b.failed.Load() {
		t.Fatalf("suppression storage failure did not pause delivery: %+v", result)
	}
}

func (b *blockedBrowserLedger) ObserveActivity(farm.NotificationSnapshot) (notification_state.Observation, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return notification_state.Observation{}, nil
}
func (*blockedBrowserLedger) ReserveBatch([]string) ([]string, error) { return nil, nil }

func TestBrowserQueueOverflowFailsClosedWithoutBlockingReducer(t *testing.T) {
	ledger := &blockedBrowserLedger{entered: make(chan struct{}), release: make(chan struct{})}
	b := newBrowserWorker(t.Context(), ledger)
	defer func() { close(ledger.release); b.cancel(); <-b.done }()
	b.submit(browserInput(farm.StateRunning, false))
	<-ledger.entered
	for i := range browserQueueCapacity + 1 {
		input := browserInput(farm.StatePaused, false)
		input.Demo = i%2 == 0
		b.submit(input)
	}
	if !b.failed.Load() {
		t.Fatal("overflow did not fail closed")
	}
	if result := b.claim(browserRequest{}); result.Error != browserLedgerError || len(result.Events) != 0 {
		t.Fatalf("overflow claim: %+v", result)
	}
}

func TestBrowserQueuedFutureSnapshotPreventsStaleClaim(t *testing.T) {
	b, _, _ := browserFixture(t)
	observeBrowser(b, browserInput(farm.StatePaused, false))
	b.submitted = 1 // another reducer transition is queued but not yet observed
	if result := b.claim(browserRequest{}); len(result.Events) != 0 || result.Cursor != 0 {
		t.Fatal("claim advanced past events before lifecycle queue drained")
	}
}

func TestBrowserClaimsDoNotConsumeNativeLedger(t *testing.T) {
	b, browser, _ := browserFixture(t)
	native := notification_state.Store{Path: browser.Path + ".native"}
	input := browserInput(farm.StatePaused, false)
	candidates, err := native.Observe(input.Snapshot)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("native observation: %v %+v", err, candidates)
	}
	observeBrowser(b, input)
	if len(b.claim(browserRequest{}).Events) != 1 {
		t.Fatal("browser claim missing")
	}
	pending, err := native.Observe(input.Snapshot)
	if err != nil || len(pending) != 1 || pending[0].ID != candidates[0].ID {
		t.Fatal("browser consumed native identity")
	}
}

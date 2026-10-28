package session_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/session"
	"github.com/cliffom/bambu-tui/internal/webserver"
)

type recordingSink struct{ snapshots []session.DisplayState }

func (s *recordingSink) PublishDisplay(state session.DisplayState) {
	s.snapshots = append(s.snapshots, state)
}

const faultReport = `{"print":{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}}`

func fixture(t *testing.T) (session.Session, time.Time) {
	t.Helper()
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := session.New(farm.New(10), false)
	for _, event := range []monitor.Event{
		{Kind: monitor.EventConnection, Connected: true, At: at},
		{Kind: monitor.EventDevices, Devices: []farm.Device{{ID: "SYNTHETIC_SERIAL", Name: "Test printer", Online: true}}, At: at},
		{Kind: monitor.EventReport, ID: "SYNTHETIC_SERIAL", Payload: []byte(faultReport), At: at},
	} {
		if err := s.ApplyEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.Farm.Active) < 2 {
		t.Fatal("fixture needs independent conditions")
	}
	return s, at
}

func report(t *testing.T, s *session.Session, payload string, at time.Time) {
	t.Helper()
	if err := s.ApplyEvent(monitor.Event{Kind: monitor.EventReport, ID: "SYNTHETIC_SERIAL", Payload: []byte(payload), At: at}); err != nil {
		t.Fatal(err)
	}
}

func decision(s *session.Session, target string, retry bool) string {
	c := webserver.Command{Target: target, Retry: retry, Reply: make(chan string, 1)}
	c.Reply <- string(s.ApplyDecision(session.DecisionCommand{Target: c.Target, Retry: c.Retry}, false, true))
	return <-c.Reply
}

func firstTarget(t *testing.T, s *session.Session) (string, string) {
	t.Helper()
	for key, target := range s.WebDecisions().Targets {
		return key, target
	}
	t.Fatal("no targets")
	return "", ""
}

func TestOccurrenceLifetimeWithoutTerminal(t *testing.T) {
	s, at := fixture(t)
	targets := s.WebDecisions().Targets
	key, target := firstTarget(t, &s)
	if decision(&s, target, false) != "applied" || !s.Farm.Active[key].Dismissed {
		t.Fatal("local hide rejected")
	}
	if len(s.Farm.Active) != len(targets) {
		t.Fatal("local decision resolved physical condition")
	}
	rev := s.Farm.ReviewRevision()
	if decision(&s, target, false) != "applied" || s.Farm.ReviewRevision() != rev {
		t.Fatal("duplicate changed review revision")
	}
	s.Acknowledge(key, true)
	s.RestoreDismissal(key)
	if s.WebDecisions().Targets[key] != target || s.Farm.Active[key].Dismissed || !s.Farm.Active[key].Acknowledged {
		t.Fatal("local reviews rotated occurrence")
	}
	// Missing families and reconnects cannot confirm physical resolution.
	report(t, &s, `{"print":{"mc_percent":20}}`, at)
	if err := s.ApplyEvent(monitor.Event{Kind: monitor.EventConnection, At: at}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyEvent(monitor.Event{Kind: monitor.EventConnection, Connected: true, At: at}); err != nil {
		t.Fatal(err)
	}
	if s.WebDecisions().Targets[key] != target {
		t.Fatal("partial report/reconnect lost occurrence")
	}
	// Clear and recur at the same timestamp, without publishing between reports.
	report(t, &s, `{"print":{"gcode_state":"RUNNING","print_error":0,"hms":[]}}`, at)
	report(t, &s, faultReport, at)
	if s.WebDecisions().Targets[key] == target || decision(&s, target, false) == "applied" || s.Farm.Active[key].Dismissed {
		t.Fatal("stale target applied to recurrence")
	}
	for _, gate := range []struct{ stopping, enabled bool }{{true, true}, {false, false}} {
		c := webserver.Command{Target: s.WebDecisions().Targets[key], Reply: make(chan string, 1)}
		c.Reply <- string(s.ApplyDecision(session.DecisionCommand{Target: c.Target, Retry: c.Retry}, gate.stopping, gate.enabled))
		if <-c.Reply == "applied" {
			t.Fatal("shutdown/disable accepted mutation")
		}
	}
}

func TestPersistenceCoalescesImmutableSnapshotsAndRetries(t *testing.T) {
	s, at := fixture(t)
	var reviews []map[string]farm.Review
	var snoozes []map[string]time.Time
	fail := true
	s.WithNoticePersistence(func(r map[string]farm.Review) error {
		reviews = append(reviews, r)
		if fail {
			return errors.New("full")
		}
		return nil
	}, nil)
	s.WithSnoozePersistence(nil, func(r map[string]time.Time) error { snoozes = append(snoozes, r); return nil }, nil)
	key, target := firstTarget(t, &s)
	decision(&s, target, false)
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	firstReview, firstSnooze := s.Persistence()
	if firstReview == nil || firstSnooze == nil || !s.PersistencePending() || s.WebDecisions().State != "saving" {
		t.Fatal("missing pending writes")
	}
	s.Acknowledge(key, true)
	s.SetSnooze("SYNTHETIC_SERIAL", 2*time.Hour, at)
	report(t, &s, `{"print":{"mc_percent":42}}`, at)
	if r, z := s.Persistence(); r != nil || z != nil {
		t.Fatal("overlapping store writes")
	}
	failed := firstReview()
	s.Complete(failed)
	s.Complete(firstSnooze())
	if reviews[0][key].Acknowledged || !snoozes[0]["SYNTHETIC_SERIAL"].Equal(at.Add(time.Hour)) {
		t.Fatal("in-flight snapshot changed")
	}
	latestReview, latestSnooze := s.Persistence()
	if latestReview == nil || latestSnooze == nil {
		t.Fatal("latest snapshots not coalesced")
	}
	s.Complete(failed) // Old completion cannot clear the newer in-flight write.
	if !s.ReviewWrites().InFlight {
		t.Fatal("stale completion accepted")
	}
	s.Complete(latestReview())
	s.Complete(latestSnooze())
	if s.WebDecisions().State != "failed" || !s.HasUnsavedPersistence() {
		t.Fatal("failed write reported saved")
	}
	s.Reconcile(at.Add(time.Second))
	if r, z := s.Persistence(); r != nil || z != nil {
		t.Fatal("automatic retry after failure")
	}
	decision(&s, target, false)
	if r, _ := s.Persistence(); r != nil {
		t.Fatal("duplicate implicitly retried")
	}
	fail = false
	if decision(&s, target, true) != "applied" {
		t.Fatal("explicit retry rejected")
	}
	retry, _ := s.Persistence()
	if retry == nil {
		t.Fatal("retry not scheduled")
	}
	s.Complete(retry())
	if s.HasUnsavedPersistence() || s.WebDecisions().State != "saved" || len(reviews) != 3 || len(snoozes) != 2 || !reflect.DeepEqual(reviews[2], s.Farm.Reviews()) {
		t.Fatal("latest complete snapshot not saved")
	}
}

func TestFailedLoadAndDemoNeverWrite(t *testing.T) {
	for _, demo := range []bool{false, true} {
		s, at := fixture(t)
		s.Demo = demo
		var loadErr error
		if !demo {
			loadErr = errors.New("unreadable")
		}
		s.WithNoticePersistence(func(map[string]farm.Review) error { t.Fatal("overwrote protected notices"); return nil }, loadErr)
		restored := map[string]time.Time{"SYNTHETIC_SERIAL": at.Add(time.Hour)}
		s.WithSnoozePersistence(restored, func(map[string]time.Time) error { t.Fatal("overwrote protected snoozes"); return nil }, loadErr)
		delete(restored, "SYNTHETIC_SERIAL")
		if len(s.Snoozes) != 1 {
			t.Fatal("startup snoozes aliased")
		}
		_, target := firstTarget(t, &s)
		decision(&s, target, false)
		s.SetSnooze("SYNTHETIC_SERIAL", 2*time.Hour, at)
		if r, z := s.Persistence(); r != nil || z != nil || s.HasUnsavedPersistence() {
			t.Fatal("session-only stores queued writes")
		}
		if decision(&s, target, true) == "applied" {
			t.Fatal("protected retry accepted")
		}
		if !strings.HasPrefix(s.WebDecisions().State, "session-only") {
			t.Fatal("protected store reported saved")
		}
	}
}

func TestSnapshotIsolationAndMalformedReport(t *testing.T) {
	s, at := fixture(t)
	sink := &recordingSink{}
	s.Publish(sink, at)
	original := sink.snapshots[0]
	if original.Decisions != "session-only" || original.Printers[0].ID == "SYNTHETIC_SERIAL" {
		t.Fatal("display exposed identity or wrong storage state")
	}
	key, target := firstTarget(t, &s)
	decision(&s, target, false)
	s.Publish(sink, at)
	if !reflect.DeepEqual(original, sink.snapshots[0]) || reflect.DeepEqual(original, sink.snapshots[1]) {
		t.Fatal("snapshot bytes not independent")
	}
	before := s.Farm.Active[key]
	if err := s.ApplyEvent(monitor.Event{Kind: monitor.EventReport, ID: "SYNTHETIC_SERIAL", Payload: []byte(`{"print":`), At: at}); err == nil {
		t.Fatal("malformed report accepted")
	}
	if s.Farm.Active[key] != before || s.WebDecisions().Targets[key] != target {
		t.Fatal("malformed report resolved occurrence")
	}
	s.Reconcile(at.Add(24 * time.Hour))
	if s.WebDecisions().Targets[key] != target {
		t.Fatal("elapsed time resolved occurrence")
	}
	s.Publish(sink, at.Add(24*time.Hour))
	if sink.snapshots[2].Printers[0].Fresh {
		t.Fatal("stale telemetry labeled current")
	}
}

func TestConfigSerializationAndSnoozeExpiry(t *testing.T) {
	s, at := fixture(t)
	var saved []config.Config
	s.WithConfigPersistence(func(c config.Config) error { saved = append(saved, c); return nil })
	cfg := config.Config{Alerts: config.DefaultAlerts()}
	cfg.Alerts.Printers = map[string]config.AlertOverride{"SYNTHETIC_SERIAL": {}}
	s.ConfigChanged()
	first := s.PersistConfig(cfg)
	delete(cfg.Alerts.Printers, "SYNTHETIC_SERIAL")
	s.ConfigChanged()
	if s.PersistConfig(cfg) != nil {
		t.Fatal("overlapping config writes")
	}
	old := first()
	s.CompleteConfig(old)
	latest := s.PersistConfig(cfg)
	if s.CompleteConfig(old) {
		t.Fatal("old config completion accepted")
	}
	s.CompleteConfig(latest())
	if len(saved) != 2 || len(saved[0].Alerts.Printers) != 1 || len(saved[1].Alerts.Printers) != 0 || s.HasUnsavedPersistence() {
		t.Fatal("config snapshot/coalescing failed")
	}
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	if expired := s.PruneSnoozes(at.Add(time.Minute)); len(expired) != 0 {
		t.Fatal("snooze expired early")
	}
	if expired := s.PruneSnoozes(at.Add(time.Hour)); len(expired) != 1 {
		t.Fatal("snooze not expired")
	}
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	if err := s.ApplyEvent(monitor.Event{Kind: monitor.EventDevices, At: at}); err != nil {
		t.Fatal(err)
	}
	if expired := s.PruneSnoozes(at); len(expired) != 1 {
		t.Fatal("removed printer snooze retained")
	}
}

func TestShutdownRejectsDecisionsWhileCoalescedWritesDrain(t *testing.T) {
	s, _ := fixture(t)
	var snapshots []map[string]farm.Review
	s.WithNoticePersistence(func(r map[string]farm.Review) error { snapshots = append(snapshots, r); return nil }, nil)
	key, target := firstTarget(t, &s)
	decision(&s, target, false)
	initial, _ := s.Persistence()
	if initial == nil {
		t.Fatal("missing first write")
	}
	s.Acknowledge(key, true)
	revision := s.Farm.ReviewRevision()
	stopping := webserver.Command{Target: target, Retry: true, Reply: make(chan string, 1)}
	stopping.Reply <- string(s.ApplyDecision(session.DecisionCommand{Target: stopping.Target, Retry: stopping.Retry}, true, true))
	if <-stopping.Reply == "applied" || s.Farm.ReviewRevision() != revision {
		t.Fatal("shutdown changed pending decision")
	}
	s.Complete(initial())
	latest, _ := s.Persistence()
	if latest == nil || !s.PersistencePending() {
		t.Fatal("latest accepted state not scheduled for drain")
	}
	s.Complete(latest())
	if s.PersistencePending() || s.HasUnsavedPersistence() || len(snapshots) != 2 || snapshots[0][key].Acknowledged || !snapshots[1][key].Acknowledged {
		t.Fatal("accepted snapshots not drained independently")
	}
}

func TestBrowserNotificationPolicyIndependentFromNative(t *testing.T) {
	s, at := fixture(t)
	prefs := config.DefaultAlerts()
	input := s.BrowserNotificationInput(at, prefs)
	if prefs.DesktopEnabled() {
		t.Fatal("fixture should default native delivery off")
	}
	if len(input.Snapshot) != 1 || len(input.Policies) != 1 {
		t.Fatal("missing normalized snapshot")
	}
	policy := input.Policies["SYNTHETIC_SERIAL"]
	allowed := false
	for _, value := range policy.Allowed {
		allowed = allowed || value
	}
	if !allowed {
		t.Fatal("native opt-out suppressed browser categories")
	}
	prefs.Filament, prefs.Extrusion, prefs.PrinterFaults, prefs.FailedPrints, prefs.UnknownPauses = false, false, false, false, false
	muted := s.BrowserNotificationInput(at, prefs)
	for _, value := range muted.Policies["SYNTHETIC_SERIAL"].Allowed {
		if value {
			t.Fatal("disabled browser category allowed")
		}
	}
	s.SetSnooze("SYNTHETIC_SERIAL", time.Hour, at)
	if !s.BrowserNotificationInput(at, prefs).Policies["SYNTHETIC_SERIAL"].Snoozed {
		t.Fatal("browser ignored local snooze")
	}
	if len(s.BrowserNotificationInput(at.Add(time.Hour), prefs).Snapshot) != 0 {
		t.Fatal("stale data entered browser events")
	}
}

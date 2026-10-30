package notification_state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/farm"
	"golang.org/x/sys/unix"
)

func fixture(t *testing.T) (Store, *farm.State, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	s := farm.New(10)
	s.Cloud(true, "", now)
	s.Discover([]farm.Device{{ID: "SYNTHETIC-PRINTER-PRIVATE", Name: "Private printer name", Online: true}}, now)
	return Store{Path: filepath.Join(t.TempDir(), "private", "state.json")}, s, now
}

func report(t *testing.T, s *farm.State, body string, now time.Time) {
	t.Helper()
	if err := s.Report("SYNTHETIC-PRINTER-PRIVATE", []byte(`{"print":`+body+`}`), now); err != nil {
		t.Fatal(err)
	}
}

func observe(t *testing.T, store Store, s *farm.State, now time.Time) []Candidate {
	t.Helper()
	items, err := store.Observe(s.NotificationSnapshot(now))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func reserve(t *testing.T, store Store, event Candidate) {
	t.Helper()
	if fresh, err := store.Reserve(event.ID); err != nil || !fresh {
		t.Fatalf("reservation: fresh=%v, err=%v", fresh, err)
	}
}

func TestIncidentIdentityAcrossReportsRestartUnknownAndRecurrence(t *testing.T) {
	store, s, now := fixture(t)
	fault := `{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}`
	report(t, s, fault, now)
	items := observe(t, store, s, now)
	if len(items) != 1 || items[0].Kind != Intervention {
		t.Fatalf("incident candidates: %+v", items)
	}
	first := items[0]
	if again := observe(t, store, s, now); len(again) != 1 || again[0].ID != first.ID {
		t.Fatal("pending report changed occurrence identity")
	}
	reserve(t, store, first)
	if err := store.RecordResult(first.ID, Accepted); err != nil {
		t.Fatal(err)
	}
	// Simulate a restarted reducer; telemetry knowledge and Since times are lost.
	_, restarted, later := fixture(t)
	later = later.Add(time.Hour)
	restarted.Cloud(false, "", later)
	if got := observe(t, store, restarted, later); len(got) != 0 {
		t.Fatal(got)
	}
	restarted.Cloud(true, "", later)
	report(t, restarted, `{"nozzle_temper":200}`, later)
	observe(t, store, restarted, later)
	report(t, restarted, fault, later)
	if got := observe(t, store, restarted, later); len(got) != 0 {
		t.Fatal("restart duplicated incident", got)
	}
	for _, unknown := range []string{`{"print_error":null}`, `{"print_error":"unknown"}`} {
		report(t, restarted, unknown, later)
		observe(t, store, restarted, later)
		report(t, restarted, fault, later)
		if got := observe(t, store, restarted, later); len(got) != 0 {
			t.Fatal("unknown telemetry retired occurrence", got)
		}
	}
	report(t, restarted, `{"print_error":0}`, later)
	observe(t, store, restarted, later)
	report(t, restarted, fault, later) // Same timestamp must still be a new occurrence.
	recurred := observe(t, store, restarted, later)
	if len(recurred) != 1 || recurred[0].ID == first.ID {
		t.Fatal("confirmed recurrence did not create new identity", recurred)
	}
	if _, err := store.Reserve(first.ID); !errors.Is(err, ErrUnknown) {
		t.Fatalf("stale occurrence remained deliverable: %v", err)
	}
}

func TestReviewsDoNotCreateOrResetNotificationIdentity(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	first := observe(t, store, s, now)[0]
	s.Acknowledge(first.ConditionKey, true)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("reviewed event remained eligible")
	}
	s.Acknowledge(first.ConditionKey, false)
	if got := observe(t, store, s, now); len(got) != 1 || got[0].ID != first.ID {
		t.Fatal("local review changed occurrence identity")
	}
	reserve(t, store, first)
	s.Dismiss(first.ConditionKey)
	observe(t, store, s, now)
	s.RestoreDismissal(first.ConditionKey)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("restoring notice duplicated reserved notification")
	}
}

func TestCompletionRequiresObservedActivityAndConfirmedJobState(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"task_id":"task-a","subtask_id":"copy-a","gcode_state":"FINISH"}`, now)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("startup FINISH notified", got)
	}
	report(t, s, `{"task_id":"task-a","subtask_id":"copy-b","gcode_state":"RUNNING","subtask_name":"same file"}`, now)
	observe(t, store, s, now)
	report(t, s, `{"mc_percent":100,"mc_remaining_time":0}`, now)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("progress/ETA inferred completion")
	}
	report(t, s, `{"gcode_state":"FINISH"}`, now)
	items := observe(t, store, s, now)
	if len(items) != 1 || items[0].Kind != Completion {
		t.Fatal("missing confirmed completion", items)
	}
	first := items[0]
	reserve(t, store, first)
	// Crash between OS submission and result persistence: Reserved stays ambiguous.
	if got := observe(t, Store{Path: store.Path}, s, now); len(got) != 0 {
		t.Fatal("ambiguous attempt retried on restart")
	}
	if err := store.RecordResult(first.ID, Accepted); err != nil {
		t.Fatal(err)
	}
	report(t, s, `{"gcode_state":"IDLE"}`, now)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal(got)
	}
	// Reprinting the same file with another reported copy identity is a new job.
	report(t, s, `{"task_id":"task-a","subtask_id":"copy-c","gcode_state":"RUNNING","subtask_name":"same file"}`, now)
	observe(t, store, s, now)
	report(t, s, `{"gcode_state":"FINISH"}`, now)
	second := observe(t, store, s, now)
	if len(second) != 1 || second[0].ID == first.ID {
		t.Fatal("different job reused completion identity", second)
	}
	reserve(t, store, second[0])
	// An old finished job replay must not replace current history or notify again.
	report(t, s, `{"task_id":"task-a","subtask_id":"copy-b","gcode_state":"FINISH"}`, now)
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("old job replay duplicated completion")
	}
}

func TestCompletionSurvivesRestartAndUnknownJobIDsAreSuppressed(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"task_id":"cloud-task","gcode_state":"RUNNING"}`, now)
	observe(t, store, s, now)
	_, restarted, _ := fixture(t)
	report(t, restarted, `{"task_id":"cloud-task","gcode_state":"FINISH"}`, now)
	if got := observe(t, store, restarted, now); len(got) != 1 || got[0].Kind != Completion {
		t.Fatal("persisted activity was lost", got)
	}
	for _, ids := range []string{`"task_id":"0","subtask_id":"0"`, `"task_id":null,"subtask_id":{}`, `"task_id":0.5`} {
		other, state, _ := fixture(t)
		report(t, state, `{`+ids+`,"gcode_state":"RUNNING"}`, now)
		observe(t, other, state, now)
		report(t, state, `{"gcode_state":"FINISH"}`, now)
		if got := observe(t, other, state, now); len(got) != 0 {
			t.Fatal("unknown/placeholder identity created completion", got)
		}
	}
}

func TestFailedDeliveryRemainsDistinctFromAcceptance(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	item := observe(t, store, s, now)[0]
	if err := store.RecordResult(item.ID, Accepted); !errors.Is(err, ErrNotReserved) {
		t.Fatalf("unreserved result accepted: %v", err)
	}
	reserve(t, store, item)
	if err := store.RecordResult(item.ID, Failed); err != nil {
		t.Fatal(err)
	}
	if got := observe(t, store, s, now); len(got) != 0 {
		t.Fatal("failed delivery retried automatically")
	}
	d, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	for _, incident := range d.Incidents {
		if incident.Attempt != Failed {
			t.Fatal("failed attempt recorded as accepted")
		}
	}
	if err := store.RecordResult(item.ID, Accepted); !errors.Is(err, ErrNotReserved) {
		t.Fatal("terminal result was silently rewritten")
	}
}

func TestConcurrentReservationsAndBusyStore(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	item := observe(t, store, s, now)[0]
	var wins atomic.Int32
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			fresh, err := (Store{Path: store.Path}).Reserve(item.ID)
			if err != nil && !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
			if fresh {
				wins.Add(1)
			}
		})
	}
	workers.Wait()
	if wins.Load() != 1 {
		t.Fatalf("reservation winners = %d", wins.Load())
	}
	if fresh, err := store.Reserve(item.ID); err != nil || fresh {
		t.Fatal("persisted reservation was lost", fresh, err)
	}
	lock, err := os.OpenFile(store.Path+".lock", os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Observe(s.NotificationSnapshot(now)); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy lock ignored: %v", err)
	}
}

func TestPrivateStorageAndAccountIsolation(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"task_id":"SENSITIVE-JOB-ID","subtask_name":"PRIVATE-JOB-NAME","gcode_state":"RUNNING","print_error":117473297}`, now)
	observe(t, store, s, now)
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"SYNTHETIC-PRINTER-PRIVATE", "SENSITIVE-JOB-ID", "PRIVATE-JOB-NAME", "Private printer name"} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatal("private identity/name leaked into state")
		}
	}
	for _, path := range []string{store.Path, store.Path + ".lock"} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("private file permissions: %v, %v", info, err)
		}
	}
	info, err := os.Stat(filepath.Dir(store.Path))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private directory permissions: %v, %v", info, err)
	}
	a, err := New("global", "synthetic-account-a")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New("china", "synthetic-account-a")
	c, _ := New("global", "synthetic-account-b")
	if a.Path == b.Path || a.Path == c.Path || strings.Contains(a.Path, "synthetic-account") {
		t.Fatal("account isolation/path privacy failed")
	}
}

func TestCorruptStateAndStorageFailuresPreventCandidates(t *testing.T) {
	for _, body := range []string{`{`, `null`, `{"version":2}`, `{"version":1,"unexpected":true}`, `{"version":1} {}`, `{"version":1,"incidents":{"bad":{}}}`} {
		store, s, now := fixture(t)
		if err := os.MkdirAll(filepath.Dir(store.Path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
		if items, err := store.Observe(s.NotificationSnapshot(now)); err == nil || len(items) != 0 {
			t.Fatal("invalid storage allowed notification", err, items)
		}
		got, err := os.ReadFile(store.Path)
		if err != nil || string(got) != body {
			t.Fatal("failed load overwrote original state")
		}
	}
	store, s, now := fixture(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store.Path = filepath.Join(blocker, "state.json")
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	if items, err := store.Observe(s.NotificationSnapshot(now)); err == nil || len(items) != 0 {
		t.Fatal("unwritable storage produced candidates")
	}
}

func TestCapacityDoesNotForgetOldIdentity(t *testing.T) {
	store, s, now := fixture(t)
	initial := farm.NotificationSnapshot{}
	for i := range maxEntries {
		id := hashParts(string(rune(i)))
		initial[id] = farm.NotificationPrinter{Job: farm.NotificationJob{Task: id, State: farm.StateRunning}}
	}
	if _, err := store.Observe(initial); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	report(t, s, `{"task_id":"one-more","gcode_state":"RUNNING"}`, now)
	if got, err := store.Observe(s.NotificationSnapshot(now)); !errors.Is(err, ErrCapacity) || len(got) != 0 {
		t.Fatal("capacity did not fail closed", err)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("capacity failure evicted deduplication history")
	}
}

func TestMalformedHMSPreservesIdentityUntilConfirmedClear(t *testing.T) {
	store, s, now := fixture(t)
	fault := `{"gcode_state":"RUNNING","print_error":0,"hms":[{"attr":117448704,"code":131073}]}`
	report(t, s, fault, now)
	items := observe(t, store, s, now)
	if len(items) != 1 {
		t.Fatal("missing HMS intervention", items)
	}
	reserve(t, store, items[0])
	for _, unknown := range []string{`{"hms":null}`, `{"hms":[{"attr":117448704,"code":null}]}`, `{"hms":[{"attr":117448704,"code":131073.5}]}`} {
		report(t, s, unknown, now)
		if got := observe(t, store, s, now); len(got) != 0 {
			t.Fatal("malformed HMS supplied eligibility", got)
		}
		report(t, s, fault, now)
		if got := observe(t, store, s, now); len(got) != 0 {
			t.Fatal("unknown HMS retired consumed occurrence", got)
		}
	}
	report(t, s, `{"hms":[]}`, now)
	observe(t, store, s, now)
	report(t, s, fault, now)
	if got := observe(t, store, s, now); len(got) != 1 || got[0].ID == items[0].ID {
		t.Fatal("HMS recurrence identity lost", got)
	}
}

func TestPauseBecomingFailedIsANewIncident(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"PAUSE","print_error":0,"hms":[]}`, now)
	first := observe(t, store, s, now)[0]
	reserve(t, store, first)
	report(t, s, `{"gcode_state":"FAILED"}`, now)
	next := observe(t, store, s, now)
	if len(next) != 1 || next[0].ID == first.ID {
		t.Fatal("failure reused paused incident identity", next)
	}
}

func TestBaselineAndCanceledJobIDsAreNotReusedForCompletions(t *testing.T) {
	for _, terminal := range []string{"FINISH", "FAILED", "IDLE"} {
		t.Run(terminal, func(t *testing.T) {
			store, s, now := fixture(t)
			report(t, s, `{"task_id":"same-task","gcode_state":"`+terminal+`"}`, now)
			observe(t, store, s, now)
			report(t, s, `{"gcode_state":"RUNNING"}`, now)
			observe(t, store, s, now)
			report(t, s, `{"gcode_state":"FINISH"}`, now)
			for _, candidate := range observe(t, store, s, now) {
				if candidate.Kind == Completion {
					t.Fatal("ambiguous reused job identity announced completion")
				}
			}
		})
	}
}

func TestFailedReservationDoesNotConsumeTheOccurrence(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires ordinary user filesystem permissions")
	}
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	item := observe(t, store, s, now)[0]
	dir := filepath.Dir(store.Path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()
	if fresh, err := store.Reserve(item.ID); err == nil || fresh {
		t.Fatal("failed persistence allowed a delivery attempt", fresh, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	reserve(t, store, item)
}

func TestReservationAcrossProcesses(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"RUNNING","print_error":117473297}`, now)
	item := observe(t, store, s, now)[0]
	child := func() *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLedgerHelperProcess$")
		cmd.Env = append(os.Environ(), "BAMBU_LEDGER_TEST_PATH="+store.Path, "BAMBU_LEDGER_TEST_ID="+item.ID)
		return cmd
	}
	lock, err := os.OpenFile(store.Path+".lock", os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	err = child().Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 20 {
		t.Fatalf("child ignored parent's lock: %v", err)
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	output, err := child().Output()
	if err != nil || string(output) != "reserved" {
		t.Fatalf("child reservation: %q, %v", output, err)
	}
	if fresh, err := store.Reserve(item.ID); err != nil || fresh {
		t.Fatalf("parent ignored child's durable reservation: %v, %v", fresh, err)
	}
}

func TestLedgerHelperProcess(t *testing.T) {
	path := os.Getenv("BAMBU_LEDGER_TEST_PATH")
	if path == "" {
		return
	}
	fresh, err := (Store{Path: path}).Reserve(os.Getenv("BAMBU_LEDGER_TEST_ID"))
	if errors.Is(err, ErrBusy) {
		os.Exit(20)
	}
	if err != nil {
		os.Exit(21)
	}
	if fresh {
		fmt.Print("reserved")
	} else {
		fmt.Print("duplicate")
	}
	os.Exit(0)
}

func TestGroupedReservationIsAtomicAndSkipsPreviouslyClaimedMembers(t *testing.T) {
	store, s, now := fixture(t)
	report(t, s, `{"gcode_state":"PAUSE","print_error":117473297,"hms":[{"attr":117448704,"code":131073}]}`, now)
	items := observe(t, store, s, now)
	if len(items) != 3 {
		t.Fatal("expected independent runout notices", items)
	}
	if ids, err := store.ReserveBatch([]string{items[0].ID, "unknown"}); !errors.Is(err, ErrUnknown) || len(ids) != 0 {
		t.Fatal("invalid group was partially claimed", ids, err)
	}
	reserve(t, store, items[0])
	ids := []string{items[0].ID, items[1].ID, items[1].ID, items[2].ID}
	claimed, err := store.ReserveBatch(ids)
	if err != nil || len(claimed) != 2 {
		t.Fatal("group did not reserve only remaining unique members", claimed, err)
	}
	if again, err := (Store{Path: store.Path}).ReserveBatch(ids); err != nil || len(again) != 0 {
		t.Fatal("group reservation was not durable", again, err)
	}
}

func TestUpgradeRetainsReservedNotifications(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	// Original length-delimited account hash and directory: independent of New.
	legacy := Store{Path: filepath.Join(root, "bambu-tui", "notifications", "427880e3826beb038aa4a698c700cbfa6da34a767fcd8372d75ee127f11db044.json")}
	_, state, now := fixture(t)
	report(t, state, `{"gcode_state":"RUNNING","print_error":117473297,"hms":[]}`, now)
	candidates := observe(t, legacy, state, now)
	if len(candidates) != 1 {
		t.Fatal(candidates)
	}
	reserve(t, legacy, candidates[0])
	current, err := New("global", "synthetic-account")
	if err != nil || current.Path != legacy.Path {
		t.Fatalf("notification history moved: %q, %v", current.Path, err)
	}
	if replay := observe(t, current, state, now); len(replay) != 0 {
		t.Fatal("rename replayed a reserved notification", replay)
	}
}

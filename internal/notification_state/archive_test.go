package notification_state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cliffom/bambustand/internal/farm"
)

func jobSnapshot(task, state string) farm.NotificationSnapshot {
	return farm.NotificationSnapshot{"SYNTHETIC": {Job: farm.NotificationJob{Task: task, State: state}}}
}

func TestJobArchiveLongLifecycleRestartReplay(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	for i := range 200 {
		task := fmt.Sprintf("job-%d", i)
		if _, err := store.Observe(jobSnapshot(task, farm.StateRunning)); err != nil {
			t.Fatal(err)
		}
		events, err := store.Observe(jobSnapshot(task, farm.StateFinished))
		if err != nil || len(events) != 1 {
			t.Fatal(events, err)
		}
		reserve(t, store, events[0])
		result := Accepted
		if i%2 == 0 {
			result = Failed
		}
		if err := store.RecordResult(events[0].ID, result); err != nil {
			t.Fatal(err)
		}
	}
	d, err := store.load()
	if err != nil || len(d.Jobs) != 1 {
		t.Fatal("unbounded live history", len(d.Jobs), err)
	}
	restarted := Store{Path: store.Path}
	for _, task := range []string{"job-0", "job-99", "job-199"} {
		for _, state := range []string{farm.StateFinished, farm.StateRunning, farm.StateFinished} {
			events, err := restarted.Observe(jobSnapshot(task, state))
			if err != nil || len(events) != 0 {
				t.Fatal("history replayed", task, state, events, err)
			}
		}
	}
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("SYNTHETIC")) || bytes.Contains(data, []byte("job-")) {
		t.Fatal("raw identity persisted")
	}
}

func TestLegacyFullLedgerCompactsWithoutForgetting(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	d := document{Version: 1, Incidents: map[string]incident{}, Jobs: map[string]job{}}
	for i := range maxEntries {
		key := hashParts("SYNTHETIC", fmt.Sprint(i), "")
		o, err := newOccurrence()
		if err != nil {
			t.Fatal(err)
		}
		d.Jobs[key] = job{occurrence: o, Printer: hashParts("SYNTHETIC"), Active: true}
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.ObserveActivity(jobSnapshot("new-job", farm.StateRunning))
	if err != nil {
		t.Fatal(err)
	}
	if result.Capacity.Entries != 1 || result.Capacity.Limit != maxEntries {
		t.Fatal(result.Capacity)
	}
	d, err = store.load()
	if err != nil || d.Version != schemaVersion || !d.Archive {
		t.Fatal(d.Version, err)
	}
	if found, err := store.archived(hashParts("SYNTHETIC", "0", ""), false); err != nil || !found {
		t.Fatal("lost legacy identity", found, err)
	}
	for _, state := range []string{farm.StateRunning, farm.StateFinished} {
		if events, err := store.Observe(jobSnapshot("0", state)); err != nil || len(events) != 0 {
			t.Fatal("legacy replay", events, err)
		}
	}
}

func TestArchiveBucketOverflowAndCorruption(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if err := store.archiveReady(true); err != nil {
		t.Fatal(err)
	}
	// Fill one valid prefix bucket directly to exercise deterministic overflow.
	data := []byte(archiveFormat)
	for i := range archiveBucketEntries {
		digest := make([]byte, digestBytes)
		digest[0] = 0xab
		digest[30] = byte(i >> 8)
		digest[31] = byte(i)
		data = append(data, digest...)
	}
	data = archiveBytes(data[len(archiveFormat):])
	path := filepath.Join(store.Path+".jobs", "ab.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	key := "abff" + fmt.Sprintf("%060x", 1)
	if _, err := store.archived(key, true); err != nil {
		t.Fatal(err)
	}
	if found, err := store.archived(key, false); err != nil || !found {
		t.Fatal(found, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("full parent modified", err)
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.archived(key, true); err == nil {
		t.Fatal("corrupt history overwritten")
	}
	after, _ = os.ReadFile(path)
	if string(after) != "broken" {
		t.Fatal("corrupt bytes changed")
	}
}

func TestArchiveRecoveryOwnershipAndUnfinishedConsequences(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(jobSnapshot("unfinished", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	owner, err := store.AcquireOwner()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompactJobs(); !errors.Is(err, ErrBusy) {
		t.Fatal("compacted under live owner", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.CompactJobs(); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{farm.StateRunning, farm.StateFinished} {
		if events, err := store.Observe(jobSnapshot("unfinished", state)); err != nil || len(events) != 0 {
			t.Fatal(events, err)
		}
	}
	if _, err := store.Observe(jobSnapshot("new", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	if events, err := store.Observe(jobSnapshot("new", farm.StateFinished)); err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
}

func TestArchiveUnknownRemovedAndWriteFailure(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(jobSnapshot("active", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []farm.NotificationSnapshot{nil, {}, jobSnapshot("different", farm.StateUnknown)} {
		if _, err := store.Observe(snapshot); err != nil {
			t.Fatal(err)
		}
		d, err := store.load()
		if err != nil || len(d.Jobs) != 1 {
			t.Fatal("unknown retired active job", err)
		}
	}
	original, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	// An inaccessible bucket fails closed without removing the live record.
	key := hashParts("SYNTHETIC", "active", "")
	path := filepath.Join(store.Path+".jobs", key[:2]+".bin")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Observe(jobSnapshot("next", farm.StateRunning)); err == nil {
		t.Fatal("archive failure allowed observation")
	}
	after, _ := os.ReadFile(store.Path)
	if !bytes.Equal(original, after) {
		t.Fatal("archive failure removed live records")
	}
}

func TestArchivedJobCannotBeReservedAfterInterruptedCompaction(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(jobSnapshot("old", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	events, err := store.Observe(jobSnapshot("old", farm.StateFinished))
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
	// Model an archive commit followed by a crash before live-ledger replacement.
	if _, err := store.archived(hashParts("SYNTHETIC", "old", ""), true); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Reserve(events[0].ID); ok || !errors.Is(err, ErrUnknown) {
		t.Fatal("stale reservation succeeded", ok, err)
	}
	if events, err := store.Observe(jobSnapshot("old", farm.StateFinished)); err != nil || len(events) != 0 {
		t.Fatal(events, err)
	}
}

func TestObserveRecommitsArchivedUnfinishedJobBeforeRemovingLiveRecord(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(jobSnapshot("unfinished", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	key := hashParts("SYNTHETIC", "unfinished", "")
	// Model compaction's visible bucket after rename but before a successful
	// directory commit or live-ledger replacement. Recovery cannot infer durability.
	if _, err := store.archived(key, true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Path+".jobs", key[:archivePrefixDigits]+".bin")
	before, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = before.Close() }()
	original, err := before.Stat()
	if err != nil {
		t.Fatal(err)
	}
	restarted := Store{Path: store.Path}
	if events, err := restarted.Observe(jobSnapshot("unfinished", farm.StateRunning)); err != nil || len(events) != 0 {
		t.Fatal(events, err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(original, after) {
		t.Fatal("overlapping archive bucket was not recommitted")
	}
	d, err := restarted.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := d.Jobs[key]; exists {
		t.Fatal("recovered archive left duplicate live record")
	}
	for _, state := range []string{farm.StateFinished, farm.StateRunning, farm.StateFinished} {
		if events, err := restarted.Observe(jobSnapshot("unfinished", state)); err != nil || len(events) != 0 {
			t.Fatal("consumed unfinished job replayed", events, err)
		}
	}
}

func TestConcurrentArchiveClaimsRemainAtMostOnce(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(jobSnapshot("job", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	events, err := store.Observe(jobSnapshot("job", farm.StateFinished))
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
	var wg sync.WaitGroup
	outcomes := make(chan bool, 2)
	for range 2 {
		wg.Go(func() {
			ok, err := store.Reserve(events[0].ID)
			if err != nil && !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
			outcomes <- ok
		})
	}
	wg.Wait()
	close(outcomes)
	count := 0
	for ok := range outcomes {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatal("duplicate claims", count)
	}
	if err := store.CompactJobs(); err != nil {
		t.Fatal(err)
	}
	if events, err := store.Observe(jobSnapshot("job", farm.StateFinished)); err != nil || len(events) != 0 {
		t.Fatal(events, err)
	}
}

func TestRetiredReplayDoesNotDisplaceCurrentJob(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	for _, task := range []string{"old", "current"} {
		if _, err := store.Observe(jobSnapshot(task, farm.StateRunning)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Observe(jobSnapshot("old", farm.StateRunning)); err != nil {
		t.Fatal(err)
	}
	if events, err := store.Observe(jobSnapshot("current", farm.StateFinished)); err != nil || len(events) != 1 {
		t.Fatal("retired replay displaced unfinished activity", events, err)
	}
}

func TestArchiveMissingMarkerAndChecksumFailClosed(t *testing.T) {
	for _, mode := range []string{"missing marker", "checksum"} {
		t.Run(mode, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
			if _, err := store.Observe(jobSnapshot("old", farm.StateRunning)); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Observe(jobSnapshot("current", farm.StateRunning)); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "missing marker" {
				if err := os.Remove(filepath.Join(store.Path+".jobs", "format")); err != nil {
					t.Fatal(err)
				}
			} else {
				key := hashParts("SYNTHETIC", "old", "")
				path := filepath.Join(store.Path+".jobs", key[:2]+".bin")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data[len(archiveFormat)+4] ^= 1
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if events, err := store.Observe(jobSnapshot("old", farm.StateFinished)); err == nil || len(events) != 0 {
				t.Fatal("incomplete/corrupt archive allowed replay", events, err)
			}
			after, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("unreadable archive overwrote ledger", err)
			}
		})
	}
}

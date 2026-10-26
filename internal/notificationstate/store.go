// Package notificationstate persists notification occurrences and delivery
// reservations independently from notice reviews and snoozes. Methods perform
// filesystem I/O and belong in a serialized worker, never the UI reducer.
package notificationstate

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/cliffom/bambu-tui/internal/atomicfile"
	"github.com/cliffom/bambu-tui/internal/farm"
	"golang.org/x/sys/unix"
)

const (
	schemaVersion = 1
	maxEntries    = 4096
	maxFileBytes  = 2 * 1024 * 1024
	nonceBytes    = 16
	digestBytes   = sha256.Size
)

var (
	ErrBusy        = errors.New("notification state is in use")
	ErrCapacity    = errors.New("notification identity capacity reached; delivery disabled rather than forgetting deduplication history")
	ErrUnknown     = errors.New("notification occurrence is no longer current")
	ErrNotReserved = errors.New("notification occurrence has not been reserved")
)

type (
	Observation struct {
		Candidates []Candidate
		Suppressed []Suppression
	}
	Suppression struct {
		Candidate
		Reason string
	}
	Store   struct{ Path string }
	Kind    string
	Attempt string

	Candidate struct {
		ID, PrinterID, ConditionKey string
		Kind                        Kind
	}

	document struct {
		Version   int                 `json:"version"`
		Incidents map[string]incident `json:"incidents"`
		Jobs      map[string]job      `json:"jobs"`
	}

	occurrence struct {
		ID      string  `json:"id"`
		Attempt Attempt `json:"attempt"`
	}

	incident struct {
		occurrence
		Printer   string `json:"printer"`
		Family    string `json:"family"`
		Signature string `json:"signature"`
	}

	job struct {
		occurrence
		Printer   string `json:"printer"`
		Active    bool   `json:"active"`
		Completed bool   `json:"completed"`
		Closed    bool   `json:"closed"`
	}
)

const (
	Intervention Kind    = "intervention"
	Completion   Kind    = "completion"
	Pending      Attempt = "pending"
	Reserved     Attempt = "reserved"
	Accepted     Attempt = "accepted"
	Failed       Attempt = "failed"
)

func New(region, username string) (Store, error) {
	if region == "" || username == "" {
		return Store{}, errors.New("notification state requires an account and region")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	account := hashParts(region, username)
	return Store{Path: filepath.Join(dir, "bambu-tui", "notifications", account+".json")}, nil
}

// Observe durably records lifetimes before returning unreserved candidates.
// No delivery is performed. Missing/stale/disconnected telemetry preserves saved
// identities. Reviews affect eligibility, never the occurrence identity.
func (s Store) Observe(snapshot farm.NotificationSnapshot) ([]Candidate, error) {
	result, err := s.ObserveActivity(snapshot)
	return result.Candidates, err
}

// ObserveActivity records the same identities and returns bounded-source decision
// metadata for this observation. Activity is never persisted in the ledger.
func (s Store) ObserveActivity(snapshot farm.NotificationSnapshot) (Observation, error) {
	var result Observation
	err := s.transact(func(d *document) (bool, error) {
		dirty := false
		for _, id := range sortedKeys(snapshot) {
			p := snapshot[id]
			printer := hashParts(id)
			for key, old := range d.Incidents {
				if old.Printer != printer {
					continue
				}
				present := false
				for condition := range p.Conditions {
					present = present || key == hashParts(id, condition)
				}
				if !present && familyKnown(p, old.Family) {
					delete(d.Incidents, key)
					dirty = true
				}
			}
			for _, condition := range sortedKeys(p.Conditions) {
				value := p.Conditions[condition]
				key := hashParts(id, condition)
				signature := hashParts(value.Family, value.Level, value.Code, value.State)
				old, present := d.Incidents[key]
				if !present || old.Signature != signature {
					o, err := newOccurrence()
					if err != nil {
						return false, err
					}
					old = incident{occurrence: o, Printer: printer, Family: value.Family, Signature: signature}
					d.Incidents[key], dirty = old, true
				}
				candidate := Candidate{ID: old.ID, PrinterID: id, ConditionKey: condition, Kind: Intervention}
				if value.Eligible && old.Attempt == Pending {
					result.Candidates = append(result.Candidates, candidate)
				} else {
					reason := attemptReason(old.Attempt)
					if value.Reviewed {
						reason = SuppressedReviewed
					} else if !value.Eligible {
						reason = SuppressedNotActionable
					}
					result.Suppressed = append(result.Suppressed, Suppression{Candidate: candidate, Reason: reason})
				}
			}
			if p.Job.Task == "" && p.Job.Subtask == "" {
				if p.Job.State == farm.StateFinished {
					result.Suppressed = append(result.Suppressed, Suppression{Candidate: Candidate{ID: hashParts("unknown-job", id), PrinterID: id, Kind: Completion}, Reason: SuppressedJobIdentity})
				}
				continue // No trustworthy protocol identity; never guess from names/ETA.
			}
			key := hashParts(id, p.Job.Task, p.Job.Subtask)
			old, present := d.Jobs[key]
			if !farmJobStateKnown(p.Job.State) {
				continue
			}
			if !present {
				o, err := newOccurrence()
				if err != nil {
					return false, err
				}
				old = job{occurrence: o, Printer: printer}
			}
			next := old
			switch p.Job.State {
			case farm.StateRunning, farm.StatePaused, farm.StatePreparing, farm.StateSlicing:
				if !old.Completed && !old.Closed {
					next.Active = true
				}
			case farm.StateFinished:
				next.Completed = true
				if !old.Active {
					next.Closed = true
				} // Startup FINISH is a baseline.
			case farm.StateFailed, farm.StateIdle:
				if !old.Completed {
					next.Closed = true
				}
			}
			if !present || next != old {
				d.Jobs[key], dirty = next, true
			}
			if p.Job.State == farm.StateFinished && next.Completed && next.Active && !next.Closed && next.Attempt == Pending {
				result.Candidates = append(result.Candidates, Candidate{ID: next.ID, PrinterID: id, Kind: Completion})
			} else if p.Job.State == farm.StateFinished {
				reason := attemptReason(next.Attempt)
				if next.Closed || !next.Active {
					reason = SuppressedBaseline
				}
				result.Suppressed = append(result.Suppressed, Suppression{Candidate: Candidate{ID: next.ID, PrinterID: id, Kind: Completion}, Reason: reason})
			}
		}
		return dirty, nil
	})
	if err != nil {
		return Observation{}, err
	}
	return result, nil
}

// Reserve commits an at-most-once attempt before the caller invokes the OS.
// A reserved attempt surviving a crash has unknown delivery outcome and is not
// automatically retried. Duplicate/concurrent reservations return false.
func (s Store) Reserve(id string) (bool, error) {
	claimed, err := s.ReserveBatch([]string{id})
	return len(claimed) != 0, err
}

// ReserveBatch atomically reserves the pending members of one delivery group.
// A missing member or storage failure commits none of the reservations.
func (s Store) ReserveBatch(ids []string) ([]string, error) {
	var claimed []string
	err := s.transact(func(d *document) (bool, error) {
		for _, id := range ids {
			_, err := changeIn(d, id, func(o *occurrence, ready bool) (bool, error) {
				if !ready || o.Attempt != Pending {
					return false, nil
				}
				o.Attempt = Reserved
				claimed = append(claimed, id)
				return true, nil
			})
			if err != nil {
				return false, err
			}
		}
		return len(claimed) != 0, nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// RecordResult separates request acceptance from failure, never claiming that a
// banner was seen. Failed/reserved occurrences remain deduplicated.
func (s Store) RecordResult(id string, result Attempt) error {
	if result != Accepted && result != Failed {
		return errors.New("invalid notification delivery result")
	}
	return s.changeOccurrence(id, func(o *occurrence, _ bool) (bool, error) {
		if o.Attempt == result {
			return false, nil
		}
		if o.Attempt != Reserved {
			return false, ErrNotReserved
		}
		o.Attempt = result
		return true, nil
	})
}

func (s Store) changeOccurrence(id string, change func(*occurrence, bool) (bool, error)) error {
	return s.transact(func(d *document) (bool, error) { return changeIn(d, id, change) })
}

func changeIn(d *document, id string, change func(*occurrence, bool) (bool, error)) (bool, error) {
	for key, value := range d.Incidents {
		if value.ID == id {
			dirty, err := change(&value.occurrence, true)
			d.Incidents[key] = value
			return dirty, err
		}
	}
	for key, value := range d.Jobs {
		if value.ID == id {
			dirty, err := change(&value.occurrence, value.Active && value.Completed && !value.Closed)
			d.Jobs[key] = value
			return dirty, err
		}
	}
	return false, ErrUnknown
}

// Each transaction reloads under a nonblocking OS lock, including between two
// TUI processes. A stale in-memory snapshot can never overwrite another claim.
func (s Store) transact(change func(*document) (bool, error)) error {
	if s.Path == "" {
		return errors.New("notification state path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create notification state directory: %w", err)
	}
	lock, err := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open notification state lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return ErrBusy
		}
		return fmt.Errorf("lock notification state: %w", err)
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()
	d, err := s.load()
	if err != nil {
		return err
	}
	dirty, err := change(&d)
	if err != nil || !dirty {
		return err
	}
	if err := validate(d); err != nil {
		return err
	}
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode notification state: %w", err)
	}
	if len(data)+1 > maxFileBytes {
		return ErrCapacity
	}
	if err := atomicfile.Replace(s.Path, ".notifications-*", append(data, '\n')); err != nil {
		return fmt.Errorf("save notification state: %w", err)
	}
	return nil
}

func (s Store) load() (document, error) {
	d := document{Version: schemaVersion, Incidents: map[string]incident{}, Jobs: map[string]job{}}
	file, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, fmt.Errorf("read notification state: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return d, fmt.Errorf("read notification state: %w", err)
	}
	if len(data) > maxFileBytes {
		return d, ErrCapacity
	}
	d = document{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, fmt.Errorf("decode notification state: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return d, errors.New("invalid trailing notification state data")
	}
	if err := validate(d); err != nil {
		return d, err
	}
	if d.Incidents == nil {
		d.Incidents = map[string]incident{}
	}
	if d.Jobs == nil {
		d.Jobs = map[string]job{}
	}
	return d, nil
}

func validate(d document) error {
	if d.Version != schemaVersion {
		return errors.New("unsupported notification state version")
	}
	if d.Incidents == nil || d.Jobs == nil {
		return errors.New("invalid notification state collections")
	}
	if len(d.Incidents)+len(d.Jobs) > maxEntries {
		return ErrCapacity
	}
	ids := map[string]bool{}
	validOccurrence := func(o occurrence) bool {
		if !validHex(o.ID, nonceBytes) || ids[o.ID] {
			return false
		}
		ids[o.ID] = true
		return o.Attempt == Pending || o.Attempt == Reserved || o.Attempt == Accepted || o.Attempt == Failed
	}
	for key, item := range d.Incidents {
		if !validHex(key, digestBytes) || !validHex(item.Printer, digestBytes) || !validHex(item.Signature, digestBytes) || !knownFamily(item.Family) || !validOccurrence(item.occurrence) {
			return errors.New("invalid notification incident state")
		}
	}
	for key, item := range d.Jobs {
		if !validHex(key, digestBytes) || !validHex(item.Printer, digestBytes) || !validOccurrence(item.occurrence) || (item.Attempt != Pending && (!item.Active || !item.Completed || item.Closed)) {
			return errors.New("invalid notification job state")
		}
	}
	return nil
}

func newOccurrence() (occurrence, error) {
	var nonce [nonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return occurrence{}, fmt.Errorf("create notification identity: %w", err)
	}
	return occurrence{ID: hex.EncodeToString(nonce[:]), Attempt: Pending}, nil
}

func hashParts(parts ...string) string {
	var data []byte
	for _, part := range parts {
		data = binary.BigEndian.AppendUint64(data, uint64(len(part)))
		data = append(data, part...)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validHex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size
}

func knownFamily(family string) bool {
	return family == farm.NotificationStateFamily || family == farm.NotificationPrintFamily || family == farm.NotificationHMSFamily
}

func familyKnown(p farm.NotificationPrinter, family string) bool {
	switch family {
	case farm.NotificationStateFamily:
		return p.StateKnown
	case farm.NotificationPrintFamily:
		return p.PrintKnown
	case farm.NotificationHMSFamily:
		return p.HMSKnown
	}
	return false
}

func farmJobStateKnown(state string) bool {
	switch state {
	case farm.StateRunning, farm.StatePaused, farm.StatePreparing, farm.StateSlicing, farm.StateFinished, farm.StateFailed, farm.StateIdle:
		return true
	}
	return false
}

func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

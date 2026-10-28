package notification_state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cliffom/bambu-tui/internal/atomicfile"
	"github.com/cliffom/bambu-tui/internal/farm"
)

const (
	archiveFormat        = "BTJOB001"
	archiveBucketEntries = 4096
	archivePrefixDigits  = 2
)

// Exact retired-job digests grow on disk, not in the live document. Each sorted
// bucket is bounded at 128 KiB; full buckets become immutable and overflow into
// progressively longer hash prefixes. No identity is expired or approximated.
func (s Store) archiveReady(create bool) error {
	path := filepath.Join(s.Path+".jobs", "format")
	file, err := os.Open(path)
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(file, int64(len(archiveFormat)+1)))
		_ = file.Close()
	}
	if errors.Is(err, os.ErrNotExist) && create {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return errors.New("cannot create notification job archive")
		}
		return atomicfile.Replace(path, ".format-*", []byte(archiveFormat))
	}
	if err == nil && string(data) == archiveFormat && create {
		return atomicfile.Replace(path, ".format-*", data)
	}
	if err != nil || string(data) != archiveFormat {
		return errors.New("notification job archive unavailable or unsupported; restore the complete ledger and archive")
	}
	return nil
}

// readArchiveBucket validates a bounded, checksummed sorted digest block. A
// missing bucket is empty; deleting individual archive files is unsupported.
func (s Store) readArchiveBucket(prefix string) ([]byte, error) {
	file, err := os.Open(filepath.Join(s.Path+".jobs", prefix+".bin"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("cannot read notification job archive")
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, int64(len(archiveFormat)+archiveBucketEntries*digestBytes+sha256.Size+1)))
	if err != nil {
		return nil, errors.New("cannot read notification job archive")
	}
	if len(data) < len(archiveFormat)+sha256.Size || string(data[:len(archiveFormat)]) != archiveFormat {
		return nil, errors.New("invalid notification job archive")
	}
	checksum := sha256.Sum256(data[:len(data)-sha256.Size])
	if !bytes.Equal(checksum[:], data[len(data)-sha256.Size:]) {
		return nil, errors.New("notification job archive checksum mismatch")
	}
	data = data[len(archiveFormat) : len(data)-sha256.Size]
	if len(data)%digestBytes != 0 || len(data) > archiveBucketEntries*digestBytes {
		return nil, errors.New("invalid notification job archive size")
	}
	for i := 0; i < len(data); i += digestBytes {
		if !strings.HasPrefix(hex.EncodeToString(data[i:i+digestBytes]), prefix) || (i > 0 && bytes.Compare(data[i-digestBytes:i], data[i:i+digestBytes]) >= 0) {
			return nil, errors.New("invalid notification job archive order")
		}
	}
	return data, nil
}

func archiveSearch(data, digest []byte) (int, bool) {
	count := len(data) / digestBytes
	at := sort.Search(count, func(i int) bool { return bytes.Compare(data[i*digestBytes:(i+1)*digestBytes], digest) >= 0 })
	return at, at < count && bytes.Equal(data[at*digestBytes:(at+1)*digestBytes], digest)
}

func (s Store) archived(key string, insert bool) (bool, error) {
	digest, err := hex.DecodeString(key)
	if err != nil || len(digest) != digestBytes {
		return false, errors.New("invalid archived job digest")
	}
	found := false
	for digits := archivePrefixDigits; digits <= len(key); digits += archivePrefixDigits {
		data, err := s.readArchiveBucket(key[:digits])
		if err != nil {
			return false, err
		}
		_, found = archiveSearch(data, digest)
		if found || len(data) < archiveBucketEntries*digestBytes {
			break
		}
	}
	if insert {
		if err := s.archiveJobKeys([]string{key}, archivePrefixDigits); err != nil {
			return false, err
		}
	}
	return found, nil
}

// Batch one observation's retirements so a legacy full history commits at most
// one replacement per touched bucket, rather than one directory sync per job.
func (s Store) archiveJobKeys(keys []string, digits int) error {
	groups := map[string][]string{}
	for _, key := range keys {
		if digits > len(key) {
			return errors.New("notification job archive prefix exhausted")
		}
		groups[key[:digits]] = append(groups[key[:digits]], key)
	}
	for _, prefix := range sortedKeys(groups) {
		data, err := s.readArchiveBucket(prefix)
		if err != nil {
			return err
		}
		var overflow []string
		changed := false
		for _, key := range groups[prefix] {
			digest, err := hex.DecodeString(key)
			if err != nil || len(digest) != digestBytes {
				return errors.New("invalid archived job digest")
			}
			at, found := archiveSearch(data, digest)
			if found {
				changed = true
				continue
			} // Recommit ambiguous prior saves before deleting live entries.
			if len(data) == archiveBucketEntries*digestBytes {
				overflow = append(overflow, key)
				continue
			}
			data = append(data, make([]byte, digestBytes)...)
			copy(data[(at+1)*digestBytes:], data[at*digestBytes:len(data)-digestBytes])
			copy(data[at*digestBytes:], digest)
			changed = true
		}
		if changed {
			if err := atomicfile.Replace(filepath.Join(s.Path+".jobs", prefix+".bin"), ".jobs-*", archiveBytes(data)); err != nil {
				return errors.New("cannot commit notification job archive; delivery paused")
			}
		}
		if len(overflow) > 0 {
			if err := s.archiveJobKeys(overflow, digits+archivePrefixDigits); err != nil {
				return err
			}
		}
	}
	return nil
}

// retireJobs needs authoritative fresh identities only for superseding unfinished
// jobs. Missing printers, unknown identities and disconnected snapshots cannot
// retire activity. Already consumed/closed terminal jobs need no future attempt.
func (s Store) retireJobs(d *document, snapshot farm.NotificationSnapshot, all bool) (bool, error) {
	current := map[string]string{}
	for id, p := range snapshot {
		if (p.Job.Task != "" || p.Job.Subtask != "") && farmJobStateKnown(p.Job.State) {
			key := hashParts(id, p.Job.Task, p.Job.Subtask)
			retired, err := s.archived(key, false)
			if err != nil {
				return false, err
			}
			// Replayed retired identities cannot displace unfinished current activity.
			if !retired {
				current[hashParts(id)] = key
			}
		}
	}
	var retired []string
	for _, key := range sortedKeys(d.Jobs) {
		j := d.Jobs[key]
		next, known := current[j.Printer]
		retire := all || (known && next != key) || (!known && (j.Closed || (j.Completed && j.Attempt != Pending)))
		if !retire {
			continue
		}
		retired = append(retired, key)
	}
	if err := s.archiveJobKeys(retired, archivePrefixDigits); err != nil {
		return false, err
	}
	for _, key := range retired {
		delete(d.Jobs, key)
	}
	return len(retired) > 0, nil
}

// CompactJobs consumes all remaining job records under exclusive delivery
// ownership. This explicit maintenance decision may suppress unfinished jobs'
// eventual completions. Incidents, reviews and prior attempts are never reset.
func (s Store) CompactJobs() error {
	owner, err := s.AcquireOwner()
	if err != nil {
		return err
	}
	defer func() { _ = owner.Close() }()
	return s.transact(func(d *document) (bool, error) { return s.retireJobs(d, nil, true) })
}

type Capacity struct{ Entries, Limit int }

func (c Capacity) NearLimit() bool { return c.Limit > 0 && c.Entries >= c.Limit*4/5 }

func archiveBytes(records []byte) []byte {
	data := append([]byte(archiveFormat), records...)
	checksum := sha256.Sum256(data)
	return append(data, checksum[:]...)
}

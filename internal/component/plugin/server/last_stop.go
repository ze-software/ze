// Design: docs/architecture/zefs-format.md -- the meta/plugin/last-stop key
// Related: failure_policy.go -- stopDaemonForPlugin, which records a plugin-caused stop
// Related: startup.go -- signalStartupComplete, which settles the record at the end of startup
//
// The last-stop record is why ze last stopped itself: a plugin whose failure
// policy is fatal failed, the boot refused a plugin, or a stop-time cleanup ran
// late. Nothing else survives the stop, so the next `ze doctor` reads this
// record to name the cause. It is written to the daemon's store before the
// stop, and deleted when the next start completes cleanly, so it never names a
// stale reason after a clean restart.

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// LastStopReason is why one entry of the record was written. The zero value is
// a reason nobody set, which RecordLastStop refuses.
type LastStopReason uint8

const (
	LastStopUnspecified LastStopReason = iota
	// LastStopFatalPolicy: a plugin whose failure policy is fatal failed, and
	// ze stopped itself (stopDaemonForPlugin).
	LastStopFatalPolicy
	// LastStopBootRefused: startup ended with an error, so ze did not start.
	LastStopBootRefused
	// LastStopCleanupLate: a plugin engine or the config transaction did not
	// finish its cleanup before the stop went on.
	LastStopCleanupLate
)

// lastStopEntriesMax bounds the record. The first entries are kept, because
// the first stop reason is the cause and later ones are usually its effects;
// the rest are counted in Dropped.
const lastStopEntriesMax = 16

// lastStopDetailMax bounds one entry's detail, as the report bus bounds a
// message.
const lastStopDetailMax = 1024

// Code is the doctor code that reports an entry of this reason.
func (r LastStopReason) Code() string {
	switch r {
	case LastStopFatalPolicy, LastStopBootRefused:
		return diagnostic.CodeDoctorPluginFatalStop
	case LastStopCleanupLate:
		return diagnostic.CodeDoctorPluginStopCleanupLate
	case LastStopUnspecified:
		panic("BUG: LastStopReason.Code on an unspecified reason")
	}
	panic("BUG: LastStopReason.Code on an unknown reason")
}

// MarshalText spells the reason for the stored JSON.
func (r LastStopReason) MarshalText() ([]byte, error) {
	switch r {
	case LastStopFatalPolicy:
		return []byte("fatal-policy"), nil
	case LastStopBootRefused:
		return []byte("boot-refused"), nil
	case LastStopCleanupLate:
		return []byte("stop-cleanup-late"), nil
	case LastStopUnspecified:
		return nil, errors.New("last-stop reason is unspecified")
	}
	return nil, fmt.Errorf("last-stop reason %d is unknown", r)
}

// UnmarshalText reads a stored reason. The record is read from disk, so an
// unknown spelling is refused rather than read as some reason.
func (r *LastStopReason) UnmarshalText(text []byte) error {
	switch string(text) {
	case "fatal-policy":
		*r = LastStopFatalPolicy
	case "boot-refused":
		*r = LastStopBootRefused
	case "stop-cleanup-late":
		*r = LastStopCleanupLate
	default:
		return fmt.Errorf("last-stop reason %q is unknown", text)
	}
	return nil
}

// LastStopEntry is one recorded reason.
type LastStopEntry struct {
	Reason LastStopReason `json:"reason"`
	// Plugin names the plugin, or for a late cleanup the owner, that caused
	// the entry. A boot refusal names it when the refusal is a missing
	// dependency (registry.MissingDependencyError); any other refusal leaves
	// it empty and Detail carries the whole refusal.
	Plugin string    `json:"plugin,omitempty"`
	Detail string    `json:"detail"`
	Time   time.Time `json:"time"`
}

// LastStop is the whole record: its entries in the order they were written,
// and how many more were written past lastStopEntriesMax.
type LastStop struct {
	Entries []LastStopEntry `json:"entries"`
	Dropped int             `json:"dropped,omitempty"`
}

// lastStopMu serializes the record's read-modify-write within this process.
var lastStopMu sync.Mutex

// RecordLastStop appends entry to the record in the daemon's store, stamped
// with the current time. It refuses an unspecified reason and an empty detail,
// and answers statestore.ErrUnavailable when the daemon has no store. Safe for
// concurrent use.
func RecordLastStop(entry LastStopEntry) error {
	if entry.Reason == LastStopUnspecified {
		return errors.New("last-stop entry has no reason")
	}
	if entry.Detail == "" {
		return errors.New("last-stop entry has no detail")
	}
	if len(entry.Detail) > lastStopDetailMax {
		entry.Detail = entry.Detail[:lastStopDetailMax]
	}
	entry.Time = time.Now().UTC()

	lastStopMu.Lock()
	defer lastStopMu.Unlock()
	record, _, err := ReadLastStop()
	if err != nil {
		return err
	}
	if len(record.Entries) < lastStopEntriesMax {
		record.Entries = append(record.Entries, entry)
	} else {
		record.Dropped++
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode last-stop record: %w", err)
	}
	return statestore.Write(zefs.KeyPluginLastStop.Pattern, data)
}

// RecordStopCleanupLate records that owner (a plugin engine, or the config
// transaction) did not finish its cleanup before the stop went on, detail
// saying what was left. A failed write is logged, because the stop goes on
// either way. Safe for concurrent use.
func RecordStopCleanupLate(owner, detail string) {
	recordLastStopLogged(LastStopEntry{Reason: LastStopCleanupLate, Plugin: owner, Detail: detail})
}

// ReadLastStop answers the record, false when none is stored, or the read or
// decode error; statestore.ErrUnavailable when this process has no store. The
// local doctor checks read it on the run after the stop. Safe for concurrent
// use.
func ReadLastStop() (LastStop, bool, error) {
	data, found, err := statestore.Read(zefs.KeyPluginLastStop.Pattern)
	if err != nil {
		return LastStop{}, false, err
	}
	if !found {
		return LastStop{}, false, nil
	}
	return decodeLastStop(data)
}

// ReadLastStopFrom answers the record from store, the handle a doctor check is
// given: the local `ze doctor` process registers no statestore, so it reads
// the daemon's store through the handle it opened. False when none is stored.
func ReadLastStopFrom(store statestore.Storage) (LastStop, bool, error) {
	data, err := store.ReadKey(zefs.KeyPluginLastStop.Pattern)
	if errors.Is(err, fs.ErrNotExist) {
		return LastStop{}, false, nil
	}
	if err != nil {
		return LastStop{}, false, fmt.Errorf("read %s: %w", zefs.KeyPluginLastStop.Pattern, err)
	}
	return decodeLastStop(data)
}

// decodeLastStop decodes a stored record.
func decodeLastStop(data []byte) (LastStop, bool, error) {
	var record LastStop
	if err := json.Unmarshal(data, &record); err != nil {
		return LastStop{}, false, fmt.Errorf("decode %s: %w", zefs.KeyPluginLastStop.Pattern, err)
	}
	return record, true, nil
}

// clearLastStop deletes the record. An absent record is no error.
func clearLastStop() error {
	lastStopMu.Lock()
	defer lastStopMu.Unlock()
	return statestore.Delete(zefs.KeyPluginLastStop.Pattern)
}

// recordLastStopLogged writes entry and logs a failure at Warn, under the
// entry's own doctor code: the stop goes on, and without the record the next
// `ze doctor` cannot name its cause.
//
// Each reason's line names its code as a constant, so the doctor-coverage
// gate reads which check declares it.
func recordLastStopLogged(entry LastStopEntry) {
	err := RecordLastStop(entry)
	if err == nil {
		return
	}
	if entry.Reason == LastStopCleanupLate {
		logger().Warn("could not record why ze is stopping, so the next ze doctor cannot name it; run ze init if this daemon has no store",
			"plugin", entry.Plugin, "detail", entry.Detail, "error", err, "code", diagnostic.CodeDoctorPluginStopCleanupLate)
		return
	}
	logger().Warn("could not record why ze is stopping, so the next ze doctor cannot name it; run ze init if this daemon has no store",
		"plugin", entry.Plugin, "detail", entry.Detail, "error", err, "code", diagnostic.CodeDoctorPluginFatalStop)
}

// recordStop is recordLastStopLogged for a stop this server causes, and marks
// the server so the end of its own startup does not delete the record.
func (s *Server) recordStop(entry LastStopEntry) {
	s.lastStopRecorded.Store(true)
	recordLastStopLogged(entry)
}

// settleLastStop runs when startup completes. A startup that ended with an
// error is a boot refusal and is recorded; a clean start deletes the record of
// the previous stop, unless this server already recorded a stop of its own
// (a fatal plugin that failed during startup).
func (s *Server) settleLastStop() {
	if s.startupErr != nil {
		entry := LastStopEntry{Reason: LastStopBootRefused, Detail: s.startupErr.Error()}
		if missing, ok := errors.AsType[*registry.MissingDependencyError](s.startupErr); ok {
			entry.Plugin = missing.Plugin
		}
		s.recordStop(entry)
		return
	}
	if s.lastStopRecorded.Load() {
		return
	}
	if err := clearLastStop(); err != nil {
		if errors.Is(err, statestore.ErrUnavailable) {
			// refuse-by-default: guards ReadLastStop -- with no store no record exists to clear, and the reader answers ErrUnavailable itself
			return
		}
		logger().Warn("could not delete the record of why ze last stopped, so ze doctor may name a stale cause; check the store with ze data registered",
			"error", err, "code", diagnostic.CodeDoctorPluginFatalStop)
	}
}

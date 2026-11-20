// Design: docs/research/vpp-deployment-reference.md -- runtime statistics ownership
// Related: runtime_stats.go -- the generation-bound owner that holds this snapshot

package vpp

import (
	"errors"
	"fmt"
	"io"
	"time"

	"go.fd.io/govpp/api"
)

// interfaceCounterSnapshot is one interface-statistics dump indexed by native
// interface index, so N session reads cost one dump and N map lookups rather
// than N dumps and N scans. It holds at most one row per interface VPP
// reported in that dump. It lives inside one runtimeStats lease and is guarded
// by that lease's mutex.
type interfaceCounterSnapshot struct {
	// taken is when the dump started, so every row was read at or after it.
	// The zero time means no snapshot: none was taken, or the last read failed.
	taken time.Time
	dump  api.InterfaceStats // reused between dumps, as the telemetry poller does
	rows  map[uint32]interfaceCounterRow
}

type interfaceCounterRow struct {
	counters api.InterfaceCounters
	// duplicate records that the dump carried this index more than once, so
	// no row can be trusted to belong to the interface the caller created.
	duplicate bool
}

// InterfaceCountersForGeneration answers one interface's counters from a dump
// that started no earlier than notBefore, taking a new dump only when the
// retained one is older. The caller states the age it accepts: notBefore set
// to the call time asks for a fresh read, and concurrent fresh callers share
// the one dump that starts after they arrived. A zero notBefore is refused,
// because it would accept a snapshot of unbounded age.
//
// It serves only the lease published for this exact owner and generation and
// re-checks the live generation on every answer, cached or not. A failed read,
// a missing row and a duplicate row are errors, never zero and never an older
// snapshot. Safe for concurrent use.
func InterfaceCountersForGeneration(owner *Connector, generation uint64, index uint32,
	notBefore time.Time) (api.InterfaceCounters, error) {
	if notBefore.IsZero() {
		return api.InterfaceCounters{}, errors.New("vpp: interface counters require a freshness bound")
	}
	reader, err := statsForGeneration(owner, generation)
	if err != nil {
		return api.InterfaceCounters{}, err
	}
	return reader.interfaceCounters(index, notBefore)
}

// interfaceCounters holds mu for the whole call, so the dump, its index and
// the lookup are one step, and a caller queued behind an in-flight dump sees
// the snapshot that dump produced. No provider lock is held across the dump,
// preserving the lock order documented on publish.
func (s *runtimeStats) interfaceCounters(index uint32, notBefore time.Time) (api.InterfaceCounters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return api.InterfaceCounters{}, io.ErrClosedPipe
	}
	if err := s.checkGeneration(); err != nil {
		return api.InterfaceCounters{}, err
	}
	if s.counters.expired(notBefore) {
		if err := s.refreshCountersLocked(); err != nil {
			return api.InterfaceCounters{}, err
		}
	}
	return s.counters.lookup(index)
}

// refreshCountersLocked MUST be called with mu held. A failed dump discards
// the previous snapshot, so the failure reaches every later caller until a
// dump succeeds, rather than being masked by older values.
func (s *runtimeStats) refreshCountersLocked() error {
	taken := s.clock.Now()
	s.counters.taken = time.Time{}
	err := s.readLocked("interfaces", func() error { return s.client.GetInterfaceStats(&s.counters.dump) })
	if err != nil {
		return err
	}
	s.counters.index(taken)
	return nil
}

func (snapshot *interfaceCounterSnapshot) expired(notBefore time.Time) bool {
	if snapshot.taken.IsZero() {
		return true
	}
	return snapshot.taken.Before(notBefore)
}

// index rebuilds rows from dump. The map keeps its buckets between dumps.
func (snapshot *interfaceCounterSnapshot) index(taken time.Time) {
	if snapshot.rows == nil {
		snapshot.rows = make(map[uint32]interfaceCounterRow, len(snapshot.dump.Interfaces))
	} else {
		clear(snapshot.rows)
	}
	for i := range snapshot.dump.Interfaces {
		counters := &snapshot.dump.Interfaces[i]
		row, seen := snapshot.rows[counters.InterfaceIndex]
		if seen {
			row.duplicate = true
			snapshot.rows[counters.InterfaceIndex] = row
			continue
		}
		snapshot.rows[counters.InterfaceIndex] = interfaceCounterRow{counters: *counters}
	}
	snapshot.taken = taken
}

func (snapshot *interfaceCounterSnapshot) lookup(index uint32) (api.InterfaceCounters, error) {
	row, found := snapshot.rows[index]
	if !found {
		return api.InterfaceCounters{}, fmt.Errorf("vpp: interface statistics have no row for interface index %d", index)
	}
	if row.duplicate {
		return api.InterfaceCounters{}, fmt.Errorf("vpp: interface statistics have duplicate rows for interface index %d", index)
	}
	return row.counters, nil
}

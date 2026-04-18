// Design: docs/guide/command-reference.md -- Counter baseline for `clear interface counters`
// Related: dispatch.go -- ListInterfaces/GetInterface/GetStats wrap raw backend
// reads with applyBaseline before returning to callers
// Related: backend.go -- ErrCountersNotResettable sentinel that drives baseline fallback

package iface

import (
	"errors"
	"sync"
)

// baselineStore holds per-interface counter baselines captured when the
// backend cannot physically reset counters in the kernel. A baseline
// represents the raw counter values at the moment ResetCounters was
// called; subsequent GetStats/GetInterface reads subtract this
// baseline so the operator sees "since last clear" deltas rather than
// the kernel's monotonic since-boot totals.
//
// The store also survives kernel-level counter resets (interface
// bounce, driver reload). If a subsequent read observes a raw value
// LOWER than the baseline, we interpret that as a wrap/reset and
// rebase the baseline to zero so the operator continues to see a
// sane monotonically-increasing delta from that point forward.
//
// Concurrent access: a sync.RWMutex guards the map. Reads on the hot
// path (every GetStats) take the read lock; ResetCounters takes the
// write lock.
type baselineStore struct {
	mu   sync.RWMutex
	data map[string]InterfaceStats
}

// baselines is the process-wide store. Keyed by interface name; value
// is the raw stats snapshot captured by ResetCounters when the backend
// could not physically clear.
var baselines = &baselineStore{data: map[string]InterfaceStats{}}

// setBaseline records the given raw stats as the baseline for name.
// Pass nil stats to drop the baseline (e.g. backend succeeded in a real
// reset, no baseline needed). All-zeros stats ARE valid -- that represents
// "cleared from the operator's viewpoint at a moment when counters
// happened to be zero", not "no baseline".
func (s *baselineStore) setBaseline(name string, stats *InterfaceStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stats == nil {
		delete(s.data, name)
		return
	}
	s.data[name] = *stats
}

// clearAllBaselines drops every baseline. Called when the backend
// confirms it performed a real reset on every interface (name == "").
func (s *baselineStore) clearAllBaselines() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]InterfaceStats{}
}

// applyBaseline subtracts the baseline from in-place raw stats when a
// baseline is present for name. Wrap detection: if any RAW counter is
// strictly less than the baseline for the same field, the kernel must
// have reset (driver reload, interface deleted+recreated). On a wrap
// we drop the baseline so subsequent reads return the raw value from
// the kernel's new zero. This is the information-preserving variant
// of "rebase" -- it keeps the counts accumulated since the kernel
// reset rather than discarding them (rebase-to-current-raw would lose
// any traffic between kernel-reset and the first post-reset read). An
// operator who wants a hard reset-to-zero at detection time can call
// `clear interface <name> counters` again; the first call's baseline
// was already dropped, so the second call captures the current raw
// (zero-from-kernel plus whatever traffic hit since) as the new
// baseline, giving a fresh zero-point.
//
// No-op when no baseline exists for name or when stats is nil.
func (s *baselineStore) applyBaseline(name string, stats *InterfaceStats) {
	if stats == nil {
		return
	}
	s.mu.RLock()
	base, ok := s.data[name]
	s.mu.RUnlock()
	if !ok {
		return
	}

	if wrapped(stats, &base) {
		// Rebase: drop the baseline so subsequent reads show raw values
		// (operator sees counters from the kernel's new zero).
		s.mu.Lock()
		delete(s.data, name)
		s.mu.Unlock()
		return
	}

	stats.RxBytes -= base.RxBytes
	stats.RxPackets -= base.RxPackets
	stats.RxErrors -= base.RxErrors
	stats.RxDropped -= base.RxDropped
	stats.TxBytes -= base.TxBytes
	stats.TxPackets -= base.TxPackets
	stats.TxErrors -= base.TxErrors
	stats.TxDropped -= base.TxDropped
}

// wrapped reports whether any raw counter dropped below its baseline,
// which indicates a kernel-level reset since the baseline was captured.
// Any single field going backwards is sufficient evidence -- we do not
// require all fields to regress simultaneously because drivers sometimes
// reset subsets.
func wrapped(raw, base *InterfaceStats) bool {
	return raw.RxBytes < base.RxBytes ||
		raw.RxPackets < base.RxPackets ||
		raw.RxErrors < base.RxErrors ||
		raw.RxDropped < base.RxDropped ||
		raw.TxBytes < base.TxBytes ||
		raw.TxPackets < base.TxPackets ||
		raw.TxErrors < base.TxErrors ||
		raw.TxDropped < base.TxDropped
}

// resetCountersViaBackend is the shared body of iface.ResetCounters. It
// first asks the backend to physically zero the kernel counters; if the
// backend returns ErrCountersNotResettable it reads the current raw
// stats and captures them as a baseline so future deltas start at zero
// from the operator's viewpoint.
//
// Splitting this out of dispatch.go keeps the baseline policy in the
// same file as the store.
func resetCountersViaBackend(b Backend, name string) error {
	err := b.ResetCounters(name)
	if err == nil {
		// Real reset succeeded: drop any stale baseline(s).
		if name == "" {
			baselines.clearAllBaselines()
		} else {
			baselines.setBaseline(name, nil)
		}
		return nil
	}
	if !errors.Is(err, ErrCountersNotResettable) {
		return err
	}

	// Fallback: capture current raw stats as the baseline. When name is
	// empty we capture every interface's baseline so subsequent reads
	// all show zero deltas.
	if name != "" {
		stats, err := b.GetStats(name)
		if err != nil {
			return err
		}
		baselines.setBaseline(name, stats)
		return nil
	}

	ifs, err := b.ListInterfaces()
	if err != nil {
		return err
	}
	baselines.mu.Lock()
	baselines.data = make(map[string]InterfaceStats, len(ifs))
	for i := range ifs {
		if ifs[i].Stats == nil {
			continue
		}
		baselines.data[ifs[i].Name] = *ifs[i].Stats
	}
	baselines.mu.Unlock()
	return nil
}

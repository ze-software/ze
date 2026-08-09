// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Per-flow delta tracking

package conntrack

import (
	"net/netip"
	"sync"
	"time"
)

// flowKey identifies a conntrack flow by its 5-tuple.
// Used instead of CTA_ID which vishvananda/netlink does not expose.
type flowKey struct {
	SrcAddr  netip.Addr
	DstAddr  netip.Addr
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8
}

func keyOf(e FlowEntry) flowKey {
	return flowKey{
		SrcAddr:  e.SrcAddr,
		DstAddr:  e.DstAddr,
		SrcPort:  e.SrcPort,
		DstPort:  e.DstPort,
		Protocol: e.Protocol,
	}
}

type lastExported struct {
	bytes    uint64
	packets  uint64
	lastSeen time.Time
	// gen is the generation of the observation that set this baseline. The dump
	// worker advances the generation once per dump cycle (BeginDump) and tags
	// every entry of that snapshot with it; a destroy event rides the latest
	// dump generation. Comparing an incoming observation's generation against the
	// baseline's is what tells a stale in-flight dump snapshot (<= baseline gen)
	// from a genuine new flow that reused the 5-tuple (a later generation). See
	// computeLocked for the full ordering rule.
	gen uint64
	// destroyedAt is zero for a live flow and set when a conntrack destroy
	// event tombstones the entry (see ComputeDeltaFinal). A tombstoned entry
	// keeps its cumulative baseline so a periodic dump whose snapshot predates
	// the teardown still computes a zero delta, but is reclaimed promptly by
	// SweepTombstones rather than waiting for Cleanup at 2x active-timeout.
	destroyedAt time.Time
}

// tombstoned reports whether this entry has been marked torn down. A tombstoned
// baseline is also a "terminal" baseline: the flow is gone, so a later
// observation at the same generation cannot be newer than it.
func (le lastExported) tombstoned() bool { return !le.destroyedAt.IsZero() }

// DeltaTracker computes per-flow byte/packet deltas between successive
// conntrack dumps. Keyed by 5-tuple. Thread-safe.
//
// SCALING RISK -- READ BEFORE RAISING active-timeout OR REUSING AT HIGH RATE.
// This tracker holds one map entry per recently-seen flow. Memory is
// (flow-residency x flow-rate x ~175 bytes), and the residency of a torn-down
// flow is bounded by SweepTombstones' grace (seconds) for clean teardowns and
// by Cleanup's 2x active-timeout for flows whose destroy event was missed.
// Two consequences that bite at scale:
//
//  1. Go maps never shrink on delete. A churn BURST sets the backing array to
//     its peak size and holds that memory for the lifetime of this tracker
//     (recreated only on config reload). Keeping residency short (the
//     tombstone path) is what stops a transient spike becoming permanent bloat.
//  2. This design is for edge / moderate scale (<= low-thousands of new
//     flows/sec). It does NOT scale to 100G internet-mix churn (10k-1M new
//     flows/sec): even at a 5s tombstone grace that is millions of live
//     entries, the full-table netlink dump and the single mutex below become
//     the bottlenecks, and the destroy multicast socket drops events. For 100G
//     flow export use SAMPLING (sFlow / IPFIX-sampled via the sampling worker),
//     whose cost is independent of flow rate -- not per-flow conntrack export.
type DeltaTracker struct {
	mu    sync.Mutex
	state map[flowKey]lastExported
	// gen is the current dump generation, advanced by BeginDump once per dump
	// cycle. Monotonic; never wraps in practice (one increment per active-timeout
	// is billions of years to overflow uint64).
	gen uint64
}

// NewDeltaTracker creates an empty delta tracker.
func NewDeltaTracker() *DeltaTracker {
	return &DeltaTracker{state: make(map[flowKey]lastExported)}
}

// BeginDump advances the generation counter for a new dump cycle and returns the
// new value. The dump worker calls this once per dump (before reading the kernel
// table) and passes the returned generation to ComputeDelta for every entry in
// that snapshot, so all entries of one dump share a generation. Generations let
// the tracker distinguish a stale in-flight dump observation (generation <= the
// recorded baseline) from a genuine new flow that reused the 5-tuple (a later
// generation) -- see computeLocked.
func (d *DeltaTracker) BeginDump() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gen++
	return d.gen
}

// computeLocked records a newer observation's cumulative counters as the new
// baseline and returns the delta since the last observation. gen is the
// observation's generation; terminal is true for a destroy event. Caller holds
// d.mu.
//
// GENERATION ORDERING (do not break -- violation silently corrupts counts):
// an observation updates the baseline and produces a delta ONLY when it is
// NEWER than the recorded baseline. "Newer" means a strictly later generation,
// or -- at the same generation -- a terminal (destroy) observation refining a
// not-yet-terminal (dump) baseline. Everything else is a stale in-flight
// snapshot: it returns a zero delta and leaves the baseline (and any tombstone)
// untouched.
//
// This ordering is what fixes the intermediate-cumulative stale-dump
// over-count. Consider: last export 600 (gen N-1); a dump snapshot captures the
// flow at an intermediate 700 (gen N) but its per-entry processing is delayed;
// the destroy event fires recording the final 800 (also gen N, terminal) and is
// processed first. The destroy is newer (terminal at gen N over a gen N-1
// baseline) -> exports residual 200, baseline becomes (800, gen N, terminal).
// The trailing dump entry (700, gen N, non-terminal) is then NOT newer than the
// terminal gen-N baseline -> zero delta. Without the generation it would read
// 700 < 800, mistake the lower cumulative for a counter reset, and re-export the
// full 700 while reviving the flow as live.
//
// Counter equality is no longer the tombstone-preservation signal (it could not
// tell an intermediate cumulative from the final one); the generation is. A
// genuine port reuse -- a fresh flow landing on a torn-down 5-tuple -- arrives
// in a LATER dump cycle (generation > the tombstone's), so it is newer, clears
// the tombstone by omission, and its reset-lower counters report in full.
func (d *DeltaTracker) computeLocked(entry FlowEntry, gen uint64, terminal bool) FlowEntry {
	key := keyOf(entry)
	prev, exists := d.state[key]

	if !exists {
		d.state[key] = lastExported{
			bytes:    entry.Bytes,
			packets:  entry.Packets,
			lastSeen: entry.LastSeen,
			gen:      gen,
		}
		return entry // full counters on first observation
	}

	// Newer iff a strictly later generation, or a terminal observation refining a
	// non-terminal baseline at the same generation (the destroy reflects kernel
	// state at least as new as the latest dump snapshot, so it wins the tie). A
	// non-terminal observation at or below the baseline generation is a stale
	// in-flight dump snapshot: report no new traffic and keep the newer baseline.
	newer := gen > prev.gen || (gen == prev.gen && terminal && !prev.tombstoned())
	if !newer {
		stale := entry
		stale.Bytes = 0
		stale.Packets = 0
		return stale
	}

	// Newer observation: replace the baseline. destroyedAt is intentionally left
	// zero here -- ComputeDeltaFinal sets it after this returns. A non-terminal
	// (dump) observation that revives a previously tombstoned key clears the
	// tombstone by this omission.
	d.state[key] = lastExported{
		bytes:    entry.Bytes,
		packets:  entry.Packets,
		lastSeen: entry.LastSeen,
		gen:      gen,
	}

	result := entry
	// Counter wrap / reset / port reuse: if the new cumulative is below the
	// baseline, report the full new value (treat it as a fresh flow from zero).
	if entry.Bytes >= prev.bytes {
		result.Bytes = entry.Bytes - prev.bytes
	} else {
		result.Bytes = entry.Bytes
	}
	if entry.Packets >= prev.packets {
		result.Packets = entry.Packets - prev.packets
	} else {
		result.Packets = entry.Packets
	}
	return result
}

// ComputeDelta returns a copy of entry with Bytes and Packets replaced by the
// delta since the last call for this flow (full counters on first observation).
// Used by the periodic dump path, which only ever observes live flows. gen is
// the dump cycle's generation from BeginDump; every entry of one dump passes the
// same generation.
func (d *DeltaTracker) ComputeDelta(entry FlowEntry, gen uint64) FlowEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.computeLocked(entry, gen, false)
}

// ComputeDeltaFinal computes the residual delta for a flow the kernel has torn
// down and tombstones the entry in one locked step, so SweepTombstones can
// reclaim it within the grace window instead of waiting for Cleanup at 2x
// active-timeout. Doing both under one lock (rather than ComputeDelta followed
// by a separate mark) closes the window where a stale dump could land between
// the two and clear the not-yet-set tombstone.
//
// The destroy event rides the current (latest) dump generation and is terminal:
// it ties the latest dump's generation but wins the tie, and it is strictly
// older than any later dump cycle (so a port-reused 5-tuple in a future dump
// still revives the entry). now is injected for testability.
func (d *DeltaTracker) ComputeDeltaFinal(entry FlowEntry, now time.Time) FlowEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	res := d.computeLocked(entry, d.gen, true)
	key := keyOf(entry)
	le := d.state[key]
	// Keep the earliest teardown time: a duplicate destroy event (stale, so
	// computeLocked left the existing tombstone in place) must not extend
	// residency by refreshing destroyedAt.
	if le.destroyedAt.IsZero() {
		le.destroyedAt = now
		d.state[key] = le
	}
	return res
}

// Cleanup removes entries not seen for longer than maxAge. Call periodically at
// 2x the active-timeout. This is the backstop for flows whose destroy event was
// never delivered (the destroy listener is unavailable, or the multicast socket
// dropped the event under load) -- cleanly torn-down flows are reclaimed faster
// by SweepTombstones. NOTE: maxAge is tied to active-timeout, so a large
// active-timeout lengthens residency for missed-event flows; see the SCALING
// RISK on DeltaTracker.
func (d *DeltaTracker) Cleanup(maxAge time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for k, le := range d.state {
		if le.lastSeen.Before(cutoff) {
			delete(d.state, k)
		}
	}
}

// SweepTombstones reclaims entries tombstoned by ComputeDeltaFinal more than
// grace ago. This is the fast reclaim path for cleanly torn-down flows; it
// keeps residency (and therefore memory and the peak map size) bounded by grace
// instead of by 2x active-timeout.
//
// GRACE BOUNDS (do not break):
//   - LOWER: grace MUST exceed the longest dump read->process window. A stale
//     dump observes a torn-down flow AFTER the destroy event tombstoned it; if
//     the tombstone is swept before that dump runs, the dump sees no baseline,
//     treats the flow as new, and re-exports its full cumulative -- the exact
//     double-count the tombstone exists to prevent.
//   - UPPER: keep grace well under 2x active-timeout, or it provides no benefit
//     over Cleanup.
//
// now is injected for testability.
func (d *DeltaTracker) SweepTombstones(grace time.Duration, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := now.Add(-grace)
	for k, le := range d.state {
		if le.tombstoned() && le.destroyedAt.Before(cutoff) {
			delete(d.state, k)
		}
	}
}

// Len returns the number of tracked flows. Used for metrics.
func (d *DeltaTracker) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.state)
}

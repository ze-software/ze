// Design: plan/spec-wire-edit-5-fanout-dedup.md -- one materialization per policy group
// Related: reactor_api_forward.go -- forwardUpdateCore, the API rail's destination loop
// Related: forward_rs.go -- reactorForwardRS, the route-server rail's destination loop
// Related: forward_build.go -- buildModifiedPayload, the materialization being counted
package reactor

import (
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/metrics"
)

// Fan-out accounting.
//
// A materialization is one per-destination payload rebuild: the plan, size and
// write walk of buildModifiedPayload plus the buffer it acquires. A fan-out of N
// destinations over G distinct edit sets performs N of them before this change
// and G of them after, so the counter is what turns "dedup works" from an
// assertion into a number a test and a running daemon can both read.
//
// Plain atomics rather than metrics alone, because the tests are the primary
// consumer and a test that must stand up a Prometheus registry to count an event
// is a test nobody writes. The Prometheus counters below are fed from the same
// three call sites, so there is one event with two readers, not two event
// sources that can disagree.
var (
	fwdMaterializations atomic.Uint64
	fwdDedupHits        atomic.Uint64
	fwdDedupCollisions  atomic.Uint64
	fwdDedupCapacity    atomic.Uint64
)

// fwdDedupMetrics holds the Prometheus face of the counters above.
type fwdDedupMetrics struct {
	materializations metrics.Counter
	dedupHits        metrics.Counter
	dedupCollisions  metrics.Counter
	dedupCapacity    metrics.Counter
}

// fwdDedupMetricsPtr stays nil until the reactor wires a registry, so every
// recorder guards its use and a build with metrics disabled costs one nil load.
var fwdDedupMetricsPtr atomic.Pointer[fwdDedupMetrics]

// SetForwardDedupMetrics creates the fan-out dedup metrics from the given
// registry. A nil registry is a no-op, leaving the Prometheus half disabled; the
// atomics above are unconditional and keep working either way.
func SetForwardDedupMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	fwdDedupMetricsPtr.Store(&fwdDedupMetrics{
		materializations: reg.Counter(
			"ze_bgp_update_materializations_total",
			"Per-destination UPDATE payload rebuilds performed while forwarding.",
		),
		dedupHits: reg.Counter(
			"ze_bgp_update_dedup_hits_total",
			"Forward destinations that reused another destination's rebuild because their edit sets were equal.",
		),
		dedupCollisions: reg.Counter(
			"ze_bgp_update_dedup_collisions_total",
			"Fingerprint matches refused by the full edit-set equality check. A non-zero value is a hash collision, never a wire error.",
		),
		dedupCapacity: reg.Counter(
			"ze_bgp_update_dedup_capacity_drops_total",
			"Equality classes not recorded because the forward call's digest arena was full. Those destinations rebuild on their own.",
		),
	})
}

// recordMaterialization counts one per-destination payload rebuild.
func recordMaterialization() {
	fwdMaterializations.Add(1)
	if m := fwdDedupMetricsPtr.Load(); m != nil {
		m.materializations.Inc()
	}
}

// recordDedupHit counts one destination that reused an existing rebuild.
func recordDedupHit() {
	fwdDedupHits.Add(1)
	if m := fwdDedupMetricsPtr.Load(); m != nil {
		m.dedupHits.Inc()
	}
}

// recordDedupCollision counts one fingerprint match the equality check refused.
//
// The fingerprint is a hint and the equality check is the authorization, so a
// refused candidate is correct behavior rather than an error. It is counted
// because a silent refusal would hide the one observable that separates "the
// hash is doing its job" from "the hash has become degenerate"
// (ai/rules/fail-closed-guards.md: a guard must fail closed or say something).
func recordDedupCollision() {
	fwdDedupCollisions.Add(1)
	if m := fwdDedupMetricsPtr.Load(); m != nil {
		m.dedupCollisions.Inc()
	}
}

// recordDedupCapacity counts one equality class the table could not record
// because its digest arena was full.
//
// The route-server rail's previous four-slot body cache stopped caching beyond
// four bodies and said nothing, so a deployment past that width silently lost
// the optimization it was configured for. The replacement is far wider and it
// counts what it drops (the umbrella's no-silent-caps rule).
func recordDedupCapacity() {
	fwdDedupCapacity.Add(1)
	if m := fwdDedupMetricsPtr.Load(); m != nil {
		m.dedupCapacity.Inc()
	}
}

// What the identity is, and why it is one pointer.
//
// The shared object is the REBUILT PAYLOAD, and buildModifiedPayload's own
// signature is the whole argument for what that payload depends on:
//
//	buildModifiedPayload(payload, mods, handlers, pp, nlriOverride)
//
// The forward rails pass a nil nlriOverride and the reactor's own handler map,
// which is fixed for the process. pp decides WHERE the bytes are written, never
// WHAT they are. So the output is a function of exactly two things: the source
// payload, and the edit set. The edit set is covered by the digest. The payload
// is covered by this.
//
// It is the *wireu.WireUpdate pointer rather than the bytes because a WireUpdate
// is immutable once built, so one pointer is one byte string. Two distinct
// pointers carrying equal bytes -- two export-policy overrides that happen to
// agree -- read as different bases and each materialize. That is a missed
// optimization in the safe direction, which is the only direction a mistake here
// is allowed to go.
//
// Notably ABSENT: the destination context, the extended-message flag and the
// maximum message size. Those decide how the payload is SPLIT and framed, which
// buildFwdBody does per destination and this change does not share. Putting them
// in the identity would split equality classes that produce identical bytes and
// buy nothing.
type fwdDedupIdentity struct {
	base *wireu.WireUpdate
}

// fwdDedupEntry is one equality class: the first destination's rebuilt bytes,
// plus the digest that authorizes another destination to reuse them.
type fwdDedupEntry struct {
	id      fwdDedupIdentity
	fp      uint64
	from    int // digest window in the table's blob
	to      int
	payload []byte
}

// fwdDedupBlobMax bounds the digest arena of one forward call.
//
// Attribute values are peer-influenceable, so an arena that grew with the
// destination count times the largest possible digest would be a memory
// amplifier a peer could aim. Past the ceiling the table stops RECORDING new
// classes and keeps answering from the ones it has, and it counts the drop.
const fwdDedupBlobMax = 64 << 10

// fwdDedupMaxClasses bounds the equality classes one forward call records, and
// fwdDedupIndexSlots is the open-addressed index over them.
//
// The bound exists so the index can be a fixed array probed in constant time
// rather than a slice scanned linearly. That is not a micro-optimization: a
// linear scan makes a fan-out where nothing shares O(destinations squared), and
// the measurement showed it costing 12% at 100 destinations in 100 groups --
// paid exactly where the feature buys nothing.
//
// 128 classes is far past any real deployment. A route reflector's clients share
// the reflector's own next-hop and cluster; RFC 7947 route-server clients share
// their community policy. Past the bound the table stops recording and counts
// the drop, which is the behavior the previous four-slot route-server cache
// owed and did not have.
const (
	fwdDedupMaxClasses  = 128
	fwdDedupIndexSlots  = 256 // power of two, twice the class bound: load factor <= 0.5
	fwdDedupIndexMask   = fwdDedupIndexSlots - 1
	fwdDedupIndexAbsent = 0 // a slot holds an entry index PLUS ONE, so zero is empty
)

// fwdDedupTable is the per-call equality-class table.
//
// Per CALL, never longer: it holds slices into per-peer pool buffers that the
// forward items own, so an entry that outlived the call would name memory
// another UPDATE had been written into. Both rails accumulate every item in
// `pending` and dispatch only after the destination loop ends, which is what
// makes holding those slices safe for the loop's duration -- no item can have
// been released while the table is alive.
type fwdDedupTable struct {
	blob    []byte
	entries []fwdDedupEntry
	// index maps a fingerprint to an entry by open addressing with linear
	// probing. A miss stops at the first empty slot, so a fan-out whose
	// destinations share nothing pays one probe each instead of a scan.
	index [fwdDedupIndexSlots]int32
}

// fwdDedupCand is a destination that missed and must materialize. It carries the
// window its digest occupies so the table can record the class afterwards, or
// roll the digest back if the destination is suppressed instead.
type fwdDedupCand struct {
	id    fwdDedupIdentity
	fp    uint64
	from  int
	to    int
	valid bool
}

var fwdDedupTablePool = sync.Pool{New: func() any { return new(fwdDedupTable) }}

// getFwdDedupTable borrows a table for one forward call.
func getFwdDedupTable() *fwdDedupTable {
	t, _ := fwdDedupTablePool.Get().(*fwdDedupTable)
	return t
}

// putFwdDedupTable clears every reference the table holds and returns it.
//
// The clear is the load-bearing half. Entries hold slices into per-peer pool
// buffers; a table pooled with those still set would keep a returned buffer
// reachable and, worse, offer it to the next call as a shareable payload.
func putFwdDedupTable(t *fwdDedupTable) {
	if len(t.entries) > 0 {
		clear(t.index[:])
	}
	clear(t.entries)
	t.entries = t.entries[:0]
	t.blob = t.blob[:0]
	fwdDedupTablePool.Put(t)
}

// begin asks whether an earlier destination in this call already rebuilt exactly
// these bytes.
//
// It returns the shared payload on a confirmed hit. On a miss it returns a
// candidate the caller passes to commit once it has materialized, or to abandon
// if the destination is suppressed instead. An edit set that cannot be digested
// returns an invalid candidate and no payload, which makes the destination
// materialize on its own -- the same outcome as before this change.
//
// The two steps have different authority and the order is the whole design. The
// fingerprint SELECTS candidates. filterapi.EditDigestEqual AUTHORIZES the
// reuse, unconditionally and with no fast path, because a fingerprint match
// between two different edit sets would otherwise send this destination another
// destination's wire (ai/rules/fail-closed-guards.md).
// A nil table is the DISABLED state, and every method reads it as "share
// nothing". That is the fail-closed direction: a destination that is told
// nothing is shareable rebuilds its own bytes, which is exactly the behavior
// this change replaces (ai/rules/fail-closed-guards.md).
func (t *fwdDedupTable) begin(id fwdDedupIdentity, mods *filterapi.ModAccumulator) ([]byte, fwdDedupCand) {
	if t == nil {
		return nil, fwdDedupCand{}
	}
	start := len(t.blob)
	blob, ok := mods.AppendEditDigest(t.blob)
	if !ok {
		return nil, fwdDedupCand{}
	}
	t.blob = blob
	digest := t.blob[start:]
	fp := filterapi.EditFingerprint(digest)

	for slot := int(fp & fwdDedupIndexMask); ; slot = (slot + 1) & fwdDedupIndexMask {
		v := t.index[slot]
		if v == fwdDedupIndexAbsent {
			break
		}
		e := &t.entries[v-1]
		if e.fp != fp || e.id != id {
			continue
		}
		if filterapi.EditDigestEqual(digest, t.blob[e.from:e.to]) {
			t.blob = t.blob[:start]
			recordDedupHit()
			return e.payload, fwdDedupCand{}
		}
		// Same hint, different edit set. The comparison refused it, which is the
		// guard working rather than failing -- but a silent refusal would leave
		// no way to tell a healthy hash from a degenerate one.
		recordDedupCollision()
	}

	return nil, fwdDedupCand{id: id, fp: fp, from: start, to: len(t.blob), valid: true}
}

// commit records a materialized destination as the first of its equality class.
func (t *fwdDedupTable) commit(c fwdDedupCand, payload []byte) {
	if t == nil {
		return
	}
	if !c.valid || payload == nil {
		t.abandon(c)
		return
	}
	if len(t.entries) >= fwdDedupMaxClasses || len(t.blob) > fwdDedupBlobMax {
		recordDedupCapacity()
		t.abandon(c)
		return
	}
	t.entries = append(t.entries, fwdDedupEntry{id: c.id, fp: c.fp, from: c.from, to: c.to, payload: payload})
	for slot := int(c.fp & fwdDedupIndexMask); ; slot = (slot + 1) & fwdDedupIndexMask {
		if t.index[slot] == fwdDedupIndexAbsent {
			t.index[slot] = int32(len(t.entries)) //nolint:gosec // G115: bounded by fwdDedupMaxClasses
			return
		}
	}
}

// abandon drops a candidate's digest when its destination produced no bytes to
// share, so a suppressed destination leaves the arena exactly as it found it.
func (t *fwdDedupTable) abandon(c fwdDedupCand) {
	if t == nil {
		return
	}
	if c.valid && c.to == len(t.blob) {
		t.blob = t.blob[:c.from]
	}
}

// copyMaterialization writes an equality class's bytes into this destination's
// OWN buffer and returns it with the pool index that releases it.
//
// This is what keeps the ownership model untouched. Sharing the buffer itself
// would give one buffer several forward items and require a reference count with
// a release ordered after the last worker's write -- the failure mode with the
// worst blast radius in this whole umbrella. The measurement said not to: the
// rebuild it replaces is 416ns and this copy is 2ns, so 99.5% of the win is
// available without one byte of shared mutable state.
//
// Mirrors acquireModBuf's tiering: the destination's own outgoing pool first, a
// plain allocation when that pool is dry.
func copyMaterialization(src []byte, pp *peerPool) ([]byte, int) {
	if pp != nil {
		if b, idx := pp.Get(); idx > 0 {
			if len(b) >= len(src) {
				copy(b, src)
				return b[:len(src)], idx
			}
			pp.Return(idx)
		}
	}
	out := make([]byte, len(src)) // pool-fallback, as buildModifiedPayload does
	copy(out, src)
	return out, 0
}

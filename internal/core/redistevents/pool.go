// Design: docs/architecture/core-design.md -- pooled batch payload
// Related: events.go -- batch and entry types pooled here
// Related: registry.go -- ProtocolID lookup used by pool consumers
//
// The batch sync.Pool exists so producers can emit at sustained burst rates
// (AC-13: 500 events / invocation) without per-event heap allocation. Per
// rules/design-principles.md "Pool strategy by goroutine shape (b)", we use
// sync.Pool seeded for peak concurrent producers; producers Get -> fill ->
// Emit -> Put.
//
// Backing slice (Entries) is recycled with the batch. Producers are expected
// to keep entry counts within the seeded capacity; growth on the hot path
// surfaces as a sizing bug in the burst .ci test, not as silent allocation.

package redistevents

import (
	"sync"

	"github.com/ze-software/ze/internal/core/memguard"
)

// EntriesCap is the seeded capacity for a fresh batch's Entries slice. Sized
// to accommodate typical bursts (L2TP session events, connected-route mass
// changes) without growth. The burst test (AC-13) drives N=500 entries
// across 500 separate batches, not within one batch -- so this seed only
// needs to cover the largest single emission.
//
// 64 is the working-set capacity used by other ze pool seeds for similar
// per-batch entry slices. If a producer regularly exceeds it, raise this
// constant rather than papering over with append() growth.
const EntriesCap = 64

var batchPool = sync.Pool{
	New: func() any {
		return &RouteChangeBatch{
			Entries: make([]RouteChangeEntry, 0, EntriesCap),
		}
	},
}

// AcquireBatch returns a clean *RouteChangeBatch from the pool. Caller MUST
// call ReleaseBatch when done with the batch (typically immediately after
// Emit returns).
//
// The returned batch has Protocol/AFI/SAFI zero and Entries empty (len=0)
// but with the seeded capacity preserved. Callers fill the fields and
// append entries before Emit.
func AcquireBatch() *RouteChangeBatch {
	b, _ := batchPool.Get().(*RouteChangeBatch)
	// Defense in depth: if the pool ever returned a non-clean batch (e.g. a
	// caller forgot to release a half-filled one), zero the header here.
	// Entries is already truncated by ReleaseBatch.
	b.Protocol = ProtocolUnspecified
	b.AFI = 0
	b.SAFI = 0
	b.ReplayID = 0
	b.OriginASN = 0
	b.Community = nil
	b.Entries = b.Entries[:0]
	return b
}

// ReleaseBatch returns a batch to the pool. Caller MUST NOT use the batch
// after this call. Safe to call with a nil batch (no-op) so producers can
// defer ReleaseBatch immediately after AcquireBatch without an extra nil
// check.
//
// Truncates Entries to len=0 but keeps the backing array so the next
// AcquireBatch reuses it. Per the EventBus contract, every subscriber has
// returned by the time Emit returns, so the producer is the sole owner here.
func ReleaseBatch(b *RouteChangeBatch) {
	if b == nil {
		return
	}
	// Drop per-entry references (Prefix / NextHop) so a recycled batch never
	// leaks a prior route's data. clear() resets EVERY entry field, including
	// value-type additions like Metric and OriginAS, so no per-field reset is
	// needed. Debug builds poison instead of clearing: a batch read after this
	// synchronous-dispatch boundary then surfaces a recognizable sentinel in the
	// scalar fields (Action/Metric/Table/OriginAS) rather than a plausible-looking
	// zero. Prefix/NextHop stay zero either way -- a zero netip is already
	// IsValid()==false (not 0.0.0.0/0) and its z pointer must stay nil for GC
	// safety, so those fields carry no sentinel (contract D;
	// docs/architecture/memory/lifetime-contracts.md). memguard.Enabled folds the
	// branch away in release.
	if memguard.Enabled {
		poisonReleasedEntries(b.Entries)
	} else {
		clear(b.Entries)
	}
	b.Entries = b.Entries[:0]
	b.Protocol = ProtocolUnspecified
	b.AFI = 0
	b.SAFI = 0
	b.ReplayID = 0
	b.OriginASN = 0
	// Drop the reference only: unlike Entries (pool-owned), the Community
	// backing array belongs to the producer (typically config-owned), so
	// clearing it would corrupt the producer's data.
	b.Community = nil
	batchPool.Put(b)
}

// actionPoison is an out-of-range RouteAction written into released entries in
// debug builds. It is never a valid action (Add/Remove), so a poisoned entry
// read after release is unmistakable.
const actionPoison RouteAction = 0xEE

// poisonReleasedEntries overwrites each released entry's SCALAR fields with a
// recognizable sentinel (debug builds only, gated by the caller on
// memguard.Enabled) so a batch retained past its synchronous-dispatch boundary
// reads an obviously-invalid Action/Metric/Table/OriginAS instead of a
// plausible-looking zero.
//
// RouteChangeEntry embeds netip.Addr/Prefix, whose internal pointer must stay
// nil for GC safety, so this cannot raw byte-poison the struct (a fabricated
// pointer would crash the GC). It therefore poisons only the scalar fields and
// leaves the netip fields zero -- a zero netip is already IsValid()==false, so
// it needs no sentinel, and its nil z pointer is GC-safe. The struct assignment
// also drops the prior entry's Prefix/NextHop references, matching clear()'s
// no-leak guarantee. See docs/architecture/memory/lifetime-contracts.md (D).
func poisonReleasedEntries(entries []RouteChangeEntry) {
	const mark uint32 = 0xDEADBEEF
	for i := range entries {
		entries[i] = RouteChangeEntry{
			Action:   actionPoison,
			Metric:   mark,
			Table:    mark,
			OriginAS: mark,
		}
	}
}

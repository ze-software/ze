// Design: docs/architecture/core-design.md -- IGP next-hop cost seam

// Package igpcost carries the IGP metric of a resolved next-hop from whoever
// computes it to whoever ranks paths by it.
//
// The producer is sysrib's next-hop resolver, which walks the unified Loc-RIB;
// the consumer is BGP best-path selection (RFC 4271 Section 9.1.2.2 (e), the
// "lowest interior cost to the NEXT_HOP" tiebreak). Both sides are optional:
// sysrib runs with no BGP engine compiled in (//go:build ze_bgp), and the BGP
// engine runs with no Loc-RIB wired. Keeping the seam here, in a leaf that
// neither side owns, lets each register or read independently -- previously
// sysrib pushed directly into the BGP RIB package, which pinned the engine into
// every binary.
//
// An unset seam yields cost 0, the same value BGP already used before a resolver
// existed: every next-hop compares equal on that tiebreak and selection falls
// through to the next rule.
package igpcost

import (
	"net/netip"
	"sync/atomic"
)

// Func returns the interior (IGP) cost to reach addr.
type Func func(addr netip.Addr) uint32

// fnPtr holds the registered lookup. Read on the best-path hot path, written
// once when a Loc-RIB is wired, so an atomic pointer beats a mutex here.
var fnPtr atomic.Pointer[Func]

// Set registers the IGP cost lookup. Called by sysrib once its next-hop
// resolver exists. A nil fn clears the seam (Lookup then reports 0).
func Set(fn Func) {
	if fn == nil {
		fnPtr.Store(nil)
		return
	}
	fnPtr.Store(&fn)
}

// Lookup returns the interior cost to addr, or 0 when no resolver is
// registered. Zero is the documented "no interior cost known" value, not an
// error: it makes the IGP-cost tiebreak a no-op rather than a failure.
func Lookup(addr netip.Addr) uint32 {
	p := fnPtr.Load()
	if p == nil {
		return 0
	}
	return (*p)(addr)
}

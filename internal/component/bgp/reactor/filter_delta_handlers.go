// Design: docs/architecture/core-design.md -- generic attribute modification handlers
// RFC: rfc/short/rfc4271.md -- path attribute flags and the Extended Length header class (Section 4.3)
// RFC: rfc/short/rfc4456.md -- ORIGINATOR_ID set-if-absent and CLUSTER_LIST prepend (Section 8)
// RFC: rfc/short/rfc4760.md -- MP_REACH_NLRI value layout (Section 3)
// Related: filter_delta.go -- textDeltaToModOps produces AttrModSet ops consumed by these handlers
// Related: forward_build.go -- buildModifiedPayload dispatches to registered handlers

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// lastSetOrSuppress returns the index of the last Set or Suppress operation and
// whether it was a Suppress. Last wins, which is the rule every handler here
// already followed and which the ordering of a filter chain depends on.
func lastSetOrSuppress(ops []filterapi.AttrOp) (idx int, suppress bool) {
	idx = -1
	for i := range ops {
		switch ops[i].Action {
		case filterapi.AttrModSet:
			idx, suppress = i, false
		case filterapi.AttrModSuppress:
			idx, suppress = i, true
		}
	}
	return idx, suppress
}

// keepOrDrop finishes a plan that has no modification to make: the source
// attribute is emitted unchanged when it exists, and nothing is emitted when it
// does not.
//
// KeepAll rather than a rebuilt copy, because rebuilding decides the header size
// class afresh and would normalize a legal-but-unusual source header, moving a
// byte on the wire for a route nobody asked to change. It is also the cheapest
// outcome: a verbatim slot coalesces into the surrounding copy run.
func keepOrDrop(p *filterapi.AttrPlan) {
	if p.Source() == nil {
		p.Drop()
		return
	}
	p.KeepAll()
}

// genericAttrSetHandler returns an AttrModHandler that supports AttrModSet for any
// attribute code. On Set it emits the operation's value bytes under a fresh header;
// on Suppress it drops the attribute; with neither it keeps the source unchanged.
//
// This enables the policy filter text-delta-to-wire bridge for attributes that don't
// have specialized handlers (community types and OTC have their own).
func genericAttrSetHandler(flags, code byte) filterapi.AttrModHandler {
	return func(p *filterapi.AttrPlan) {
		setIdx, suppress := lastSetOrSuppress(p.Ops())

		// Suppress: emit nothing, removing the attribute.
		if setIdx >= 0 && suppress {
			p.Drop()
			return
		}
		if setIdx < 0 {
			keepOrDrop(p)
			return
		}

		// The operation's bytes are named, not copied: they already exist in the
		// accumulator and stay valid for the whole rebuild.
		p.Op(setIdx)
		p.Emit(flags, code)
	}
}

// Attribute flags per RFC 4271 Section 4.3:
//   - 0x40 = Well-known, Transitive (ORIGIN, AS_PATH, NEXT_HOP, LOCAL_PREF, ATOMIC_AGGREGATE)
//   - 0x80 = Optional, Non-transitive (MED)
//   - 0xC0 = Optional, Transitive (AGGREGATOR, ORIGINATOR_ID, CLUSTER_LIST)

// genericAttrCodes lists attribute codes that need generic set handlers.
// Community types (8, 16, 32) and OTC (35) already have specialized handlers
// registered by their plugins.
var genericAttrCodes = []struct {
	code  attribute.AttributeCode
	flags byte
}{
	{attribute.AttrOrigin, 0x40},          // Well-known mandatory
	{attribute.AttrASPath, 0x40},          // Well-known mandatory
	{attribute.AttrAS4Path, 0xC0},         // Optional transitive (RFC 6793)
	{attribute.AttrNextHop, 0x40},         // Well-known mandatory
	{attribute.AttrMED, 0x80},             // Optional non-transitive
	{attribute.AttrLocalPref, 0x40},       // Well-known (IBGP)
	{attribute.AttrAtomicAggregate, 0x40}, // Well-known discretionary
	{attribute.AttrAggregator, 0xC0},      // Optional transitive
	{attribute.AttrAS4Aggregator, 0xC0},   // Optional transitive (RFC 6793 Section 3)
	{attribute.AttrOriginatorID, 0x80},    // Optional non-transitive (RFC 4456)
	{attribute.AttrClusterList, 0x80},     // Optional non-transitive (RFC 4456)
	{attribute.AttrAIGP, 0xC0},            // Optional transitive (RFC 7311)
	{attribute.AttrPrefixSID, 0xC0},       // Optional transitive (RFC 8669)
}

// originatorIDHandler handles ORIGINATOR_ID (type 9, RFC 4456 Section 8).
// AttrModSet: set only if the attribute is absent in the source. If already present,
// the existing value is kept unchanged (the original originator is preserved).
func originatorIDHandler() filterapi.AttrModHandler {
	return func(p *filterapi.AttrPlan) {
		// RFC 4456 Section 8: "the ORIGINATOR_ID SHOULD NOT be changed" by a
		// reflector that finds one already present.
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		for i, op := range p.Ops() {
			if op.Action != filterapi.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			p.Op(i)
			// Flags: 0x80 = Optional, Non-transitive (RFC 4456).
			p.Emit(0x80, byte(attribute.AttrOriginatorID))
			return
		}
		p.Drop()
	}
}

// clusterListHandler handles CLUSTER_LIST (type 10, RFC 4456 Section 8).
// AttrModPrepend: prepends a 4-byte cluster-id before any existing cluster-list values.
// AttrModSet: replaces the entire cluster-list.
//
// The prepend path is a fragment list: the new cluster-ids come from their
// operation buffers and the existing list is named where it already sits, so a
// long cluster list is never copied into an intermediate.
func clusterListHandler() filterapi.AttrModHandler {
	code := byte(attribute.AttrClusterList)
	return func(p *filterapi.AttrPlan) {
		ops := p.Ops()
		setIdx := -1
		prepends := 0
		for i := range ops {
			switch {
			case ops[i].Action == filterapi.AttrModSet:
				setIdx = i
			case ops[i].Action == filterapi.AttrModPrepend && len(ops[i].Buf) == 4:
				prepends++
			}
		}

		// Set overrides everything.
		if setIdx >= 0 {
			p.Op(setIdx)
			p.Emit(0x80, code) // Optional, Non-transitive
			return
		}
		if prepends == 0 {
			keepOrDrop(p)
			return
		}

		for i := range ops {
			if ops[i].Action == filterapi.AttrModPrepend && len(ops[i].Buf) == 4 {
				p.Op(i)
			}
		}
		if val := p.Value(); len(val) > 0 {
			p.Keep(0, len(val))
		}
		p.Emit(0x80, code)
	}
}

// mpReachNextHopHandler handles next-hop rewriting inside MP_REACH_NLRI
// (type 14, RFC 4760 Section 3).
//
// The op Buf carries the new next-hop bytes (16 bytes for an IPv6 global
// address, or 32 bytes for a global + link-local pair per RFC 2545). This is the
// handler the fragment model was generalized from: the AFI/SAFI header and the
// whole NLRI tail are NAMED where they already sit, one synthesized byte carries
// the new next-hop length, and the next-hop itself comes from its operation
// buffer. The NLRI tail is therefore copied exactly once, straight into the
// output, however many prefixes the route carries.
//
// If the source attribute is absent or its value is too short to parse, the
// handler emits nothing: MP_REACH_NLRI carries the route's NLRI itself, so the
// rewrite only makes sense when a source attribute already exists.
//
// Only AttrModSet ops are honored (last-wins). AttrModSuppress on a
// MP_REACH_NLRI would strip the entire route, which is a withdraw -- that is
// expressed via ModAccumulator.SetWithdraw(), not via this handler.
func mpReachNextHopHandler() filterapi.AttrModHandler {
	return func(p *filterapi.AttrPlan) {
		// Pick the last Set op.
		setIdx := -1
		ops := p.Ops()
		for i := range ops {
			if ops[i].Action == filterapi.AttrModSet && len(ops[i].Buf) > 0 {
				setIdx = i
			}
		}

		// No rewrite requested: keep the source unchanged.
		if setIdx < 0 {
			keepOrDrop(p)
			return
		}

		// Cannot construct MP_REACH_NLRI from nothing -- the NLRI portion lives
		// in the source attribute value.
		src := p.Source()
		if src == nil {
			p.Drop()
			return
		}

		// Value layout: AFI(2) + SAFI(1) + NHLen(1) + NH(NHLen) + Reserved(1) + NLRI.
		val := p.Value()
		if len(val) < 5 {
			p.Drop()
			return
		}
		nhLen := int(val[3])
		nhEnd := 4 + nhLen
		if nhEnd+1 > len(val) { // +1 for the reserved byte
			p.Drop()
			return
		}

		// The new next-hop must be exactly one of the allowed lengths:
		//   - 4  bytes: IPv4 next-hop (used by labeled unicast / VPN families).
		//   - 16 bytes: IPv6 global-only next-hop.
		//   - 32 bytes: IPv6 global + link-local per RFC 2545 Section 3.
		// A mismatched op length is a caller bug; the route is left unchanged
		// (the caller should have produced a valid op).
		newNHLen := len(ops[setIdx].Buf)
		if newNHLen != 4 && newNHLen != 16 && newNHLen != 32 {
			p.KeepAll()
			return
		}

		p.Keep(0, 3)              // AFI + SAFI, already on the wire
		p.NewByte(byte(newNHLen)) // the one byte that exists nowhere else
		p.Op(setIdx)              // the new next-hop
		p.Keep(nhEnd, len(val)-nhEnd)

		// The source flags carry the attribute's optional/transitive bits; Emit
		// decides the Extended Length bit from the FINAL value length, which is
		// what a next-hop that grows or shrinks across the 255-octet boundary
		// needs (RFC 4271 Section 4.3).
		p.Emit(src[0], byte(attribute.AttrMPReachNLRI))
	}
}

// aspathHandler handles AS_PATH (type 2) with support for AttrModPrepend.
// Prepend inserts a new AS_SEQUENCE segment before the existing AS_PATH value.
// Set replaces the entire attribute (via genericAttrSetHandler fallback).
func aspathHandler() filterapi.AttrModHandler {
	setHandler := genericAttrSetHandler(0x40, byte(attribute.AttrASPath))

	return func(p *filterapi.AttrPlan) {
		ops := p.Ops()
		hasPrepend := false
		for i := range ops {
			if ops[i].Action == filterapi.AttrModPrepend && len(ops[i].Buf) > 0 {
				hasPrepend = true
				break
			}
		}
		if !hasPrepend {
			// No prepend: delegate to the generic set handler.
			setHandler(p)
			return
		}

		setIdx, suppress := lastSetOrSuppress(ops)
		if setIdx >= 0 && suppress {
			p.Drop()
			return
		}

		// Prepend: the new segments first, then the base path. The base is the
		// Set value when one is present, otherwise the source value where it
		// already sits.
		for i := range ops {
			if ops[i].Action == filterapi.AttrModPrepend && len(ops[i].Buf) > 0 {
				p.Op(i)
			}
		}
		if setIdx >= 0 {
			p.Op(setIdx)
		} else if val := p.Value(); len(val) > 0 {
			p.Keep(0, len(val))
		}
		p.Emit(0x40, byte(attribute.AttrASPath)) // Well-known, Transitive
	}
}

// attrModHandlersWithDefaults returns the registered AttrModHandler map with
// generic set handlers filled in for attribute codes that lack specialized handlers.
// Called by the reactor at startup instead of filterapi.AttrModHandlers() directly.
func attrModHandlersWithDefaults() map[uint8]filterapi.AttrModHandler {
	handlers := filterapi.AttrModHandlers()
	for _, entry := range genericAttrCodes {
		code := byte(entry.code)
		if handlers[code] == nil {
			handlers[code] = genericAttrSetHandler(entry.flags, code)
		}
	}
	// Override AS_PATH with handler supporting prepend (policy as-path-prepend).
	handlers[byte(attribute.AttrASPath)] = aspathHandler()
	// Override ORIGINATOR_ID and CLUSTER_LIST with specialized handlers
	// that support set-if-absent and prepend semantics (RFC 4456).
	handlers[byte(attribute.AttrOriginatorID)] = originatorIDHandler()
	handlers[byte(attribute.AttrClusterList)] = clusterListHandler()
	// MP_REACH_NLRI next-hop rewriting (RFC 4760 §3, RFC 2545 §3).
	handlers[byte(attribute.AttrMPReachNLRI)] = mpReachNextHopHandler()
	return handlers
}

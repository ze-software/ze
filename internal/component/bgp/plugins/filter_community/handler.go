// Design: docs/architecture/core-design.md — community filter egress AttrModHandlers
// RFC: rfc/short/rfc1997.md — COMMUNITY, a list of 4-octet values
// RFC: rfc/short/rfc4360.md — EXTENDED_COMMUNITY, a list of 8-octet values
// RFC: rfc/short/rfc8092.md — LARGE_COMMUNITY, a list of 12-octet values
// Overview: filter_community.go — plugin entry point
// Related: egress.go — egress filter accumulates ops
// Related: filter.go — ingress filter (direct payload mutation)

package filter_community

import (
	"bytes"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// communityAttrModHandler handles AttrModAdd/Remove for COMMUNITY (code 8, 4-byte values).
// Called by the rebuild in reactor/forward_build.go when a route is forwarded.
func communityAttrModHandler(p *filterapi.AttrPlan) {
	genericCommunityHandler(attribute.AttrCommunity, 4, p)
}

// largeCommunityAttrModHandler handles AttrModAdd/Remove for LARGE_COMMUNITY (code 32, 12-byte values).
func largeCommunityAttrModHandler(p *filterapi.AttrPlan) {
	genericCommunityHandler(attribute.AttrLargeCommunity, 12, p)
}

// extCommunityAttrModHandler handles AttrModAdd/Remove for EXTENDED_COMMUNITY (code 16, 8-byte values).
func extCommunityAttrModHandler(p *filterapi.AttrPlan) {
	genericCommunityHandler(attribute.AttrExtCommunity, 8, p)
}

// genericCommunityHandler implements Add/Remove/Set for any community attribute type.
//
// A community value is a fixed-width entry in a list, so the values a removal
// RETAINS are a subsequence of the bytes already on the wire. That is the
// cleanest case in the whole edit set: the handler names the retained runs where
// they sit and copies nothing at all. Adjacent retained values coalesce into one
// fragment, so stripping a control community from the middle of a list costs two
// fragments rather than one per surviving value.
//
// It replaced three heap allocations per attribute per destination: a copy of
// the whole list, an append that grew it, and a second copy on Set.
func genericCommunityHandler(code attribute.AttributeCode, valueSize int, p *filterapi.AttrPlan) {
	ops := p.Ops()

	// Set and Suppress both override all prior Remove/Add ops, and the LAST of
	// the two wins -- the same rule filterapi.LastSetOrSuppress applies to every
	// generically handled attribute code.
	//
	// Reading Set alone here was a live fail-open, not a missing feature. A lone
	// Suppress left setIdx at -1, no Remove named any value, so every source
	// value was retained below, the length was non-zero and the attribute was
	// re-emitted intact. The suppression was consumed and thrown away in
	// silence. That is the whole of `session { community { send ... } }` for any
	// value other than `all`: both forward rails call applyFactsSendCommunity
	// (reactor/peer_forward_facts.go), which records exactly this operation for
	// codes 8, 16 and 32, and every one of them was ignored. A peer configured
	// to receive no communities received all of them.
	//
	// The Suppress-aware genericAttrSetHandler never covered for it: reactor's
	// genericAttrCodes omits the three community codes precisely BECAUSE this
	// file registers handlers for them.
	setIdx, suppress := filterapi.LastSetOrSuppress(ops)
	if setIdx >= 0 && suppress {
		// Unconditional, and deliberately ahead of the empty-buffer check
		// below. Every producer today records Suppress with a nil Buf, which
		// would ALSO reach the empty-Set drop, so relying on that would make
		// this branch look redundant while quietly re-overloading "empty Set"
		// as the spelling of suppression -- the exact ambiguity AttrModSuppress
		// exists to remove. The ACTION decides, never the buffer length.
		p.Drop() // Attribute removed from the UPDATE entirely.
		return
	}
	if setIdx >= 0 {
		if len(ops[setIdx].Buf) == 0 {
			p.Drop() // Attribute omitted entirely.
			return
		}
		p.Op(setIdx)
		emitCommunity(p, code)
		return
	}

	// Report a malformed Remove buffer ONCE per operation, before any value is
	// tested against it.
	//
	// Refuse THIS operation and say so, rather than preserving the data silently
	// as the original size guard did. The offending producer is identifiable from
	// one line, which is the whole point: the route-server arity violation this
	// replaced went unnoticed from the day it was introduced because the guard
	// never spoke (ai/rules/evidence.md).
	//
	// The attribute's other operations still apply. Dropping them would turn one
	// producer's bug into a second, wider behavior change.
	for i := range ops {
		if ops[i].Action != filterapi.AttrModRemove {
			continue
		}
		if wholeValues(ops[i].Buf, valueSize) {
			continue
		}
		logger().Warn("attribute-modification remove buffer is not a whole number of wire values; operation refused",
			"attribute-code", int(code),
			"value-size", valueSize,
			"buffer-length", len(ops[i].Buf))
		// Counted as well as logged: the refusal is silent at the wire (the route
		// goes out with the attribute unchanged), so without a metric you have to
		// already suspect the contract is being violated before you would go
		// looking for the log line.
		filterapi.RecordRemoveBufferRefused(byte(code))
	}

	// The source value is a LIST of fixed-width values, so a length that is not a
	// whole number of them is malformed. Refuse the modification rather than let
	// the retained-run loop below step past the trailing partial value: that loop
	// would emit an attribute one whole value shorter than the peer sent, which is
	// a wire-visible attribute this router invented (ai/rules/evidence.md).
	//
	// Refusing suppresses the route to this destination, and the reactor already
	// says so and counts it: forward_build.go turns a refused plan into
	// modifyFailureHandlerFault, which recordModifyFailure rate-limits and reports.
	//
	// Nothing can reach here today, and the check is here so that a new producer
	// which could is heard on the first route rather than after the leak. A
	// peer-sourced COMMUNITY whose length is not a multiple of 4 is treat-as-withdraw
	// at receive (message.validateCommunityAttr, in the attrValidators table that
	// Session.enforceRFC7606 drives through message.ValidateUpdateRFC7606AddPath),
	// and the synthesized withdrawal carries no COMMUNITY into the forward path at
	// all; codes 16 and 32 have the same rule in validateExtCommunityAttr and
	// validateLargeCommunityAttr. Locally originated routes take their value from
	// reactor.writeCommunitiesAttr, which writes 4 bytes per community.
	val := p.Value()
	if !wholeValues(val, valueSize) {
		logger().Warn("attribute value is not a whole number of wire values; modification refused",
			"attribute-code", int(code),
			"value-size", valueSize,
			"value-length", len(val))
		p.Fail()
		return
	}

	// Keep every source value no Remove operation names. Nothing is copied here:
	// each retained run is a fragment over the bytes already on the wire.
	//
	// The membership structure is chosen ONCE, above the loop, and not once per
	// value. That placement is the fix for a peer-reachable quadratic; see
	// newRemovalSet.
	if valueSize > 0 {
		set := newRemovalSet(ops, valueSize, len(val)/valueSize)
		for i := 0; i+valueSize <= len(val); i += valueSize {
			if !set.has(val[i : i+valueSize]) {
				p.Keep(i, valueSize)
			}
		}
	}

	// Then the additions, straight from their operation buffers.
	for i := range ops {
		if ops[i].Action == filterapi.AttrModAdd {
			p.Op(i)
		}
	}

	if p.ValueLen() == 0 {
		p.Drop() // Every value went away: the attribute leaves the UPDATE.
		return
	}
	emitCommunity(p, code)
}

// emitCommunity writes the header these handlers have always written: Optional
// Transitive with the Extended Length bit set, whatever the value length.
//
// EmitExtended rather than Emit, so the header size class does not change for
// any community attribute Ze forwards. Letting the length decide the class would
// be equally legal (RFC 4271 Section 4.3) and one byte shorter, but it would
// move bytes on the wire for every short community list in a change whose
// purpose is to move none.
func emitCommunity(p *filterapi.AttrPlan, code attribute.AttributeCode) {
	p.EmitExtended(0xC0, byte(code))
}

// wholeValues reports whether buf is a whole number of valueSize-byte wire values.
//
// Zero is a whole number: an empty buffer names no value, so it is admitted and
// removes nothing. No producer sends one today (each gates on a non-empty set),
// and the alternative would report a contract violation to an operator whose
// producer broke no contract (TestGenericCommunityHandlerEmptyRemoveIsSilentNoOp).
//
// toRemove is a SET: a whole number of valueSize-byte wire values, concatenated.
// One value is the common case and is a whole number, so every producer that
// emits one operation per value (reactor/filter_delta.go splits on valueSize;
// egress.go is per value) is unaffected.
//
// The multi-value form is what made the original single-value rule a live defect
// rather than a style point. wireu.StripControlCommunities accumulates EVERY
// matching control community into one slice, and both route-server rails pass
// that slice as a single Remove operation. Under the old
// `len(toRemove) != valueSize` guard a route carrying two or more control
// communities had NONE of them removed, and leaked the route server's internal
// tags to its clients.
func wholeValues(buf []byte, valueSize int) bool {
	return valueSize > 0 && len(buf)%valueSize == 0
}

// removedByAny reports whether want appears in any well-formed Remove operation.
// An operation whose buffer is not a whole number of values is skipped: it
// cannot be interpreted as wire values at all, so no removal from it is safe.
// removalIndexThreshold is the value count above which membership is answered
// from a map instead of a scan, measured on min(source values, removal values).
//
// The arithmetic it comes from: a linear answer costs n*m byte comparisons, an
// indexed one costs m insertions plus n lookups, and a map operation is roughly
// twenty times a bytes.Equal over four to twelve bytes. Indexing therefore pays
// once n*m > 20*(n+m), which for n == m is n > 40. Thirty-two is the power of
// two below that, so the crossover is crossed slightly early and the common
// shapes -- one configured strip value, or a route carrying three control
// communities -- stay on the scan and allocate nothing.
const removalIndexThreshold = 32

// removalSet answers "does a Remove operation name this value" for the
// retained-run loop, from whichever representation is cheaper.
//
// THE PLACEMENT IS THE POINT. It is built once per attribute, above the loop
// over source values. Answering from ops directly, per value, is quadratic in
// two operands a PEER controls, which was a remote denial of service on the
// route-server forward path rather than a micro-optimisation:
// wireu.StripControlCommunities derives the removal buffer from the forwarded
// route's OWN COMMUNITY attribute, so one route tagged with 16383 control
// communities sets len(val) and len(set) together. That shape measured 874 to
// 889 ms of CPU PER DESTINATION PEER, multiplied by the fan-out. The cost cap
// used to be an unrelated arity guard, `len(toRemove) != valueSize`, and
// removing that defect removed the cap with it.
//
// The index also collapses duplicates as it is built, so a peer that repeats
// one control community 16383 times stores one entry rather than 16383.
//
// Entries stored is not bytes allocated, and this comment used to stop at the
// entry count. While it did, the map was built with the raw value count as its
// size hint and inserted every repeat unconditionally: one entry, 939,312 bytes
// (measured by TestRemovalSetIndexAllocatesByDistinctValues on the same input).
// That is 93% of the megabyte that got an earlier candidate fix reverted for
// building a set per destination with no deduplication -- the defect the comment
// claimed the code avoided. It costs 272 bytes now. The bytes are what the test
// asserts, because the entry count is what could not see this
// (ai/rules/evidence.md).
type removalSet struct {
	ops       []filterapi.AttrOp
	valueSize int
	index     map[string]struct{} // nil means answer by scanning ops
}

// newRemovalSet picks the representation. It reads valueCount rather than
// deriving it, because the caller has already divided.
func newRemovalSet(ops []filterapi.AttrOp, valueSize, valueCount int) removalSet {
	set := removalSet{ops: ops, valueSize: valueSize}
	if valueSize <= 0 {
		return set
	}
	removals := 0
	for i := range ops {
		if ops[i].Action != filterapi.AttrModRemove {
			continue
		}
		// Malformed buffers are refused above and contribute no values, so they
		// must not count toward the threshold either.
		if !wholeValues(ops[i].Buf, valueSize) {
			continue
		}
		removals += len(ops[i].Buf) / valueSize
	}
	// min, not max: the scan costs n*m, so either operand being small keeps it
	// cheap. Thresholding on the removal count alone would index a large set
	// against a two-value attribute and pay for a map nothing reads.
	if min(valueCount, removals) <= removalIndexThreshold {
		return set
	}
	// No size hint, and the trade that buys is measured on BOTH shapes a peer
	// can choose. removals counts RAW values, and a hint is allocated in full
	// before the first insert. TotalAlloc delta for 16383 four-byte values:
	//
	//	shape          shipped (no hint)   hint restored
	//	16383 x 0:0              272 B         873,744 B
	//	16383 x 0:X        1,812,792 B         939,264 B
	//
	// The hint is therefore right for one shape and wrong for the other. On a
	// duplicate-heavy buffer it sizes the table by the buffer length, the very
	// quantity the index exists to stop paying for. On an all-distinct buffer
	// the buffer length IS the distinct count, so the hint is exactly
	// right-sized, and the hintless map instead grows geometrically and
	// discards every intermediate table. Peak peer-controlled bytes ROSE,
	// 939,264 to 1,812,792, 1.93x.
	//
	// The trade was taken deliberately. It removes a peer-controlled CPU
	// quadratic measured at 874 to 889 ms per destination peer (above). It
	// costs transient memory, freed when the attribute is written, on a path
	// whose exponent is now linear. Sizing correctly for both shapes needs a
	// distinct count that does not exist before the walk, and a counting walk
	// costs a second pass over the same peer-controlled buffer.
	//
	// The left column is what ships and both its rows carry a byte ceiling:
	// TestRemovalSetIndexAllocatesByDistinctValues holds the first,
	// TestRemovalSetIndexBoundsAllDistinctValues the second. The right column is
	// the counterfactual, measured with the hint restored in a scratch harness.
	set.index = make(map[string]struct{})
	for i := range ops {
		if ops[i].Action != filterapi.AttrModRemove {
			continue
		}
		if !wholeValues(ops[i].Buf, valueSize) {
			continue
		}
		buf := ops[i].Buf
		for off := 0; off+valueSize <= len(buf); off += valueSize {
			val := buf[off : off+valueSize]
			// Look up before inserting. A map READ keyed on string(byteslice)
			// allocates nothing, an insert must copy the key, and the two lines
			// together are what makes the cost track DISTINCT values: a repeated
			// value takes the read and stops there.
			if _, seen := set.index[string(val)]; seen {
				continue
			}
			set.index[string(val)] = struct{}{}
		}
	}
	return set
}

// has reports whether any Remove operation names want.
func (s removalSet) has(want []byte) bool {
	if s.index != nil {
		// The compiler does not allocate for a []byte converted to string
		// solely to index a map.
		_, ok := s.index[string(want)]
		return ok
	}
	return removedByAny(s.ops, s.valueSize, want)
}

// indexed reports whether this set answers from a map.
//
// It exists for the regression guard. The defense against the quadratic is the
// REPRESENTATION, so a test asserts the representation and stays deterministic.
// A test that timed the loop instead would pass on a quiet host and fail on a
// loaded one, and a test that only checked the retained values would stay green
// with the index deleted, which is how the previous guard came to be decorative.
func (s removalSet) indexed() bool { return s.index != nil }

func removedByAny(ops []filterapi.AttrOp, valueSize int, want []byte) bool {
	for i := range ops {
		if ops[i].Action != filterapi.AttrModRemove {
			continue
		}
		if !wholeValues(ops[i].Buf, valueSize) {
			continue
		}
		if containsValue(ops[i].Buf, valueSize, want) {
			return true
		}
	}
	return false
}

// containsValue reports whether want appears as one of the valueSize-byte values
// in set. set is assumed to be a whole number of values; removedByAny checks that
// before calling.
//
// A linear scan, used only when newRemovalSet has decided the set is small
// enough for one. The caller is the branch that matters, not this function.
//
// This comment used to state that the sets here "hold a handful of values (the
// control communities on one route, or one configured strip value)" and drew the
// conclusion that a map never pays. The premise is false on the route-server
// path, where wireu.StripControlCommunities builds the removal buffer from the
// forwarded route's own COMMUNITY attribute, so a peer sizes it. The conclusion
// was a remote denial of service, and the comment is recorded here rather than
// deleted because it is what kept the defect open: a belief stated as a fact
// reads exactly like a measurement (ai/rules/evidence.md).
func containsValue(set []byte, valueSize int, want []byte) bool {
	for i := 0; i+valueSize <= len(set); i += valueSize {
		if bytes.Equal(set[i:i+valueSize], want) {
			return true
		}
	}
	return false
}

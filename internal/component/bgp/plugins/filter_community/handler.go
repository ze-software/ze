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

	// Set intentionally overrides all prior Remove/Add ops. Last Set wins.
	setIdx := -1
	for i := range ops {
		if ops[i].Action == filterapi.AttrModSet {
			setIdx = i
		}
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
	// never spoke (ai/rules/fail-closed-guards.md).
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

	// Keep every source value no Remove operation names. Nothing is copied here:
	// each retained run is a fragment over the bytes already on the wire.
	val := p.Value()
	if valueSize > 0 {
		for i := 0; i+valueSize <= len(val); i += valueSize {
			if !removedByAny(ops, valueSize, val[i:i+valueSize]) {
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
// A linear scan rather than a map: the sets here hold a handful of values (the
// control communities on one route, or one configured strip value), so building a
// map per attribute would cost more than it saves on a per-UPDATE forwarding path.
func containsValue(set []byte, valueSize int, want []byte) bool {
	for i := 0; i+valueSize <= len(set); i += valueSize {
		if bytes.Equal(set[i:i+valueSize], want) {
			return true
		}
	}
	return false
}

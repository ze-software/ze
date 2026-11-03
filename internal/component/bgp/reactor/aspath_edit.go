// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking
// Related: filter_library.go -- rewriteLibrary reaches the prepend and remove-private intents here
// Related: forward_build.go -- buildModifiedPayload consumes ModAccumulator ops
// RFC: rfc/short/rfc6996.md -- Private Use ASN ranges (remove-private rewriting)
// RFC: rfc/short/rfc6793.md -- AS4_PATH handling (four-octet AS number space)

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// prependASPathOps records the AS_PATH prepend of count copies of localAS,
// and the AS4_PATH RFC 6793 Section 4.2.2 owes beside it, on mods. count is
// 1..policychain.ASPathPrependMax, which the caller checks. It is the wire
// half of the prepend intent the library chain (libraryASPathOps) resolves.
// asn4 is the width of the payload the segment joins, which each call site
// derives from the session the payload was encoded for.
//
// RFC 4271 Section 5.1.2: "Whenever the modification of the AS_PATH attribute calls for
// including or prepending the AS number of the local system, the local
// system MAY include/prepend more than one instance of its own AS
// number in the AS_PATH attribute."
func prependASPathOps(scratch *valueScratch, attrs *attribute.AttributesWire, asn4 bool, localAS uint32, count uint8, mods *filterapi.ModAccumulator) {
	// The AS numbers this directive prepends. They are all localAS, but the
	// slice is what the AS4_PATH derivation below takes, and it is the same
	// shape wireu.ASPathIntent.Prepend carries on the ordinary prepend rail.
	n := int(count)
	asns := scratch.carveASNs(n)
	for range n {
		asns = append(asns, localAS)
	}

	// The AS4_PATH half is derived BEFORE either operation is recorded, because
	// a derivation that fails must leave the accumulator untouched. Recording
	// the AS_PATH first and then failing would put AS_TRANS on the wire with
	// the real local AS carried nowhere, which RFC 6793 Section 4.2.2 forbids
	// and which is worse than performing no prepend at all.
	as4Value, ok := prependAS4PathValue(scratch, attrs, asn4, asns, mods)
	if !ok {
		return
	}

	// One AS_SEQUENCE segment, encoded at the width of the payload it joins.
	//
	// RFC 6793 Section 4.2.2: "In the AS_PATH attribute encoded with two-octet
	// AS numbers, non-mappable four-octet AS numbers are represented by the
	// well-known two-octet AS number, AS_TRANS."
	//
	// That substitution is NOT written here: ASPath.WriteToWithASN4 is where Ze
	// declares it (writeSegmentWithSplit, internal/core/bgp/attribute/aspath.go),
	// and removePrivateASOps reaches it the same way.
	segs := scratch.carveSegments(1)
	segs = append(segs, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: asns})
	segment := &attribute.ASPath{Segments: segs}
	buf := scratch.carveBytes(segment.LenWithASN4(asn4))
	segment.WriteToWithASN4(buf, 0, asn4)
	mods.Op(byte(attribute.AttrASPath), filterapi.AttrModPrepend, buf)

	// RFC 6793 Section 4.2.2: "The NEW BGP speaker MUST also send the AS path
	// information in the AS4_PATH attribute (encoded with four-octet AS
	// numbers), except for the case where all of the AS path information is
	// composed of mappable four-octet AS numbers only. In this case, the NEW
	// BGP speaker MUST NOT send the AS4_PATH attribute."
	if as4Value != nil {
		mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSet, as4Value)
	}
}

// prependAS4PathValue returns the AS4_PATH value that RFC 6793 Section 4.2.2
// obliges alongside a two-octet AS_PATH the prepend above just widened. It
// returns a nil value when the section obliges none, and ok false when the
// AS-path family could not be read, which tells the caller to record nothing at
// all.
//
// The decision and the value both come from wireu.AS4PathForRewrite, which is
// the single declaration of that rule (aspath_as4.go) and already serves the
// ordinary prepend rail. This site edits ONE payload rather than transcoding
// between two, so it passes asn4 for both of that function's widths; the
// answer is then nil for every case that owes no AS4_PATH, which is why no
// condition is repeated here.
//
// A MAPPABLE local AS at two-octet width owes nothing, and that is a ruling of
// the RFC rather than an omission. RFC 6793 Section 4.2.3 reconstructs by
// "taking as many AS numbers and path segments as necessary from the leading
// part of the AS_PATH attribute, and then prepending them to the AS4_PATH
// attribute", and a prepend lands exactly at that leading part, so the
// reconstruction picks the prepended AS numbers up from AS_PATH by itself.
func prependAS4PathValue(scratch *valueScratch, attrs *attribute.AttributesWire, asn4 bool, asns []uint32, mods *filterapi.ModAccumulator) (value []byte, ok bool) {
	// RFC 6793 Section 4.1: "The new attributes, AS4_PATH and AS4_AGGREGATOR,
	// MUST NOT be carried in an UPDATE message between NEW BGP speakers."
	// AS4PathForRewrite returns nil here too; returning first also spares the
	// four-octet path a parse it would throw away.
	if asn4 {
		return nil, true
	}

	// A nil attrs is what WireUpdate.Attrs answers for a body it could not parse
	// and for one whose attribute section is absent or truncated (wire_update.go,
	// via UpdateSections.Attrs). An UNINDEXABLE section is NOT this case: that
	// one yields an AttributesWire carrying its indexErr, and GetRaw below
	// surfaces the error rather than a value.
	//
	// So the family cannot be read here, and this derivation cannot say whether
	// RFC 6793 Section 4.2.2 owes an AS4_PATH. It answers "record nothing"
	// rather than "nothing owed", which is what the unparseable-AS_PATH branch
	// below answers for the same reason. Without the branch the call is not
	// merely wrong, it panics: AttributesWire.GetRaw reads a field of its
	// receiver.
	//
	// Debug rather than Warn: a withdrawal-only UPDATE carries no attribute
	// section by construction and has no AS_PATH to prepend to, so this line
	// describes the ordinary case as often as it describes a defect.
	if attrs == nil {
		fwdLogger().Debug("as-path-prepend: no attribute section, no prepend recorded")
		return nil, false
	}

	// What the outgoing AS_PATH will hold once the prepend applies. Read from
	// the accumulator when an earlier rewrite already Set it, because
	// libraryASPathOps runs removePrivateASOps first and deriving from the wire
	// instead would restore the private AS numbers it stripped.
	path := &attribute.ASPath{}
	if value, dropped, found := pendingAttrValue(mods, attribute.AttrASPath); !dropped {
		if !found {
			value, _ = attrs.GetRaw(attribute.AttrASPath) //nolint:errcheck // an absent AS_PATH is an empty base, not a failure
		}
		if len(value) > 0 {
			parsed, err := attribute.ParseASPath(value, asn4)
			if err != nil {
				// Fail closed: half an AS-path family is worse on the wire than
				// no prepend at all, so the caller records neither operation.
				fwdLogger().Warn("as-path-prepend: parse AS_PATH failed, no prepend recorded", "error", err)
				return nil, false
			}
			path = parsed
		}
	}
	for _, asn := range asns {
		path.Prepend(asn)
	}

	// The AS4_PATH the prepend joins, read from the same two places and in the
	// same order. A Suppress leaves recvAS4 nil, which is what it means.
	var recvAS4 *attribute.AS4Path
	if value, dropped, found := pendingAttrValue(mods, attribute.AttrAS4Path); !dropped {
		if !found {
			value, _ = attrs.GetRaw(attribute.AttrAS4Path) //nolint:errcheck // an absent AS4_PATH is the common case
		}
		if len(value) > 0 {
			// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
			// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker
			// MUST discard the attribute and continue processing the UPDATE
			// message." A discarded one leaves recvAS4 nil, and the derivation
			// then builds the AS4_PATH from AS_PATH.
			if parsed, err := attribute.ParseAS4Path(value); err == nil {
				recvAS4 = parsed
			}
		}
	}

	out := wireu.AS4PathForRewrite(path, recvAS4, asn4, asn4)
	if out == nil {
		return nil, true
	}
	as4Value := scratch.carveBytes(out.Len())
	out.WriteTo(as4Value, 0)
	return as4Value, true
}

// pendingAttrValue reports the last Set or Suppress an earlier extractor
// recorded on code. dropped is true when that last operation was a Suppress,
// which means the attribute is on its way out and its wire bytes MUST NOT be
// read as a base. found is false when no operation names the code at all, and
// the caller then falls back to the wire.
//
// It is the accumulator-wide counterpart of filterapi.LastSetOrSuppress, which
// a handler calls over the operations of ONE attribute it has already been
// handed.
func pendingAttrValue(mods *filterapi.ModAccumulator, code attribute.AttributeCode) (value []byte, dropped, found bool) {
	for _, op := range mods.Ops() {
		if op.Code != byte(code) {
			continue
		}
		switch op.Action {
		case filterapi.AttrModSet:
			value, dropped, found = op.Buf, false, true
		case filterapi.AttrModSuppress:
			value, dropped, found = nil, true, true
		}
	}
	return value, dropped, found
}

// removePrivateASOps records the AS_PATH and AS4_PATH rewrite that removes,
// or with RemovePrivatePeerAS replaces by peerAS, every private-use AS
// number, segment by segment. It is the wire half of the remove-private
// intent the library chain (libraryASPathOps) resolves.
//
// RFC 6996 Section 4: "Private Use ASNs MUST be removed from AS path attributes (including
// AS4_PATH if utilizing a four-octet AS number space) before being
// advertised to the global Internet."
func removePrivateASOps(scratch *valueScratch, attrs *attribute.AttributesWire, asn4 bool, mode policychain.RemovePrivateMode, peerAS uint32, mods *filterapi.ModAccumulator) {
	if attrs == nil {
		return
	}

	rawASPath, err := attrs.GetRaw(attribute.AttrASPath)
	if err == nil && len(rawASPath) > 0 {
		if rewritten, changed := rewriteASPathRemovePrivate(scratch, rawASPath, asn4, mode, peerAS); changed {
			mods.Op(byte(attribute.AttrASPath), filterapi.AttrModSet, rewritten)
		}
	} else if err != nil {
		fwdLogger().Warn("remove-private-as: AS_PATH raw lookup failed", "error", err)
	}

	rawAS4Path, err := attrs.GetRaw(attribute.AttrAS4Path)
	if err == nil && len(rawAS4Path) > 0 {
		if rewritten, changed := rewriteAS4PathRemovePrivate(scratch, rawAS4Path, mode, peerAS); changed {
			if len(rewritten) == 0 {
				mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSuppress, nil)
			} else {
				mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSet, rewritten)
			}
		}
	} else if err != nil {
		fwdLogger().Warn("remove-private-as: AS4_PATH raw lookup failed", "error", err)
	}
}
func rewriteASPathRemovePrivate(scratch *valueScratch, value []byte, asn4 bool, mode policychain.RemovePrivateMode, peerAS uint32) ([]byte, bool) {
	path, err := attribute.ParseASPath(value, asn4)
	if err != nil {
		fwdLogger().Warn("remove-private-as: parse AS_PATH failed", "error", err)
		return nil, false
	}
	segments, changed := rewritePrivateASSegments(scratch, path.Segments, mode, peerAS)
	if !changed {
		return nil, false
	}
	rewritten := &attribute.ASPath{Segments: segments}
	buf := scratch.carveBytes(rewritten.LenWithASN4(asn4))
	rewritten.WriteToWithASN4(buf, 0, asn4)
	return buf, true
}

func rewriteAS4PathRemovePrivate(scratch *valueScratch, value []byte, mode policychain.RemovePrivateMode, peerAS uint32) ([]byte, bool) {
	path, err := attribute.ParseAS4Path(value)
	if err != nil {
		fwdLogger().Warn("remove-private-as: parse AS4_PATH failed", "error", err)
		return nil, false
	}
	segments, changed := rewritePrivateASSegments(scratch, path.Segments, mode, peerAS)
	if !changed {
		return nil, false
	}
	rewritten := &attribute.AS4Path{Segments: segments}
	buf := scratch.carveBytes(rewritten.Len())
	rewritten.WriteTo(buf, 0)
	return buf, true
}

func rewritePrivateASSegments(scratch *valueScratch, segments []attribute.ASPathSegment, mode policychain.RemovePrivateMode, peerAS uint32) ([]attribute.ASPathSegment, bool) {
	out := scratch.carveSegments(len(segments))
	changed := false
	for _, seg := range segments {
		asns := scratch.carveASNs(len(seg.ASNs))
		for _, asn := range seg.ASNs {
			if !isRFC6996PrivateASN(asn) {
				asns = append(asns, asn)
				continue
			}
			changed = true
			if mode == policychain.RemovePrivatePeerAS {
				asns = append(asns, peerAS)
			}
		}
		if len(asns) == 0 {
			continue
		}
		out = append(out, attribute.ASPathSegment{Type: seg.Type, ASNs: asns})
	}
	if !changed {
		return nil, false
	}
	return out, true
}

// RFC 6996 Section 5: Private Use ASNs are 64512-65534 and
// 4200000000-4294967294, inclusive.
func isRFC6996PrivateASN(asn uint32) bool {
	return (asn >= 64512 && asn <= 65534) || (asn >= 4200000000 && asn <= 4294967294)
}

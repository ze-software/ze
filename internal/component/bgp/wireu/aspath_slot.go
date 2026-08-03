// Design: docs/architecture/wire/attributes.md — the AS-path family as generate slots
// RFC: rfc/short/rfc4271.md — AS_PATH prepend to an EBGP peer (Section 9.1.2), ascending attribute order (Section 5)
// RFC: rfc/short/rfc6793.md — AS4_PATH obligation and AGGREGATOR/AS_TRANS (Section 4.2.2), malformed AS4_PATH discard (Section 6)
// RFC: rfc/short/rfc7947.md — a route server MUST NOT modify AS_PATH for an RS client (Section 2.2.2)
// Overview: aspath_rewrite.go — RewriteASPath, the whole-payload rewrite this replaces
// Related: aspath_transcode.go — TranscodeASPath, the transcode-only rail this replaces
// Related: aspath_as4.go — the shared AS4_PATH derivation rule both rails already used
//
// Why this file exists.
//
// RewriteASPath and TranscodeASPath each produce a WHOLE new UPDATE payload as a
// pass BEFORE the per-destination edit set runs, so an EBGP destination carrying
// any policy paid two full payload copies back to back: one to prepend, one to
// apply the policy. The second cannot be amortized across destinations, because
// the edit set differs per destination.
//
// Here the same decisions are recorded as INTENT on the accumulator instead. The
// AS-path family becomes ordinary attribute operations, and the exactly-sized
// one-pass writer emits them into the destination buffer once, alongside every
// other edit. An ASN4 transcode re-encodes every AS number, so no fragment list
// over the source can express it and pre-building it would stage the value in a
// scratch buffer first; that is what filterapi.AttrGenerator exists for, and the
// size-then-write pair it needs is the one this package already had
// (LenWithASN4 / WriteToWithASN4).

package wireu

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// asTrans is the reserved AS number an OLD speaker sees in place of a
// four-octet AS number it cannot represent (RFC 6793 Section 4.2.2).
const asTrans = 23456

// maxMappableASN is the largest AS number a two-octet field can carry.
const maxMappableASN = 65535

// ErrASPathIntentEmpty reports a prepend recorded with no AS numbers. It is a
// caller bug rather than a wire condition: the EBGP rail always has a local AS.
var ErrASPathIntentEmpty = errors.New("AS_PATH intent: no ASNs to prepend")

// ASPathIntent is what ONE destination needs done to the AS-path family.
//
// Prepend holds the AS numbers to prepend, innermost FIRST, so the last element
// ends up outermost -- closest to the peer. RFC 7705 Section 3.3 fixes that
// order for the dual-AS local-as mode: the speaker appends the globally
// configured AS number first and the "Local AS" override immediately after, so
// the override lands closest to the peer. The order is carried as an ordered
// sequence rather than a set for exactly that reason.
//
// An empty Prepend means transcode only, which is the RFC 7947 Section 2.2.2
// route-server case: an RS client's AS_PATH is never modified, but an ASN4
// transcode still applies when the widths differ.
type ASPathIntent struct {
	Prepend []uint32
	SrcASN4 bool
	DstASN4 bool
}

// ASPathEdit resolves the AS-path family for one destination and records it as
// attribute operations.
//
// Every generator it can record is stored INLINE, so one value hoisted above a
// destination loop serves the whole fan-out without allocating per destination.
// It is reused by Record, which overwrites the generators it needs.
//
// NOT safe for concurrent use, and it MUST NOT be reused until the rebuild that
// consumed its operations has returned: the accumulator holds pointers to these
// generators, and the writer reads them (filterapi.AttrGenerator).
type ASPathEdit struct {
	shift  asPathShiftGen
	encode asPathEncodeGen
	as4    as4PathGen
	agg    aggregatorGen
	as4Agg as4AggregatorGen
}

// Record resolves the AS-path family against payload and records the resulting
// operations on mods. It reports whether anything was recorded.
//
// Nothing is written and nothing is staged: the operations name generators that
// write straight into the destination buffer when the rebuild materializes.
//
// A false return means the destination needs no AS-path change at all, which is
// the matching-width route-server case and the no-AS_PATH-no-AGGREGATOR case.
// An error means the payload could not be resolved; the caller MUST suppress the
// route for this destination rather than forward it carrying an AS_PATH the
// destination cannot read (ai/rules/evidence.md).
func (e *ASPathEdit) Record(mods *filterapi.ModAccumulator, payload []byte, in ASPathIntent) (bool, error) {
	section, err := aspathAttrSection(payload)
	if err != nil {
		return false, err
	}

	spans, err := attribute.BuildSpanIndex(section)
	if err != nil {
		return false, fmt.Errorf("AS_PATH intent: index attributes: %w", err)
	}

	if len(in.Prepend) == 0 {
		return e.recordTranscode(mods, section, &spans, in)
	}
	return e.recordPrepend(mods, section, &spans, in)
}

// aspathAttrSection returns the path-attribute section of an UPDATE body.
func aspathAttrSection(payload []byte) ([]byte, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("AS_PATH intent: %w", ErrUpdateTruncated)
	}
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	if len(payload) < 2+wdLen+2 {
		return nil, fmt.Errorf("AS_PATH intent: %w", ErrUpdateTruncated)
	}
	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	if len(payload) < attrsStart+attrLen {
		return nil, fmt.Errorf("AS_PATH intent: %w", ErrUpdateTruncated)
	}
	return payload[attrsStart : attrsStart+attrLen], nil
}

// spanValue returns the VALUE bytes of one indexed attribute.
func spanValue(section []byte, s attribute.Span) []byte {
	return section[s.Offset : int(s.Offset)+int(s.Length)]
}

// recordPrepend records the EBGP prepend case: RFC 4271 Section 9.1.2 obliges
// the local AS number on the front of the path, and RFC 6793 Section 4.2.2 can
// oblige an AS4_PATH, an AGGREGATOR rewritten to AS_TRANS, and an
// AS4_AGGREGATOR carrying the real value alongside it.
func (e *ASPathEdit) recordPrepend(mods *filterapi.ModAccumulator, section []byte, spans *attribute.SpanIndex, in ASPathIntent) (bool, error) {
	if len(in.Prepend) == 0 {
		return false, ErrASPathIntentEmpty
	}

	aspSpan, hasASPath := spans.Find(attribute.AttrASPath)

	// The fast path: one mappable AS number onto a leading AS_SEQUENCE that has
	// room, with the widths already matching. It shifts bytes rather than
	// re-encoding, exactly as tryDirectPrepend does, so it needs no parse and no
	// generator state beyond a head and a tail.
	if hasASPath && e.tryShift(mods, section, aspSpan, in) {
		return true, nil
	}

	// The slow path re-encodes, so every AS number in the outgoing path is
	// written at the destination's width and the RFC 6793 derivations apply.
	var path *attribute.ASPath
	if hasASPath {
		parsed, err := attribute.ParseASPath(spanValue(section, aspSpan), in.SrcASN4)
		if err != nil {
			return false, fmt.Errorf("AS_PATH intent: parse existing: %w", err)
		}
		path = parsed
	} else {
		// RFC 4271 Section 5: AS_PATH is well-known mandatory, so an UPDATE
		// reaching an EBGP peer without one gets a complete attribute created
		// rather than a prepend applied to nothing.
		path = &attribute.ASPath{}
	}

	// A received AS4_PATH holds the real four-octet AS numbers an OLD source
	// masked with AS_TRANS. It is read BEFORE the prepend, because the derivation
	// prepends the same AS numbers to both.
	var recvAS4 *attribute.AS4Path
	if s, ok := spans.Find(attribute.AttrAS4Path); ok {
		parsed, err := attribute.ParseAS4Path(spanValue(section, s))
		if err != nil {
			// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed
			// AS4_PATH attribute in an UPDATE message from an OLD BGP speaker MUST
			// discard the attribute and continue processing the UPDATE message."
			recvAS4 = nil
		} else {
			recvAS4 = parsed
		}
	}

	for _, asn := range in.Prepend {
		path.Prepend(asn)
	}

	e.encode = asPathEncodeGen{path: path, asn4: in.DstASN4}
	mods.OpGen(byte(attribute.AttrASPath), &e.encode)

	e.recordAS4Path(mods, spans, as4PathForRewrite(path, recvAS4, in.Prepend, in.SrcASN4, in.DstASN4))
	e.recordAggregator(mods, section, spans, in)
	return true, nil
}

// recordTranscode records the transcode-only case: RFC 7947 Section 2.2.2
// forbids a route server from modifying an RS client's AS_PATH, while RFC 6793
// Section 4.2.2 still obliges the encoding width the client negotiated.
func (e *ASPathEdit) recordTranscode(mods *filterapi.ModAccumulator, section []byte, spans *attribute.SpanIndex, in ASPathIntent) (bool, error) {
	if in.SrcASN4 == in.DstASN4 {
		// Same width: the client reads the received bytes as they are. Recording
		// nothing is what keeps RFC 7947 Section 2.2.2 literal -- the AS_PATH is
		// not merely unchanged in meaning, it is untouched.
		return false, nil
	}

	aspSpan, hasASPath := spans.Find(attribute.AttrASPath)
	_, hasAgg := spans.Find(attribute.AttrAggregator)
	if !hasASPath && !hasAgg {
		return false, nil
	}

	var path *attribute.ASPath
	if hasASPath {
		parsed, err := attribute.ParseASPath(spanValue(section, aspSpan), in.SrcASN4)
		if err != nil {
			return false, fmt.Errorf("AS_PATH intent: transcode parse: %w", err)
		}
		path = parsed
		e.encode = asPathEncodeGen{path: path, asn4: in.DstASN4}
		mods.OpGen(byte(attribute.AttrASPath), &e.encode)
	}

	// RFC 6793 Section 4.1: "The new attributes, AS4_PATH and AS4_AGGREGATOR,
	// MUST NOT be carried in an UPDATE message between NEW BGP speakers." A
	// received AS4_PATH is therefore replaced by the derived one, or dropped
	// outright when none is obliged.
	e.recordAS4Path(mods, spans, as4PathForPath(path, in.DstASN4))
	e.recordAggregator(mods, section, spans, in)
	return true, nil
}

// recordAS4Path records the AS4_PATH decision: emit the derived attribute, or
// drop a received one that must not travel onward.
func (e *ASPathEdit) recordAS4Path(mods *filterapi.ModAccumulator, spans *attribute.SpanIndex, derived *attribute.AS4Path) {
	if derived != nil {
		e.as4 = as4PathGen{path: derived}
		mods.OpGen(byte(attribute.AttrAS4Path), &e.as4)
		return
	}
	if spans.Has(attribute.AttrAS4Path) {
		// RFC 6793 Section 4.1 again: nothing derived means nothing may be
		// carried, so a received AS4_PATH leaves the UPDATE here.
		mods.Op(byte(attribute.AttrAS4Path), filterapi.AttrModSuppress, nil)
	}
}

// recordAggregator records the RFC 6793 Section 4.2.2 AGGREGATOR rewrite.
//
// Only a width CHANGE touches AGGREGATOR. When the widths match, the attribute
// is optional transitive and is propagated unchanged (RFC 4271 Section 5.1.7),
// so nothing is recorded and the writer carries it through verbatim.
func (e *ASPathEdit) recordAggregator(mods *filterapi.ModAccumulator, section []byte, spans *attribute.SpanIndex, in ASPathIntent) {
	if in.SrcASN4 == in.DstASN4 {
		return
	}
	span, ok := spans.Find(attribute.AttrAggregator)
	if !ok {
		return
	}
	val := spanValue(section, span)

	var asn uint32
	var ip [4]byte
	switch {
	case in.SrcASN4 && len(val) == 8:
		asn = binary.BigEndian.Uint32(val[0:4])
		copy(ip[:], val[4:8])
	case !in.SrcASN4 && len(val) == 6:
		asn = uint32(binary.BigEndian.Uint16(val[0:2]))
		copy(ip[:], val[2:6])
	default:
		// The AGGREGATOR does not carry the width its sender's encoding implies.
		// Leaving it alone propagates it unchanged, which is what RFC 4271
		// Section 5.1.7 asks of an optional transitive attribute this speaker has
		// no basis to rewrite.
		return
	}

	e.agg = aggregatorGen{asn: asn, ip: ip, wide: in.DstASN4}
	mods.OpGen(byte(attribute.AttrAggregator), &e.agg)

	if in.DstASN4 || asn <= maxMappableASN {
		// Narrowing a mappable AS number loses nothing, and widening never needs a
		// companion. A received AS4_AGGREGATOR would contradict the rewritten
		// AGGREGATOR, so it goes.
		if spans.Has(attribute.AttrAS4Aggregator) {
			mods.Op(byte(attribute.AttrAS4Aggregator), filterapi.AttrModSuppress, nil)
		}
		return
	}

	// RFC 6793 Section 4.2.2: "set the AS number field in the existing AGGREGATOR
	// attribute to the reserved AS number, AS_TRANS" and carry the real value in
	// AS4_AGGREGATOR.
	e.as4Agg = as4AggregatorGen{asn: asn, ip: ip}
	mods.OpGen(byte(attribute.AttrAS4Aggregator), &e.as4Agg)
}

// tryShift records the allocation-free prepend and reports whether it applied.
//
// It is the fold's form of tryDirectPrepend: one mappable AS number, matching
// widths, a leading AS_SEQUENCE with room for one more. The outgoing value is
// then a rewritten two-byte segment head, the new AS number, and the source
// value's tail unchanged -- which needs no parse, no re-encode and no
// allocation, and writes the tail exactly once.
//
// The header size class is NOT checked here, unlike tryDirectPrepend, which
// refuses a class change because it is shifting bytes inside a payload whose
// other offsets must stay put. Here the writer decides the class from the final
// value length (filterapi.AttrPlan.Emit), so a value crossing 255 octets is
// simply emitted with a 4-octet header.
func (e *ASPathEdit) tryShift(mods *filterapi.ModAccumulator, section []byte, span attribute.Span, in ASPathIntent) bool {
	if in.SrcASN4 != in.DstASN4 || len(in.Prepend) != 1 {
		return false
	}
	asn := in.Prepend[0]
	if !in.DstASN4 && asn > maxMappableASN {
		// The AS number becomes AS_TRANS for an OLD speaker, which obliges an
		// AS4_PATH (RFC 6793 Section 4.2.2). This path cannot add one, so the
		// re-encoding path takes it.
		return false
	}
	val := spanValue(section, span)
	if len(val) < 2 {
		return false
	}
	if attribute.ASPathSegmentType(val[0]) != attribute.ASSequence ||
		int(val[1]) >= attribute.MaxASPathSegmentLength {
		return false
	}

	e.shift = asPathShiftGen{tail: val[2:]}
	e.shift.head[0] = val[0]
	e.shift.head[1] = val[1] + 1
	if in.DstASN4 {
		binary.BigEndian.PutUint32(e.shift.head[2:], asn)
		e.shift.headLen = 6
	} else {
		binary.BigEndian.PutUint16(e.shift.head[2:], uint16(asn)) //nolint:gosec // G115: bounded by the maxMappableASN check above
		e.shift.headLen = 4
	}
	mods.OpGen(byte(attribute.AttrASPath), &e.shift)
	return true
}

// asPathShiftGen writes an AS_PATH value as a rewritten segment head followed by
// the source value's tail. The tail is a window into the base payload and is
// copied exactly once, into the destination.
type asPathShiftGen struct {
	head    [6]byte
	headLen int
	tail    []byte
}

func (g *asPathShiftGen) GenLen() int { return g.headLen + len(g.tail) }

func (g *asPathShiftGen) GenWrite(buf []byte, off int) int {
	n := copy(buf[off:], g.head[:g.headLen])
	n += copy(buf[off+n:], g.tail)
	return n
}

// asPathEncodeGen writes an AS_PATH value re-encoded at the destination's AS
// number width. LenWithASN4 and WriteToWithASN4 are the size-then-write pair
// this package already used for the same job.
type asPathEncodeGen struct {
	path *attribute.ASPath
	asn4 bool
}

func (g *asPathEncodeGen) GenLen() int { return g.path.LenWithASN4(g.asn4) }

func (g *asPathEncodeGen) GenWrite(buf []byte, off int) int {
	return g.path.WriteToWithASN4(buf, off, g.asn4)
}

// as4PathGen writes an AS4_PATH value (RFC 6793 Section 3).
type as4PathGen struct{ path *attribute.AS4Path }

func (g *as4PathGen) GenLen() int { return g.path.Len() }

func (g *as4PathGen) GenWrite(buf []byte, off int) int { return g.path.WriteTo(buf, off) }

// aggregatorGen writes an AGGREGATOR value at the destination's AS number width
// (RFC 4271 Section 5.1.7, RFC 6793 Section 4.2.2).
type aggregatorGen struct {
	asn  uint32
	ip   [4]byte
	wide bool
}

func (g *aggregatorGen) GenLen() int {
	if g.wide {
		return 8
	}
	return 6
}

func (g *aggregatorGen) GenWrite(buf []byte, off int) int {
	if g.wide {
		binary.BigEndian.PutUint32(buf[off:], g.asn)
		copy(buf[off+4:], g.ip[:])
		return 8
	}
	// RFC 6793 Section 4.2.2: an AS number an OLD speaker cannot represent is
	// replaced by AS_TRANS, and the real value travels in AS4_AGGREGATOR.
	asn := g.asn
	if asn > maxMappableASN {
		asn = asTrans
	}
	binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec // G115: AS_TRANS handles anything above the 16-bit range
	copy(buf[off+2:], g.ip[:])
	return 6
}

// as4AggregatorGen writes an AS4_AGGREGATOR value: the real four-octet AS number
// the AGGREGATOR had to mask with AS_TRANS (RFC 6793 Section 4.2.2).
type as4AggregatorGen struct {
	asn uint32
	ip  [4]byte
}

func (g *as4AggregatorGen) GenLen() int { return 8 }

func (g *as4AggregatorGen) GenWrite(buf []byte, off int) int {
	binary.BigEndian.PutUint32(buf[off:], g.asn)
	copy(buf[off+4:], g.ip[:])
	return 8
}

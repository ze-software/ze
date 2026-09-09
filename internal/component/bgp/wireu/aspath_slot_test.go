// Design: docs/architecture/wire/attributes.md — the AS-path family as generate slots
// RFC: rfc/short/rfc4271.md — AS_PATH prepend to an EBGP peer (Section 5.1.2 b)
// RFC: rfc/short/rfc6793.md — AS4_PATH obligation, AGGREGATOR to AS_TRANS (Section 4.2.2), malformed AS4_PATH discard (Section 6)
// RFC: rfc/short/rfc7947.md — a route server MUST NOT modify AS_PATH for an RS client (Section 2.2.2)

package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// probeAdvertisedNLRI is 10.1.0.0/24 in RFC 4271 Section 4.3 NLRI encoding.
//
// Every prepend fixture below carries it, and that is a PRECONDITION rather than
// decoration: RFC 4271 Section 5.1.2 obliges the prepend only "when a given BGP
// speaker advertises the route to an external peer", so ASPathEdit.Record
// resolves a payload with no reachable NLRI as transcode-only and records no
// prepend at all (advertise_test.go). A fixture without NLRI would exercise the
// withdraw-only rail while claiming to test the prepend.
var probeAdvertisedNLRI = []byte{24, 10, 1, 0}

// recordedOp finds the operation a Record call left for one attribute code.
func recordedOp(t *testing.T, mods *filterapi.ModAccumulator, code attribute.AttributeCode) (filterapi.AttrOp, bool) {
	t.Helper()
	for _, op := range mods.Ops() {
		if op.Code == byte(code) {
			return op, true
		}
	}
	return filterapi.AttrOp{}, false
}

// recordedGen resolves an operation's generator through the accumulator's store,
// which is the same one-based indexing the rebuild uses.
func recordedGen(t *testing.T, mods *filterapi.ModAccumulator, op filterapi.AttrOp) filterapi.AttrGenerator {
	t.Helper()
	require.NotZero(t, op.GenIdx, "expected the operation to carry a generator")
	gens := mods.Gens()
	require.LessOrEqual(t, int(op.GenIdx), len(gens), "generator index out of range")
	return gens[op.GenIdx-1]
}

// materialize runs a generator through the exact contract the rebuild uses: ask
// the length, write into a buffer of exactly that length, and require the write
// to report the same number. It returns the bytes.
//
// This is the property the whole generate path rests on, so every case below
// goes through it rather than reading GenWrite's output directly.
func materialize(t *testing.T, g filterapi.AttrGenerator) []byte {
	t.Helper()
	require.NotNil(t, g, "expected a generator")
	n := g.GenLen()
	require.GreaterOrEqual(t, n, 0, "a generator must not answer a negative length")
	buf := make([]byte, n)
	written := g.GenWrite(buf, 0)
	require.Equal(t, n, written,
		"GenWrite must write exactly GenLen bytes; a mismatch is what makes an "+
			"attribute header contradict its contents")
	// Asking twice must answer the same, because the rebuild asks during the plan
	// and again as it checks the write.
	require.Equal(t, n, g.GenLen(), "GenLen must be stable across calls")
	return buf
}

// probeASPath4 packs a four-octet AS_SEQUENCE value.
func probeASPath4(asns ...uint32) []byte {
	val := []byte{byte(attribute.ASSequence), byte(len(asns))}
	for _, a := range asns {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], a)
		val = append(val, b[:]...)
	}
	return val
}

// VALIDATES: the fast prepend path -- one mappable ASN onto a leading
// AS_SEQUENCE with matching widths -- produces the same value bytes the
// byte-shifting rewrite produces, and sizes itself exactly.
// PREVENTS: a re-encode creeping into the common EBGP case, which would parse
// and rebuild a path that only needed two bytes changed and one tail copied.
func TestASPathSlotShiftPrependIsExact(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500, 64501))...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{Prepend: []uint32{64510}})
	require.NoError(t, err)
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok, "an EBGP prepend must record an AS_PATH operation")
	require.NotZero(t, op.GenIdx, "the prepend is written straight into the destination")

	got := materialize(t, recordedGen(t, &mods, op))
	want := probeASPath2(64510, 64500, 64501)
	assert.Equal(t, want, got, "the prepended ASN must land outermost")

	// Nothing else in the family is touched when the widths already match.
	_, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
	assert.False(t, hasAS4, "matching widths oblige no AS4_PATH (RFC 6793 Section 4.1)")
}

// VALIDATES: AC-11 / RFC 7705 Section 3.3 -- the dual-AS local-as order, with
// the override outermost and the real AS immediately behind it.
// PREVENTS: the ordered prepend intent being read as an unordered set, which
// would put the router's real AS where the peer expects the override.
func TestASPathSlotDualOrder(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath2(64499))
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	// The rail records innermost first, so the override is the LAST element.
	changed, err := edit.Record(&mods, payload, ASPathIntent{Prepend: []uint32{64500, 64510}})
	require.NoError(t, err)
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	got := materialize(t, recordedGen(t, &mods, op))

	// RFC 7705 Section 3.2 shows exactly this result: 64510 64500 64499.
	assert.Equal(t, probeASPath2(64510, 64500, 64499), got)
}

// VALIDATES: AC-6 -- an UPDATE with no AS_PATH forwarded to an EBGP peer gains a
// complete AS_PATH attribute.
// PREVENTS: a prepend applied to nothing emitting an empty or absent AS_PATH,
// which RFC 4271 Section 5 makes malformed (well-known mandatory).
func TestASPathSlotInsertsWhenAbsent(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{Prepend: []uint32{64510}})
	require.NoError(t, err)
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok, "an absent AS_PATH must be created, not skipped")
	got := materialize(t, recordedGen(t, &mods, op))
	assert.Equal(t, probeASPath2(64510), got)
}

// VALIDATES: AC-3 / A-2 -- a producer that records only a prepend still yields
// an AS4_PATH when RFC 6793 Section 4.2.2 obliges one, and the AS_PATH it emits
// carries AS_TRANS in the non-mappable position.
// PREVENTS: the derivation leaking out to every caller, which is what would
// happen if the producer had to declare AS4_PATH itself.
func TestASPathSlotDerivesAS4Path(t *testing.T) {
	// A four-octet source path carrying a non-mappable ASN, going to an OLD peer.
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(196618, 64501))
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)
	require.True(t, changed)

	aspOp, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	asp := materialize(t, recordedGen(t, &mods, aspOp))
	// AS_PATH is two-octet now: 64510, AS_TRANS (for 196618), 64501.
	assert.Equal(t, probeASPath2(64510, asTrans, 64501), asp,
		"a non-mappable ASN must be AS_TRANS for an OLD speaker")

	as4Op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, ok, "RFC 6793 Section 4.2.2 obliges an AS4_PATH here")
	as4 := materialize(t, recordedGen(t, &mods, as4Op))
	assert.Equal(t, probeASPath4(64510, 196618, 64501), as4,
		"AS4_PATH carries the real four-octet values")
}

// VALIDATES: RFC 6793 Section 4.2.2 -- a path composed only of mappable ASNs
// obliges NO AS4_PATH, and a received one must not travel onward.
// PREVENTS: emitting an AS4_PATH the RFC forbids ("MUST NOT send"), which is the
// mirror of the obligation above and is just as normative.
func TestASPathSlotSuppressesAS4PathWhenNotObliged(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64500))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAS4Path, probeASPath4(64500))...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)

	op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, ok, "a received AS4_PATH must be acted on, not ignored")
	assert.Zero(t, op.GenIdx, "nothing is derived, so nothing is generated")
	assert.Equal(t, filterapi.AttrModSuppress, op.Action,
		"the received AS4_PATH leaves the UPDATE")
}

// VALIDATES: AC-7 / RFC 6793 Section 6 -- "MUST discard the attribute and
// continue processing the UPDATE message".
// PREVENTS: a malformed AS4_PATH from a peer aborting the forward, which would
// let one peer's bad attribute stop a route reaching everyone else.
func TestASPathSlotDiscardsMalformedAS4Path(t *testing.T) {
	// AS4_PATH whose segment claims 4 ASNs but carries 1: malformed.
	bad := []byte{byte(attribute.ASSequence), 4, 0, 1, 0, 1}
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath2(64501))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAS4Path, bad)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510}, SrcASN4: false, DstASN4: false,
	})
	require.NoError(t, err, "a malformed AS4_PATH is discarded, never fatal")
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok, "processing continues: the AS_PATH prepend still happens")
	assert.Equal(t, probeASPath2(64510, 64501), materialize(t, recordedGen(t, &mods, op)))
}

// VALIDATES: AC-5 -- AGGREGATOR is set to AS_TRANS and AS4_AGGREGATOR carries
// the real value (RFC 6793 Section 4.2.2).
// PREVENTS: a four-octet aggregator AS number being truncated into a two-octet
// field, which would name a different AS as the aggregator.
func TestASPathSlotDerivesAggregatorASTrans(t *testing.T) {
	ip := [4]byte{192, 0, 2, 1}
	aggVal := make([]byte, 8)
	binary.BigEndian.PutUint32(aggVal[0:4], 196618)
	copy(aggVal[4:8], ip[:])

	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64501))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, aggVal)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)

	aggOp, ok := recordedOp(t, &mods, attribute.AttrAggregator)
	require.True(t, ok, "a width change must rewrite AGGREGATOR")
	agg := materialize(t, recordedGen(t, &mods, aggOp))
	require.Len(t, agg, 6, "a two-octet peer gets a 6-octet AGGREGATOR")
	assert.Equal(t, uint16(asTrans), binary.BigEndian.Uint16(agg[0:2]),
		"the non-mappable aggregator AS becomes AS_TRANS")
	assert.Equal(t, ip[:], agg[2:6], "the aggregator IP is unchanged")

	as4Op, ok := recordedOp(t, &mods, attribute.AttrAS4Aggregator)
	require.True(t, ok, "the real value must survive in AS4_AGGREGATOR")
	as4 := materialize(t, recordedGen(t, &mods, as4Op))
	require.Len(t, as4, 8)
	assert.Equal(t, uint32(196618), binary.BigEndian.Uint32(as4[0:4]))
	assert.Equal(t, ip[:], as4[4:8])
}

// VALIDATES: AC-4 / RFC 7947 Section 2.2.2 -- an RS client's AS_PATH is not
// modified, while RFC 6793 Section 4.2.2 transcoding still applies when the
// widths differ.
// PREVENTS: the two halves being conflated. "No prepend" is not "no work", and
// "transcode needed" is not "prepend allowed".
func TestASPathSlotRSClientSkipsPrepend(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(196618, 64501))
	// The NLRI matters here for the OPPOSITE reason it does in the prepend
	// fixtures. Without it this payload advertises nothing, and Record would
	// reach recordTranscode through the withdraw-only branch rather than through
	// the empty-Prepend one -- so the test would stay green with the RS-client
	// half of the condition deleted, and would be proving nothing about
	// RFC 7947 Section 2.2.2.
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	t.Run("matching widths touch nothing at all", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, payload, ASPathIntent{SrcASN4: true, DstASN4: true})
		require.NoError(t, err)
		assert.False(t, changed, "RFC 7947 Section 2.2.2: the AS_PATH is untouched")
		assert.Empty(t, mods.Ops(), "no operation is recorded, so no rebuild is provoked")
	})

	t.Run("differing widths transcode but never prepend", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		changed, err := edit.Record(&mods, payload, ASPathIntent{SrcASN4: true, DstASN4: false})
		require.NoError(t, err)
		require.True(t, changed)

		op, ok := recordedOp(t, &mods, attribute.AttrASPath)
		require.True(t, ok)
		got := materialize(t, recordedGen(t, &mods, op))
		// Same AS numbers, narrower encoding, and NOTHING prepended.
		assert.Equal(t, probeASPath2(asTrans, 64501), got,
			"the path keeps its own AS numbers; only the encoding changes")

		as4Op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
		require.True(t, ok, "RFC 6793 Section 4.2.2 still obliges the AS4_PATH")
		assert.Equal(t, probeASPath4(196618, 64501), materialize(t, recordedGen(t, &mods, as4Op)))
	})
}

// VALIDATES: a truncated UPDATE body is refused rather than resolved, so the
// caller suppresses the destination.
// PREVENTS: a fail-open read of peer-controlled length fields
// (ai/rules/evidence.md).
func TestASPathSlotRefusesTruncatedPayload(t *testing.T) {
	var mods filterapi.ModAccumulator
	var edit ASPathEdit

	_, err := edit.Record(&mods, []byte{0, 0}, ASPathIntent{Prepend: []uint32{1}})
	require.Error(t, err, "a body too short to hold its own length fields is refused")

	// Attribute section longer than the bytes present.
	body := []byte{0, 0, 0xFF, 0xFF, 0x40}
	_, err = edit.Record(&mods, body, ASPathIntent{Prepend: []uint32{1}})
	require.Error(t, err, "an attribute length past the end of the body is refused")
}

// VALIDATES: a prepend recorded with no AS numbers is a caller bug and is
// refused, rather than silently forwarding an unprepended path to an EBGP peer.
// PREVENTS: a missing prepend, which is a routing-LOOP risk: the prepend is what
// makes RFC 4271 Section 9.1.2 loop detection work at the receiving AS.
func TestASPathSlotEmptyPrependIsRefused(t *testing.T) {
	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	// An empty Prepend on the public entry point reads as transcode-only, which
	// with matching widths is a no-op. The caller bug this guards is the EBGP
	// rail reaching recordPrepend with nothing to prepend.
	changed, err := edit.recordPrepend(&mods, []byte{}, &attribute.SpanIndex{}, ASPathIntent{})
	require.ErrorIs(t, err, ErrASPathIntentEmpty)
	assert.False(t, changed)
}

// The tests below moved here from aspath_rewrite_test.go when the whole-payload
// rewrite was deleted on 2026-09-09. Each asserts the same property over the
// rail that performs it now (test/weakened/49b0956f.md). Their fixtures gained
// probeAdvertisedNLRI, because RFC 4271 Section 5.1.2 b conditions the prepend
// on a route being advertised and the old rail did not check.

// VALIDATES: RFC 4271 Section 4.3 -- an AS_SET is unordered, so a prepend cannot
// join it. A new AS_SEQUENCE is placed in front of it instead.
// PREVENTS: inserting the local AS into an AS_SET, which would say ze is one of
// an unordered group of transit ASes rather than the last hop.
func TestASPathSlotPrependsBeforeALeadingASSet(t *testing.T) {
	set := []byte{byte(attribute.ASSet), 2}
	set = append(set, 0x00, 0x00, 0xFC, 0x00, 0x00, 0x00, 0xFC, 0x01) // 64512, 64513
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, set)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true,
	})
	require.NoError(t, err)
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	parsed, err := attribute.ParseASPath(materialize(t, recordedGen(t, &mods, op)), true)
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 2, "a new segment is created rather than the AS_SET joined")
	assert.Equal(t, attribute.ASSequence, parsed.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, parsed.Segments[0].ASNs)
	assert.Equal(t, attribute.ASSet, parsed.Segments[1].Type)
	assert.Equal(t, []uint32{64512, 64513}, parsed.Segments[1].ASNs)
}

// VALIDATES: RFC 4271 Section 4.3 caps a path segment at 255 AS numbers, so a
// prepend onto a full leading AS_SEQUENCE starts a new segment.
// PREVENTS: writing a 256th AS number into a one-octet count field, which wraps
// to 0 and truncates the whole path.
func TestASPathSlotStartsANewSegmentWhenTheLeadingOneIsFull(t *testing.T) {
	full := []byte{byte(attribute.ASSequence), 255}
	for i := range 255 {
		full = append(full, 0x00, 0x00, byte((100+i)>>8), byte(100+i)) //nolint:gosec // fixture ASNs are small
	}
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	// The AS_PATH value is 1022 octets, so its header is extended-length.
	hdr := []byte{0x50, byte(attribute.AttrASPath), byte(len(full) >> 8), byte(len(full))}
	attrs = append(attrs, append(hdr, full...)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{65000}, SrcASN4: true, DstASN4: true,
	})
	require.NoError(t, err)
	require.True(t, changed)

	op, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	parsed, err := attribute.ParseASPath(materialize(t, recordedGen(t, &mods, op)), true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(parsed.Segments), 2, "the full segment cannot take another AS number")
	assert.Equal(t, attribute.ASSequence, parsed.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, parsed.Segments[0].ASNs)
	assert.Len(t, parsed.Segments[1].ASNs, 255, "the full segment keeps every AS number it had")
}

// VALIDATES: RFC 6793 Section 4.2.2 and the Terminology section's definition of
// a mappable AS ("high two octets are zero"). The boundary that decides whether
// AS_TRANS is written, and whether an AS4_PATH is owed, is 65535 against 65536.
// PREVENTS: an off-by-one at that boundary, which would either corrupt a
// mappable local AS or omit the AS4_PATH that carries a non-mappable one.
func TestASPathSlotPrependsASTransForANonMappableLocalAS(t *testing.T) {
	cases := []struct {
		name     string
		localAS  uint32
		wantHead uint32
		wantAS4  bool
	}{
		{name: "last mappable", localAS: 65535, wantHead: 65535},
		{name: "first non-mappable", localAS: 65536, wantHead: asTrans, wantAS4: true},
		{name: "max four-octet AS", localAS: 4294967295, wantHead: asTrans, wantAS4: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64512))
			payload := buildProbePayload(attrs, probeAdvertisedNLRI)

			var mods filterapi.ModAccumulator
			var edit ASPathEdit
			_, err := edit.Record(&mods, payload, ASPathIntent{
				Prepend: []uint32{tc.localAS}, SrcASN4: true, DstASN4: false,
			})
			require.NoError(t, err)

			asns, ok := recordedASPath(t, &mods, false)
			require.True(t, ok)
			assert.Equal(t, []uint32{tc.wantHead, 64512}, asns)

			as4Op, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
			if !tc.wantAS4 {
				assert.False(t, hasAS4, "RFC 6793 Section 4.2.2: a mappable path owes no AS4_PATH")
				return
			}
			require.True(t, hasAS4, "RFC 6793 Section 4.2.2: a non-mappable AS owes an AS4_PATH")
			assert.Equal(t, probeASPath4(tc.localAS, 64512),
				materialize(t, recordedGen(t, &mods, as4Op)),
				"AS4_PATH carries the real four-octet local AS")
		})
	}
}

// VALIDATES: the dual-AS prepend inserts BOTH AS numbers in order when the
// source carries no AS_PATH at all, matching the order the prepend branch uses.
// PREVENTS: the insert and prepend branches disagreeing on order, which would
// report a different AS path topology depending on what the source sent.
func TestASPathSlotDualInsertsWhenAbsent(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	changed, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{65000, 65100}, SrcASN4: true, DstASN4: true,
	})
	require.NoError(t, err)
	require.True(t, changed)

	asns, ok := recordedASPath(t, &mods, true)
	require.True(t, ok, "an absent AS_PATH is created rather than skipped")
	assert.Equal(t, []uint32{65100, 65000}, asns,
		"RFC 7705 Section 3.3: the override lands outermost, the real AS behind it")
}

// VALIDATES: RFC 6793 Section 4.2.2 applied to the dual-AS prepend: the pair is
// narrowed for an OLD speaker in the same order, and a non-mappable override
// still rides the AS4_PATH.
// PREVENTS: a two-octet peer seeing the two AS numbers swapped, and the dual
// path losing a four-octet override behind AS_TRANS with no recovery.
func TestASPathSlotDualNarrowsToOldSpeaker(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64512))
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	t.Run("mappable override", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		_, err := edit.Record(&mods, payload, ASPathIntent{
			Prepend: []uint32{65000, 65100}, SrcASN4: true, DstASN4: false,
		})
		require.NoError(t, err)

		asns, ok := recordedASPath(t, &mods, false)
		require.True(t, ok)
		assert.Equal(t, []uint32{65100, 65000, 64512}, asns)
		_, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
		assert.False(t, hasAS4, "every AS number is mappable, so no AS4_PATH is owed")
	})

	t.Run("non-mappable override", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		_, err := edit.Record(&mods, payload, ASPathIntent{
			Prepend: []uint32{65000, 200000}, SrcASN4: true, DstASN4: false,
		})
		require.NoError(t, err)

		asns, ok := recordedASPath(t, &mods, false)
		require.True(t, ok)
		assert.Equal(t, []uint32{asTrans, 65000, 64512}, asns)

		as4Op, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
		require.True(t, hasAS4, "the dual prepend owes an AS4_PATH for its four-octet override")
		assert.Equal(t, probeASPath4(200000, 65000, 64512),
			materialize(t, recordedGen(t, &mods, as4Op)))
	})
}

// VALIDATES: the exact VALUE bytes of the AS-path family a two-octet peer
// receives, derived from RFC 6793 and RFC 4271 rather than from ze's output.
//
//	AS_PATH  (RFC 4271 type 2, two-octet AS numbers per RFC 6793 Section 4.2.2):
//	  02 03 | 5BA0 FC00 FC01 -- AS_SEQUENCE, 3 AS numbers,
//	  23456 (AS_TRANS, 0x5BA0), 64512 (0xFC00), 64513 (0xFC01)
//	AS4_PATH (RFC 6793 Section 3, four-octet AS numbers):
//	  02 03 | 00030D40 0000FC00 0000FC01 -- AS_SEQUENCE, 3 AS numbers,
//	  200000 (0x00030D40), 64512, 64513
//
// PREVENTS: a field-order or width defect that a decode-then-compare assertion
// would miss, because the same decoder would read back what the encoder wrote.
func TestASPathSlotAS4PathWireBytes(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64512, 64513))
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{200000}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)

	aspOp, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, []byte{
		0x02, 0x03,
		0x5B, 0xA0,
		0xFC, 0x00,
		0xFC, 0x01,
	}, materialize(t, recordedGen(t, &mods, aspOp)))

	as4Op, ok := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, ok)
	assert.Equal(t, []byte{
		0x02, 0x03,
		0x00, 0x03, 0x0D, 0x40,
		0x00, 0x00, 0xFC, 0x00,
		0x00, 0x00, 0xFC, 0x01,
	}, materialize(t, recordedGen(t, &mods, as4Op)))
}

// VALIDATES: RFC 6793 Section 4.2.2 applies whenever the DESTINATION is an OLD
// speaker, whatever the source encoding was. A two-octet source with a
// non-mappable local AS still owes an AS4_PATH.
// PREVENTS: the matching-width fast path skipping the AS4_PATH obligation,
// which is the branch where nothing else forces the derivation to run.
func TestASPathSlotDerivesAS4PathFromAnOldSpeakerSource(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath2(64512, 64513))
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{200000}, SrcASN4: false, DstASN4: false,
	})
	require.NoError(t, err)

	asns, ok := recordedASPath(t, &mods, false)
	require.True(t, ok)
	assert.Equal(t, []uint32{asTrans, 64512, 64513}, asns)

	as4Op, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, hasAS4, "a non-mappable local AS owes an AS4_PATH at any source width")
	assert.Equal(t, probeASPath4(200000, 64512, 64513),
		materialize(t, recordedGen(t, &mods, as4Op)))
}

// VALIDATES: RFC 6793 Section 4.2.2 -- "Whenever the AS path information
// contains the AS_CONFED_SEQUENCE or AS_CONFED_SET path segment, the NEW BGP
// speaker MUST exclude such path segments from the AS4_PATH attribute being
// constructed." The segments stay in AS_PATH; only AS4_PATH excludes them.
// PREVENTS: leaking a confederation member AS outside the confederation through
// the one attribute that is not filtered on the way out.
func TestASPathSlotExcludesConfedFromAS4Path(t *testing.T) {
	path := []byte{byte(attribute.ASConfedSequence), 1, 0x00, 0x00, 0xFD, 0xE9} // 65001
	path = append(path, byte(attribute.ASSequence), 1, 0x00, 0x03, 0x0D, 0x40)  // 200000
	attrs := probeAttr(0x40, attribute.AttrASPath, path)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{65000}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)

	aspOp, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	asPath, err := attribute.ParseASPath(materialize(t, recordedGen(t, &mods, aspOp)), false)
	require.NoError(t, err)
	require.Len(t, asPath.Segments, 3, "the confederation segment stays in AS_PATH")
	assert.Equal(t, attribute.ASConfedSequence, asPath.Segments[1].Type)

	as4Op, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, hasAS4, "the AS_SEQUENCE holds a non-mappable AS, so an AS4_PATH is owed")
	as4, err := attribute.ParseAS4Path(materialize(t, recordedGen(t, &mods, as4Op)))
	require.NoError(t, err)
	for _, seg := range as4.Segments {
		assert.NotEqual(t, attribute.ASConfedSequence, seg.Type, "AS4_PATH MUST NOT carry AS_CONFED_SEQUENCE")
		assert.NotEqual(t, attribute.ASConfedSet, seg.Type, "AS4_PATH MUST NOT carry AS_CONFED_SET")
		assert.NotContains(t, seg.ASNs, uint32(65001))
	}
}

// VALIDATES: RFC 6793 Section 4.2.2 -- a MAPPABLE aggregating AS is narrowed
// into the six-octet AGGREGATOR and gets NO AS4_AGGREGATOR: "if the AS number is
// mappable, then the AS4_AGGREGATOR attribute MUST NOT be sent."
// PREVENTS: reading TestASPathSlotDerivesAggregatorASTrans as "always emit the
// pair", which would send an attribute the same sentence forbids.
func TestASPathSlotNarrowsAMappableAggregator(t *testing.T) {
	ip := [4]byte{192, 0, 2, 1}
	aggVal := make([]byte, 8)
	binary.BigEndian.PutUint32(aggVal[0:4], 65001)
	copy(aggVal[4:8], ip[:])

	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath4(64512))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, aggVal)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{64510}, SrcASN4: true, DstASN4: false,
	})
	require.NoError(t, err)

	aggOp, ok := recordedOp(t, &mods, attribute.AttrAggregator)
	require.True(t, ok, "a width change must rewrite AGGREGATOR")
	agg := materialize(t, recordedGen(t, &mods, aggOp))
	require.Len(t, agg, 6, "a two-octet peer gets a 6-octet AGGREGATOR")
	assert.Equal(t, uint16(65001), binary.BigEndian.Uint16(agg[0:2]),
		"a mappable aggregating AS is carried as itself, never as AS_TRANS")
	assert.Equal(t, ip[:], agg[2:6])

	as4Op, hasAS4Agg := recordedOp(t, &mods, attribute.AttrAS4Aggregator)
	if hasAS4Agg {
		assert.Equal(t, filterapi.AttrModSuppress, as4Op.Action,
			"RFC 6793 Section 4.2.2 forbids an AS4_AGGREGATOR for a mappable aggregating AS")
	}
}

// VALIDATES: the RECEIVER's own rule is what a prepend toward an OLD speaker has
// to satisfy. attribute.MergeAS4Path is ze's declaration of RFC 6793 Section
// 4.2.3, so this reconstructs the path a peer reads out of the two attributes ze
// emits and compares it with the path ze meant to send.
// PREVENTS: prepending to AS_PATH alone, or to the received AS4_PATH alone. Both
// keep the AS number counts consistent and still reconstruct wrongly, because
// the leading part the receiver takes from AS_PATH is then the AS_TRANS
// placeholders ze just wrote.
func TestASPathSlotPrependedAS4PathReconstructsTheWholePath(t *testing.T) {
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500, asTrans, 64496))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAS4Path, probeASPath4(4200000002, 64496))...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	var mods filterapi.ModAccumulator
	var edit ASPathEdit
	_, err := edit.Record(&mods, payload, ASPathIntent{
		Prepend: []uint32{4200000001}, SrcASN4: false, DstASN4: false,
	})
	require.NoError(t, err)

	aspOp, ok := recordedOp(t, &mods, attribute.AttrASPath)
	require.True(t, ok)
	asPath, err := attribute.ParseASPath(materialize(t, recordedGen(t, &mods, aspOp)), false)
	require.NoError(t, err)

	as4Op, hasAS4 := recordedOp(t, &mods, attribute.AttrAS4Path)
	require.True(t, hasAS4, "the prepended AS is non-mappable, so an AS4_PATH is owed")
	as4, err := attribute.ParseAS4Path(materialize(t, recordedGen(t, &mods, as4Op)))
	require.NoError(t, err)

	reconstructed := attribute.MergeAS4Path(asPath, as4)
	require.Len(t, reconstructed.Segments, 1)
	assert.Equal(t,
		[]uint32{4200000001, 64500, 4200000002, 64496},
		reconstructed.Segments[0].ASNs,
		"the peer reconstructs the local AS at the head and loses no AS number behind it")
}

// FuzzASPathEditRecord drives arbitrary payloads through the EBGP rail.
//
// VALIDATES: no input panics the resolver. A peer chooses every length field in
// the attribute section, so the walk is over data an attacker controls.
// PREVENTS: a malformed UPDATE ending the daemon for every peer at once, which
// is what a panic on the session read goroutine costs (ze-go-style.md).
func FuzzASPathEditRecord(f *testing.F) {
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath4(64512))...)
	f.Add(buildProbePayload(attrs, probeAdvertisedNLRI), uint32(65000), true, true)
	f.Add(buildProbePayload(attrs, probeAdvertisedNLRI), uint32(200000), true, false)
	f.Add([]byte{0, 0, 0, 0}, uint32(65000), true, true)
	f.Add([]byte{}, uint32(1), false, false)

	f.Fuzz(func(t *testing.T, payload []byte, localAS uint32, srcASN4, dstASN4 bool) {
		var mods filterapi.ModAccumulator
		var edit ASPathEdit
		// The return is discarded on purpose: an error is a legitimate answer for
		// a malformed payload, and the property under test is that neither answer
		// is a panic.
		_, _ = edit.Record(&mods, payload, ASPathIntent{
			Prepend: []uint32{localAS}, SrcASN4: srcASN4, DstASN4: dstASN4,
		})
	})
}

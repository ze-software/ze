// Design: docs/architecture/wire/attributes.md — the AS-path family as generate slots
// RFC: rfc/short/rfc4271.md — AS_PATH prepend to an EBGP peer (Section 9.1.2)
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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
	payload := buildProbePayload(attrs, nil)

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
// (ai/rules/fail-closed-guards.md).
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

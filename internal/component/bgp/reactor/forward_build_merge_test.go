package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// attrCodesOf lists the attribute type codes of a rebuilt UPDATE body, in the
// order they appear on the wire.
func attrCodesOf(t *testing.T, body []byte) []uint8 {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "body must carry both length fields")
	wdLen := int(binary.BigEndian.Uint16(body[0:2]))
	attrOff := 2 + wdLen
	require.GreaterOrEqual(t, len(body), attrOff+2)
	attrLen := int(binary.BigEndian.Uint16(body[attrOff : attrOff+2]))
	start := attrOff + 2
	end := start + attrLen
	require.LessOrEqual(t, end, len(body), "declared attribute length must fit the body")

	var codes []uint8
	for off := start; off < end; {
		require.LessOrEqual(t, off+3, end, "attribute header must fit the section")
		flags := body[off]
		codes = append(codes, body[off+1])
		hdr, valLen := 3, int(body[off+2])
		if flags&0x10 != 0 {
			require.LessOrEqual(t, off+4, end)
			hdr = 4
			valLen = int(binary.BigEndian.Uint16(body[off+2 : off+4]))
		}
		off += hdr + valLen
	}
	return codes
}

// VALIDATES: AC-5 — an attribute the edit ADDS is emitted at its ascending
// type-code position, not appended after every source attribute.
// PREVENTS: one route reaching the wire in two different byte orders depending
// on which path built it. RFC 4271 Section 5 describes path attributes ordered
// by ascending type code, and both announce rails already emit that order; the
// forward-modify path appended, so a reflector's output disagreed with its own
// announce output for the same route.
func TestMergeInsertAscendingOrder(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})            // code 1
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100}) // code 5
	community := makeAttr(0xC0, 8, []byte{0, 100, 0, 1}) // code 8
	nlri := []byte{24, 10, 0, 0}

	cases := []struct {
		name string
		src  []byte
		mods func(*filterapi.ModAccumulator)
		want []uint8
	}{
		{
			// The case the old build got wrong: MED (4) added to a source that
			// already carries LOCAL_PREF (5).
			name: "added code sorts before an existing one",
			src:  slices.Concat(origin, localPref),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(4, filterapi.AttrModSet, []byte{0, 0, 0, 50})
			},
			want: []uint8{1, 4, 5},
		},
		{
			name: "added code sorts between two existing ones",
			src:  slices.Concat(origin, community),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 7})
			},
			want: []uint8{1, 5, 8},
		},
		{
			// AS4_PATH (17) rather than OTC (35): the OTC handler is registered
			// by the role plugin's init and this test binary does not link it, so
			// a code-35 fixture would exercise the missing-handler suppression
			// instead of the ordering this test is about.
			name: "added code sorts after every existing one",
			src:  slices.Concat(origin, localPref),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(17, filterapi.AttrModSet, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8})
			},
			want: []uint8{1, 5, 17},
		},
		{
			name: "two added codes both merge into place",
			src:  slices.Concat(origin, community),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 7})
				m.Op(4, filterapi.AttrModSet, []byte{0, 0, 0, 50})
			},
			want: []uint8{1, 4, 5, 8},
		},
		{
			// RFC 4456 Section 8 reflection, the real reason merge-insert exists:
			// ORIGINATOR_ID(9) and CLUSTER_LIST(10) are both new on an iBGP
			// forward and must land before COMMUNITY(8)'s successors.
			name: "reflection injects two attributes in order",
			src:  slices.Concat(origin, localPref, makeAttr(0xC0, 32, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3})),
			mods: func(m *filterapi.ModAccumulator) {
				m.Op(9, filterapi.AttrModSet, []byte{10, 0, 0, 1})
				m.Op(10, filterapi.AttrModPrepend, []byte{0, 0, 0, 7})
			},
			want: []uint8{1, 5, 9, 10, 32},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildModTestPayload(tc.src, nlri)
			var mods filterapi.ModAccumulator
			tc.mods(&mods)

			result, _, fail := buildModifiedPayload(payload, &mods,
				attrModHandlersWithDefaults(), nil, nil)

			require.Equal(t, modifyFailureNone, fail)
			require.NotNil(t, result)
			assert.Equal(t, tc.want, attrCodesOf(t, result),
				"emitted attribute codes must be ascending")

			// The NLRI tail survives the reordering.
			attrLen := int(binary.BigEndian.Uint16(result[2:4]))
			assert.Equal(t, nlri, result[4+attrLen:], "NLRI preserved after merge-insert")
		})
	}
}

// VALIDATES: R-2 — an UPDATE that gains NO new attribute is never reordered,
// even when its source attributes are not already ascending.
// PREVENTS: merge-insert changing bytes on the pure-forward path. A received
// UPDATE may carry attributes in any order; re-sorting them would break the
// zero-copy identity and rewrite routes nobody asked to change.
func TestUntouchedAttributesKeepBaseOrder(t *testing.T) {
	// Deliberately descending on the wire: COMMUNITY(8) before LOCAL_PREF(5).
	community := makeAttr(0xC0, 8, []byte{0, 100, 0, 1})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	origin := makeAttr(0x40, 1, []byte{0x00})
	src := slices.Concat(community, localPref, origin)
	payload := buildModTestPayload(src, []byte{24, 10, 0, 0})

	var mods filterapi.ModAccumulator
	mods.Op(5, filterapi.AttrModSet, []byte{0, 0, 0, 200}) // touches an EXISTING code only

	result, _, fail := buildModifiedPayload(payload, &mods,
		attrModHandlersWithDefaults(), nil, nil)

	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result)
	assert.Equal(t, []uint8{8, 5, 1}, attrCodesOf(t, result),
		"source order is preserved when nothing is added")
}

// VALIDATES: AC-6 — an MP_REACH next-hop rewrite copies the NLRI tail exactly
// once, straight into the output.
// PREVENTS: the intermediate buffer the fragment model exists to remove. The
// tail is the large part of a route carrying many prefixes, so a second copy of
// it is the cost that scales with the route.
func TestFragmentListNoIntermediateCopy(t *testing.T) {
	// AFI(2) + SAFI(1) + NHLen(1) + NH(16) + Reserved(1) + a long NLRI tail.
	tail := make([]byte, 200)
	for i := range tail {
		tail[i] = byte(i)
	}
	val := make([]byte, 0, 5+16+len(tail))
	val = append(val, 0x00, 0x02, 0x01, 16)
	val = append(val, make([]byte, 16)...)
	val = append(val, 0x00)
	val = append(val, tail...)
	mpReach := makeAttr(0x80, 14, val)
	payload := buildModTestPayload(mpReach, nil)

	newNH := make([]byte, 16)
	for i := range newNH {
		newNH[i] = 0xF0 | byte(i&0x0F)
	}
	var mods filterapi.ModAccumulator
	mods.Op(14, filterapi.AttrModSet, newNH)

	result, _, fail := buildModifiedPayload(payload, &mods,
		attrModHandlersWithDefaults(), nil, nil)
	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result)

	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	attr := result[4 : 4+attrLen]
	require.Equal(t, byte(14), attr[1], "MP_REACH emitted")

	hdr := 3
	if attr[0]&0x10 != 0 {
		hdr = 4
	}
	got := attr[hdr:]
	assert.Equal(t, []byte{0x00, 0x02, 0x01}, got[:3], "AFI and SAFI kept from the source")
	assert.Equal(t, byte(16), got[3], "the new next-hop length")
	assert.Equal(t, newNH, got[4:20], "the new next-hop")
	assert.Equal(t, byte(0x00), got[20], "the reserved byte kept from the source")
	assert.Equal(t, tail, got[21:], "the NLRI tail is byte-identical to the source")
}

// VALIDATES: AC-9 — a destination whose edit set touches nothing acquires no
// buffer and the forward stays zero-copy.
// PREVENTS: the rebuild running for a route it has nothing to do to, which would
// cost a copy per destination on the commonest path there is.
func TestNoEditSetNoBuffer(t *testing.T) {
	payload := buildModTestPayload(makeAttr(0x40, 1, []byte{0x00}), []byte{24, 10, 0, 0})
	pp := newPeerPool(message.MaxMsgLen)
	before := pp.available()

	var mods filterapi.ModAccumulator
	result, bufIdx, fail := buildModifiedPayload(payload, &mods,
		attrModHandlersWithDefaults(), pp, nil)

	assert.Nil(t, result, "nothing to apply produces no payload")
	assert.Equal(t, modifyFailureNone, fail, "nothing to apply is not a failure")
	assert.Zero(t, bufIdx, "no per-peer buffer is handed out")
	assert.Equal(t, before, pp.available(), "no buffer was taken from the pool")
}

// VALIDATES: AC-3 — the rebuild allocates nothing per destination once the
// accumulator is hoisted and a per-peer buffer is available.
// PREVENTS: the three allocation sites this child removed coming back. The old
// path allocated a 256-entry array of slices plus one heap slice per touched
// code in the grouping, and three more inside the community handler, on EVERY
// destination of EVERY fan-out.
func TestModifyPathZeroAlloc(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	community := makeAttr(0xC0, 8, []byte{
		0, 100, 0, 1,
		0, 100, 0, 2,
		0, 100, 0, 3,
	})
	localPref := makeAttr(0x40, 5, []byte{0, 0, 0, 100})
	payload := buildModTestPayload(slices.Concat(origin, community, localPref), []byte{24, 10, 0, 0})

	handlers := attrModHandlersWithDefaults()
	pp := newPeerPool(message.MaxMsgLen)

	// One accumulator for the whole fan-out, Reset per destination: the shape
	// the forward rails use.
	var mods filterapi.ModAccumulator

	// The operation values live OUTSIDE the measured closure. A composite
	// literal inside it escapes into the accumulator and would be counted here,
	// measuring the fixture rather than the rebuild. The real producers pass
	// values they already hold (peer_forward_facts.go) or the accumulator's own
	// inline arena (OpCopy), so neither allocates per destination either.
	newLocalPref := []byte{0, 0, 0, 200}
	originatorID := []byte{10, 0, 0, 1}

	allocs := testing.AllocsPerRun(200, func() {
		mods.Reset()
		mods.Op(5, filterapi.AttrModSet, newLocalPref)
		mods.Op(9, filterapi.AttrModSet, originatorID)
		_, idx, _ := buildModifiedPayload(payload, &mods, handlers, pp, nil)
		if idx > 0 {
			pp.Return(idx)
		}
	})

	assert.Zero(t, allocs, "the rebuild must not allocate per destination")
}

// VALIDATES: AC-7 — removing a subset of a community list emits the retained
// values as fragments over the bytes already on the wire, allocating nothing.
// PREVENTS: the community handler's three heap allocations per attribute per
// destination: a copy of the whole list, an append that grew it, and a second
// copy on Set.
func TestCommunityRemoveZeroAllocAndCorrect(t *testing.T) {
	// Six values; the strip removes the second and the fifth, so the retained
	// values form two runs and exercise fragment coalescing.
	list := []byte{
		0, 100, 0, 1,
		0, 100, 0, 2,
		0, 100, 0, 3,
		0, 100, 0, 4,
		0, 100, 0, 5,
		0, 100, 0, 6,
	}
	payload := buildModTestPayload(
		slices.Concat(makeAttr(0x40, 1, []byte{0x00}), makeAttr(0xC0, 8, list)),
		[]byte{24, 10, 0, 0})

	// ONE operation carrying BOTH values, the multi-value form the route-server
	// strip emits.
	strip := []byte{0, 100, 0, 2, 0, 100, 0, 5}
	handlers := attrModHandlersWithDefaults()

	var mods filterapi.ModAccumulator
	mods.Op(8, filterapi.AttrModRemove, strip)

	result, _, fail := buildModifiedPayload(payload, &mods, handlers, nil, nil)
	require.Equal(t, modifyFailureNone, fail)
	require.NotNil(t, result)

	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	section := result[4 : 4+attrLen]
	// ORIGIN(4 bytes) then the community attribute with its 4-byte header.
	comm := section[4:]
	require.Equal(t, byte(8), comm[1], "COMMUNITY emitted")
	require.Equal(t, byte(0x10), comm[0]&0x10, "the community header class is unchanged")
	got := comm[4:]
	want := []byte{
		0, 100, 0, 1,
		0, 100, 0, 3,
		0, 100, 0, 4,
		0, 100, 0, 6,
	}
	assert.Equal(t, hex.EncodeToString(want), hex.EncodeToString(got),
		"every stripped value is gone and every other value survives in order")

	pp := newPeerPool(message.MaxMsgLen)
	allocs := testing.AllocsPerRun(200, func() {
		mods.Reset()
		mods.Op(8, filterapi.AttrModRemove, strip)
		_, idx, _ := buildModifiedPayload(payload, &mods, handlers, pp, nil)
		if idx > 0 {
			pp.Return(idx)
		}
	})
	assert.Zero(t, allocs, "a community removal must allocate nothing")
}

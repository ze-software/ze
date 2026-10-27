package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// buildUpdateWithCommunities builds the UPDATE payload shape the route-server
// rails hand to StripControlCommunities and ParseCommunityPolicy: a two-byte
// withdrawn-routes length, a two-byte path-attribute length, then a single
// extended-length COMMUNITY (type 8) attribute holding the given values.
//
// The wire layout is built here rather than imported so these tests pin the
// PARSER against bytes, not against another helper that could drift with it.
func buildUpdateWithCommunities(values ...uint32) []byte {
	attrData := make([]byte, len(values)*4)
	for i, v := range values {
		binary.BigEndian.PutUint32(attrData[i*4:], v)
	}

	attr := make([]byte, 4+len(attrData))
	attr[0] = 0xC0 | 0x10 // Optional Transitive + Extended Length
	attr[1] = 8           // COMMUNITY
	binary.BigEndian.PutUint16(attr[2:4], uint16(len(attrData)))
	copy(attr[4:], attrData)

	payload := make([]byte, 4+len(attr))
	binary.BigEndian.PutUint16(payload[0:2], 0) // no withdrawn routes
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(attr)))
	copy(payload[4:], attr)
	return payload
}

// community packs a 16-bit high half and 16-bit low half into a wire value.
func community(high, low uint16) uint32 {
	return uint32(high)<<16 | uint32(low)
}

// TestStripControlCommunitiesMultiValue pins the buffer SHAPE the route-server
// rails produce: every matching control community concatenated into one slice.
//
// VALIDATES: spec-fixit-rs-community-strip-arity -- this is the producer half of
// the arity contract. reactor.reactorAPIAdapter.forwardUpdateCore and
// reactor.reactorForwardRS pass this whole slice to the accumulator as ONE
// AttrModRemove operation, so the consumer
// (filter_community.genericCommunityHandler, through filter_community.wholeValues)
// must admit a whole number of values. Cited by symbol: the line numbers this
// comment carried named unrelated code by the time anybody read them.
// PREVENTS: a future change that splits here instead, silently making the
// consumer's multi-value support dead code and leaving the contract untested.
func TestStripControlCommunitiesMultiValue(t *testing.T) {
	const rsASN = 65000

	payload := buildUpdateWithCommunities(
		community(0, 65001),     // control: 0:65001
		community(1, 1),         // ordinary
		community(rsASN, 65002), // control: RS:65002
		community(2, 2),         // ordinary
		community(0, 65003),     // control: 0:65003
	)

	got := StripControlCommunities(payload, rsASN)

	require.Len(t, got, 12, "three control communities, concatenated, 4 bytes each")
	assert.Equal(t, community(0, 65001), binary.BigEndian.Uint32(got[0:4]))
	assert.Equal(t, community(rsASN, 65002), binary.BigEndian.Uint32(got[4:8]))
	assert.Equal(t, community(0, 65003), binary.BigEndian.Uint32(got[8:12]))
}

// TestStripControlCommunitiesPreservesOrdinary verifies that only control values
// are selected.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-2 -- a client's own
// communities must survive the strip.
// PREVENTS: the failure mode that would be WORSE than the leak this spec fixes:
// removing communities a client depends on, silently breaking its downstream
// policy.
func TestStripControlCommunitiesPreservesOrdinary(t *testing.T) {
	const rsASN = 65000

	payload := buildUpdateWithCommunities(
		community(1, 1),
		community(2, 2),
		community(65001, 3), // high half is neither 0 nor the RS ASN
	)

	assert.Nil(t, StripControlCommunities(payload, rsASN),
		"no control communities present: nothing to strip")
}

// TestStripControlCommunitiesSingleValue pins the case that WORKED before the
// consumer fix, so the producer keeps emitting it in the same shape.
//
// VALIDATES: spec-fixit-rs-community-strip-arity AC-3.
// PREVENTS: a regression in the only arity the old consumer could handle, which
// is also the only one with any prior field exposure.
func TestStripControlCommunitiesSingleValue(t *testing.T) {
	const rsASN = 65000

	payload := buildUpdateWithCommunities(community(0, 65001), community(1, 1))

	got := StripControlCommunities(payload, rsASN)

	require.Len(t, got, 4)
	assert.Equal(t, community(0, 65001), binary.BigEndian.Uint32(got))
}

// TestStripControlCommunitiesMatchesLowSixteenBitsOfRSASN documents the matching
// rule, which is a genuine limitation rather than an accident.
//
// VALIDATES: the `rsHigh := uint16(rsASN)` truncation in
// StripControlCommunities: a COMMUNITY high half is 16 bits, so a 4-byte
// community can only ever carry the low half of a 32-bit route-server ASN.
// PREVENTS: someone "fixing" the truncation without realizing RFC 1997 gives the
// high half only 16 bits, and that a 4-octet-ASN route server must use LARGE
// communities to express the same thing unambiguously.
func TestStripControlCommunitiesMatchesLowSixteenBitsOfRSASN(t *testing.T) {
	const rsASN = 0x0001_FDE8 // 4-octet ASN; low 16 bits are 0xFDE8

	payload := buildUpdateWithCommunities(community(0xFDE8, 7))

	got := StripControlCommunities(payload, rsASN)

	require.Len(t, got, 4, "matched on the low 16 bits of the route-server ASN")
	assert.Equal(t, community(0xFDE8, 7), binary.BigEndian.Uint32(got))
}

// TestShouldForwardToUnaffectedByStrip verifies that the forwarding DECISION is
// independent of the stripping.
//
// VALIDATES: spec-fixit-rs-community-strip-arity A-3 -- the arity defect leaks
// control tags but does not mis-route. ParseCommunityPolicy and ShouldForwardTo
// walk the payload independently of StripControlCommunities.
// PREVENTS: this fix being mistaken for, or accidentally widened into, a change
// in which peers receive a route.
func TestShouldForwardToUnaffectedByStrip(t *testing.T) {
	const rsASN = 65000

	// 0:<asn> is the blacklist form: do not advertise to <asn>.
	payload := buildUpdateWithCommunities(community(0, 65001), community(0, 65002))

	policy := ParseCommunityPolicy(payload, rsASN)

	assert.False(t, policy.ShouldForwardTo(65001), "blacklisted peer suppressed")
	assert.False(t, policy.ShouldForwardTo(65002), "blacklisted peer suppressed")
	assert.True(t, policy.ShouldForwardTo(65003), "unlisted peer still receives it")

	// And the same payload yields BOTH control communities for stripping, which
	// is the arity the consumer must handle. Asserting both here ties the two
	// halves together in one test: the decision is right, the strip buffer is
	// multi-value, and only the consumer was wrong.
	assert.Len(t, StripControlCommunities(payload, rsASN), 8)
}

// TestStripControlCommunitiesMalformedPayloads verifies the walk refuses to read
// past its bounds.
//
// VALIDATES: the length guards in StripControlCommunities (short payload, path
// attributes past the end, attribute length past the end).
// PREVENTS: a panic on a truncated or hostile UPDATE, on a path that runs for
// every route forwarded by a route server.
func TestStripControlCommunitiesMalformedPayloads(t *testing.T) {
	const rsASN = 65000

	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"empty", nil},
		{"one byte", []byte{0x00}},
		{"withdrawn length past end", []byte{0xFF, 0xFF}},
		{"no path-attribute length", []byte{0x00, 0x00}},
		{"path-attribute length past end", []byte{0x00, 0x00, 0xFF, 0xFF}},
		{"attribute length past end", []byte{0x00, 0x00, 0x00, 0x04, 0xC0, 0x08, 0xFF, 0x00}},
		{"truncated mid-header", []byte{0x00, 0x00, 0x00, 0x01, 0xC0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				StripControlCommunities(tc.payload, rsASN)
			})
		})
	}
}

// largeValue packs one RFC 8092 large community: Global Administrator, Local
// Data Part 1, Local Data Part 2, four octets each.
func largeValue(ga, ld1, ld2 uint32) []byte {
	var v [12]byte
	binary.BigEndian.PutUint32(v[0:4], ga)
	binary.BigEndian.PutUint32(v[4:8], ld1)
	binary.BigEndian.PutUint32(v[8:12], ld2)
	return v[:]
}

// rsTestASN is the route server's ASN in the route-server control tests below.
const rsTestASN = 65000

// TestPrependCountReachesGenerator drives a parsed prepend request all the way
// to the AS_PATH bytes the rebuild writes for one destination.
//
// VALIDATES: spec-bcp194-1-communities AC-11 and AC-14 at the producer. The
// count PrependCount answers for the destination's ASN becomes that many copies
// of the route server's ASN in the AS_PATH generator ASPathEdit records, which is
// the call both route-server rails make (CommunityPolicy.RSPrepend). A request
// naming another client, or carrying another AS's Global Administrator, changes
// nothing: no AS_PATH operation is recorded at all.
// PREVENTS: PrependCount answering correctly to tests while nothing on the wire
// ever read it, which is how the request went unhonored before this spec.
func TestPrependCountReachesGenerator(t *testing.T) {
	tests := []struct {
		name    string
		large   [][]byte
		peerASN uint32
		want    []uint32 // the AS_PATH sent, nil when no AS_PATH operation is owed
	}{
		{"101 prepends once to the named client", [][]byte{largeValue(rsTestASN, 101, 65002)}, 65002, []uint32{65000, 65001}},
		{"102 prepends twice", [][]byte{largeValue(rsTestASN, 102, 65002)}, 65002, []uint32{65000, 65000, 65001}},
		{"103 prepends three times", [][]byte{largeValue(rsTestASN, 103, 65002)}, 65002, []uint32{65000, 65000, 65000, 65001}},
		{"target 0 names every client", [][]byte{largeValue(rsTestASN, 101, 0)}, 65003, []uint32{65000, 65001}},
		{"the named client wins over target 0", [][]byte{largeValue(rsTestASN, 101, 0), largeValue(rsTestASN, 103, 65002)}, 65002, []uint32{65000, 65000, 65000, 65001}},
		{"a repeated target keeps the highest count", [][]byte{largeValue(rsTestASN, 103, 65002), largeValue(rsTestASN, 101, 65002)}, 65002, []uint32{65000, 65000, 65000, 65001}},
		{"a client not named is untouched", [][]byte{largeValue(rsTestASN, 101, 65002)}, 65003, nil},
		{"another AS's Global Administrator is not a request", [][]byte{largeValue(65099, 101, 65002)}, 65002, nil},
		{"function 1 is not a prepend", [][]byte{largeValue(rsTestASN, 1, 65002)}, 65002, nil},
		{"function 104 is not a prepend", [][]byte{largeValue(rsTestASN, 104, 65002)}, 65002, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var large []byte
			for _, v := range tt.large {
				large = append(large, v...)
			}
			attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
			attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath4(65001))...)
			attrs = append(attrs, probeAttr(0xC0, attribute.AttrLargeCommunity, large)...)
			payload := buildProbePayload(attrs, probeAdvertisedNLRI)

			policy := ParseCommunityPolicy(payload, rsTestASN)
			var buf [3]uint32
			prepend := policy.RSPrepend(buf[:0], tt.peerASN, rsTestASN)

			var mods filterapi.ModAccumulator
			var edit ASPathEdit
			changed, err := edit.Record(&mods, payload, ASPathIntent{Prepend: prepend, SrcASN4: true, DstASN4: true})
			require.NoError(t, err)

			op, ok := recordedOp(t, &mods, attribute.AttrASPath)
			if tt.want == nil {
				assert.False(t, changed, "no request for this client: the AS_PATH leaves as it arrived")
				assert.False(t, ok, "no AS_PATH operation may be recorded without a request")
				return
			}
			require.True(t, changed)
			require.True(t, ok, "a prepend request must record an AS_PATH operation")
			assert.Equal(t, probeASPath4(tt.want...), materialize(t, recordedGen(t, &mods, op)),
				"the route server's ASN, as many times as requested, outermost")
		})
	}
}

// TestRSPrependAllocatesNothing pins the hot-path contract RSPrepend states: a
// caller buffer of capacity three holds the largest request, so a destination
// costs no allocation.
func TestRSPrependAllocatesNothing(t *testing.T) {
	attrs := probeAttr(0xC0, attribute.AttrLargeCommunity, largeValue(rsTestASN, 103, 65002))
	policy := ParseCommunityPolicy(buildProbePayload(attrs, probeAdvertisedNLRI), rsTestASN)
	var buf [3]uint32
	allocs := testing.AllocsPerRun(100, func() {
		_ = policy.RSPrepend(buf[:0], 65002, rsTestASN)
	})
	assert.Zero(t, allocs)
	assert.Equal(t, []uint32{65000, 65000, 65000}, policy.RSPrepend(buf[:0], 65002, rsTestASN))
}

// TestStripControlCommunitiesLarge pins the code 32 removal buffer both
// route-server rails record.
//
// VALIDATES: spec-bcp194-1-communities AC-12 at the producer. Every
// LARGE_COMMUNITY value whose Global Administrator is the route server's full
// four-octet ASN is returned, concatenated in wire order; every other value is
// left alone. The standard-community strip does not reach code 32, and this one
// does not reach code 8.
// PREVENTS: the route server's large control values reaching its clients after
// it consumed them, while the equivalent standard values were already removed.
func TestStripControlCommunitiesLarge(t *testing.T) {
	prepend := largeValue(rsTestASN, 101, 65002)
	foreign := largeValue(65099, 101, 65002)
	other := largeValue(rsTestASN, 0, 65003)
	ordinary := largeValue(65001, 1, 2)

	var large []byte
	for _, v := range [][]byte{prepend, foreign, other, ordinary} {
		large = append(large, v...)
	}
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrCommunity, []byte{0x00, 0x00, 0xFD, 0xE6})...)
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrLargeCommunity, large)...)
	payload := buildProbePayload(attrs, probeAdvertisedNLRI)

	got := StripLargeControlCommunities(payload, rsTestASN)
	assert.Equal(t, append(append([]byte(nil), prepend...), other...), got,
		"exactly the two values in the route server's namespace, in wire order")
	assert.Equal(t, []byte{0x00, 0x00, 0xFD, 0xE6}, StripControlCommunities(payload, rsTestASN),
		"the standard strip still returns code 8 values only")

	// A four-octet route server matches on the whole ASN, never the low sixteen
	// bits the standard form is limited to.
	const rs4 = 4200000000
	same16 := largeValue(rs4&0xFFFF, 101, 65002)
	own := largeValue(rs4, 101, 65002)
	payload4 := buildProbePayload(probeAttr(0xC0, attribute.AttrLargeCommunity, append(append([]byte(nil), same16...), own...)), probeAdvertisedNLRI)
	assert.Equal(t, own, StripLargeControlCommunities(payload4, rs4))

	assert.Nil(t, StripLargeControlCommunities(buildProbePayload(probeAttr(0xC0, attribute.AttrLargeCommunity, ordinary), probeAdvertisedNLRI), rsTestASN),
		"nothing in the route server's namespace, nothing to remove")
	assert.Nil(t, StripLargeControlCommunities(payload[:len(payload)-8], rsTestASN),
		"a truncated attribute section yields no removal buffer")
}

// TestRSControlPredicates pins the one declaration of the route-server control
// set, which the RFC 7454 Section 11 scrub keeps on a route-server client's
// session and which the parser and the strip read. Each boundary is a case:
// a value one past the set must not be kept, or a forged own-GA value survives
// the scrub; a value inside it must be, or the route server never sees the
// request.
//
// VALIDATES: AC-17 (spec-bcp194-1-communities), the shared predicate.
// PREVENTS: the scrub's keep and the route server's reader disagreeing about
// which values are control communities.
func TestRSControlPredicates(t *testing.T) {
	const rsASN uint32 = 65000
	standard := []struct {
		name string
		high uint16
		want bool
	}{
		{"0:X do-not-announce", 0, true},
		{"RS:X announce-only", 65000, true},
		{"RS-1", 64999, false},
		{"RS+1", 65001, false},
		{"well-known high half", 0xFFFF, false},
	}
	for _, tt := range standard {
		t.Run("standard/"+tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRSControlCommunity(tt.high, rsASN))
		})
	}

	large := []struct {
		name     string
		ga       uint32
		function uint32
		want     bool
	}{
		{"function 100", rsASN, 100, false},
		{"function 101", rsASN, 101, true},
		{"function 102", rsASN, 102, true},
		{"function 103", rsASN, 103, true},
		{"function 104", rsASN, 104, false},
		{"foreign GA", rsASN + 1, 101, false},
	}
	for _, tt := range large {
		t.Run("large/"+tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRSControlLargeCommunity(tt.ga, tt.function, rsASN))
		})
	}
}

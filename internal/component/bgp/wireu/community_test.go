package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

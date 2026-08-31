// Design: encode.go — FlowSpec UPDATE encoding from a route command
// RFC: rfc/short/rfc8955.md — traffic filtering actions (Section 7)

package flowspec

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodeFlowSpecActions returns the traffic-action extended communities that
// EncodeRoute puts on the wire for a route command.
//
// It reads them out of the UPDATE body rather than out of the helper that built
// them, because the defect this file exists for was a route command reaching the
// wire with its action missing. Attribute 16 is written by EncodeRoute as
// 0xC0, 0x10, <length>, so the search finds that header and returns the value.
func encodeFlowSpecActions(t *testing.T, routeCmd string) []byte {
	t.Helper()

	body, _, err := EncodeRoute(routeCmd, "ipv4/flow", 65000, false, true, false)
	require.NoError(t, err)

	header := []byte{0xC0, 0x10}
	at := bytes.Index(body, header)
	if at < 0 {
		return nil
	}
	length := int(body[at+2])
	require.LessOrEqual(t, at+3+length, len(body), "attribute 16 runs past the UPDATE body")
	return body[at+3 : at+3+length]
}

// TestEncodeRouteEmitsZeroValuedActions verifies that a zero rate limit and a
// zero DSCP reach the wire.
//
// VALIDATES: `then rate-limit 0`, `then rate-limit 0 packets` and `then mark 0`
// each produce their extended community in the UPDATE.
//
// PREVENTS: an operator's rule being discarded in silence. The encoder read the
// numeric value alone, so a zero meant "no action asked for": `then mark 0` sent
// no marking at all, and `then rate-limit 0` sent a FlowSpec route with no
// action, which RFC 8955 Section 7 leaves at the default accept -- the opposite
// of the discard the operator wrote.
func TestEncodeRouteEmitsZeroValuedActions(t *testing.T) {
	t.Parallel()

	// RFC 8955 Section 7.5: the DSCP is the 6 least significant bits of the value
	// and the "reserved (r): MUST be set to 0 on encoding".
	// RFC requirement: RFC8955-7.5-1 positive -- an encoded traffic-marking action carries zero in every reserved octet and in both reserved bits (§7.5)
	got := encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then mark 0")
	require.Len(t, got, 8, "a marking action must reach the wire, DSCP 0 included")
	assert.Equal(t, []byte{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, got)

	// RFC 8955 Section 7.1: "A traffic-rate of 0 should result on all traffic for
	// the particular flow to be discarded."
	got = encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then rate-limit 0")
	require.Len(t, got, 8, "a zero byte rate is a discard, not an absent action")
	assert.Equal(t, []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, got)

	got = encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then rate-limit 0 packets")
	require.Len(t, got, 8, "a zero packet rate is a discard, not an absent action")
	assert.Equal(t, []byte{0x80, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, got)

	// A route that asks for no action still carries none, so the flags above add
	// an action rather than inventing one.
	got = encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then accept")
	assert.Empty(t, got, "accept asks for no traffic filtering action")
}

// TestEncodeRouteMarkKeepsReservedBitsZero verifies that a marked DSCP occupies
// only the six bits RFC 8955 Section 7.5 gives it.
//
// VALIDATES: `then mark 63` encodes 0x80,0x09 with five zero octets and 0x3f
// last; `then mark 64` is refused rather than truncated.
//
// PREVENTS: an out-of-range DSCP setting the two reserved bits. 63 is the last
// valid value and 64 is the first invalid one, so the pair pins the boundary
// rather than a value in the middle of the range.
func TestEncodeRouteMarkKeepsReservedBitsZero(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC8955-7.5-1 positive -- the largest valid DSCP encodes with both reserved bits clear (§7.5)
	got := encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then mark 63")
	require.Len(t, got, 8)
	assert.Equal(t, []byte{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3f}, got)
	assert.Zero(t, got[7]&0xC0, "the two reserved bits must be 0")

	// RFC requirement: RFC8955-7.5-1 negative -- a DSCP that would set the reserved bits is refused rather than encoded (§7.5)
	_, _, err := EncodeRoute("match destination-ipv4 10.0.0.0/8 then mark 64", "ipv4/flow", 65000, false, true, false)
	require.Error(t, err, "a DSCP above 63 must be refused")
}

// TestEncodeRouteRedirectEncodings verifies that the plugin reaches all three
// RFC 8955 Section 7.4 rt-redirect encodings.
//
// VALIDATES: a 2-octet AS gives type 0x80, an IPv4 address gives type 0x81, and
// a 4-octet AS gives type 0x82.
//
// PREVENTS: the type 0x81 form staying unimplemented on the one path that builds
// a FlowSpec UPDATE from a route command.
func TestEncodeRouteRedirectEncodings(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		target string
		want   []byte
	}{
		{"65000:33756718", []byte{0x80, 0x08, 0xfd, 0xe8, 0x02, 0x03, 0x16, 0x2e}},
		{"192.0.2.1:100", []byte{0x81, 0x08, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64}},
		{"4200000000:100", []byte{0x82, 0x08, 0xfa, 0x56, 0xea, 0x00, 0x00, 0x64}},
	} {
		got := encodeFlowSpecActions(t, "match destination-ipv4 10.0.0.0/8 then redirect "+tc.target)
		assert.Equal(t, tc.want, got, "redirect %s", tc.target)
	}

	_, _, err := EncodeRoute("match destination-ipv4 10.0.0.0/8 then redirect 192.0.2.1:65536",
		"ipv4/flow", 65000, false, true, false)
	require.Error(t, err, "the IPv4 form holds a 2-octet value, so 65536 must be refused")
}

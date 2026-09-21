// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 wire-format proofs
// Related: wire_test.go -- codec round trips; frr.go -- SESSION_ATTRIBUTE codec
//
// Each test here carries an RFC 3209 requirement tag and asserts the bytes the
// codec writes or the error the decoder returns, so a stub that writes nothing
// or accepts anything fails it.
package rsvpte

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// subobjectLengths walks the subobjects of one encoded ERO or RRO object body
// and returns each Length octet. The walk trusts the Length it reads, which is
// what a receiver does, so a wrong Length would surface as a wrong count.
func subobjectLengths(t *testing.T, body []byte) []int {
	t.Helper()
	var lengths []int
	for off := 0; off < len(body); {
		require.Less(t, off+1, len(body), "subobject header inside the body")
		l := int(body[off+1])
		require.Positive(t, l, "a zero Length would never advance")
		lengths = append(lengths, l)
		off += l
	}
	return lengths
}

// objectBodies splits an encoded message into (Class-Num, body) pairs in wire
// order, using the object headers alone.
func objectBodies(t *testing.T, raw []byte) []objectHeader {
	t.Helper()
	var objs []objectHeader
	for off := rsvpHdrLen; off < len(raw); {
		hdr, err := decodeObjectHeader(raw[off:])
		require.NoError(t, err)
		objs = append(objs, hdr)
		off += int(hdr.Length)
	}
	return objs
}

// RFC requirement: RFC3209-4.2.1-1 positive — encodeLabelRequest writes the two reserved octets of a C-Type 1 LABEL_REQUEST as zero over a buffer that held 0xFF, with the L3PID in the last two octets
func TestRFC3209LabelRequestReservedZeroOnSend(t *testing.T) {
	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = 0xFF
	}
	n := encodeLabelRequest(buf, labelRequest{L3PID: 0x86DD})
	require.Equal(t, 8, n)
	assert.Equal(t, CTypeGeneric, buf[3], "C-Type 1: no label range")
	assert.Equal(t, []byte{0, 0}, buf[4:6], "reserved octets are zero on transmission")
	assert.Equal(t, uint16(0x86DD), binary.BigEndian.Uint16(buf[6:8]))
}

// RFC requirement: RFC3209-4.2.1-1 negative — a LABEL_REQUEST whose reserved octets are non-zero on receipt is not refused: decodeLabelRequest returns the L3PID and no error
func TestRFC3209LabelRequestReservedIgnoredOnReceipt(t *testing.T) {
	body := []byte{0xFF, 0xFF, 0x08, 0x00}
	lr, err := decodeLabelRequest(body)
	require.NoError(t, err, "reserved octets are ignored on receipt")
	assert.Equal(t, labelRequest{L3PID: 0x0800}, lr)
}

// RFC requirement: RFC3209-4.3.3-1 positive — every EXPLICIT_ROUTE subobject encodeERO writes has a Length of 8 (IPv4 prefix) or 20 (IPv6 prefix), each at least 4 and a multiple of 4
func TestRFC3209EROSubobjectLengthOnSend(t *testing.T) {
	hops := []eroHop{
		{Address: netip.MustParsePrefix("10.0.0.5/32")},
		{Loose: true, Address: netip.MustParsePrefix("2001:db8::1/128")},
		{Address: netip.MustParsePrefix("10.0.0.9/32")},
	}
	buf := make([]byte, maxRSVPMessage)
	n := encodeERO(buf, hops)
	lengths := subobjectLengths(t, buf[objHdrLen:n])
	assert.Equal(t, []int{8, 20, 8}, lengths)
	for _, l := range lengths {
		assert.GreaterOrEqual(t, l, 4)
		assert.Zero(t, l%4, "Length %d is a multiple of 4", l)
	}
}

// RFC requirement: RFC3209-4.3.3-1 negative — an EXPLICIT_ROUTE subobject whose Length is 3 (below the minimum of 4) is refused by decodeERO with errShortERO
func TestRFC3209EROSubobjectLengthShortRefused(t *testing.T) {
	// One well-formed IPv4 subobject followed by a Length 3 subobject.
	body := []byte{
		EROSubIPv4Prefix, 8, 10, 0, 0, 5, 32, 0,
		EROSubIPv4Prefix, 3, 10,
	}
	hops, err := decodeERO(body)
	require.ErrorIs(t, err, errShortERO)
	assert.Len(t, hops, 1, "the subobjects before the bad one are returned with the error")
}

// RFC requirement: RFC3209-4.4.1-1 positive — every RECORD_ROUTE subobject encodeRRO writes has a Length of 8 (IPv4, Label) or 20 (IPv6), each at least 4 and a multiple of 4
func TestRFC3209RROSubobjectLengthOnSend(t *testing.T) {
	entries := []rroEntry{
		{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.5")},
		{Type: RROSubLabel, Label: 1001},
		{Type: RROSubIPv6, Address: netip.MustParseAddr("2001:db8::5")},
	}
	buf := make([]byte, maxRSVPMessage)
	n := encodeRRO(buf, entries)
	lengths := subobjectLengths(t, buf[objHdrLen:n])
	assert.Equal(t, []int{8, 8, 20}, lengths)
	for _, l := range lengths {
		assert.GreaterOrEqual(t, l, 4)
		assert.Zero(t, l%4, "Length %d is a multiple of 4", l)
	}
}

// RFC requirement: RFC3209-4.4.1-1 negative — a RECORD_ROUTE subobject whose Length is 3 (below the minimum of 4) is refused by decodeRRO with errShortRRO
func TestRFC3209RROSubobjectLengthShortRefused(t *testing.T) {
	body := []byte{
		RROSubIPv4, 8, 10, 0, 0, 5, 32, 0,
		RROSubIPv4, 3, 10,
	}
	entries, err := decodeRRO(body)
	require.ErrorIs(t, err, errShortRRO)
	assert.Len(t, entries, 1)
}

// RFC requirement: RFC3209-4.7.3-1 positive — encodeSessionAttr writes a Length that is a multiple of 4 and at least 8 for Session Names of 0 to 5 octets, and the header Length equals the bytes written
func TestRFC3209SessionAttributeLengthOnSend(t *testing.T) {
	for _, name := range []string{"", "a", "ab", "abc", "abcd", "abcde"} {
		buf := make([]byte, 64)
		n := encodeSessionAttr(buf, sessionAttribute{SetupPrio: 7, HoldPrio: 7, Name: name})
		hdr, err := decodeObjectHeader(buf)
		require.NoError(t, err)
		assert.Equal(t, n, int(hdr.Length), "name %q", name)
		assert.GreaterOrEqual(t, int(hdr.Length), 8, "name %q", name)
		assert.Zero(t, int(hdr.Length)%4, "name %q: Length %d is a multiple of 4", name, hdr.Length)
		assert.Equal(t, uint8(len(name)), buf[7], "Name Length octet")
	}
}

// RFC requirement: RFC3209-4.7.3-1 negative — a SESSION_ATTRIBUTE whose body is shorter than the four fixed octets (object Length below 8) is refused by decodeSessionAttr with errShortObject
func TestRFC3209SessionAttributeLengthShortRefused(t *testing.T) {
	_, err := decodeSessionAttr([]byte{7, 7, 0}, CTypeSessionAttr)
	require.ErrorIs(t, err, errShortObject)
	_, err = decodeSessionAttr(make([]byte, 12+3), CTypeSessionAttrRA)
	require.ErrorIs(t, err, errShortObject, "C-Type 1 adds 12 octets of affinities before the same four")
}

// RFC requirement: RFC3209-3-2 positive — a RESV whose objects are reversed on the wire decodes through DecodeMessage to the same SESSION, STYLE, FLOWSPEC, sender and LABEL as the canonical order
func TestRFC3209ObjectsAcceptedInAnyOrder(t *testing.T) {
	rsb := &resvStateBlock{
		Session:  sessionIPv4{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 42, ExtTunnelID: 0x0a000001},
		FlowSpec: FlowSpec{TokenRate: 1e8, TokenBucket: 1e8, PeakRate: 1e8},
		Label:    labelObject{Label: 16050},
		Style:    StyleSharedExplicit,
		RRO:      []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}},
	}
	filter := senderTemplateIPv4{SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 7}
	canonical := buildResv(rsb, filter, DefaultRefreshPeriod, netip.MustParseAddr("10.0.0.5"))

	// Reverse the object order behind the common header. The header carries no
	// object offsets and DecodeMessage reads no checksum, so nothing else moves.
	objs := objectBodies(t, canonical)
	require.Len(t, objs, 8, "SESSION, HOP, TIME_VALUES, STYLE, FLOWSPEC, sender, LABEL, RRO")
	reversed := make([]byte, 0, len(canonical))
	reversed = append(reversed, canonical[:rsvpHdrLen]...)
	end := len(canonical)
	for i := len(objs) - 1; i >= 0; i-- {
		start := end - int(objs[i].Length)
		reversed = append(reversed, canonical[start:end]...)
		end = start
	}
	require.Equal(t, rsvpHdrLen, end)
	require.NotEqual(t, canonical, reversed)

	want, err := DecodeMessage(canonical)
	require.NoError(t, err)
	got, err := DecodeMessage(reversed)
	require.NoError(t, err, "object order is not a reason to refuse")
	assert.Equal(t, want.Session, got.Session)
	assert.Equal(t, want.Style, got.Style)
	assert.Equal(t, want.FlowSpec, got.FlowSpec)
	assert.Equal(t, want.SenderTemplate, got.SenderTemplate)
	assert.Equal(t, want.Label, got.Label)
	assert.Equal(t, want.RRO, got.RRO)
	assert.Equal(t, uint32(16050), got.Label.Label)
}

// RFC: rfc/short/rfc8669.md — BGP Prefix-SID attribute (code 40) SR-MPLS TLVs
// Overview: routeattr_prefixsid.go — ParsePrefixSID builds the Label-Index and
// Originator SRGB TLV wire bytes carried in attribute 40.
//
// RFC 8669 §3.1 and §3.2 require the sender to clear the Label-Index TLV Reserved
// and Flags fields and the Originator SRGB TLV Flags field. These tests pin the
// emitted bytes at the only place ze encodes those TLVs, so a future edit that
// starts writing configuration into a reserved octet fails here.

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission pins the SR-MPLS
// Label-Index TLV (type 1) bytes produced from a bare label-index configuration.
//
// VALIDATES: RFC 8669 §3.1 — Reserved (1 octet) and Flags (2 octets) are transmitted
// as zero, the TLV type is 1 and the length is the mandated 7.
// PREVENTS: an encoder that repurposes the reserved octets, which a conforming
// receiver is free to ignore today and to reject once those bits gain a meaning.
//
// RFC requirement: RFC8669-3.1-3 positive -- the Label-Index TLV Reserved octet is emitted as 0 by the sender.
// RFC requirement: RFC8669-3.1-5 positive -- the Label-Index TLV Flags field is emitted as 0 by the sender.
func TestRFC8669LabelIndexTLVReservedAndFlagsClearOnTransmission(t *testing.T) {
	sid, err := ParsePrefixSID("777")
	require.NoError(t, err)
	require.Len(t, sid.Bytes, 10, "Label-Index TLV = Type(1) + Length(2) + Value(7)")

	require.Equal(t, byte(1), sid.Bytes[0], "TLV type MUST be 1 (Label-Index)")
	require.Equal(t, []byte{0, 7}, sid.Bytes[1:3], "Label-Index TLV length MUST be 7")

	require.Equal(t, byte(0), sid.Bytes[3], "RFC 8669 §3.1: Reserved MUST be clear on transmission")
	require.Equal(t, []byte{0, 0}, sid.Bytes[4:6], "RFC 8669 §3.1: Flags MUST be clear on transmission")

	require.Equal(t, []byte{0x00, 0x00, 0x03, 0x09}, sid.Bytes[6:10],
		"the 4-octet Label Index carries the configured value 777")
}

// TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission pins the Originator SRGB TLV
// (type 3) bytes produced from an "index, [(base,range)...]" configuration.
//
// VALIDATES: RFC 8669 §3.2 — the SRGB TLV Flags field is transmitted as zero and each
// SRGB entry is a 3-octet base followed by a 3-octet range.
// PREVENTS: a flags octet that carries encoder state, and an SRGB entry layout drift
// that would silently shift every advertised label range.
//
// RFC requirement: RFC8669-3.2-1 positive -- the Originator SRGB TLV Flags field is emitted as 0 by the sender.
func TestRFC8669OriginatorSRGBTLVFlagsClearOnTransmission(t *testing.T) {
	sid, err := ParsePrefixSID("300, [( 800000,4096) ,( 1000000,5000)]")
	require.NoError(t, err)

	// Label-Index TLV occupies the first 10 octets; the SRGB TLV follows.
	require.Greater(t, len(sid.Bytes), 10, "the SRGB TLV must follow the Label-Index TLV")
	srgb := sid.Bytes[10:len(sid.Bytes):len(sid.Bytes)]
	require.Len(t, srgb, 17, "SRGB TLV = Type(1) + Length(2) + Flags(2) + 2 entries * 6")

	require.Equal(t, byte(3), srgb[0], "TLV type MUST be 3 (Originator SRGB)")
	// Value = Flags(2) + 2 entries * 6 octets = 14.
	require.Equal(t, []byte{0, 14}, srgb[1:3], "SRGB TLV length is 2 + 6*N")
	require.Equal(t, []byte{0, 0}, srgb[3:5], "RFC 8669 §3.2: Flags MUST be clear on transmission")

	require.Equal(t, []byte{
		0x0c, 0x35, 0x00, // base 800000
		0x00, 0x10, 0x00, // range 4096
		0x0f, 0x42, 0x40, // base 1000000
		0x00, 0x13, 0x88, // range 5000
	}, srgb[5:17], "each SRGB entry is 3-octet base + 3-octet range, big-endian")
}

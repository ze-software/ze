// Design: plugin_encode_text.go — FlowSpec text components encoded to NLRI wire
// RFC: rfc/short/rfc8955.md — numeric operator AND bit (Section 4.2.1.1) and
// fragment reserved bits (Section 4.2.2.12)

package flowspec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8955TextLeadingAndEncodedUnset drives the text encoder with a
// tcp-flags operand spelled with a leading "&". It is one of the two bitmask
// text forms (tcp-flags, fragment) whose parser hands the encoder a first
// {op, value} pair asking for AND.
//
// VALIDATES: parseTCPFlagsComponentText sets FlowMatch.And from the token's own
// "&" prefix, so "&syn" reaches numericComponent.writeToAFI (core
// types_numeric.go) as a first match with And set, and the encoder alone keeps
// the 'a' bit (0x40) off the first operator octet.
//
// PREVENTS: a leading AND bit on the wire, which RFC 8955 Section 4.2.1.1
// forbids and which a peer reads as an AND against no previous term.
//
// RFC 8955 Section 4.2.1.1: "In the first operator octet of a sequence, it
// MUST be encoded as unset and MUST be treated as always unset on decoding."
//
// MUTATION: core types_numeric.go writeToAFI `if i > 0 && m.And` reduced to
// `if m.And` emits 0xc0 as the first operator octet and reddens this test.
func TestRFC8955TextLeadingAndEncodedUnset(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC8955-4.2.1.1-2 positive -- the text tcp-flags operand "&syn" asks for AND on the first pair, and the encoded NLRI is exactly 03 09 80 02: the first operator octet is 0x80 (end-of-list only) with the AND bit 0x40 clear (§4.2.1.1).
	leading, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{"tcp-flags", "&syn"})
	require.NoError(t, err)
	require.Equal(t, []byte{0x03, 0x09, 0x80, 0x02}, leading)

	// The same encoder does write the AND bit where it is allowed, so the
	// assertion above is about position, not about an encoder that never sets it.
	continued, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{"tcp-flags", "syn", "&ack"})
	require.NoError(t, err)
	require.Equal(t, []byte{0x05, 0x09, 0x00, 0x02, 0xc0, 0x10}, continued)
}

// TestRFC8955TextFragmentIPv4ReservedBitsCleared drives the text encoder with
// numeric fragment operands whose reserved high nibble is set.
//
// VALIDATES: parseBitmaskValue accepts any numeric fragment operand from 0 to
// 255, so "fragment 242" (0xf2) and "fragment 255" (0xff) reach the IPv4 NLRI
// encoder with the reserved bits set, and numericComponent.writeToAFI (core
// types_numeric.go) masks the value with fragmentMask(AFIIPv4) = 0x0f: the
// reserved high nibble is cleared and the IPv4 Don't Fragment bit (0x01), which
// the IPv6 mask 0x0e would drop, is kept.
//
// PREVENTS: reserved fragment bits leaking onto the wire, where a receiver
// would read undefined fragment match flags.
//
// RFC 8955 Section 4.2.2.12: "0: MUST be set to 0 on NLRI encoding and MUST be
// ignored during decoding".
//
// MUTATION: core types_numeric.go writeToAFI with `m.Value &= fragmentMask(afi)`
// removed, or fragmentMask returning 0xff in place of the IPv4 mask 0x0f,
// emits 0xf2 and 0xff and reddens this test. The recorded break halts
// fragmentMask itself, which reddens it the same way.
func TestRFC8955TextFragmentIPv4ReservedBitsCleared(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC8955-4.2.2.12-2 positive -- the IPv4 text operands "fragment 242" and "fragment 255" encode as the single fragment value octets 0x02 and 0x0f: the reserved high nibble is zero and the low IPv4 flag bits, Don't Fragment included, are kept (§4.2.2.12).
	for _, tc := range []struct {
		operand string
		want    []byte
	}{
		{"242", []byte{0x03, 0x0c, 0x80, 0x02}},
		{"255", []byte{0x03, 0x0c, 0x80, 0x0f}},
	} {
		got, err := EncodeFlowSpecComponents(IPv4FlowSpec, []string{"fragment", tc.operand})
		require.NoError(t, err, "fragment %s", tc.operand)
		require.Equal(t, tc.want, got, "fragment %s", tc.operand)
	}
}

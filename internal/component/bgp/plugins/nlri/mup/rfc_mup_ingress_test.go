package mup

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
)

// These tests drive the receive decision ze really makes for a BGP-MUP NLRI
// section: nlritype.Retain with the recognizer this plugin's init registered,
// which is the call the reactor's RFC 7606 Section 5.4 pass makes
// (session_validation_nlritype.go, typedNLRIEdit) before an UPDATE is installed
// or relayed. The kept slice is the NLRI section every peer is then sent.

// isdNLRIHex is an Interwork Segment Discovery route: architecture 1, route type 1,
// RD 65001:100, prefix 10.0.0.0/24.
const isdNLRIHex = "0100010c0000fde900000064180a0000"

// retainMUP runs the registered ingress recognizer of fam over one NLRI section.
func retainMUP(t *testing.T, fam Family, section []byte) (kept []byte, dropped int) {
	t.Helper()
	recognize := nlritype.Get(fam)
	require.NotNil(t, recognize, "the mup plugin registers its ingress recognizer for %s", fam)
	kept, dropped, err := nlritype.Retain(recognize, fam, section, false)
	require.NoError(t, err, "a well-framed section is never an error")
	return kept, dropped
}

// concatNLRI joins NLRIs into one MP_REACH NLRI section.
func concatNLRI(parts ...[]byte) []byte {
	var section []byte
	for _, p := range parts {
		section = append(section, p...)
	}
	return section
}

// TestMUPOtherRouteTypesSilentlyIgnoredAtIngress pins what a received route type
// outside 1..4 becomes under the 3gpp-5g architecture, the only one ze supports.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1, other route types are removed
// from the section with no error, and the ISD route between them is kept intact.
// PREVENTS: an unknown route type being stored or relayed, or refusing the UPDATE.
func TestMUPOtherRouteTypesSilentlyIgnoredAtIngress(t *testing.T) {
	isd, err := hex.DecodeString(isdNLRIHex)
	require.NoError(t, err)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1-1 positive -- under both MUP families, route types 0, 5, 99 and 0xFFFF under the 3gpp-5g architecture are removed from the received NLRI section without an error, and the ISD route in the same section is kept byte for byte
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		section := concatNLRI(
			mupWireNLRI(byte(MUPArch3GPP5G), 0, 0xaa),
			mupWireNLRI(byte(MUPArch3GPP5G), 5, 0xde, 0xad),
			isd,
			mupWireNLRI(byte(MUPArch3GPP5G), 99),
			mupWireNLRI(byte(MUPArch3GPP5G), 0xffff, 0x01, 0x02, 0x03),
		)
		kept, dropped := retainMUP(t, fam, section)
		assert.Equal(t, 4, dropped, "%s: every other route type is ignored", fam)
		assert.Equal(t, isd, kept, "%s: only the defined route type survives", fam)
	}
}

// TestMUPDefinedRouteTypesSurviveIngress pins that the ignore rule is confined to
// the route types Section 3.1 does not define.
//
// VALIDATES: the four defined route types under the 3gpp-5g architecture are all
// kept, so the section reaches the RIB and the relay unchanged.
// PREVENTS: a recognizer that ignores every MUP route type.
func TestMUPDefinedRouteTypesSurviveIngress(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1-1 negative -- route types 1 to 4 are not other route types: a section carrying one route of each is kept whole, with nothing dropped, under both MUP families
	for _, fam := range []Family{IPv4MUP, IPv6MUP} {
		section := concatNLRI(
			mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPISD), 0xaa),
			mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPDSD), 0xbb),
			mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPT1ST), 0xcc),
			mupWireNLRI(byte(MUPArch3GPP5G), uint16(MUPT2ST), 0xdd),
		)
		kept, dropped := retainMUP(t, fam, section)
		assert.Zero(t, dropped, "%s: no defined route type is ignored", fam)
		assert.Equal(t, section, kept, "%s: the section is kept whole", fam)
	}
}

// TestMUPUnknownTLVPropagatedUnchangedAtIngress pins what reaches the relay for a
// T1ST route that carries a TLV type ze does not know.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.3.1, the route with the unknown
// TLV is kept and its octets, TLV included, are the octets ze relays, also when the
// section around it is rewritten.
// PREVENTS: stripping or rewriting an unknown TLV on re-advertisement.
func TestMUPUnknownTLVPropagatedUnchangedAtIngress(t *testing.T) {
	withUnknown := t1stNLRI(t, t1stBody+"00"+"F003010203") // type 0xF0, length 3

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9 positive -- a T1ST route carrying unknown TLV type 0xF0 is kept by the ingress pass, and the section ze relays holds its octets unchanged, TLV included, both when nothing else is dropped and when an unknown route type beside it forces the section to be rebuilt
	kept, dropped := retainMUP(t, IPv4MUP, withUnknown)
	assert.Zero(t, dropped)
	assert.Equal(t, withUnknown, kept)

	rebuilt := concatNLRI(mupWireNLRI(byte(MUPArch3GPP5G), 99, 0x01), withUnknown)
	kept, dropped = retainMUP(t, IPv4MUP, rebuilt)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, withUnknown, kept)

	// The route ze keeps is the one it reads locally: the unknown TLV is ignored and
	// the mandatory fields are those of the same route without it.
	base, _, err := ParseMUP(AFIIPv4, t1stNLRI(t, t1stBody))
	require.NoError(t, err)
	m, rest, err := ParseMUP(AFIIPv4, kept)
	require.NoError(t, err)
	assert.Empty(t, rest)
	assert.Equal(t, base.teid, m.teid)
	assert.Equal(t, base.endpoint, m.endpoint)
}

// TestMUPUnknownTLVNotStrippedAtIngress pins that the relayed octets are not the
// TLV-less form of the route.
//
// VALIDATES: the kept route is longer than the route without its TLV by exactly the
// TLV's five octets, and its Length octet still counts them.
// PREVENTS: a relay that re-encodes the route from its decoded fields, which drops
// every TLV ze does not know.
func TestMUPUnknownTLVNotStrippedAtIngress(t *testing.T) {
	plain := t1stNLRI(t, t1stBody+"00")
	withUnknown := t1stNLRI(t, t1stBody+"00"+"F003010203")

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9 negative -- the relayed route is not the TLV-less route: it is five octets longer than that route, its Length octet counts the TLV, and its last five octets are the unknown TLV
	kept, _ := retainMUP(t, IPv4MUP, concatNLRI(mupWireNLRI(byte(MUPArch3GPP5G), 99), withUnknown))
	require.Len(t, kept, len(plain)+5)
	assert.NotEqual(t, plain, kept[:len(plain)])
	assert.Equal(t, plain[3]+5, kept[3], "the Length octet counts the TLV")
	assert.Equal(t, []byte{0xF0, 0x03, 0x01, 0x02, 0x03}, kept[len(kept)-5:])
}

// t2stEndpointLength encodes a T2ST route through the operator's route syntax and
// returns the Endpoint Length octet ze wrote, or the encoder's error.
//
// The octet sits after the 4-octet NLRI header and the 8-octet RD.
func t2stEndpointLength(t *testing.T, fam, address, teid string) (uint8, error) {
	t.Helper()
	out, err := EncodeNLRIHex(fam, []string{"route-type", "mup-t2st", "rd", "100:100", "address", address, "teid", teid})
	if err != nil {
		return 0, err
	}
	wire, decodeErr := hex.DecodeString(out)
	require.NoError(t, decodeErr)
	require.Greater(t, len(wire), 12)
	return wire[12], nil
}

// TestMUPT2STEncoderKeepsEndpointLengthWithinTEID pins the Endpoint Length ze writes
// for a T2ST route.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.4.1, the Endpoint Length is the
// address bits plus a TEID of at most 32 bits, and the encoded route reads back.
// PREVENTS: an advertised Endpoint Length that claims bits past the TEID field.
func TestMUPT2STEncoderKeepsEndpointLengthWithinTEID(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3 positive -- the encoder writes an Endpoint Length of the address bits plus 0, 16 or 32 TEID bits (32 when no bit length is given), for an IPv4 and an IPv6 endpoint, and each encoded route parses back with that length
	cases := []struct {
		fam, address, teid string
		want               uint8
	}{
		{"ipv4/mup", "10.0.0.1", "0/0", 32},
		{"ipv4/mup", "10.0.0.1", "12345/16", 48},
		{"ipv4/mup", "10.0.0.1", "12345/32", 64},
		{"ipv4/mup", "10.0.0.1", "12345", 64},
		{"ipv6/mup", "2001:db8::1", "12345/32", 160},
	}
	for _, tc := range cases {
		got, err := t2stEndpointLength(t, tc.fam, tc.address, tc.teid)
		require.NoError(t, err, "%s teid %s", tc.fam, tc.teid)
		assert.Equal(t, tc.want, got, "%s teid %s", tc.fam, tc.teid)
	}
}

// TestMUPT2STEncoderRefusesEndpointLengthBeyondTEID pins that no operator input
// makes ze advertise an Endpoint Length past the TEID field.
//
// VALIDATES: a TEID bit length above 32, below 0 or unreadable is refused with
// errTeidBits and no NLRI is written.
// PREVENTS: Endpoint Length 72 (IPv4 plus 40 TEID bits) reaching a peer, and a typo
// silently becoming the 32-bit default.
func TestMUPT2STEncoderRefusesEndpointLengthBeyondTEID(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3 negative -- a TEID bit length of 33 or 40, which would make the Endpoint Length extend beyond the 4-octet TEID field, is refused for an IPv4 and an IPv6 endpoint, and so are a negative and an unreadable bit length
	cases := []struct{ fam, address, teid string }{
		{"ipv4/mup", "10.0.0.1", "12345/33"},
		{"ipv4/mup", "10.0.0.1", "12345/40"},
		{"ipv6/mup", "2001:db8::1", "12345/40"},
		{"ipv4/mup", "10.0.0.1", "12345/-8"},
		{"ipv4/mup", "10.0.0.1", "12345/x"},
	}
	for _, tc := range cases {
		_, err := t2stEndpointLength(t, tc.fam, tc.address, tc.teid)
		assert.ErrorIs(t, err, errTeidBits, "%s teid %s", tc.fam, tc.teid)
	}
}

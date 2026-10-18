package reactor

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// These tests drive whole UPDATE bodies through enforceRFC7606, the receive path, with
// the UPDATE-level check the mup plugin's init() registers (mup_test.go links the plugin
// into this binary). Every UPDATE is ORIGIN IGP, an empty AS_PATH and an MP_REACH_NLRI
// for AFI 1 / SAFI 85 with next hop 192.0.2.1; draft-ietf-bess-mup-safi Section 3.1
// frames each NLRI as [architecture-type:1][route-type:2][length:1][value], RD 65001:100.
const (
	// dsdISDHex is an Interwork Segment Discovery route for 10.0.0.0/24 (Section 3.1.1).
	dsdISDHex = "0100010c0000fde90000006418" + "0a0000"
	// dsdDSDHex is a Direct Segment Discovery route for 192.0.2.1 (Section 3.1.2).
	dsdDSDHex = "0100020c0000fde900000064" + "c0000201"
	// dsdT2STHex is a Type 2 Session Transformed route, endpoint 192.0.2.1, TEID 1
	// (Section 3.1.4).
	dsdT2STHex = "010004110000fde90000006440" + "c0000201" + "00000001"
	// dsdPrefixSIDHex is a Prefix-SID attribute (type 40) holding an SRv6 L3 Service TLV
	// (RFC 9252 Section 2) with one SRv6 SID Information Sub-TLV: SID 2001:db8::1,
	// behavior End.DT4.
	dsdPrefixSIDHex = "c0281c" + "05001900" + "01001500" +
		"20010db8000000000000000000000001" + "00001300"
)

// dsdUpdateBody decodes one UPDATE body: no withdrawn routes, the attribute section
// attrsHex, no IPv4 NLRI.
func dsdUpdateBody(t *testing.T, attrsHex string) []byte {
	t.Helper()
	attrs, err := hex.DecodeString(attrsHex)
	require.NoError(t, err)
	return makeUpdateBody(nil, attrs, nil)
}

// dsdUpdateAttrsHex builds ORIGIN, AS_PATH and an MP_REACH_NLRI holding nlriHex,
// followed by tail (another attribute, or nothing).
func dsdUpdateAttrsHex(t *testing.T, nlriHex, tail string) string {
	t.Helper()
	nlri, err := hex.DecodeString(nlriHex)
	require.NoError(t, err)
	mpReach := hex.EncodeToString([]byte{0x80, 0x0e, byte(9 + len(nlri))}) +
		"00015504c000020100" + nlriHex
	return "40010100" + "400200" + mpReach + tail
}

// TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive drives the Section 3.3.6 sentence through
// the receive path: "When a BGP speaker receives a MP_REACH_NLRI attribute update message
// with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be
// treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw
// procedure of [RFC7606] is applied.  A BGP speaker MUST skip such NLRIs and continue
// processing of rest of the Update message."
//
// VALIDATES: a DSD route with no Prefix-SID leaves MP_REACH_NLRI and is withdrawn in an
// MP_UNREACH_NLRI while its ISD sibling stays; the same holds when the Prefix-SID is
// present but the RFC 8669 Section 4 EBGP rule discards it; no session reset.
// PREVENTS: a DSD route that carries no SID being installed and relayed.
func TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive(t *testing.T) {
	isd, err := hex.DecodeString(dsdISDHex)
	require.NoError(t, err)
	dsd, err := hex.DecodeString(dsdDSDHex)
	require.NoError(t, err)

	cases := []struct {
		name   string
		accept bool
		tail   string
	}{
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 positive -- a DSD route in an MP_REACH_NLRI with no Prefix-SID attribute leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, its ISD sibling is kept, and the session is not reset
		{"no Prefix-SID attribute", true, ""},
		{"Prefix-SID discarded from an EBGP peer", false, dsdPrefixSIDHex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := rfc8669EBGPSession(tc.accept)
			body := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdISDHex+dsdDSDHex, tc.tail))
			wu, _, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err, "a DSD route without a Prefix-SID is never a session reset")

			kept, found := mpReachNLRIOf(t, wu.Payload())
			require.True(t, found)
			assert.Equal(t, isd, kept, "only the DSD route leaves the announcement")
			withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
			require.True(t, found, "the DSD route is withdrawn, not dropped")
			assert.Equal(t, dsd, withdrawn)
		})
	}
}

// TestMUPDSDWithPrefixSIDKeptOnReceive is the other polarity.
//
// VALIDATES: a DSD route whose UPDATE carries a Prefix-SID is returned unchanged, and
// MUP routes of the other types with no Prefix-SID are returned unchanged.
// PREVENTS: the check withdrawing a DSD route that carries its SID, or reaching a route
// type the Section 3.3.6 sentence does not name.
func TestMUPDSDWithPrefixSIDKeptOnReceive(t *testing.T) {
	cases := []struct {
		name  string
		attrs string
	}{
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 negative -- a DSD route in an MP_REACH_NLRI whose UPDATE carries a Prefix-SID attribute is kept, and ISD and Type 2 ST routes with no Prefix-SID are kept
		{"DSD with a Prefix-SID", dsdUpdateAttrsHex(t, dsdISDHex+dsdDSDHex, dsdPrefixSIDHex)},
		{"ISD and T2ST without a Prefix-SID", dsdUpdateAttrsHex(t, dsdISDHex+dsdT2STHex, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := rfc8669EBGPSession(true)
			body := dsdUpdateBody(t, tc.attrs)
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionNone, action)
			assert.Equal(t, body, wu.Payload(), "the UPDATE is returned byte for byte")
		})
	}
}

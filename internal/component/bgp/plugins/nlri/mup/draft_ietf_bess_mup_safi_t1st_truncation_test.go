package mup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// draft-ietf-bess-mup-safi Section 3.1.3.1: "- < 0:   the mandatory fields exceed the
// declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw."
// parseBodyT1ST holds two bounds besides the endpoint one: the six fixed octets (TEID,
// QFI, Endpoint Address Length) and a non-zero Source Address. Each case below ends its
// Length inside one of them and is followed by a whole second NLRI, so the route's body
// slice has capacity past its Length: a parser that skipped the bound would read the
// following route's octets as its own, or panic, instead of refusing.

// t1stThroughPrefix is a T1ST body up to the end of its prefix: RD 100:100, prefix
// 192.168.0.1/32.
const t1stThroughPrefix = "0000006400000064" + "20C0A80001"

// t1stFollowedByRoute frames body as a T1ST NLRI and appends a whole well-formed T1ST
// NLRI after it, the way two routes sit in one MP_REACH_NLRI.
func t1stFollowedByRoute(t *testing.T, body string) []byte {
	t.Helper()
	following := t1stNLRI(t, t1stBody)
	return append(t1stNLRI(t, body), following...)
}

// TestMUPT1STSourceAddressWithinLength pins the bound on a carried Source Address.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, a Length that ends inside a non-zero
// Source Address is refused as truncated.
// PREVENTS: a source address read out of the octets of the next NLRI.
// Method: parse a route whose Source Address Length is 32 and whose Length leaves only
// two of the four source octets, with a second NLRI behind it.
func TestMUPT1STSourceAddressWithinLength(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 negative -- a T1ST route whose
	// Source Address Length is 32 and whose declared Length ends after two of the four
	// Source Address octets, followed by another NLRI, makes ParseMUP return ErrMUPTruncated.
	body := t1stThroughEndpoint + "20" + "C000"
	_, _, err := ParseMUP(AFIIPv4, t1stFollowedByRoute(t, body))
	require.ErrorIs(t, err, ErrMUPTruncated)
}

// TestMUPT1STFixedOctetsWithinLength pins the bound on the six fixed octets that follow
// the prefix.
//
// VALIDATES: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6, a Length that ends inside the TEID,
// before the QFI, or before the Endpoint Address Length is refused as truncated.
// PREVENTS: a TEID, QFI or Endpoint Address Length read out of the octets of the next NLRI.
// Method: parse one route per cut point, each with a second NLRI behind it.
func TestMUPT1STFixedOctetsWithinLength(t *testing.T) {
	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 negative -- a T1ST route whose
	// declared Length ends after two TEID octets, after the whole TEID with no QFI, or after
	// the QFI with no Endpoint Address Length, each followed by another NLRI, makes ParseMUP
	// return ErrMUPTruncated.
	cases := []struct {
		name string
		body string
	}{
		{name: "inside TEID", body: t1stThroughPrefix + "0000"},
		{name: "before QFI", body: t1stThroughPrefix + "00003039"},
		{name: "before Endpoint Address Length", body: t1stThroughPrefix + "00003039" + "09"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseMUP(AFIIPv4, t1stFollowedByRoute(t, tc.body))
			require.ErrorIs(t, err, ErrMUPTruncated)
		})
	}
}

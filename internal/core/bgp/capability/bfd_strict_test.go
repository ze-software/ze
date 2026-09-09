package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBFDStrictModeCapabilityEncoding pins the two facts
// draft-ietf-idr-bgp-bfd-strict-mode Section 5 states about the capability:
// "Capability code: 74" and "Capability length: 0 octets".
//
// VALIDATES: Code() answers 74, Len() answers the two header octets, and
// WriteTo lays down exactly 0x4A 0x00 and touches nothing after it.
//
// PREVENTS: A one-octet drift in either field. A wrong code advertises somebody
// else's capability, and a non-zero length is refused by every peer that reads
// Section 5.
func TestBFDStrictModeCapabilityEncoding(t *testing.T) {
	capability := &BFDStrictMode{}
	require.Equal(t, Code(74), capability.Code())
	require.Equal(t, 2, capability.Len())

	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	written := capability.WriteTo(buf, 1)
	require.Equal(t, 2, written)
	require.Equal(t, []byte{0xFF, 0x4A, 0x00, 0xFF}, buf,
		"code 74 and length 0, written at the offset and nowhere else")
}

// TestBFDStrictModeCapabilityParse is the receive side of Section 5's length.
//
// VALIDATES: An empty value parses to a *BFDStrictMode, and a value of any
// other length is refused with ErrInvalidLength rather than accepted or
// silently ignored.
//
// PREVENTS: A malformed TLV negotiating strict mode. RFC 5492 Section 3 lets ze
// ignore a capability it does not know; a capability it DOES know and whose
// length is wrong is a malformed OPEN, and the session says so.
func TestBFDStrictModeCapabilityParse(t *testing.T) {
	parsed, err := parseCapability(CodeBFDStrictMode, nil)
	require.NoError(t, err)
	require.IsType(t, &BFDStrictMode{}, parsed)

	for _, data := range [][]byte{{0x00}, {0x01, 0x02}} {
		_, err := parseCapability(CodeBFDStrictMode, data)
		require.ErrorIs(t, err, ErrInvalidLength,
			"draft Section 5 gives the capability a length of 0 octets and no other")
	}
}

// TestBFDStrictModeCapabilityRoundTrip walks the whole optional-parameter path,
// which is what an OPEN actually carries.
//
// VALIDATES: A capability written into a buffer and read back through Parse
// arrives as a *BFDStrictMode with code 74.
//
// PREVENTS: An encoder and a parser that each work alone and disagree at the
// TLV boundary.
func TestBFDStrictModeCapabilityRoundTrip(t *testing.T) {
	buf := make([]byte, 2)
	(&BFDStrictMode{}).WriteTo(buf, 0)

	caps, err := Parse(buf)
	require.NoError(t, err)
	require.Len(t, caps, 1)
	require.Equal(t, CodeBFDStrictMode, caps[0].Code())
	require.IsType(t, &BFDStrictMode{}, caps[0])
}

// TestNegotiateBFDStrictMode is draft-ietf-idr-bgp-bfd-strict-mode Section 6:
// "If both the local and remote BGP speakers include the BFD Strict-Mode
// Capability, the BfdStrictNegotiated session attribute ... is set to TRUE."
//
// VALIDATES: The attribute is true only when both sides advertised the
// capability, false in each of the three other combinations, and a one-sided
// advertisement is recorded as a Mismatch the operator can see.
//
// PREVENTS: A one-sided strict mode. A speaker that waits for BFD against a
// peer that never will is a session that never comes up, which is the failure
// Section 1 names: "always using 'strict-mode' would preclude BGP operation in
// an environment where not all routers support BFD strict-mode".
func TestNegotiateBFDStrictMode(t *testing.T) {
	cases := []struct {
		name     string
		local    bool
		remote   bool
		want     bool
		mismatch bool
	}{
		{"both advertise", true, true, true, false},
		{"local only", true, false, false, true},
		{"remote only", false, true, false, true},
		{"neither", false, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var local, remote []Capability
			if tc.local {
				local = append(local, &BFDStrictMode{})
			}
			if tc.remote {
				remote = append(remote, &BFDStrictMode{})
			}

			neg := Negotiate(local, remote, 65001, 65002)
			require.Equal(t, tc.want, neg.BFDStrictMode)
			require.Equal(t, tc.want, neg.Session.BFDStrictMode,
				"the session sub-component carries the same answer")
			require.Equal(t, tc.remote, neg.PeerAdvertised(CodeBFDStrictMode))

			found := false
			for _, m := range neg.Mismatches {
				if m.Code == CodeBFDStrictMode {
					found = true
					require.Equal(t, tc.local, m.LocalSupported)
					require.Equal(t, tc.remote, m.PeerSupported)
				}
			}
			require.Equal(t, tc.mismatch, found,
				"a one-sided advertisement is reported, an agreed one is not")
		})
	}
}

// TestBFDStrictModeRequiredCode covers the `require` capability mode over code
// 74, which is the enforcement path a strict operator can ask for.
//
// VALIDATES: CheckRequiredCodes reports code 74 as missing when the peer did
// not advertise it, and reports nothing when both sides did.
//
// PREVENTS: The code being absent from the negotiated map in
// CheckRequiredCodes, where an unlisted code defaults to false and would be
// reported missing even on a session that negotiated it.
func TestBFDStrictModeRequiredCode(t *testing.T) {
	both := Negotiate([]Capability{&BFDStrictMode{}}, []Capability{&BFDStrictMode{}}, 1, 2)
	require.Empty(t, both.CheckRequiredCodes([]Code{CodeBFDStrictMode}))

	oneSided := Negotiate([]Capability{&BFDStrictMode{}}, nil, 1, 2)
	require.Equal(t, []Code{CodeBFDStrictMode}, oneSided.CheckRequiredCodes([]Code{CodeBFDStrictMode}))
}

// TestBFDStrictModeCodeString keeps the operator-facing name of code 74 exact.
//
// VALIDATES: Code(74).String() names the capability and its number.
//
// PREVENTS: A NOTIFICATION log or a capability mismatch report showing
// "Unknown(74)" for a capability ze implements.
func TestBFDStrictModeCodeString(t *testing.T) {
	require.Equal(t, "BFD Strict-Mode(74)", CodeBFDStrictMode.String())
}

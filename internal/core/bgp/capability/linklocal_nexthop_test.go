package capability

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLinkLocalNextHopCapabilityEncoding pins the two facts
// draft-ietf-idr-linklocal-capability Section 2 states about the capability:
// "Its Capability code is 77 and its Capability Length is 0."
//
// VALIDATES: Code() answers 77, Len() answers the two header octets, and WriteTo
// lays down exactly 0x4D 0x00 and touches nothing after it.
//
// PREVENTS: A one-octet drift in either field. Code 77 is what tells the peer ze
// will send an IPv6 Link-Local-only next hop, and a non-zero length is refused by
// every peer that reads Section 2.
func TestLinkLocalNextHopCapabilityEncoding(t *testing.T) {
	capability := &LinkLocalNextHop{}
	require.Equal(t, Code(77), capability.Code())
	require.Equal(t, 2, capability.Len())

	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	written := capability.WriteTo(buf, 1)
	require.Equal(t, 2, written)
	require.Equal(t, []byte{0xFF, 0x4D, 0x00, 0xFF}, buf,
		"code 77 and length 0, written at the offset and nowhere else")
}

// TestLinkLocalNextHopCapabilityParse is the receive side of Section 2's length.
//
// VALIDATES: An empty value parses to a *LinkLocalNextHop, and a value of any
// other length is refused with ErrInvalidLength.
//
// PREVENTS: The capability arriving as an Unknown, which is what it did before:
// nothing typed it, so nothing could negotiate it, and every Section 3 to
// Section 6 procedure had no condition to read.
func TestLinkLocalNextHopCapabilityParse(t *testing.T) {
	parsed, err := parseCapability(CodeLinkLocalNextHop, nil)
	require.NoError(t, err)
	require.IsType(t, &LinkLocalNextHop{}, parsed)

	for _, data := range [][]byte{{0x00}, {0x01, 0x02}} {
		_, err := parseCapability(CodeLinkLocalNextHop, data)
		require.ErrorIs(t, err, ErrInvalidLength,
			"Section 2 gives the capability a length of 0 and no other")
	}
}

// TestLinkLocalNextHopNegotiatedOnlyWhenBothAdvertiseIt drives Section 2's scope
// sentence: "all procedures described are applicable only when the capability
// described herein has been successfully advertised by both BGP speakers; i.e.,
// negotiated."
//
// VALIDATES: Negotiate sets LinkLocalNextHop for two speakers that both
// advertised code 77, and leaves it false when one of them did not, in which case
// the one-sided advertisement is reported as a mismatch rather than dropped.
//
// PREVENTS: A procedure of Sections 3 to 6 running on a session the peer never
// agreed to. Ze sends a 16-octet Link-Local-only Next Hop only where this is
// true (Peer.linkLocalOnlyNextHopPermitted,
// internal/component/bgp/reactor/peer.go).
func TestLinkLocalNextHopNegotiatedOnlyWhenBothAdvertiseIt(t *testing.T) {
	both := Negotiate([]Capability{&LinkLocalNextHop{}}, []Capability{&LinkLocalNextHop{}}, 65000, 65001)
	require.True(t, both.LinkLocalNextHop, "both speakers advertised it")
	require.True(t, both.Session.LinkLocalNextHop, "the session view carries the same answer")

	for _, tc := range []struct {
		name          string
		local, remote []Capability
	}{
		{"local only", []Capability{&LinkLocalNextHop{}}, nil},
		{"remote only", nil, []Capability{&LinkLocalNextHop{}}},
		{"neither", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			neg := Negotiate(tc.local, tc.remote, 65000, 65001)
			require.False(t, neg.LinkLocalNextHop,
				"one side advertising it is not a negotiation")
		})
	}

	oneSided := Negotiate([]Capability{&LinkLocalNextHop{}}, nil, 65000, 65001)
	found := false
	for _, m := range oneSided.Mismatches {
		if m.Code == CodeLinkLocalNextHop {
			found = true
			require.True(t, m.LocalSupported)
			require.False(t, m.PeerSupported)
		}
	}
	require.True(t, found, "RFC 5492 Section 3: the mismatch is reported")
}

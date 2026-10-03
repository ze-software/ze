// Design: docs/architecture/wire/messages.md -- RFC 4271 receive error handling
// Related: rfc4271_recognized_attribute_errors_test.go -- the other recognized attribute lengths of the same rule

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC4271LocalPrefLengthFromInternalPeer drives the RFC 4271 Section 6.3 length
// check for LOCAL_PREF, whose length is fixed by its type code at four octets.
//
// RFC 7606 Section 7.5 revises the RFC 4271 NOTIFICATION for this attribute: "If the
// LOCAL_PREF attribute is received from an internal neighbor, it SHALL be considered
// malformed if its length is not equal to 4. If malformed, the UPDATE message SHALL
// be handled using the approach of "treat-as-withdraw"." The rule binds only an
// internal neighbor, so the session here is iBGP (both sides AS 65001).
//
// Method: one iBGP session receives three UPDATEs for one prefix over the real
// receive path: LOCAL_PREF at four octets, then at three octets, then at four
// octets again. The malformed UPDATE must reach consumers as a withdrawal of the
// prefix and nothing else, with the session still Established (firstASReceive
// fails otherwise). The correct ones must keep the prefix with LOCAL_PREF byte for
// byte.
//
// RFC requirement: RFC4271-6.3-6 positive -- a LOCAL_PREF from an internal peer whose length is 3, not 4, withdraws the prefix under RFC 7606 Section 7.5 and leaves the session Established.
// RFC requirement: RFC4271-6.3-6 negative -- a LOCAL_PREF from an internal peer at its expected length of 4 keeps the prefix announced with the LOCAL_PREF value unchanged.
func TestRFC4271LocalPrefLengthFromInternalPeer(t *testing.T) {
	origin := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
	path := collapseAttr(0x40, byte(attribute.AttrASPath), nil)
	nextHop := collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})
	localPref := []byte{0, 0, 0, 200}
	good := concatRecognizedAttrs(origin, path, nextHop, collapseAttr(0x40, byte(attribute.AttrLocalPref), localPref))
	bad := concatRecognizedAttrs(origin, path, nextHop, collapseAttr(0x40, byte(attribute.AttrLocalPref), localPref[1:]))
	prefix := []byte{24, 203, 0, 113}

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	require.True(t, settings.IsIBGP(), "the session under test must be internal")
	s, client := firstASSession(t, settings, nil, nil)

	for i, attrs := range [][]byte{good, bad, good} {
		wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, prefix))
		if i == 1 {
			require.Equal(t, makeUpdateBody(prefix, nil, nil), wu.Payload(),
				"a LOCAL_PREF of length 3 from an internal peer must withdraw the prefix and nothing else")
			continue
		}
		nlri, err := wu.NLRI()
		require.NoError(t, err)
		require.Equal(t, prefix, nlri, "the prefix must stay announced")
		value, present := collapseAttrValue(t, wu, attribute.AttrLocalPref)
		require.True(t, present, "a LOCAL_PREF of length 4 must reach the route")
		require.Equal(t, localPref, value)
	}
}

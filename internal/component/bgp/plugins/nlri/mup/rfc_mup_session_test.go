package mup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// These tests drive capability.Negotiate, the RFC 4760 negotiation every ze
// session runs on the two OPEN messages, with the Multiprotocol capabilities for
// the two BGP-MUP families this plugin registers.

// mupCapabilities returns one Multiprotocol capability for each family.
func mupCapabilities(fams ...Family) []capability.Capability {
	caps := make([]capability.Capability, 0, len(fams))
	for _, fam := range fams {
		caps = append(caps, &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
	}
	return caps
}

// TestMUPSessionNegotiatesBothAFIs pins that one session carries BGP-MUP for both
// AFIs when both speakers advertise both MUP families.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.3, ipv4/mup (AFI 1, SAFI 85) and
// ipv6/mup (AFI 2, SAFI 85) are both negotiated on one session, and each announced
// Type 1 ST route is sent under one of the negotiated families.
// PREVENTS: a MUP session that can exchange only one of the two AFIs.
func TestMUPSessionNegotiatesBothAFIs(t *testing.T) {
	both := mupCapabilities(IPv4MUP, IPv6MUP)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3-1 positive -- when both speakers advertise the Multiprotocol capability for AFI 1 and AFI 2 with SAFI 85, the one session negotiates both ipv4/mup and ipv6/mup
	neg := capability.Negotiate(both, both, capability.PeerIdentity{})
	require.NotNil(t, neg)
	assert.True(t, neg.SupportsFamily(family.Family{AFI: family.AFIIPv4, SAFI: SAFIMUP}), "ipv4/mup negotiated")
	assert.True(t, neg.SupportsFamily(family.Family{AFI: family.AFIIPv6, SAFI: SAFIMUP}), "ipv6/mup negotiated")

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2 positive -- on that session, a Type 1 ST route of each AFI is announced in MP_REACH_NLRI with the route's AFI and SAFI 85, which is a family the session negotiated
	for _, tc := range []struct{ fam, cmd string }{
		{"ipv4/mup", "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 10.0.0.2"},
		{"ipv6/mup", "mup-t1st 2001:db8:1:1::2/128 rd 100:100 teid 12345 qfi 9 endpoint 2001::1 next-hop 2001::2"},
	} {
		update, _, err := EncodeRoute(tc.cmd, tc.fam, 65000, true, true, false)
		require.NoError(t, err)
		afi, safi := mpReachAFISAFI(t, update)
		sent := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}
		assert.True(t, neg.SupportsFamily(sent), "%s: sent under %v, which the session negotiated", tc.fam, sent)
	}
}

// TestMUPSessionMissingAFIIsNotNegotiated pins that the session carries a MUP AFI
// only when both speakers advertised it.
//
// VALIDATES: a peer that advertises only ipv4/mup leaves ipv6/mup un-negotiated,
// and a peer that advertises neither leaves both un-negotiated.
// PREVENTS: ze treating a one-AFI MUP session as one that exchanges both AFIs.
func TestMUPSessionMissingAFIIsNotNegotiated(t *testing.T) {
	both := mupCapabilities(IPv4MUP, IPv6MUP)
	ipv4 := family.Family{AFI: family.AFIIPv4, SAFI: SAFIMUP}
	ipv6 := family.Family{AFI: family.AFIIPv6, SAFI: SAFIMUP}

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3-1 negative -- a session whose peer advertises only AFI 1 with SAFI 85 does not negotiate ipv6/mup, and one whose peer advertises no MUP family negotiates neither, so a session exchanges MUP NLRI for both AFIs only when both are advertised
	oneAFI := capability.Negotiate(both, mupCapabilities(IPv4MUP), capability.PeerIdentity{})
	require.NotNil(t, oneAFI)
	assert.True(t, oneAFI.SupportsFamily(ipv4))
	assert.False(t, oneAFI.SupportsFamily(ipv6))

	none := capability.Negotiate(both, mupCapabilities(), capability.PeerIdentity{})
	require.NotNil(t, none)
	assert.False(t, none.SupportsFamily(ipv4))
	assert.False(t, none.SupportsFamily(ipv6))
}

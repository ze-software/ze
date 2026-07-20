// RFC: rfc/short/rfc7752.md — BGP-LS capability negotiation (§3.2)
//
// RFC 7752 Section 3.2 requires two speakers to use BGP Capabilities
// Advertisement before exchanging Link-State NLRI. Ze reaches (16388, 71) the
// same way as any other family: reactor config turns a configured family into
// a Multiprotocol capability, and Negotiate intersects the two advertisements.

package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bgpLSFamily is AFI 16388 / SAFI 71 (RFC 7752 Section 3.2).
var bgpLSFamily = Family{AFI: AFIBGPLS, SAFI: SAFIBGPLS}

// TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated proves the BGP-LS family is
// carried by the RFC 4760 Multiprotocol capability (code 1, length 4) and that
// Negotiate marks it supported only after both OPENs carry it.
//
// VALIDATES: Multiprotocol.WriteTo (capability.go:298) and Negotiate
// (negotiated.go:116) handle AFI 16388 / SAFI 71 like any other family.
// PREVENTS: BGP-LS NLRI being exchanged without a capability handshake.
func TestRFC7752BGPLSCapabilityAdvertisedAndNegotiated(t *testing.T) {
	// RFC requirement: RFC7752-3.2-1 positive -- Link-State NLRI exchange is gated on both speakers advertising the (16388, 71) Multiprotocol capability (§3.2)
	// RFC requirement: RFC9552-5.2-7 positive -- BGP Capabilities Advertisement is what establishes that both speakers can process Link-State NLRI (§5.2)
	mp := &Multiprotocol{AFI: AFIBGPLS, SAFI: SAFIBGPLS}
	buf := make([]byte, mp.Len())
	n := mp.WriteTo(buf, 0)
	require.Equal(t, 6, n)
	assert.Equal(t, byte(CodeMultiprotocol), buf[0], "capability code 1")
	assert.Equal(t, byte(4), buf[1], "capability length 4")
	assert.Equal(t, []byte{0x40, 0x04}, buf[2:4], "AFI 16388")
	assert.Equal(t, byte(0), buf[4], "reserved octet")
	assert.Equal(t, byte(71), buf[5], "SAFI 71")

	parsed, err := Parse([]byte{byte(CodeMultiprotocol), 4, 0x40, 0x04, 0x00, 71})
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	round, ok := parsed[0].(*Multiprotocol)
	require.True(t, ok)
	assert.Equal(t, AFIBGPLS, round.AFI)
	assert.Equal(t, SAFIBGPLS, round.SAFI)

	local := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, mp}
	remote := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, &Multiprotocol{AFI: AFIBGPLS, SAFI: SAFIBGPLS}}
	neg := Negotiate(local, remote, 65001, 65002)
	assert.True(t, neg.SupportsFamily(bgpLSFamily),
		"both speakers advertised BGP-LS, so the family is negotiated")
}

// TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent is the counter-case: a
// speaker that advertises BGP-LS to a peer that does not must not consider the
// family usable.
//
// VALIDATES: Negotiate takes the intersection, not the union.
// PREVENTS: sending Link-State NLRI to a peer that never claimed to parse it.
func TestRFC7752BGPLSCapabilityNotNegotiatedWhenPeerSilent(t *testing.T) {
	// RFC requirement: RFC7752-3.2-1 negative -- when the peer OPEN omits the (16388, 71) capability the family is not negotiated, so no Link-State NLRI is exchanged (§3.2)
	// RFC requirement: RFC9552-5.2-7 negative -- a peer that never advertised the Link-State capability does not get the family negotiated, so Link-State NLRI is never exchanged unilaterally (§5.2)
	local := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, &Multiprotocol{AFI: AFIBGPLS, SAFI: SAFIBGPLS}}
	remote := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}}

	neg := Negotiate(local, remote, 65001, 65002)
	assert.False(t, neg.SupportsFamily(bgpLSFamily),
		"the peer never advertised BGP-LS, so the family is not negotiated")
	assert.True(t, neg.SupportsFamily(Family{AFI: AFIIPv4, SAFI: SAFIUnicast}),
		"the shared family is still negotiated")
	assert.NotContains(t, neg.Families(), bgpLSFamily)

	// The VPN link-state SAFI is negotiated separately from the non-VPN one.
	vpnOnly := []Capability{&Multiprotocol{AFI: AFIBGPLS, SAFI: SAFIBGPLSVPN}}
	negVPN := Negotiate(vpnOnly, vpnOnly, 65001, 65002)
	assert.False(t, negVPN.SupportsFamily(bgpLSFamily),
		"advertising SAFI 72 does not negotiate SAFI 71")
}

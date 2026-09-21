// RFC: rfc/short/rfc4684.md -- Route Target membership capability advertisement (Section 5)
//
// RFC 4684 Section 5: "A BGP speaker that wishes to exchange Route Target
// membership information must use the Multiprotocol Extensions Capability
// Code, as defined in RFC 2858 [5], to advertise the corresponding (AFI, SAFI)
// pair." The pair is (AFI 1, SAFI 132). Ze reaches it the way it reaches every
// family: reactor config looks the family up by name and appends one
// Multiprotocol capability for it (internal/component/bgp/reactor/config.go,
// the families loop), and Negotiate intersects both OPENs.

package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// rtcFamily is AFI 1 / SAFI 132 (RFC 4684 Section 4).
var rtcFamily = Family{AFI: AFIIPv4, SAFI: family.SAFIRTC}

// TestRFC4684RTCFamilyIsAMultiprotocolCapability proves the Route Target
// membership family is advertised as the RFC 2858/4760 Multiprotocol
// Extensions capability, code 1, carrying AFI 1 and SAFI 132, and that both
// speakers advertising it negotiates the family.
//
// VALIDATES: Multiprotocol.WriteTo emits code 1, length 4, AFI 0x0001,
// reserved 0, SAFI 132 for the "ipv4/rtc" family, Parse reads the same bytes
// back, and Negotiate marks the family supported when both sides carry it.
// PREVENTS: the RTC pair reaching the OPEN under any other capability code, or
// under a different AFI/SAFI than the one the family registry names.
func TestRFC4684RTCFamilyIsAMultiprotocolCapability(t *testing.T) {
	// RFC requirement: RFC4684-5-2 positive -- the "ipv4/rtc" family (AFI 1, SAFI 132) encodes as the Multiprotocol Extensions capability: code 1, length 4, AFI 0x0001, reserved 0, SAFI 132; the bytes parse back to the same pair, and two OPENs carrying it negotiate the family (Section 5)
	family.RegisterTestFamilies()
	fam, ok := family.LookupFamily("ipv4/rtc")
	require.True(t, ok, "the RTC family is registered under ipv4/rtc")
	require.Equal(t, rtcFamily, fam)

	mp := &Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI}
	buf := make([]byte, mp.Len())
	n := mp.WriteTo(buf, 0)
	require.Equal(t, 6, n)
	assert.Equal(t, []byte{0x01, 0x04, 0x00, 0x01, 0x00, 0x84}, buf,
		"code 1, length 4, AFI 1, reserved 0, SAFI 132")

	parsed, err := Parse(buf)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	round, ok := parsed[0].(*Multiprotocol)
	require.True(t, ok, "the bytes parse as a Multiprotocol capability")
	assert.Equal(t, AFIIPv4, round.AFI)
	assert.Equal(t, family.SAFIRTC, round.SAFI)

	local := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, mp}
	remote := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, &Multiprotocol{AFI: AFIIPv4, SAFI: family.SAFIRTC}}
	neg := Negotiate(local, remote, PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	assert.True(t, neg.SupportsFamily(rtcFamily),
		"both speakers advertised (1, 132), so the RTC family is negotiated")
}

// TestRFC4684RTCPairUnderAnotherCodeIsNotTheFamily is the counter-case: the
// (1, 132) value under a capability code other than 1 is not a Multiprotocol
// advertisement, so the family is not negotiated from it.
//
// VALIDATES: Parse hands an unassigned-code TLV carrying the RTC pair to the
// unknown path, never to parseMultiprotocol, and Negotiate does not negotiate
// RTC from it.
// PREVENTS: a decoder that keys on the value bytes and treats any 4-octet
// AFI/SAFI-shaped capability as Multiprotocol.
func TestRFC4684RTCPairUnderAnotherCodeIsNotTheFamily(t *testing.T) {
	// RFC requirement: RFC4684-5-2 negative -- the (AFI 1, SAFI 132) value under capability code 200 does not parse as a Multiprotocol capability, and a peer that advertised only that does not get the RTC family negotiated (Section 5)
	parsed, err := Parse([]byte{200, 0x04, 0x00, 0x01, 0x00, 0x84})
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	_, isMP := parsed[0].(*Multiprotocol)
	assert.False(t, isMP, "code 200 is not the Multiprotocol Extensions capability")
	assert.Equal(t, Code(200), parsed[0].Code())

	local := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}, &Multiprotocol{AFI: AFIIPv4, SAFI: family.SAFIRTC}}
	neg := Negotiate(local, parsed, PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	assert.False(t, neg.SupportsFamily(rtcFamily),
		"the peer never advertised (1, 132) under code 1, so RTC is not negotiated")
	assert.NotContains(t, neg.Families(), rtcFamily)
}

// Conformance tests for draft-abraitis-bgp-version-capability at the session
// receive boundary: what a received Software Version Capability (code 75) is
// allowed to change about a BGP session.
//
// The draft text is rfc/drafts/draft-abraitis-bgp-version-capability.txt and the
// extracted checklist is rfc/short/draft-abraitis-bgp-version-capability.md.

package capability

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// softverSessionCaps returns the capability list a peer would advertise. When
// withVersion is set, the Software Version Capability sits between the two
// Multiprotocol entries, so a code-75 branch that touched the family order would
// show in the order Families answers. Everything else is held identical so a
// comparison of two Negotiated results isolates code 75.
func softverSessionCaps(withVersion bool) []Capability {
	caps := []Capability{&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast}}
	if withVersion {
		caps = append(caps, &Unknown{code: codeSoftwareVersion, Data: []byte{8, 'Z', 'e', '/', '0', '.', '1', '.', '0'}})
	}
	return append(caps,
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&ASN4{ASN: 65002},
		&RouteRefresh{},
		&ExtendedMessage{},
	)
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1 negative -- the Software
// Version Capability is used for no session decision: an OPEN carrying code 75 between its
// two Multiprotocol capabilities negotiates the same families in the same order, the same
// Identity, Encoding and Session, and every other negotiated field equal to the same OPEN
// without it; the only difference is that the record of advertised codes gains code 75 and
// no other code.
//
// Method: negotiate both OPENs against one local list, compare Families with its order,
// Identity, Encoding and Session, check that the record of advertised codes with code 75
// equals the record without it plus code 75 alone, then compare the whole Negotiated with
// that record cleared on both sides.
func TestSoftwareVersionCapabilityDecidesNothing(t *testing.T) {
	t.Parallel()
	local := softverSessionCaps(false)

	without := Negotiate(local, softverSessionCaps(false), PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	with := Negotiate(local, softverSessionCaps(true), PeerIdentity{LocalASN: 65001, PeerASN: 65002})

	require.NotNil(t, without)
	require.NotNil(t, with)
	assert.Equal(t, without.Families(), with.Families(), "code 75 must not change the families or their order")
	assert.Equal(t, without.Identity, with.Identity)
	assert.Equal(t, without.Encoding, with.Encoding)
	assert.Equal(t, without.Session, with.Session)
	assert.Equal(t, without.ASN4, with.ASN4)
	assert.Equal(t, without.RouteRefresh, with.RouteRefresh)
	assert.Equal(t, without.ExtendedMessageRecv, with.ExtendedMessageRecv)
	assert.Equal(t, without.ExtendedMessageSend, with.ExtendedMessageSend)
	assert.Equal(t, without.EnhancedRouteRefresh, with.EnhancedRouteRefresh)
	assert.Equal(t, without.BFDStrictMode, with.BFDStrictMode)
	assert.Equal(t, without.LinkLocalNextHop, with.LinkLocalNextHop)
	assert.Equal(t, without.HoldTime, with.HoldTime)
	assert.Equal(t, without.GracefulRestart, with.GracefulRestart)
	assert.Empty(t, with.Mismatches, "an unrecognized capability is no mismatch")

	// The record of advertised codes is the one field code 75 may change, and it may
	// change only by gaining code 75 itself: the positive test below reads it.
	expectedCodes := make(map[Code]bool, len(without.peerCodes)+1)
	maps.Copy(expectedCodes, without.peerCodes)
	expectedCodes[codeSoftwareVersion] = true
	assert.Equal(t, expectedCodes, with.peerCodes, "code 75 must add only itself to the advertised codes")

	// Every other field must be equal.
	withoutCleared, withCleared := *without, *with
	withoutCleared.peerCodes, withCleared.peerCodes = nil, nil
	assert.Equal(t, withoutCleared, withCleared, "code 75 must change nothing negotiated")
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1 positive -- what ze does with
// a received code 75 is record that the peer advertised it, which is what a troubleshooting
// display reads. parseCapability returns it as an Unknown holding the raw value, so the
// version reaches an operator and reaches no decision.
func TestSoftwareVersionCapabilityIsRecordedForDisplay(t *testing.T) {
	t.Parallel()
	value := []byte{8, 'Z', 'e', '/', '0', '.', '1', '.', '0'}

	parsed, err := parseCapability(codeSoftwareVersion, value)
	require.NoError(t, err)
	unknown, isUnknown := parsed.(*Unknown)
	require.True(t, isUnknown, "code 75 must stay unrecognized on the session path")
	assert.Equal(t, codeSoftwareVersion, unknown.Code())
	assert.Equal(t, value, unknown.Data)

	neg := Negotiate(softverSessionCaps(false), softverSessionCaps(true), PeerIdentity{LocalASN: 65001, PeerASN: 65002})
	assert.True(t, neg.PeerAdvertised(codeSoftwareVersion), "the peer's advertisement is recorded")
}

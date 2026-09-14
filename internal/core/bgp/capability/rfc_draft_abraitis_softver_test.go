// Conformance tests for draft-abraitis-bgp-version-capability at the session
// receive boundary: what a received Software Version Capability (code 75) is
// allowed to change about a BGP session.
//
// The draft text is rfc/drafts/draft-abraitis-bgp-version-capability.txt and the
// extracted checklist is rfc/short/draft-abraitis-bgp-version-capability.md.

package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// codeSoftwareVersion is the Software Version Capability's code. The package has
// no Code constant for it because parseCapability has no case for it: an OPEN
// carrying it produces an Unknown, which is the whole point of the tests below.
const codeSoftwareVersion Code = 75

// softverSessionCaps returns the capability list a peer would advertise, with the
// Software Version Capability appended when withVersion is set. Everything else
// is held identical so a comparison of two Negotiated results isolates code 75.
func softverSessionCaps(withVersion bool) []Capability {
	caps := []Capability{
		&Multiprotocol{AFI: AFIIPv4, SAFI: SAFIUnicast},
		&Multiprotocol{AFI: AFIIPv6, SAFI: SAFIUnicast},
		&ASN4{ASN: 65002},
		&RouteRefresh{},
		&ExtendedMessage{},
	}
	if withVersion {
		caps = append(caps, &Unknown{code: codeSoftwareVersion, Data: []byte{8, 'Z', 'e', '/', '0', '.', '1', '.', '0'}})
	}
	return caps
}

// RFC requirement: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1 negative -- the Software
// Version Capability is used for no session decision: an OPEN carrying code 75 negotiates
// exactly the families, ASN4, route-refresh and extended-message state that the same OPEN
// without it negotiates, so nothing in ze's session behavior turns on the version.
func TestSoftwareVersionCapabilityDecidesNothing(t *testing.T) {
	t.Parallel()
	local := softverSessionCaps(false)

	without := Negotiate(local, softverSessionCaps(false), 65001, 65002)
	with := Negotiate(local, softverSessionCaps(true), 65001, 65002)

	require.NotNil(t, with)
	assert.Equal(t, without.ASN4, with.ASN4)
	assert.Equal(t, without.RouteRefresh, with.RouteRefresh)
	assert.Equal(t, without.ExtendedMessage, with.ExtendedMessage)
	assert.Equal(t, without.EnhancedRouteRefresh, with.EnhancedRouteRefresh)
	assert.Equal(t, without.BFDStrictMode, with.BFDStrictMode)
	assert.Equal(t, without.GracefulRestart, with.GracefulRestart)
	assert.ElementsMatch(t, without.Families(), with.Families())
	assert.Equal(t, without.Encoding, with.Encoding)
	assert.Empty(t, with.Mismatches, "an unrecognized capability is no mismatch")
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

	neg := Negotiate(softverSessionCaps(false), softverSessionCaps(true), 65001, 65002)
	assert.True(t, neg.PeerAdvertised(codeSoftwareVersion), "the peer's advertisement is recorded")
}

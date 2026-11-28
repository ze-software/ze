// Design: docs/architecture/route-selection.md -- unit proofs for SRv6 best-path fixture helpers.
// Related: srv6_bestpath_fixture.go -- pure packet builders and JSON response matchers tested here.
package fixture

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestSRv6BestPathFixtureHelpers verifies the pure packet-building and JSON
// inspection helpers used by the srv6-bestpath functional test fixtures.
// VALIDATES: srv6BuildIPv4Update, srv6BuildIPv6Update, srv6DecodeIPv4Prefixes,
//
//	srv6RowMatchesIneligible, and srv6DoctorHasSRv6Finding.
//
// PREVENTS: malformed fixture wire bytes or vacuous JSON row/doctor matchers.
func TestSRv6BestPathFixtureHelpers(t *testing.T) {
	sid := netip.MustParseAddr(srv6BestPathServiceSID)

	pktV4, err := srv6BuildIPv4Update(srv6BestPathSubjectPfx, srv6BestPathSRNextHop, srv6BestPathSRASN, 200, &sid)
	require.NoError(t, err)
	require.Greater(t, len(pktV4), message.HeaderLen)
	updV4, err := message.UnpackUpdate(pktV4[message.HeaderLen:])
	require.NoError(t, err)
	_, _, psidV4, foundV4 := attribute.AttrFind(updV4.PathAttributes, attribute.AttrPrefixSID)
	require.True(t, foundV4)
	require.Equal(t, byte(5), psidV4[0])
	decodedV4, err := srv6DecodeIPv4Prefixes(updV4.NLRI)
	require.NoError(t, err)
	require.Equal(t, []string{srv6BestPathSubjectPfx}, decodedV4)

	pktV6, err := srv6BuildIPv6Update(srv6BestPathBypassV6Pfx, srv6BestPathV6NextHop, srv6BestPathSRASN, 200, sid)
	require.NoError(t, err)
	updV6, err := message.UnpackUpdate(pktV6[message.HeaderLen:])
	require.NoError(t, err)
	_, _, mpReach, foundMP := attribute.AttrFind(updV6.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, foundMP)
	require.Equal(t, byte(2), mpReach[1]) // AFI IPv6
	_, _, psidV6, foundV6 := attribute.AttrFind(updV6.PathAttributes, attribute.AttrPrefixSID)
	require.True(t, foundV6)
	require.Equal(t, byte(5), psidV6[0])

	falseVal := false
	trueVal := true
	rows := []srv6RIBRow{
		{
			Peer:              srv6BestPathSRSourcePeer,
			Prefix:            srv6BestPathSubjectPfx,
			SRv6SIDResolvable: &falseVal,
			IneligibleReason:  srv6BestPathIneligibleKey,
		},
		{
			Peer:   srv6BestPathFallbackPeer,
			Prefix: srv6BestPathSubjectPfx,
		},
		{
			Peer:              "127.0.0.9",
			Prefix:            srv6BestPathSubjectPfx,
			SRv6SIDResolvable: &trueVal,
		},
	}
	require.Equal(t, 3, srv6CountPrefixRows(rows, srv6BestPathSubjectPfx))
	require.True(t, srv6RowMatchesIneligible(rows, srv6BestPathSRSourcePeer, srv6BestPathSubjectPfx, true))
	require.False(t, srv6RowMatchesIneligible(rows, srv6BestPathSRSourcePeer, srv6BestPathSubjectPfx, false))
	require.True(t, srv6RowMatchesIneligible(rows, srv6BestPathFallbackPeer, srv6BestPathSubjectPfx, false))
	require.False(t, srv6RowMatchesIneligible(rows, "127.0.0.9", srv6BestPathSubjectPfx, true))

	doc := srv6DoctorAnswer{
		Diagnostics: []srv6DoctorFinding{
			{
				Code:     srv6BestPathDoctorCode,
				Severity: "warning",
				Message:  "1 BGP route is excluded from best-path selection because its SRv6 Service SID is unresolvable: 198.19.160.0/24 via SRv6 SID fc00:0:99::1.",
				Fix:      srv6BestPathDoctorFix,
			},
		},
	}
	require.True(t, srv6DoctorHasSRv6Finding(doc, srv6BestPathSubjectPfx, srv6BestPathServiceSID))
	require.False(t, srv6DoctorHasSRv6Finding(doc, "10.0.0.0/24", srv6BestPathServiceSID))
	require.True(t, srv6DoctorHasCode(doc, srv6BestPathDoctorCode))
	require.False(t, srv6DoctorHasCode(doc, "doctor-other"))
}

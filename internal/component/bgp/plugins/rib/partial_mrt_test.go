// Design: docs/architecture/mrt.md -- retained attributes in the RIB snapshot bridge.
// Related: rib_mrt.go -- dumpRIBForMRT and its pooled attribute encoder.
// Related: storage/routeentry.go -- received Partial facts independent of shared values.

package rib

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestReceivedPartialSurvivesMRTSnapshot reads actual retained entries through
// the RIB dump bridge implementation, including a complete same-value twin.
// Each dedicated optional-transitive attribute independently owns its Partial
// fact; an unrelated short Extended Length header must also remain unchanged.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." This is a retained snapshot observation, not a
// Session/socket or foreign-peer propagation proof.
func TestReceivedPartialSurvivesMRTSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags [4]byte
	}{
		{"complete", [4]byte{0xc0, 0xc0, 0xc0, 0xc0}},
		{"aggregator-partial", [4]byte{0xe0, 0xc0, 0xc0, 0xc0}},
		{"community-partial", [4]byte{0xc0, 0xe0, 0xc0, 0xc0}},
		{"extended-community-partial", [4]byte{0xc0, 0xc0, 0xe0, 0xc0}},
		{"large-community-partial", [4]byte{0xc0, 0xc0, 0xc0, 0xe0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			peer := netip.MustParseAddr("192.0.2.71")
			routes := storage.NewPeerRIB(peer.String())
			const owner uint64 = 71
			r.bgpPeers[owner] = routes
			r.peerMeta[owner] = &peerMetadata{Address: peer, PeerASN: 65010, RemoteRouterID: 0xc0000247}
			t.Cleanup(routes.Release)
			partialMRTStore(t, routes, partialMRTAttributes(tc.flags), []byte{24, 198, 18, 71})
			complete := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
			partialMRTStore(t, routes, partialMRTAttributes(complete), []byte{24, 198, 18, 72})

			captured := make(map[netip.Prefix][]byte)
			peers := 0
			r.dumpRIBForMRT(registry.RIBDumpVisitor{
				OnPeer: func(address string, asn uint32, identifier [4]byte, ipv6 bool) uint16 {
					peers++
					require.Equal(t, peer.String(), address)
					require.Equal(t, uint32(65010), asn)
					require.Equal(t, [4]byte{192, 0, 2, 71}, identifier)
					require.False(t, ipv6)
					return 7
				},
				OnRoute: func(index, afi, safi uint16, bits uint8, nlri, attrs []byte) {
					require.Equal(t, uint16(7), index)
					require.Equal(t, uint16(1), afi)
					require.Equal(t, uint16(1), safi)
					require.Equal(t, uint8(24), bits)
					require.Len(t, nlri, 3)
					var address [4]byte
					copy(address[:], nlri)
					prefix := netip.PrefixFrom(netip.AddrFrom4(address), int(bits))
					// The visitor MUST consume or copy attributes before returning;
					// the bridge reuses its buffer for the next retained route.
					captured[prefix] = bytes.Clone(attrs)
				},
			})
			require.Equal(t, 1, peers)
			require.Len(t, captured, 2, "both useful retained route keys survive the snapshot")
			for _, route := range []struct {
				prefix netip.Prefix
				flags  [4]byte
			}{
				{netip.MustParsePrefix("198.18.72.0/24"), complete},
				{netip.MustParsePrefix("198.18.71.0/24"), tc.flags},
			} {
				attrs, found := captured[route.prefix]
				require.True(t, found)
				partialMRTExpect(t, attrs, route.flags)
			}
		})
	}
}

// partialMRTStore transfers the clone's attributes to the actual PeerRIB.
// The original MUST release before insertion; InsertEntry MUST retain its own
// reference, and the caller MUST release its clone after that transfer.
func partialMRTStore(t *testing.T, routes *storage.PeerRIB, attrs, nlri []byte) {
	t.Helper()
	entry, fingerprint, length, err := storage.ParseRouteEntry(attrs)
	require.NoError(t, err)
	clone := entry.Clone()
	entry.Release()
	require.NotNil(t, clone)
	defer clone.Release()
	routes.InsertEntry(family.IPv4Unicast, *clone, fingerprint, length, nlri)
}

func partialMRTAttributes(flags [4]byte) []byte {
	return []byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xf2,
		0x40, 3, 4, 192, 0, 2, 71,
		flags[0], 7, 8, 0, 0, 0xfd, 0xe9, 192, 0, 2, 9,
		flags[1], 8, 4, 0, 1, 0, 1,
		flags[2], 16, 8, 0, 2, 0xfd, 0xe9, 0, 0, 0, 1,
		flags[3], 32, 12, 0, 0, 0xfd, 0xe9, 0, 0, 0, 2, 0, 0, 0, 3,
		0xf0, 250, 0, 3, 0xaa, 0xbb, 0xcc,
		0xc0, 251, 2, 0xdd, 0xee,
	}
}

func partialMRTExpect(t *testing.T, attrs []byte, expected [4]byte) {
	t.Helper()
	for _, control := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{
		{attribute.AttrOrigin, []byte{0}},
		{attribute.AttrASPath, []byte{2, 1, 0, 0, 0xfd, 0xf2}},
		{attribute.AttrNextHop, []byte{192, 0, 2, 71}},
		{attribute.AttributeCode(251), []byte{0xdd, 0xee}},
	} {
		_, _, value, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "attribute %d remains reachable", control.code)
		require.Equal(t, control.value, value)
	}
	start, flags, value, found := attribute.AttrFind(attrs, attribute.AttributeCode(250))
	require.True(t, found)
	require.Equal(t, byte(0xf0), byte(flags))
	require.Equal(t, []byte{0xf0, 250, 0, 3}, attrs[start:start+4], "a legal short Extended Length header survives")
	require.Equal(t, []byte{0xaa, 0xbb, 0xcc}, value)
	for i, target := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{
		{attribute.AttrAggregator, []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}},
		{attribute.AttrCommunity, []byte{0, 1, 0, 1}},
		{attribute.AttrExtCommunity, []byte{0, 2, 0xfd, 0xe9, 0, 0, 0, 1}},
		{attribute.AttrLargeCommunity, []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 2, 0, 0, 0, 3}},
	} {
		_, received, value, found := attribute.AttrFind(attrs, target.code)
		require.True(t, found, "retained attribute %d survives", target.code)
		require.Equal(t, target.value, value)
		require.Equal(t, expected[i], byte(received), "attribute %d preserves its independent Partial fact", target.code)
	}
}

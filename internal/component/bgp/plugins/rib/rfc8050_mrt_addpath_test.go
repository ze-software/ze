// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2 with ADD-PATH
// RFC: rfc/short/rfc8050.md -- Section 4.1, RIB Entries store the Path Identifier
// Related: rib_mrt.go -- dumpRIBForMRT

package rib

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050MRTSnapshotPreservesPathIDs verifies that dumpRIBForMRT hands each
// route of a family stored with ADD-PATH to OnRouteAddPath with its 4-octet
// Path Identifier and clean prefix, so two paths for the same prefix from one
// peer are distinguishable in TABLE_DUMP_V2 ADD-PATH entries, and hands each
// route of a family stored without ADD-PATH to OnRoute, never to
// OnRouteAddPath.
//
// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store
// the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
// RIB_IPV6_MULTICAST_ADDPATH subtypes."
//
// VALIDATES: two ADD-PATH paths for 203.0.113.0/24 (path IDs 7 and 42) from
// one peer survive dumpRIBForMRT and TABLE_DUMP_V2 encoding/decoding with
// their exact Path Identifiers and /24 prefix; a non-ADD-PATH peer's route
// arrives on OnRoute alone with its clean prefix.
// PREVENTS: writing zero PathIDs, passing the PathID bytes as prefixLen, or
// handing a route to the callback of the other ADD-PATH state during MRT RIB
// snapshots.
//
// RFC requirement: RFC8050-4.1-1 positive -- dumpRIBForMRT supplies each
// ADD-PATH route's exact Path Identifier (7 and 42) and clean prefix to
// OnRouteAddPath across all four AFI/SAFI-specific subtypes
// (RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH,
// RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH).
// RFC requirement: RFC8050-4.1-1 negative -- routes from non-ADD-PATH
// unicast and multicast families arrive on OnRoute, never on OnRouteAddPath,
// with clean prefixes without stripping prefix bytes.
func TestRFC8050MRTSnapshotPreservesPathIDs(t *testing.T) {
	t.Parallel()

	families := []struct {
		name      string
		fam       family.Family
		prefixLen uint8
		prefix    []byte
		plainPfx  []byte
		apSubtype uint16
	}{
		{"ipv4-unicast", family.IPv4Unicast, 24, []byte{203, 0, 113}, []byte{198, 51, 100}, mrtfmt.TDV2RIBIPv4UnicastAP},
		{"ipv4-multicast", family.IPv4Multicast, 24, []byte{233, 252, 0}, []byte{233, 252, 1}, mrtfmt.TDV2RIBIPv4MulticastAP},
		{"ipv6-unicast", family.IPv6Unicast, 48, []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}, []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02}, mrtfmt.TDV2RIBIPv6UnicastAP},
		{"ipv6-multicast", family.IPv6Multicast, 48, []byte{0xff, 0x3e, 0x00, 0x00, 0x00, 0x01}, []byte{0xff, 0x3e, 0x00, 0x00, 0x00, 0x02}, mrtfmt.TDV2RIBIPv6MulticastAP},
	}

	for _, tc := range families {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)

			apPeer := netip.MustParseAddr("192.0.2.10")
			apRIB := storage.NewPeerRIB(apPeer.String())
			apRIB.SetAddPath(tc.fam, true)
			r.bgpPeers[10] = apRIB
			r.peerMeta[10] = &peerMetadata{Address: apPeer, PeerASN: 65010, RemoteRouterID: 0xc000020a}
			t.Cleanup(apRIB.Release)

			attrs1 := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 10}
			attrs2 := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea, 0x40, 3, 4, 192, 0, 2, 11}
			nlriPath7 := append(binary.BigEndian.AppendUint32(nil, 7), tc.prefixLen)
			nlriPath7 = append(nlriPath7, tc.prefix...)
			nlriPath42 := append(binary.BigEndian.AppendUint32(nil, 42), tc.prefixLen)
			nlriPath42 = append(nlriPath42, tc.prefix...)
			apRIB.Insert(tc.fam, attrs1, nlriPath7)
			apRIB.Insert(tc.fam, attrs2, nlriPath42)

			plainPeer := netip.MustParseAddr("192.0.2.20")
			plainRIB := storage.NewPeerRIB(plainPeer.String())
			r.bgpPeers[20] = plainRIB
			r.peerMeta[20] = &peerMetadata{Address: plainPeer, PeerASN: 65020, RemoteRouterID: 0xc0000214}
			t.Cleanup(plainRIB.Release)
			plainNLRI := append([]byte{tc.prefixLen}, tc.plainPfx...)
			plainRIB.Insert(tc.fam, attrs1, plainNLRI)

			peerAddrs := make(map[uint16]string)
			apPathIDs := make(map[uint32]bool)
			var plainCount int

			r.dumpRIBForMRT(registry.RIBDumpVisitor{
				OnPeer: func(addr string, _ uint32, _ [4]byte, _ bool) uint16 {
					idx := uint16(len(peerAddrs))
					peerAddrs[idx] = addr
					return idx
				},
				OnRouteAddPath: func(peerIndex, afi, safi uint16, pathID uint32, prefixLen uint8, prefix, attrs []byte) {
					require.Equal(t, uint16(tc.fam.AFI), afi)
					require.Equal(t, uint16(tc.fam.SAFI), safi)
					require.Equal(t, tc.prefixLen, prefixLen)
					require.Equal(t, apPeer.String(), peerAddrs[peerIndex],
						"only the ADD-PATH peer's routes may arrive on OnRouteAddPath")
					require.Equal(t, tc.prefix, prefix)
					buf := make([]byte, 512)
					off := mrtfmt.WriteRIBHeader(buf, 0, 1, prefixLen, prefix)
					off += mrtfmt.WriteRIBEntries(buf, off, []mrtfmt.RIBEntry{{PeerIndex: peerIndex, PathID: pathID, Attributes: attrs}}, true)
					rec, err := mrtfmt.DecodeRIBRecord(tc.apSubtype, buf[:off])
					require.NoError(t, err)
					require.Len(t, rec.Entries, 1)
					require.Equal(t, pathID, rec.Entries[0].PathID)
					apPathIDs[rec.Entries[0].PathID] = true
				},
				OnRoute: func(peerIndex, afi, safi uint16, prefixLen uint8, prefix, _ []byte) {
					require.Equal(t, uint16(tc.fam.AFI), afi)
					require.Equal(t, uint16(tc.fam.SAFI), safi)
					require.Equal(t, tc.prefixLen, prefixLen)
					require.Equal(t, plainPeer.String(), peerAddrs[peerIndex],
						"only the non-ADD-PATH peer's routes may arrive on OnRoute")
					require.Equal(t, tc.plainPfx, prefix)
					plainCount++
				},
			})

			require.Equal(t, map[uint32]bool{7: true, 42: true}, apPathIDs,
				"both ADD-PATH routes for %s must retain their distinct Path IDs", tc.name)
			require.Equal(t, 1, plainCount, "non-ADD-PATH control route for %s must dump cleanly on OnRoute", tc.name)
		})
	}
}

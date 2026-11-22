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

// TestRFC8050MRTSnapshotPreservesPathIDs verifies that dumpRIBForMRT extracts
// each ADD-PATH route's 4-octet Path Identifier and clean prefix so two paths
// for the same prefix from one peer are distinguishable in TABLE_DUMP_V2
// RIB_IPV4_UNICAST_ADDPATH entries.
//
// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store
// the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
// RIB_IPV6_MULTICAST_ADDPATH subtypes."
//
// VALIDATES: two ADD-PATH paths for 203.0.113.0/24 (path IDs 7 and 42) from
// one peer survive dumpRIBForMRT and TABLE_DUMP_V2 encoding/decoding with
// their exact Path Identifiers and /24 prefix; a non-ADD-PATH peer's route
// reports pathID 0 and its clean /24 prefix.
// PREVENTS: writing zero PathIDs or passing the PathID bytes as prefixLen
// during MRT RIB snapshots.
//
// RFC requirement: RFC8050-4.1-1 positive -- dumpRIBForMRT supplies each
// ADD-PATH route's exact Path Identifier (7 and 42) and clean /24 prefix to
// OnRouteAddPath, so both paths for 203.0.113.0/24 decode from
// RIB_IPV4_UNICAST_ADDPATH with distinct Path Identifiers.
// RFC requirement: RFC8050-4.1-1 negative -- a route from a non-ADD-PATH
// family reports PathID 0 and clean /24 prefix without stripping prefix bytes.
func TestRFC8050MRTSnapshotPreservesPathIDs(t *testing.T) {
	t.Parallel()

	r := newTestRIBManager(t)

	apPeer := netip.MustParseAddr("192.0.2.10")
	apRIB := storage.NewPeerRIB(apPeer.String())
	apRIB.SetAddPath(family.IPv4Unicast, true)
	r.bgpPeers[10] = apRIB
	r.peerMeta[10] = &peerMetadata{Address: apPeer, PeerASN: 65010, RemoteRouterID: 0xc000020a}
	t.Cleanup(apRIB.Release)

	attrs1 := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 10}
	attrs2 := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xea, 0x40, 3, 4, 192, 0, 2, 11}
	nlriPath7 := binary.BigEndian.AppendUint32(nil, 7)
	nlriPath7 = append(nlriPath7, 24, 203, 0, 113)
	nlriPath42 := binary.BigEndian.AppendUint32(nil, 42)
	nlriPath42 = append(nlriPath42, 24, 203, 0, 113)
	partialMRTStore(t, apRIB, attrs1, nlriPath7)
	partialMRTStore(t, apRIB, attrs2, nlriPath42)

	plainPeer := netip.MustParseAddr("192.0.2.20")
	plainRIB := storage.NewPeerRIB(plainPeer.String())
	r.bgpPeers[20] = plainRIB
	r.peerMeta[20] = &peerMetadata{Address: plainPeer, PeerASN: 65020, RemoteRouterID: 0xc0000214}
	t.Cleanup(plainRIB.Release)
	partialMRTStore(t, plainRIB, attrs1, []byte{24, 198, 51, 100})

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
			require.Equal(t, uint16(1), afi)
			require.Equal(t, uint16(1), safi)
			require.Equal(t, uint8(24), prefixLen)
			switch peerAddrs[peerIndex] {
			case apPeer.String():
				require.Equal(t, []byte{203, 0, 113}, prefix)
				buf := make([]byte, 512)
				off := mrtfmt.WriteRIBHeader(buf, 0, 1, prefixLen, prefix)
				off += mrtfmt.WriteRIBEntries(buf, off, []mrtfmt.RIBEntry{{PeerIndex: peerIndex, PathID: pathID, Attributes: attrs}}, true)
				rec, err := mrtfmt.DecodeRIBRecord(mrtfmt.TDV2RIBIPv4UnicastAP, buf[:off])
				require.NoError(t, err)
				require.Len(t, rec.Entries, 1)
				require.Equal(t, pathID, rec.Entries[0].PathID)
				apPathIDs[rec.Entries[0].PathID] = true
			case plainPeer.String():
				require.Zero(t, pathID)
				require.Equal(t, []byte{198, 51, 100}, prefix)
				plainCount++
			default:
				t.Fatalf("unexpected peer index %d", peerIndex)
			}
		},
	})

	require.Equal(t, map[uint32]bool{7: true, 42: true}, apPathIDs,
		"both ADD-PATH routes for 203.0.113.0/24 must retain their distinct Path IDs")
	require.Equal(t, 1, plainCount, "non-ADD-PATH control route must dump cleanly with pathID 0")
}

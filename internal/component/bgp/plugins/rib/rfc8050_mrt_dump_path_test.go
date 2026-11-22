// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2 with ADD-PATH
// RFC: rfc/short/rfc8050.md -- Sections 4.1 and 4.2, the subtype follows the stored route
// Related: rib_mrt.go -- dumpRIBForMRT
// Related: ../../../../plugins/mrt/dump.go -- Component.writeTableDumpV2, the writer these tests drive

package rib

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
)

// mrtDumpRecord is one RIB record of a TABLE_DUMP_V2 file: its subtype and
// its body as written, after the MRT common header.
type mrtDumpRecord struct {
	subtype uint16
	body    []byte
}

// mrtDumpAttrs is the stored attribute set of every route below: ORIGIN IGP,
// AS_PATH {65001}, NEXT_HOP 192.0.2.10.
var mrtDumpAttrs = []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 10}

// mrtDumpFlowSpec is an IPv4 FlowSpec NLRI with its shortest length field:
// length 5, then destination prefix 192.0.2.0/24.
var mrtDumpFlowSpec = []byte{0x05, 0x01, 0x18, 0xc0, 0x00, 0x02}

var mrtDumpFlowSpecFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

// addMRTDumpPeer gives r one peer at address with no ADD-PATH family yet. The
// owner key only has to be unique within r.
func addMRTDumpPeer(t *testing.T, r *RIBManager, owner uint64, address string) *storage.PeerRIB {
	t.Helper()
	addr := netip.MustParseAddr(address)
	peerRIB := storage.NewPeerRIB(addr.String())
	r.bgpPeers[owner] = peerRIB
	r.peerMeta[owner] = &peerMetadata{Address: addr, PeerASN: 65000 + uint32(owner), RemoteRouterID: 0xc0000200 + uint32(owner)}
	t.Cleanup(peerRIB.Release)
	return peerRIB
}

// framedNLRI returns nlri framed with an RFC 7911 Path Identifier, the way an
// ADD-PATH session carries it and the RIB stores it.
func framedNLRI(pathID uint32, nlri ...byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, pathID), nlri...)
}

// dumpRIBThroughMRT runs the MRT plugin's own TABLE_DUMP_V2 writer over r on
// the default configuration (a routes stream and nothing else), the way the
// routes interval and `request mrt dump-rib` do: the writer reaches r through
// the provider this plugin registers (RIBDumpBridge). It returns the peers of
// the PEER_INDEX_TABLE in index order and every RIB record as written.
//
// It publishes r as the active manager, so a test using it MUST NOT be
// parallel.
func dumpRIBThroughMRT(t *testing.T, r *RIBManager) ([]netip.Addr, []mrtDumpRecord) {
	t.Helper()
	activeManager.Store(r)
	t.Cleanup(func() { activeManager.Store(nil) })

	path := filepath.Join(t.TempDir(), "rib.mrt")
	comp := mrtplugin.New(mrtplugin.Config{RoutesPath: path, RoutesInterval: time.Hour}, nil)
	comp.Start(nil)
	comp.DumpRIB()
	comp.Stop()

	var peers []netip.Addr
	var records []mrtDumpRecord
	err := mrtfmt.ReadFile(path, &mrtfmt.Handler{
		OnHeader: func(h mrtfmt.Header, _ uint32, data []byte) error {
			require.Equal(t, mrtfmt.TypeTableDumpV2, h.Type)
			if h.Subtype != mrtfmt.TDV2PeerIndexTable {
				records = append(records, mrtDumpRecord{subtype: h.Subtype, body: bytes.Clone(data)})
			}
			return nil
		},
		OnPeerIndex: func(_ mrtfmt.Header, table *mrtfmt.PeerIndexTable) error {
			for i := range table.Peers {
				addr, ok := netip.AddrFromSlice(table.Peers[i].IP)
				require.True(t, ok, "peer %d address %x", i, table.Peers[i].IP)
				peers = append(peers, addr.Unmap())
			}
			return nil
		},
	})
	require.NoError(t, err)
	return peers, records
}

// prefixRecord decodes an AFI/SAFI-specific RIB record holding one entry and
// requires its body to be exactly the layout its subtype declares: seq(4),
// prefix length(1), prefix, entry count(2), peer index(2), originated time(4),
// the Path Identifier(4) under an ADD-PATH subtype only, attribute length(2)
// and the attributes.
func prefixRecord(t *testing.T, rec mrtDumpRecord) *mrtfmt.RIBRecord {
	t.Helper()
	decoded, err := mrtfmt.DecodeRIBRecord(rec.subtype, rec.body)
	require.NoError(t, err)
	require.Len(t, decoded.Entries, 1)
	want := 4 + 1 + len(decoded.Prefix) + 2 + 2 + 4 + 2 + len(decoded.Entries[0].Attributes)
	if mrtfmt.IsAddPathRIBSubtype(rec.subtype) {
		want += 4
	}
	require.Len(t, rec.body, want, "subtype %d body does not have the layout the subtype declares", rec.subtype)
	return decoded
}

// requireGenericRecord requires a RIB_GENERIC or RIB_GENERIC_ADDPATH record of
// IPv4 FlowSpec whose NLRI field is exactly wantNLRI and whose one RIB Entry
// is the RFC 6396 one, with no Path Identifier field: entry count(2), peer
// index(2), originated time(4), attribute length(2), attributes.
func requireGenericRecord(t *testing.T, rec mrtDumpRecord, wantNLRI []byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(rec.body), 7+len(wantNLRI)+10)
	require.Equal(t, []byte{0x00, 0x01, 133}, rec.body[4:7], "AFI IPv4, SAFI FlowSpec")
	require.Equal(t, wantNLRI, rec.body[7:7+len(wantNLRI)])
	entries := rec.body[7+len(wantNLRI):]
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(entries[0:2]), "entry count")
	attrLen := int(binary.BigEndian.Uint16(entries[8:10]))
	require.Len(t, entries, 2+2+4+2+attrLen, "the RIB Entry holds no Path Identifier field")
}

// TestRFC8050MRTDumpDefaultConfigKeepsPathIDs proves that on the default MRT
// configuration a peer's family stored with ADD-PATH is dumped through the
// real path (RIB walk into the MRT writer) under RIB_IPV4_UNICAST_ADDPATH, two
// paths of one prefix as two records with their distinct Path Identifiers.
//
// RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store
// the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH,
// RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and
// RIB_IPV6_MULTICAST_ADDPATH subtypes."
//
// RFC requirement: RFC8050-4.1-1 positive -- on the default configuration two
// ADD-PATH paths (7 and 42) of 203.0.113.0/24 from one peer are written as two
// TDV2RIBIPv4UnicastAP (8) records whose RIB Entries carry Path Identifiers 7
// and 42, each body exactly the ADD-PATH layout.
func TestRFC8050MRTDumpDefaultConfigKeepsPathIDs(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.SetAddPath(family.IPv4Unicast, true)
	peerRIB.Insert(family.IPv4Unicast, mrtDumpAttrs, framedNLRI(7, 24, 203, 0, 113))
	peerRIB.Insert(family.IPv4Unicast, mrtDumpAttrs, framedNLRI(42, 24, 203, 0, 113))

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 2)
	pathIDs := map[uint32]bool{}
	for _, rec := range records {
		require.Equal(t, mrtfmt.TDV2RIBIPv4UnicastAP, rec.subtype)
		decoded := prefixRecord(t, rec)
		require.Equal(t, uint8(24), decoded.PrefixLength)
		require.Equal(t, []byte{203, 0, 113}, decoded.Prefix)
		pathIDs[decoded.Entries[0].PathID] = true
	}
	require.Equal(t, map[uint32]bool{7: true, 42: true}, pathIDs)
}

// TestRFC8050MRTDumpGenericWithoutAddPath proves that a FlowSpec route of a
// peer whose FlowSpec family has no ADD-PATH is dumped through the real path
// as RIB_GENERIC (6) whose NLRI is the route alone, with no Path Identifier
// before it or in the RIB Entry.
//
// RFC 8050 Section 4.2: "These fields continue to encapsulate the raw and additional-path-
// enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries."
//
// RFC requirement: RFC8050-4.2-1 negative -- an IPv4 FlowSpec route stored
// without ADD-PATH is written as TDV2RIBGeneric (6) whose NLRI field is
// exactly the 6-octet FlowSpec NLRI and whose RIB Entry has no Path Identifier.
func TestRFC8050MRTDumpGenericWithoutAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.Insert(mrtDumpFlowSpecFamily, mrtDumpAttrs, mrtDumpFlowSpec)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	require.Equal(t, mrtfmt.TDV2RIBGeneric, records[0].subtype)
	requireGenericRecord(t, records[0], mrtDumpFlowSpec)
}

// TestRFC8050MRTDumpGenericWithAddPath proves that a FlowSpec route of a peer
// whose FlowSpec family has ADD-PATH is dumped through the real path as
// RIB_GENERIC_ADDPATH (12) whose NLRI carries the Path Identifier ahead of the
// route, while its RIB Entry stays the RFC 6396 one.
//
// RFC 8050 Section 4.2: "These fields continue to encapsulate the raw and additional-path-
// enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries."
// RFC 8050 Section 4.2: "For clarity, the RIB Entries in this subtype are not
// redefined."
//
// RFC requirement: RFC8050-4.2-1 positive -- an IPv4 FlowSpec route stored
// with ADD-PATH under Path Identifier 9 is written as TDV2RIBGenericAP (12)
// whose NLRI field is exactly 00 00 00 09 followed by the FlowSpec NLRI and
// whose RIB Entry has no Path Identifier field.
func TestRFC8050MRTDumpGenericWithAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.SetAddPath(mrtDumpFlowSpecFamily, true)
	peerRIB.Insert(mrtDumpFlowSpecFamily, mrtDumpAttrs, framedNLRI(9, mrtDumpFlowSpec...))

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	require.Equal(t, mrtfmt.TDV2RIBGenericAP, records[0].subtype)
	requireGenericRecord(t, records[0], framedNLRI(9, mrtDumpFlowSpec...))
}

// TestRFC8050MRTDumpMixedAddPathState proves that one dump holding families of
// both ADD-PATH states writes every record under the subtype of the route it
// holds: peer A has ADD-PATH for IPv4 unicast and none for IPv6 unicast, and
// peer B holds the same IPv4 prefix without ADD-PATH. Each record holds one
// RIB entry, so two peers of one prefix that differ in ADD-PATH state never
// share a record whose subtype one of them contradicts.
//
// RFC 8050 Section 2: "In order to support Path Identifier information as
// defined in [RFC7911], new subtypes need to be added."
//
// RFC requirement: RFC8050-4.1-1 negative -- in a dump beside an ADD-PATH
// family, IPv6 unicast of the same peer and IPv4 unicast of a peer without
// ADD-PATH are written under the base subtypes TDV2RIBIPv6Unicast (4) and
// TDV2RIBIPv4Unicast (2) at the base layout, while the ADD-PATH peer's IPv4
// paths are TDV2RIBIPv4UnicastAP (8) with Path Identifiers 7 and 42.
func TestRFC8050MRTDumpMixedAddPathState(t *testing.T) {
	r := newTestRIBManager(t)
	peerA := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerA.SetAddPath(family.IPv4Unicast, true)
	peerA.Insert(family.IPv4Unicast, mrtDumpAttrs, framedNLRI(7, 24, 203, 0, 113))
	peerA.Insert(family.IPv4Unicast, mrtDumpAttrs, framedNLRI(42, 24, 203, 0, 113))
	peerA.Insert(family.IPv6Unicast, mrtDumpAttrs, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})
	peerB := addMRTDumpPeer(t, r, 20, "192.0.2.20")
	peerB.Insert(family.IPv4Unicast, mrtDumpAttrs, []byte{24, 203, 0, 113})

	peers, records := dumpRIBThroughMRT(t, r)

	addressA := netip.MustParseAddr("192.0.2.10")
	addressB := netip.MustParseAddr("192.0.2.20")
	require.Len(t, records, 4)
	pathIDsA := map[uint32]bool{}
	var ipv6A, ipv4B int
	for _, rec := range records {
		decoded := prefixRecord(t, rec)
		index := int(decoded.Entries[0].PeerIndex)
		require.Less(t, index, len(peers))
		peer := peers[index]
		switch {
		case peer == addressA && rec.subtype == mrtfmt.TDV2RIBIPv4UnicastAP:
			require.Equal(t, []byte{203, 0, 113}, decoded.Prefix)
			pathIDsA[decoded.Entries[0].PathID] = true
		case peer == addressA && rec.subtype == mrtfmt.TDV2RIBIPv6Unicast:
			require.Equal(t, uint8(48), decoded.PrefixLength)
			ipv6A++
		case peer == addressB && rec.subtype == mrtfmt.TDV2RIBIPv4Unicast:
			require.Equal(t, []byte{203, 0, 113}, decoded.Prefix)
			ipv4B++
		default:
			t.Fatalf("peer %s written under subtype %d, which its route's ADD-PATH state contradicts", peer, rec.subtype)
		}
	}
	require.Equal(t, map[uint32]bool{7: true, 42: true}, pathIDsA)
	require.Equal(t, 1, ipv6A)
	require.Equal(t, 1, ipv4B)
}

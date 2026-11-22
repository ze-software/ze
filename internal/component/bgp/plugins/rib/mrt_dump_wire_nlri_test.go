// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2, the RIB_GENERIC NLRI
// Related: rib_mrt.go -- dumpRIBForMRT
// Related: storage/familyrib_wire.go -- FamilyRIB.IterateWire, the walk these tests reach through the dump
// Related: rfc8050_mrt_dump_labeled_test.go -- the one-label records, and the helpers used here

package rib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestMRTDumpLabeledUnicastLabelStack dumps a labeled unicast route that binds
// a two-label stack through the real path.
//
// VALIDATES: RIB_GENERIC carries both label entries in stack order, the
// bottom-of-stack bit on the last alone, and a Length that counts both.
// PREVENTS: a stack cut to its first label, or a Length that counts one.
func TestMRTDumpLabeledUnicastLabelStack(t *testing.T) {
	// RFC 8277 Section 2.3: Length 72 = 2 x 24 label bits + 24 prefix bits,
	// label 100 with S clear, label 200 with S set, prefix 203.0.113.
	wire := []byte{0x48, 0x00, 0x06, 0x40, 0x00, 0x0c, 0x81, 203, 0, 113}
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, wire, false)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGeneric, labeledFamily,
		[]byte{0x48, 0x00, 0x06, 0x40, 0x00, 0x0c, 0x81, 203, 0, 113})
}

// TestMRTDumpLabeledUnicastAddPathKeepsEachPathsLabels dumps two ADD-PATH
// paths of one labeled prefix that bind different labels.
//
// VALIDATES: each RIB_GENERIC_ADDPATH record carries its own path's Path
// Identifier and its own path's label.
// PREVENTS: one path's label written for every path of the prefix.
func TestMRTDumpLabeledUnicastAddPathKeepsEachPathsLabels(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.SetAddPath(labeledFamily, true)
	// Path 7 binds label 100, path 42 binds label 200, both to 203.0.113.0/24.
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, framedNLRI(7, 0x30, 0x00, 0x06, 0x41, 203, 0, 113), true)
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, framedNLRI(42, 0x30, 0x00, 0x0c, 0x81, 203, 0, 113), true)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 2)
	wantByPathID := map[byte][]byte{
		7:  {0, 0, 0, 7, 0x30, 0x00, 0x06, 0x41, 203, 0, 113},
		42: {0, 0, 0, 42, 0x30, 0x00, 0x0c, 0x81, 203, 0, 113},
	}
	seen := map[byte]bool{}
	for _, rec := range records {
		require.Greater(t, len(rec.body), 10)
		pathID := rec.body[10]
		want, known := wantByPathID[pathID]
		require.True(t, known, "record names Path Identifier %d, which no stored path holds", pathID)
		requireGenericNLRI(t, rec, mrtfmt.TDV2RIBGenericAP, labeledFamily, want)
		seen[pathID] = true
	}
	require.Equal(t, map[byte]bool{7: true, 42: true}, seen)
}

// TestMRTDumpLabeledUnicastIPv6 dumps an IPv6 labeled unicast route.
//
// VALIDATES: RIB_GENERIC of AFI 2, SAFI 4 carries the label and the IPv6
// prefix octets its Length counts.
// PREVENTS: an IPv6 prefix cut or padded by a buffer sized for IPv4.
func TestMRTDumpLabeledUnicastIPv6(t *testing.T) {
	labeledIPv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}
	// Length 72 = 24 label bits + 48 prefix bits, label 100, 2001:db8:1::/48.
	wire := []byte{0x48, 0x00, 0x06, 0x41, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	ingestLabeledForDump(t, r, peerRIB, labeledIPv6, wire, false)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGeneric, labeledIPv6,
		[]byte{0x48, 0x00, 0x06, 0x41, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01})
}

// TestMRTDumpUnicastBesideLabeledIsUnchanged dumps a peer that holds one
// prefix as IPv4 unicast and as labeled unicast.
//
// VALIDATES: the unicast route is still RIB_IPV4_UNICAST (2) with its prefix
// length and prefix split in the header, at exactly that layout.
// PREVENTS: the wire walk changing what a family with no side-data writes.
func TestMRTDumpUnicastBesideLabeledIsUnchanged(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.Insert(family.IPv4Unicast, mrtDumpAttrs, []byte{24, 203, 0, 113})
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, mrtDumpLabeledOne, false)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 2)
	unicast := 0
	for _, rec := range records {
		if rec.subtype != mrtfmt.TDV2RIBIPv4Unicast {
			continue
		}
		decoded := prefixRecord(t, rec)
		require.Equal(t, uint8(24), decoded.PrefixLength)
		require.Equal(t, []byte{203, 0, 113}, decoded.Prefix)
		unicast++
	}
	require.Equal(t, 1, unicast)
}

// TestMRTDumpVPNKeepsLabelAndRouteDistinguisher dumps a VPN-IPv4 route, whose
// stored route key drops the label.
//
// VALIDATES: RIB_GENERIC carries the NLRI the route was received with: label,
// Route Distinguisher and prefix.
// PREVENTS: the route key written in its place, which has no label field.
func TestMRTDumpVPNKeepsLabelAndRouteDistinguisher(t *testing.T) {
	wire := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.Insert(vpnv4Family, mrtDumpAttrs, wire)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	// Length 96 = 24 label bits + 64 RD bits + 8 prefix bits, label 100 with S
	// set, RD 0:1, prefix 10.
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGeneric, vpnv4Family,
		[]byte{0x60, 0x00, 0x06, 0x41, 0, 0, 0, 0, 0, 0, 0, 1, 0x0a})
}

// TestMRTDumpVPNAddPathKeepsLabelAndRouteDistinguisher dumps a VPN-IPv4 route
// received with ADD-PATH.
//
// VALIDATES: RIB_GENERIC_ADDPATH carries the Path Identifier, then the NLRI
// the route was received with: label, Route Distinguisher and prefix.
// PREVENTS: the Path Identifier followed by the label-less route key.
func TestMRTDumpVPNAddPathKeepsLabelAndRouteDistinguisher(t *testing.T) {
	wire := vpnv4NLRI(100, vpnRouteKeyRD, 0x0a)
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.SetAddPath(vpnv4Family, true)
	peerRIB.Insert(vpnv4Family, mrtDumpAttrs, framedNLRI(9, wire...))

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGenericAP, vpnv4Family,
		[]byte{0, 0, 0, 9, 0x60, 0x00, 0x06, 0x41, 0, 0, 0, 0, 0, 0, 0, 1, 0x0a})
}

// TestMRTDumpLabeledUnicastWithoutLabelStackIsLeftOut dumps a labeled unicast
// route stored with no label stack bound, the state between the two steps of
// its ingest (insertLabeledEntry stores the route, then binds the labels).
//
// VALIDATES: the snapshot leaves the route out, and still writes the route
// beside it that has its label stack.
// PREVENTS: a RIB_GENERIC record of SAFI 4 holding a bare prefix, which is not
// a labeled unicast NLRI and which a parser reads as label bits.
func TestMRTDumpLabeledUnicastWithoutLabelStackIsLeftOut(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.Insert(labeledFamily, mrtDumpAttrs, []byte{24, 198, 51, 100})
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, mrtDumpLabeledOne, false)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGeneric, labeledFamily, mrtDumpLabeledOne)
}

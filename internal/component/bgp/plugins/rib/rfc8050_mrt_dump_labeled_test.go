// Design: docs/architecture/mrt.md -- RIB snapshot bridge for MRT TABLE_DUMP_V2, the RIB_GENERIC NLRI
// RFC: rfc/short/rfc8050.md -- Section 4.2, the raw NLRI of RIB_GENERIC and RIB_GENERIC_ADDPATH
// Related: rib_mrt.go -- dumpRIBForMRT
// Related: storage/familyrib_wire.go -- FamilyRIB.IterateWire, the walk that rebuilds a labeled NLRI
// Related: rfc8050_mrt_dump_path_test.go -- dumpRIBThroughMRT, the real path these tests drive

package rib

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// mrtDumpLabeledOne is the labeled unicast NLRI (RFC 8277 Section 2.2) that
// binds label 100 to 203.0.113.0/24, as an UPDATE carries it:
//
//	0x30        Length: 48 bits, 24 of label entry and 24 of prefix
//	0x00 06 41  Label 100 (20 bits), Rsrv 0, S 1
//	cb 00 71    Prefix 203.0.113
var mrtDumpLabeledOne = []byte{0x30, 0x00, 0x06, 0x41, 203, 0, 113}

// ingestLabeledForDump stores one labeled unicast announcement the way a
// received UPDATE is stored: the ingest splits wire into the prefix the route
// is keyed by and the label stack held beside it. wire is framed with its Path
// Identifier when addPath is set.
func ingestLabeledForDump(t *testing.T, r *RIBManager, peerRIB *storage.PeerRIB, fam family.Family, wire []byte, addPath bool) {
	t.Helper()
	var affected []affectedPrefix
	r.insertLabeled(peerRIB, fam, mrtDumpAttrs, wire, addPath, false, 1, 0, 0, &affected, nil)
	require.Len(t, affected, 1, "the ingest stored the labeled route with its label stack")
}

// requireGenericNLRI requires rec to be a record of subtype for fam whose NLRI
// field is exactly wantNLRI, at exactly that length: the one RIB Entry starts
// right after it and is the RFC 6396 one, with no Path Identifier field: entry
// count(2), peer index(2), originated time(4), attribute length(2), attributes.
func requireGenericNLRI(t *testing.T, rec mrtDumpRecord, subtype uint16, fam family.Family, wantNLRI []byte) {
	t.Helper()
	require.Equal(t, subtype, rec.subtype)
	require.GreaterOrEqual(t, len(rec.body), 7+len(wantNLRI)+10)
	require.Equal(t, []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}, rec.body[4:7], "AFI and SAFI")
	require.Equal(t, wantNLRI, rec.body[7:7+len(wantNLRI)], "the NLRI field is the NLRI an UPDATE carries")
	entries := rec.body[7+len(wantNLRI):]
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(entries[0:2]), "entry count right after the NLRI")
	attrLen := int(binary.BigEndian.Uint16(entries[8:10]))
	require.Len(t, entries, 2+2+4+2+attrLen, "the RIB Entry holds no Path Identifier field")
}

// TestRFC8050MRTDumpLabeledGenericWithoutAddPath proves that a labeled unicast
// route received without ADD-PATH is dumped through the real path (ingest,
// RIB walk, MRT writer) as RIB_GENERIC (6) whose NLRI is the labeled NLRI the
// UPDATE carried, label stack included, with no Path Identifier.
//
// VALIDATES: the record's NLRI bytes equal the wire NLRI of the stored route.
// PREVENTS: the stored key written in its place, a bare prefix 18 cb 00 71
// that a labeled unicast parser reads as a 24-bit label field and no prefix.
//
// RFC 6396 Section 4.3.3: "It consists of an AFI, Subsequent AFI (SAFI), and a
// single NLRI entry.  The NLRI information is specific to the AFI and SAFI values."
// RFC 8050 Section 4.2: "These fields continue to encapsulate the raw and additional-path-
// enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries."
//
// RFC requirement: RFC8050-4.2-1 negative -- an IPv4 labeled unicast route
// (label 100, 203.0.113.0/24) stored without ADD-PATH is written as
// TDV2RIBGeneric (6) whose NLRI field is exactly 30 00 06 41 cb 00 71 and
// whose RIB Entry has no Path Identifier field.
func TestRFC8050MRTDumpLabeledGenericWithoutAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, mrtDumpLabeledOne, false)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGeneric, labeledFamily, []byte{0x30, 0x00, 0x06, 0x41, 203, 0, 113})
}

// TestRFC8050MRTDumpLabeledGenericWithAddPath proves that a labeled unicast
// route received with ADD-PATH is dumped through the real path as
// RIB_GENERIC_ADDPATH (12) whose NLRI is the Path Identifier followed by the
// labeled NLRI the UPDATE carried, while its RIB Entry stays the RFC 6396 one.
//
// VALIDATES: the record's NLRI bytes equal the ADD-PATH wire NLRI of the route.
// PREVENTS: the Path Identifier followed by the bare stored prefix.
//
// RFC 8050 Section 4.2: "These fields continue to encapsulate the raw and additional-path-
// enabled AFI/SAFI/NLRI in the record, and the raw attributes in the RIB Entries."
// RFC 8277 Section 2.2: "If the procedures of [RFC7911] are being used, a
// four-octet "path identifier" (as defined in Section 3 of [RFC7911]) is part
// of the NLRI and precedes the Length field."
//
// RFC requirement: RFC8050-4.2-1 positive -- an IPv4 labeled unicast route
// (label 100, 203.0.113.0/24) stored with ADD-PATH under Path Identifier 9 is
// written as TDV2RIBGenericAP (12) whose NLRI field is exactly 00 00 00 09
// 30 00 06 41 cb 00 71 and whose RIB Entry has no Path Identifier field.
func TestRFC8050MRTDumpLabeledGenericWithAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peerRIB := addMRTDumpPeer(t, r, 10, "192.0.2.10")
	peerRIB.SetAddPath(labeledFamily, true)
	ingestLabeledForDump(t, r, peerRIB, labeledFamily, framedNLRI(9, mrtDumpLabeledOne...), true)

	_, records := dumpRIBThroughMRT(t, r)

	require.Len(t, records, 1)
	requireGenericNLRI(t, records[0], mrtfmt.TDV2RIBGenericAP, labeledFamily,
		[]byte{0, 0, 0, 9, 0x30, 0x00, 0x06, 0x41, 203, 0, 113})
}

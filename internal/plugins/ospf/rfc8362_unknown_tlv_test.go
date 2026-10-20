// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPFv3 Extended LSA export.
//
// VALIDATES: RFC 8362 Section 6.3 rule 1, an unrecognized top-level TLV in an Extended LSA
// is ignored by both readers of received Extended LSAs: the BGP-LS exporter and the
// Prefix-SID reception that feeds segment-routing installation.
// PREVENTS: an unknown TLV being read as a known one, or making the whole LSA unusable.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// rfc8362UnknownTLVType is a top-level Extended-LSA TLV type no RFC Ze implements defines.
const rfc8362UnknownTLVType uint16 = 65001

// rfc8362PrefixSIDValue returns rfc8362PrefixValue(word) followed by one Prefix-SID
// sub-TLV (RFC 8666 Section 6) carrying index.
func rfc8362PrefixSIDValue(word byte, index uint32) []byte {
	sid := sr.EncodePrefixSIDValueV6(sr.PrefixSID{Flags: sr.SIDFlags{NP: true}, Index: index})
	return v3packet.AppendSubTLVs(rfc8362PrefixValue(word), []v3packet.ExtendedTLV{{Type: sr.V6TypePrefixSID, Value: sid}})
}

// rfc8362UnknownBesideKnown returns an E-Inter-Area-Prefix-LSA body whose first TLV is
// unrecognized and carries a well-formed prefix body for 2001:db8:0:1::/64 (Prefix-SID
// index 21), and whose second TLV is the Inter-Area-Prefix TLV for 2001:db8::/64 (index 20).
// The unknown TLV is shaped like a known one, so reading it as known would be visible.
func rfc8362UnknownBesideKnown() []byte {
	return v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: []v3packet.ExtendedTLV{
		{Type: rfc8362UnknownTLVType, Value: rfc8362PrefixSIDValue(1, 21)},
		{Type: extTLVInterAreaPrefix, Value: rfc8362PrefixSIDValue(0, 20)},
	}})
}

// rfc8362ExportedUnknownBesideKnown installs rfc8362UnknownBesideKnown from 2.2.2.2 and
// returns the first BGP-LS snapshot that carries a prefix.
func rfc8362ExportedUnknownBesideKnown(t *testing.T) linkstateevents.Snapshot {
	t.Helper()
	e, _, events := bgplsTestSource(t, true)
	rfc8362Install(t, e, types.RouterID{2, 2, 2, 2}, v3types.LSTypeEInterAreaPrefix, 1, rfc8362UnknownBesideKnown())
	return bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(s.Prefixes) != 0 })
}

// rfc8362ReceivedUnknownBesideKnown installs rfc8362UnknownBesideKnown from 9.9.9.9 and
// returns every Prefix-SID the reception path reads from the LSDB.
func rfc8362ReceivedUnknownBesideKnown(t *testing.T) []v6ReceivedPrefixSID {
	t.Helper()
	e, _, _ := bgplsTestSource(t, true)
	rfc8362Install(t, e, types.RouterID{9, 9, 9, 9}, v3types.LSTypeEInterAreaPrefix, 1, rfc8362UnknownBesideKnown())
	return e.v6ReceivedPrefixSIDs()
}

// TestRFC8362UnknownTLVKeepsLSAForBGPLS proves the positive half of Section 6.3 rule 1 at
// the BGP-LS exporter.
//
// Goal: an unrecognized TLV does not make the Extended LSA malformed or unread.
// Method: install an E-Inter-Area-Prefix-LSA whose unrecognized TLV comes before its
// Inter-Area-Prefix TLV, and read the BGP-LS snapshot. The known TLV is exported as
// 2001:db8::/64, inter-area route type 2.
//
// RFC requirement: RFC8362-6.3-1 positive -- an E-Inter-Area-Prefix-LSA whose first
// top-level TLV is unrecognized still has its Inter-Area-Prefix TLV exported through BGP-LS
// as 2001:db8::/64 with route type 2 (inter-area).
func TestRFC8362UnknownTLVKeepsLSAForBGPLS(t *testing.T) {
	snapshot := rfc8362ExportedUnknownBesideKnown(t)
	known := netip.MustParsePrefix("2001:db8::/64")
	for i := range snapshot.Prefixes {
		if snapshot.Prefixes[i].Prefix == known && snapshot.Prefixes[i].RouteType == 2 {
			return
		}
	}
	t.Fatalf("prefixes = %+v, want the Inter-Area-Prefix TLV's %v with route type 2", snapshot.Prefixes, known)
}

// TestRFC8362UnknownTLVIgnoredByBGPLS proves the negative half of Section 6.3 rule 1 at the
// BGP-LS exporter.
//
// Goal: an unrecognized TLV contributes nothing, even when its value has a known TLV's shape.
// Method: the same LSA as the positive test. The snapshot carries exactly one prefix, the
// known TLV's, and no link: 2001:db8:0:1::/64 from the unknown TLV is absent.
//
// RFC requirement: RFC8362-6.3-1 negative -- an unrecognized top-level TLV whose value is a
// well-formed prefix body for 2001:db8:0:1::/64 exports nothing through BGP-LS: the
// snapshot holds exactly one prefix, the Inter-Area-Prefix TLV's 2001:db8::/64, and no link.
func TestRFC8362UnknownTLVIgnoredByBGPLS(t *testing.T) {
	snapshot := rfc8362ExportedUnknownBesideKnown(t)
	if len(snapshot.Prefixes) != 1 || snapshot.Prefixes[0].Prefix != netip.MustParsePrefix("2001:db8::/64") {
		t.Fatalf("prefixes = %+v, want only the Inter-Area-Prefix TLV's 2001:db8::/64", snapshot.Prefixes)
	}
	if len(snapshot.Links) != 0 {
		t.Fatalf("an unrecognized TLV produced links: %+v", snapshot.Links)
	}
}

// TestRFC8362UnknownTLVKeepsLSAForPrefixSIDs proves the positive half of Section 6.3 rule 1
// at the Prefix-SID reception path.
//
// Goal: an unrecognized TLV does not make the Extended LSA malformed for SR reception.
// Method: install the LSA in the LSDB and read v6ReceivedPrefixSIDs, which walks every
// Extended prefix LSA there. The known TLV's Prefix-SID is read: 2001:db8::/64, index 20,
// originator 9.9.9.9, from the E-Inter-Area-Prefix-LSA.
//
// RFC requirement: RFC8362-6.3-1 positive -- an E-Inter-Area-Prefix-LSA whose first
// top-level TLV is unrecognized still yields its Inter-Area-Prefix TLV's Prefix-SID from the
// LSDB: prefix 2001:db8::/64, index 20, originator 9.9.9.9.
func TestRFC8362UnknownTLVKeepsLSAForPrefixSIDs(t *testing.T) {
	known := netip.MustParsePrefix("2001:db8::/64")
	for _, r := range rfc8362ReceivedUnknownBesideKnown(t) {
		if r.Prefix == known && r.SID.Index == 20 && r.Originator == (types.RouterID{9, 9, 9, 9}) && r.LSType == v3types.LSTypeEInterAreaPrefix {
			return
		}
	}
	t.Fatalf("the Inter-Area-Prefix TLV's Prefix-SID for %v index 20 was not read", known)
}

// TestRFC8362UnknownTLVIgnoredByPrefixSIDs proves the negative half of Section 6.3 rule 1 at
// the Prefix-SID reception path.
//
// Goal: an unrecognized TLV yields no Prefix-SID, even when its value carries one.
// Method: the same LSA as the positive test. Exactly one Prefix-SID is read, the known
// TLV's; the unknown TLV's 2001:db8:0:1::/64 index 21 is absent.
//
// RFC requirement: RFC8362-6.3-1 negative -- an unrecognized top-level TLV whose value
// carries a well-formed prefix body and Prefix-SID sub-TLV (2001:db8:0:1::/64, index 21)
// yields no Prefix-SID: exactly one is read from the LSA, the Inter-Area-Prefix TLV's index 20.
func TestRFC8362UnknownTLVIgnoredByPrefixSIDs(t *testing.T) {
	got := rfc8362ReceivedUnknownBesideKnown(t)
	if len(got) != 1 || got[0].Prefix != netip.MustParsePrefix("2001:db8::/64") || got[0].SID.Index != 20 {
		t.Fatalf("received Prefix-SIDs = %+v, want only the Inter-Area-Prefix TLV's index 20", got)
	}
}

package ospf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	v3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	v3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// VALIDATES: RFC 8666 Section 10, an Extended-LSA carrying an Adj-SID, LAN Adj-SID or
// SID/Label sub-TLV of invalid length is ignored by the reader that consumes it.
// PREVENTS: a reader using part of an LSA the RFC declares malformed.

// rfc8666ERouterBody returns an E-Router-LSA body (RFC 8362 Section 3.2: 4 octets of flags
// and options, then TLVs) with one point-to-point Router-Link TLV to neighbor, carrying the
// given encoded sub-TLVs after the 16-octet link layout.
func rfc8666ERouterBody(neighbor types.RouterID, subs ...[]byte) []byte {
	value := make([]byte, 16)
	value[0] = v3packet.RouterLinkTypeP2P
	binary.BigEndian.PutUint16(value[2:4], 10)
	binary.BigEndian.PutUint32(value[4:8], 7)
	binary.BigEndian.PutUint32(value[8:12], 8)
	copy(value[12:16], neighbor[:])
	for _, s := range subs {
		value = append(value, s...)
	}
	ext := v3packet.EncodeExtendedLSABody(v3packet.ExtendedLSA{TLVs: []v3packet.ExtendedTLV{{Type: extTLVRouterLink, Value: value}}})
	return append(make([]byte, 4), ext...)
}

// rfc8666ExportedLinks installs 2.2.2.2's E-Router-LSA carrying subs, then a sentinel
// E-Router-LSA from 3.3.3.3, waits for the BGP-LS snapshot that holds the sentinel's link,
// and returns the links that snapshot exports from 2.2.2.2.
func rfc8666ExportedLinks(t *testing.T, subs ...[]byte) []linkstateevents.Link {
	t.Helper()
	e, _, events := bgplsTestSource(t, true)
	lsa := bgplsNativeV3(t, v3types.RouterID{2, 2, 2, 2}, v3types.LSTypeERouter, 0,
		rfc8666ERouterBody(types.RouterID{4, 4, 4, 4}, subs...), types.InitialSequenceNumber, false)
	if !e.lsdb.Install(types.BackboneArea, lsa) {
		t.Fatal("install 2.2.2.2 E-Router-LSA")
	}
	sentinel := bgplsNativeV3(t, v3types.RouterID{3, 3, 3, 3}, v3types.LSTypeERouter, 0,
		rfc8666ERouterBody(types.RouterID{4, 4, 4, 4}), types.InitialSequenceNumber, false)
	if !e.lsdb.Install(types.BackboneArea, sentinel) {
		t.Fatal("install 3.3.3.3 E-Router-LSA")
	}
	from := func(s *linkstateevents.Snapshot, router []byte) []linkstateevents.Link {
		var out []linkstateevents.Link
		for i := range s.Links {
			if bytes.Equal(s.Links[i].Local.RouterID, router) {
				out = append(out, s.Links[i])
			}
		}
		return out
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool { return len(from(s, []byte{3, 3, 3, 3})) != 0 })
	return from(&snapshot, []byte{2, 2, 2, 2})
}

// TestRFC8666AdjSIDLengthInvalidIgnoredByReader proves the OSPFv3 Adj-SID reader (BGP-LS
// export of E-Router-LSA links) ignores an LSA whose Adj-SID or LAN Adj-SID has an invalid
// length. RFC 8666 Section 10: "For any new TLVs/sub-TLVs defined in this document, if the
// length is invalid, the LSA in which it is advertised is considered malformed and MUST be
// ignored."
//
// Goal: the clause holds at a reader, not only at the srV3ExtendedLengthInvalid predicate.
// Method: a real E-Router-LSA from 2.2.2.2 is installed in the LSDB and exported through
// BGP-LS. A well-formed Adj-SID or LAN Adj-SID reaches the exported link as its attribute
// (1099, 1100); each invalid length leaves the LSA wholly unused, so no link is exported
// from 2.2.2.2 at all, while a sentinel LSA from 3.3.3.3 proves the snapshot was built.
func TestRFC8666AdjSIDLengthInvalidIgnoredByReader(t *testing.T) {
	adj := sr.EncodeAdjSIDValueV6(sr.AdjSID{Index: 3})
	lan := sr.EncodeLANAdjSIDValueV6(sr.AdjSID{Index: 3, NeighborID: [4]byte{4, 4, 4, 4}})

	// RFC requirement: RFC8666-10-1 positive -- an E-Router-LSA whose Adj-SID or LAN Adj-SID
	// has a valid length is used by the reader: BGP-LS exports its link with the Adj-SID
	// (1099) or LAN Adj-SID (1100) attribute.
	for name, c := range map[string]struct {
		sub  []byte
		attr uint16
	}{
		"adj-sid":     {srTestTLVBytes(sr.V6TypeAdjSID, adj), 1099},
		"lan-adj-sid": {srTestTLVBytes(sr.V6TypeLANAdjSID, lan), 1100},
	} {
		links := rfc8666ExportedLinks(t, c.sub)
		if len(links) != 1 || bgplsTestAttribute(links[0].Attributes, c.attr) == nil {
			t.Fatalf("%s: well-formed E-Router-LSA not exported with attribute %d: %+v", name, c.attr, links)
		}
	}

	// RFC requirement: RFC8666-10-1 negative -- an E-Router-LSA whose Adj-SID or LAN Adj-SID
	// is one octet too long or too short is ignored whole by the reader: BGP-LS exports no
	// link from it.
	for name, sub := range map[string][]byte{
		"adj-sid too long":      srTestTLVBytes(sr.V6TypeAdjSID, append(append([]byte{}, adj...), 0)),
		"adj-sid too short":     srTestTLVBytes(sr.V6TypeAdjSID, adj[:len(adj)-2]),
		"lan-adj-sid too long":  srTestTLVBytes(sr.V6TypeLANAdjSID, append(append([]byte{}, lan...), 0)),
		"lan-adj-sid too short": srTestTLVBytes(sr.V6TypeLANAdjSID, lan[:len(lan)-2]),
	} {
		if links := rfc8666ExportedLinks(t, sub); len(links) != 0 {
			t.Fatalf("%s: a link from a malformed E-Router-LSA was exported: %+v", name, links)
		}
	}
}

// TestRFC8666SIDLabelLengthInvalidIgnoresLSA proves an Extended-LSA whose SID/Label sub-TLV
// (RFC 8666 Section 3.1: "Length: 3 or 4 octets") has another length is ignored. RFC 8666
// Section 10: "For any new TLVs/sub-TLVs defined in this document, if the length is invalid,
// the LSA in which it is advertised is considered malformed and MUST be ignored."
//
// Goal: the SID/Label sub-TLV is one of the sub-TLVs RFC 8666 defines (Section 9.2, type 7),
// so an invalid length condemns the LSA wherever it is advertised.
// Method: the IPv6 Prefix-SID reader (srRemotePrefixSIDsV6) over an E-Intra-Area-Prefix-LSA
// whose prefix TLV carries a well-formed Prefix-SID and a SID/Label sub-TLV.
func TestRFC8666SIDLabelLengthInvalidIgnoresLSA(t *testing.T) {
	r5 := types.RouterID{5, 5, 5, 5}
	a := netip.MustParsePrefix("2001:db8::5/128")
	good := v6TestSubTLV(v6TestPrefixSID(6, 0))

	// RFC requirement: RFC8666-10-1 positive -- a SID/Label sub-TLV of length 3 (label) or 4
	// (SID) leaves the LSA in use: its Prefix-SID is received.
	for _, length := range []int{3, 4} {
		eng := newV6RIEngine(t)
		installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, a, good, srTestTLVBytes(sr.V6TypeSIDLabel, make([]byte, length))))
		if sids := eng.srRemotePrefixSIDsV6(); sids[a].SID.Index != 6 {
			t.Fatalf("SID/Label length %d: the LSA must stay in use: %+v", length, sids)
		}
	}

	// RFC requirement: RFC8666-10-1 negative -- a SID/Label sub-TLV of length 2 or 5 makes the
	// LSA malformed: no Prefix-SID from it is received.
	for _, length := range []int{2, 5} {
		eng := newV6RIEngine(t)
		installRemoteV6ExtTLVs(t, eng, r5, v6TestPrefixTLV(t, a, good, srTestTLVBytes(sr.V6TypeSIDLabel, make([]byte, length))))
		if sids := eng.srRemotePrefixSIDsV6(); len(sids) != 0 {
			t.Fatalf("SID/Label length %d: a Prefix-SID from a malformed Extended-LSA was received: %+v", length, sids)
		}
	}
}

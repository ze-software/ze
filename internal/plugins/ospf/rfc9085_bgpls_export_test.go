// Design: docs/architecture/wire/nlri-bgpls.md -- native OSPF BGP-LS origination.
// RFC: rfc/short/rfc9085.md
// Related: bgpls_export.go -- routerInformation translates the RI SRGB and SRLB
// Related: bgpls_export.go -- bgplsAdjAttribute, bgplsPrefixAttribute, prefixRange translate the SIDs
//
// VALIDATES: the SR Capabilities TLV 1034 and the SR Local Block TLV 1036 Ze
// originates from an OSPFv2 Router Information LSA carry a Flags octet and a
// Reserved octet of 0, as RFC 9085 Sections 2.1.2 and 2.1.4 require for OSPF,
// whatever the native TLV carried in its own reserved octet. The SID TLVs
// 1099, 1100, 1158 and 1159 carry Reserved 0 (Sections 2.2.1, 2.2.2, 2.3.1,
// 2.3.5), and TLVs 1034 and 1036 sit only on the Node NLRI of the router that
// advertised them (Section 2.1).
// PREVENTS: a native OSPF reserved octet, or any source bit, reaching the
// Flags or Reserved octet a collector reads; an SR Capabilities TLV attached
// to another node, a link or a prefix.
package ospf

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc9085OriginatedRanges installs a Router Information LSA carrying one SRGB
// (base 16000, size 8000) and one SRLB (base 15000, size 1000), each with the
// given native reserved octet, and returns the TLV 1034 and TLV 1036 values Ze
// originates for the advertising router.
func rfc9085OriginatedRanges(t *testing.T, reserved byte) (srgb, srlb []byte) {
	t.Helper()
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install base router")
	}
	global := sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 8000})
	local := sr.EncodeRangeValue(sr.LabelRange{Base: 15000, Size: 1000})
	// RFC 8665 Sections 3.2/3.3: value octet 3 is the native Reserved octet.
	global[3] = reserved
	local[3] = reserved
	bgplsInstallOpaque(t, e, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRGB, Value: global}, {Type: sr.V4TypeSRLB, Value: local},
	}))
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	snapshot := bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		for i := range s.Nodes {
			if bgplsTestAttribute(s.Nodes[i].Attributes, 1036) != nil {
				return true
			}
		}
		return false
	})
	for i := range snapshot.Nodes {
		srlb = bgplsTestAttribute(snapshot.Nodes[i].Attributes, 1036)
		if srlb != nil {
			return bgplsTestAttribute(snapshot.Nodes[i].Attributes, 1034), srlb
		}
	}
	t.Fatal("no node carries TLV 1036")
	return nil, nil
}

// The expected wire values: Flags 0, Reserved 0, Range Size, then SID/Label
// sub-TLV 1161 of length 3 carrying the first label.
var (
	rfc9085OSPFSRGB = []byte{0, 0, 0x00, 0x1f, 0x40, 0x04, 0x89, 0, 3, 0x00, 0x3e, 0x80}
	rfc9085OSPFSRLB = []byte{0, 0, 0x00, 0x03, 0xe8, 0x04, 0x89, 0, 3, 0x00, 0x3a, 0x98}
)

// TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero originates an
// ordinary OSPF SRGB and SRLB and compares the exact TLV 1034 and 1036 values.
//
// RFC requirement: RFC9085-2.1.2-1 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-2 positive -- the SR Capabilities TLV 1034 Ze originates from an OSPF SRGB has Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-1 positive -- the SR Local Block TLV 1036 Ze originates from an OSPF SRLB has Flags octet 0 (§2.1.4).
// RFC requirement: RFC9085-2.1.4-2 positive -- the SR Local Block TLV 1036 Ze originates from an OSPF SRLB has Reserved octet 0 (§2.1.4).
func TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero(t *testing.T) {
	srgb, srlb := rfc9085OriginatedRanges(t, 0)
	if !bytes.Equal(srgb, rfc9085OSPFSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, rfc9085OSPFSRGB)
	}
	if !bytes.Equal(srlb, rfc9085OSPFSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, rfc9085OSPFSRLB)
	}
}

// TestRFC9085OSPFSourceReservedNeverOriginated hands the producer native
// ranges whose reserved octet is 0xff, the input that reaches the Flags or
// Reserved octet if the producer copies the native header.
//
// RFC requirement: RFC9085-2.1.2-1 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Flags octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.2-2 negative -- an OSPF SRGB whose native reserved octet is ff is originated as TLV 1034 with Reserved octet 0 (§2.1.2).
// RFC requirement: RFC9085-2.1.4-1 negative -- an OSPF SRLB whose native reserved octet is ff is originated as TLV 1036 with Flags octet 0 (§2.1.4).
// RFC requirement: RFC9085-2.1.4-2 negative -- an OSPF SRLB whose native reserved octet is ff is originated as TLV 1036 with Reserved octet 0 (§2.1.4).
func TestRFC9085OSPFSourceReservedNeverOriginated(t *testing.T) {
	srgb, srlb := rfc9085OriginatedRanges(t, 0xff)
	if !bytes.Equal(srgb, rfc9085OSPFSRGB) {
		t.Fatalf("TLV 1034 = %x, want %x", srgb, rfc9085OSPFSRGB)
	}
	if !bytes.Equal(srlb, rfc9085OSPFSRLB) {
		t.Fatalf("TLV 1036 = %x, want %x", srlb, rfc9085OSPFSRLB)
	}
}

// The advertising router of every scenario LSA, and its SR Capabilities node.
var rfc9085OSPFOriginator = []byte{2, 2, 2, 2}

// The two prefixes the scenario's Extended Prefix LSA describes.
var (
	rfc9085OSPFPrefix = netip.MustParsePrefix("192.0.2.0/24")
	rfc9085OSPFRange  = netip.MustParsePrefix("198.51.100.0/24")
)

// rfc9085OSPFScenario builds an OSPFv2 backbone of two routers. Router 2.2.2.2
// has a point-to-point link to 3.3.3.3 and advertises an RI LSA (SRGB 16000/8000,
// SRLB 15000/1000), an Extended Link LSA carrying an Adj-SID (label 24001,
// weight 5) and a LAN Adj-SID (neighbor 4.4.4.4, label 24002, weight 6) on that
// link, and an Extended Prefix LSA carrying a Prefix-SID (algorithm 1, index 77)
// on 192.0.2.0/24 and a Prefix Range of 16 (index 100) on 198.51.100.0/24.
// Router 3.3.3.3 advertises its Router LSA and nothing else. Every native
// Reserved field the scenario writes carries the given octet. It returns the
// first snapshot that holds the SR node, link and prefix attributes.
func rfc9085OSPFScenario(t *testing.T, reserved byte) linkstateevents.Snapshot {
	t.Helper()
	e, bus, events := bgplsTestSource(t, false)
	if !e.lsdb.Install(types.BackboneArea, bgplsTestRouter(10, types.InitialSequenceNumber)) {
		t.Fatal("install router 2.2.2.2")
	}
	peer := bgplsReachabilityRouter(types.RouterID{3, 3, 3, 3}, packet.RouterLink{
		Type: packet.RouterLinkTypeP2P, LinkID: types.LinkStateID{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 3}, Metric: 10,
	})
	if !e.lsdb.Install(types.BackboneArea, peer) {
		t.Fatal("install router 3.3.3.3")
	}
	global := sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 8000})
	local := sr.EncodeRangeValue(sr.LabelRange{Base: 15000, Size: 1000})
	global[3], local[3] = reserved, reserved
	bgplsInstallOpaque(t, e, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRGB, Value: global}, {Type: sr.V4TypeSRLB, Value: local},
	}))
	// RFC 8665 Sections 6.1 and 6.2: Flags (V and L), Reserved, MT-ID, Weight,
	// the LAN form's Neighbor ID, then a 3-octet label.
	adjacency := []byte{0x60, reserved, 0, 5, 0x00, 0x5d, 0xc1}
	lan := []byte{0x60, reserved, 0, 6, 4, 4, 4, 4, 0x00, 0x5d, 0xc2}
	// Link Type 1 is point-to-point (RFC 7684 Section 3.1).
	bgplsInstallOpaque(t, e, packet.ExtLinkOpaqueType, packet.EncodeExtLinkLSA(packet.ExtLinkTLV{
		LinkType: 1, LinkID: [4]byte{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 0, 2},
		SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypeAdjSID, Value: adjacency}, {Type: sr.V4TypeLANAdjSID, Value: lan}},
	}))
	// RFC 8665 Section 5: Flags (NP), Reserved, MT-ID, Algorithm, a 4-octet index.
	prefixSID := []byte{0x40, reserved, 0, 1, 0, 0, 0, 77}
	// RFC 8665 Section 4: Prefix Length, AF, Range Size, Flags (IA), 3 Reserved
	// octets, Address Prefix, then one Prefix-SID sub-TLV (type 2, length 8).
	prefixRange := []byte{24, 0, 0, 16, 0x80, reserved, reserved, reserved, 198, 51, 100, 0,
		0, 2, 0, 8, 0x40, reserved, 0, 0, 0, 0, 0, 100}
	bgplsInstallOpaque(t, e, packet.ExtPrefixOpaqueType, packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{
		Prefixes: []packet.ExtPrefixTLV{{
			RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{192, 0, 2, 0},
			SubTLVs: []packet.ExtSubTLV{{Type: sr.V4TypePrefixSID, Value: prefixSID}},
		}},
		Ranges: []packet.ExtPrefixRangeTLV{{Value: prefixRange}},
	}))
	if _, err := linkstateevents.Request.Emit(bus); err != nil {
		t.Fatal(err)
	}
	return bgplsAwait(t, events, func(s *linkstateevents.Snapshot) bool {
		return rfc9085OSPFNodeAttribute(s, rfc9085OSPFOriginator, 1036) != nil &&
			rfc9085OSPFLinkAttribute(s, 1100) != nil &&
			rfc9085OSPFPrefixAttribute(s, rfc9085OSPFRange, 1159) != nil
	})
}

// rfc9085OSPFNodeAttribute returns the value of TLV typ on the Node NLRI of the
// given router, or nil.
func rfc9085OSPFNodeAttribute(s *linkstateevents.Snapshot, router []byte, typ uint16) []byte {
	for i := range s.Nodes {
		if bytes.Equal(s.Nodes[i].ID.RouterID, router) {
			return bgplsTestAttribute(s.Nodes[i].Attributes, typ)
		}
	}
	return nil
}

// rfc9085OSPFLinkAttribute returns the value of TLV typ on the first Link NLRI
// carrying it, or nil.
func rfc9085OSPFLinkAttribute(s *linkstateevents.Snapshot, typ uint16) []byte {
	for i := range s.Links {
		if value := bgplsTestAttribute(s.Links[i].Attributes, typ); value != nil {
			return value
		}
	}
	return nil
}

// rfc9085OSPFPrefixAttribute returns the value of TLV typ on the first Prefix
// NLRI for prefix carrying it, or nil.
func rfc9085OSPFPrefixAttribute(s *linkstateevents.Snapshot, prefix netip.Prefix, typ uint16) []byte {
	for i := range s.Prefixes {
		if s.Prefixes[i].Prefix != prefix {
			continue
		}
		if value := bgplsTestAttribute(s.Prefixes[i].Attributes, typ); value != nil {
			return value
		}
	}
	return nil
}

// The expected SID TLV values: Reserved is 0 in each, every other field is the
// native one.
var (
	rfc9085OSPFAdjacencySID = []byte{0x60, 5, 0, 0, 0x00, 0x5d, 0xc1}
	rfc9085OSPFLANSID       = []byte{0x60, 6, 0, 0, 4, 4, 4, 4, 0x00, 0x5d, 0xc2}
	rfc9085OSPFPrefixSID    = []byte{0x40, 1, 0, 0, 0, 0, 0, 77}
	rfc9085OSPFRangeTLV     = []byte{0x80, 0, 0, 16, 0x04, 0x86, 0, 8, 0x40, 0, 0, 0, 0, 0, 0, 100}
)

// rfc9085CheckOSPFSIDs fails unless the snapshot carries the four expected SID
// TLVs, compared byte for byte.
func rfc9085CheckOSPFSIDs(t *testing.T, s *linkstateevents.Snapshot) {
	t.Helper()
	if got := rfc9085OSPFLinkAttribute(s, 1099); !bytes.Equal(got, rfc9085OSPFAdjacencySID) {
		t.Fatalf("TLV 1099 = %x, want %x", got, rfc9085OSPFAdjacencySID)
	}
	if got := rfc9085OSPFLinkAttribute(s, 1100); !bytes.Equal(got, rfc9085OSPFLANSID) {
		t.Fatalf("TLV 1100 = %x, want %x", got, rfc9085OSPFLANSID)
	}
	if got := rfc9085OSPFPrefixAttribute(s, rfc9085OSPFPrefix, 1158); !bytes.Equal(got, rfc9085OSPFPrefixSID) {
		t.Fatalf("TLV 1158 = %x, want %x", got, rfc9085OSPFPrefixSID)
	}
	if got := rfc9085OSPFPrefixAttribute(s, rfc9085OSPFRange, 1159); !bytes.Equal(got, rfc9085OSPFRangeTLV) {
		t.Fatalf("TLV 1159 = %x, want %x", got, rfc9085OSPFRangeTLV)
	}
}

// TestRFC9085OSPFOriginatedSIDReservedZero originates OSPFv2 Adj-SID, LAN
// Adj-SID, Prefix-SID and Prefix Range TLVs whose native Reserved fields are 0,
// and compares the exact TLV 1099, 1100, 1158 and 1159 values.
//
// RFC requirement: RFC9085-2.2.1-1 positive -- the Adjacency SID TLV 1099 Ze originates from an OSPFv2 Adj-SID has Reserved 00 00 and the native flags, weight and label (§2.2.1).
// RFC requirement: RFC9085-2.2.2-1 positive -- the LAN Adjacency SID TLV 1100 Ze originates from an OSPFv2 LAN Adj-SID has Reserved 00 00 and the native flags, weight, neighbor ID and label (§2.2.2).
// RFC requirement: RFC9085-2.3.1-1 positive -- the Prefix-SID TLV 1158 Ze originates from an OSPFv2 Prefix-SID has Reserved 00 00 and the native flags, algorithm and index (§2.3.1).
// RFC requirement: RFC9085-2.3.5-1 positive -- the Range TLV 1159 Ze originates from an OSPFv2 Extended Prefix Range has Reserved octet 0 and the native flags, range size and Prefix-SID (§2.3.5).
func TestRFC9085OSPFOriginatedSIDReservedZero(t *testing.T) {
	snapshot := rfc9085OSPFScenario(t, 0)
	rfc9085CheckOSPFSIDs(t, &snapshot)
}

// TestRFC9085OSPFSourceReservedNeverReachesSIDReserved hands the producer the
// same OSPFv2 SIDs with every native Reserved octet set to ff, the input that
// reaches the BGP-LS Reserved field if the producer copies a native header, and
// requires the same TLV values as the ordinary input.
//
// RFC requirement: RFC9085-2.2.1-1 negative -- an OSPFv2 Adj-SID whose native Reserved octet is ff is originated as TLV 1099 with Reserved 00 00 (§2.2.1).
// RFC requirement: RFC9085-2.2.2-1 negative -- an OSPFv2 LAN Adj-SID whose native Reserved octet is ff is originated as TLV 1100 with Reserved 00 00 (§2.2.2).
// RFC requirement: RFC9085-2.3.1-1 negative -- an OSPFv2 Prefix-SID whose native Reserved octet is ff is originated as TLV 1158 with Reserved 00 00 (§2.3.1).
// RFC requirement: RFC9085-2.3.5-1 negative -- an OSPFv2 Extended Prefix Range whose native Reserved octets are ff is originated as TLV 1159 with Reserved octet 0 (§2.3.5).
func TestRFC9085OSPFSourceReservedNeverReachesSIDReserved(t *testing.T) {
	snapshot := rfc9085OSPFScenario(t, 0xff)
	rfc9085CheckOSPFSIDs(t, &snapshot)
}

// TestRFC9085OSPFCapabilitiesOnOriginatorNode reads the Node NLRI of router
// 2.2.2.2, the router whose RI LSA carries the SRGB and SRLB.
//
// RFC requirement: RFC9085-2.1-1 positive -- the SR Capabilities TLV 1034 and SR Local Block TLV 1036 translated from router 2.2.2.2's RI LSA are on the Node NLRI of router 2.2.2.2 (§2.1).
func TestRFC9085OSPFCapabilitiesOnOriginatorNode(t *testing.T) {
	snapshot := rfc9085OSPFScenario(t, 0)
	if got := rfc9085OSPFNodeAttribute(&snapshot, rfc9085OSPFOriginator, 1034); !bytes.Equal(got, rfc9085OSPFSRGB) {
		t.Fatalf("router 2.2.2.2 TLV 1034 = %x, want %x", got, rfc9085OSPFSRGB)
	}
	if got := rfc9085OSPFNodeAttribute(&snapshot, rfc9085OSPFOriginator, 1036); !bytes.Equal(got, rfc9085OSPFSRLB) {
		t.Fatalf("router 2.2.2.2 TLV 1036 = %x, want %x", got, rfc9085OSPFSRLB)
	}
}

// TestRFC9085OSPFCapabilitiesOnNoOtherNLRI reads every other NLRI of the same
// scenario: the Node NLRI of router 3.3.3.3, which originates no RI LSA, and
// every Link and Prefix NLRI, which carry SR SIDs of their own.
//
// RFC requirement: RFC9085-2.1-1 negative -- TLV 1034 and TLV 1036 are on no Node NLRI but router 2.2.2.2's (router 3.3.3.3 has none), and on no Link or Prefix NLRI (§2.1).
func TestRFC9085OSPFCapabilitiesOnNoOtherNLRI(t *testing.T) {
	snapshot := rfc9085OSPFScenario(t, 0)
	others := 0
	for i := range snapshot.Nodes {
		if bytes.Equal(snapshot.Nodes[i].ID.RouterID, rfc9085OSPFOriginator) {
			continue
		}
		others++
		for _, typ := range []uint16{1034, 1036} {
			if value := bgplsTestAttribute(snapshot.Nodes[i].Attributes, typ); value != nil {
				t.Fatalf("node %x carries TLV %d = %x", snapshot.Nodes[i].ID.RouterID, typ, value)
			}
		}
	}
	if others == 0 {
		t.Fatalf("no Node NLRI besides router 2.2.2.2: %+v", snapshot.Nodes)
	}
	if len(snapshot.Links) == 0 || len(snapshot.Prefixes) == 0 {
		t.Fatalf("links %d, prefixes %d: want both", len(snapshot.Links), len(snapshot.Prefixes))
	}
	for _, typ := range []uint16{1034, 1036} {
		if value := rfc9085OSPFLinkAttribute(&snapshot, typ); value != nil {
			t.Fatalf("a Link NLRI carries TLV %d = %x", typ, value)
		}
		for i := range snapshot.Prefixes {
			if value := bgplsTestAttribute(snapshot.Prefixes[i].Attributes, typ); value != nil {
				t.Fatalf("Prefix NLRI %s carries TLV %d = %x", snapshot.Prefixes[i].Prefix, typ, value)
			}
		}
	}
}
